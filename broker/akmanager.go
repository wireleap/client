package broker

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/version"
	"github.com/wireleap/common/api/accesskey"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/consume"
	"github.com/wireleap/common/api/interfaces/clientcontract"
	"github.com/wireleap/common/api/pof"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

type AKManager struct {
	c  *clientcfg.C
	cl *client.Client
	fm fsdir.T
	mu sync.Mutex

	sk   *servicekey.T
	pofs []*pof.T
}

func NewAKManager(fm fsdir.T, c *clientcfg.C, cl *client.Client) (t *AKManager, err error) {
	t = &AKManager{c: c, fm: fm, cl: cl}
	if err = t.fm.Get(&t.pofs, filenames.Pofs); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrNotExist) {
			err = fmt.Errorf("could not get previous pofs: %w", err)
		}
	}
	return
}

func download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"%s download request returned code %d: %s",
			url, res.StatusCode, res.Status,
		)
	}
	log.Printf("Downloading %s...", url)
	return io.ReadAll(res.Body)
}

func (t *AKManager) Import(url string) (err error) {
	data := []byte{}

	switch {
	case strings.HasPrefix(url, "http://"):
		return fmt.Errorf("HTTP import URLs are vulnerable to MitM attacks. Use HTTPS instead.")
	case strings.HasPrefix(url, "https://"):
		data, err = download(url)
	default:
		data, err = ioutil.ReadFile(url)
	}

	if err != nil {
		return fmt.Errorf("could not read accesskey file: %s", err)
	}

	ak := &accesskey.T{}
	err = json.Unmarshal(data, &ak)

	if err != nil {
		return fmt.Errorf("could not unmarshal accesskey file: ", err)
	}

	switch {
	case ak == nil,
		ak.Version == nil,
		ak.Contract == nil,
		ak.Pofs == nil,
		ak.Contract.Endpoint == nil,
		ak.Contract.PublicKey == nil:
		return fmt.Errorf("malformed accesskey file")
	}

	if ak.Version.Minor != accesskey.VERSION.Minor {
		return fmt.Errorf(
			"incompatible accesskey version: %s, expected 0.%d.x",
			ak.Version,
			accesskey.VERSION.Minor,
		)
	}

	sc0 := clientlib.ContractURL(t.fm)
	if sc0 != nil && *sc0 != *ak.Contract.Endpoint {
		return fmt.Errorf(
			"you are trying to import accesskeys for a contract %s different from the currently defined %s; please set up a separate wireleap directory for your %s needs and import %s accesskeys there",
			ak.Contract.Endpoint,
			sc0,
			ak.Contract.Endpoint,
			url,
		)
	}

	cl := client.New(nil, clientcontract.T)
	ci, d, err := clientlib.GetContractInfo(cl, ak.Contract.Endpoint)

	if err != nil {
		return fmt.Errorf(
			"could not get contract info for %s: %s",
			ak.Contract.Endpoint, err,
		)
	}

	if !bytes.Equal(ak.Contract.PublicKey, ci.Pubkey) {
		return fmt.Errorf(
			"contract public key mismatch; expecting %s from accesskey file, got %s from live contract",
			ak.Contract.PublicKey,
			base64.RawURLEncoding.EncodeToString(ci.Pubkey),
		)
	}

	if err = clientlib.SaveContractInfo(t.fm, ci, d); err != nil {
		return fmt.Errorf(
			"could not save contract info for %s: %s",
			ak.Contract.Endpoint,
			err,
		)
	}
	sc := ci.Endpoint
	for _, p := range ak.Pofs {
		if p.Expiration <= time.Now().Unix() {
			log.Printf("skipping expired accesskey %s", p.Digest())
			continue
		}
		dup := false
		for _, p0 := range t.pofs {
			if p0.Digest() == p.Digest() {
				log.Printf("skipping duplicate accesskey %s", p.Digest())
				dup = true
				break
			}
		}
		if !dup {
			t.pofs = append(t.pofs, p)
		}
	}

	if err = t.fm.Set(t.pofs, filenames.Pofs); err != nil {
		return fmt.Errorf(
			"could not save new pofs for %s: %s",
			sc.String(), err,
		)
	}
	di, err := consume.DirectoryInfo(cl, sc)
	if err != nil {
		return fmt.Errorf("could not get contract directory info: %s", err)
	}
	// maybe there's an upgrade available?
	var upgradev *semver.Version
	if di.UpgradeChannels.Client != nil {
		if v, ok := di.UpgradeChannels.Client[version.Channel]; ok && v.GT(version.VERSION) {
			upgradev = &v
		}
	}
	var pid int
	if err = t.fm.Get(&pid, filenames.Pid); err == nil {
		if upgradev != nil {
			log.Printf(
				"Upgrade available to %s, current version is %s. Please run `wireleap upgrade`.",
				upgradev, version.VERSION,
			)
		}
		process.Reload(pid)
	}
	return
}

func (t *AKManager) Get(fetch bool) (*servicekey.T, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sk == nil {
		t.fm.Get(&t.sk, "servicekey.json")
	}
	if t.sk != nil && t.sk.Contract != nil && !t.sk.IsExpiredAt(time.Now().Unix()) {
		log.Printf(
			"found existing servicekey %s",
			t.sk.PublicKey,
		)
		return t.sk, nil
	}
	if !t.c.Broker.Accesskey.UseOnDemand {
		return nil, fmt.Errorf("no fresh servicekey available and accesskey.use_on_demand is false")
	}
	if !fetch {
		return nil, fmt.Errorf("no activated servicekey available")
	}
	// discard old servicekey & get a new one
	return t.RefreshSK()
}

func (t *AKManager) NewSKFromPof(skurl string, p *pof.T) (*servicekey.T, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	sk := servicekey.New(priv)
	req := &pof.SKActivationRequest{Pubkey: sk.PublicKey, Pof: p}
	if err = t.cl.Perform(http.MethodPost, skurl, req, sk.Contract); err != nil {
		return nil, fmt.Errorf("error while performing SK activation request: %w", err)
	}
	return sk, nil
}

func (t *AKManager) RefreshSK() (sk *servicekey.T, err error) {
	ps := []*pof.T{}
	if err = t.fm.Get(&ps, filenames.Pofs); err != nil {
		return nil, fmt.Errorf(
			"could not open %s: %s; did you run `wireleap import`?",
			filenames.Pofs,
			err,
		)
	}
	ps = t.PickPofs()
	if len(ps) == 0 {
		return nil, fmt.Errorf("no fresh pofs available")
	}
	newps := []*pof.T{}
	// filter pofs & get sk
	for _, p := range ps {
		if sk == nil {
			log.Printf(
				"generating new servicekey from pof %s...",
				p.Digest(),
			)
			if clientlib.ContractURL(t.fm) == nil {
				return nil, fmt.Errorf("no contract defined")
			}
			sk, err = t.NewSKFromPof(
				clientlib.ContractURL(t.fm).String()+"/servicekey/activate",
				p,
			)
			if err != nil {
				log.Printf(
					"failed generating new servicekey from pof %s: %s",
					p.Digest(),
					err,
				)
				if errors.Is(err, status.ErrSneakyPof) {
					// skip already used pof
					continue
				}
				// keep if other error
				newps = append(newps, p)
				continue
			}
			// skip successfully-used pof
			continue
		}
		// keep the rest untouched
		newps = append(newps, p)
	}
	// write new pofs
	if err = t.fm.Set(&newps, filenames.Pofs); err != nil {
		return nil, fmt.Errorf(
			"could not write new %s: %s",
			filenames.Pofs,
			err,
		)
	}
	if sk == nil {
		return nil, fmt.Errorf("no servicekey available")
	}
	// write new servicekey
	if err = t.fm.Set(&sk, filenames.Servicekey); err != nil {
		return nil, fmt.Errorf(
			"could not write new %s: %s",
			filenames.Servicekey,
			err,
		)
	}
	return sk, nil
}

func (t *AKManager) PickPofs() (r []*pof.T) {
	for _, p := range t.pofs {
		if !p.IsExpiredAt(time.Now().Unix()) {
			// this one has not expired yet
			r = append(r, p)
		}
	}
	return r
}

func (t *AKManager) CurrentSK() *servicekey.T { return t.sk }
func (t *AKManager) CurrentPofs() []*pof.T    { return t.pofs }
