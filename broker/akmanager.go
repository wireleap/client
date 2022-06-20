// Copyright (c) 2022 Wireleap

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
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/version"
	"github.com/wireleap/common/api/accesskey"
	"github.com/wireleap/common/api/consume"
	"github.com/wireleap/common/api/pof"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/api/status"
)

func (t *T) download(u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
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
			u, res.StatusCode, res.Status,
		)
	}
	t.l.Printf("Downloading %s...", u)
	return io.ReadAll(res.Body)
}

func (t *T) Import(u url.URL) (ak *accesskey.T, err error) {
	data := []byte{}

	switch {
	case u.Scheme == "http":
		return nil, fmt.Errorf("HTTP import URLs are vulnerable to MitM attacks. Use HTTPS instead.")
	case u.Scheme == "https":
		data, err = t.download(u.String())
	case u.Scheme == "file":
		data, err = ioutil.ReadFile(u.Path)
	case !u.IsAbs():
		return nil, fmt.Errorf("url is not absolute: %s", u.String())
	case u.Scheme == "":
		return nil, fmt.Errorf("empty import url scheme in %s", u.String())
	default:
		return nil, fmt.Errorf("could not parse url: %s", u.String())
	}
	if err != nil {
		return nil, fmt.Errorf("could not read accesskey file: %s", err)
	}
	ak = &accesskey.T{}
	err = json.Unmarshal(data, &ak)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal accesskey file: %s", err)
	}
	switch {
	case ak == nil,
		ak.Version == nil,
		ak.Contract == nil,
		ak.Pofs == nil,
		ak.Contract.Endpoint == nil,
		ak.Contract.PublicKey == nil:
		return nil, fmt.Errorf("malformed accesskey file")
	}
	if ak.Version.Minor != accesskey.VERSION.Minor {
		return nil, fmt.Errorf(
			"incompatible accesskey version: %s, expected 0.%d.x",
			ak.Version,
			accesskey.VERSION.Minor,
		)
	}
	sc0 := clientlib.ContractURL(t.Fd)
	if sc0 != nil && *sc0 != *ak.Contract.Endpoint {
		return nil, fmt.Errorf(
			"you are trying to import accesskeys for a contract %s different from the currently defined %s; please set up a separate wireleap directory for your %s needs and import %s accesskeys there",
			ak.Contract.Endpoint,
			sc0,
			ak.Contract.Endpoint,
			u.String(),
		)
	}

	ci, d, err := clientlib.GetContractInfo(t.cl, ak.Contract.Endpoint)

	if err != nil {
		return nil, fmt.Errorf(
			"could not get contract info for %s: %s",
			ak.Contract.Endpoint, err,
		)
	}

	if !bytes.Equal(ak.Contract.PublicKey, ci.Pubkey) {
		return nil, fmt.Errorf(
			"contract public key mismatch; expecting %s from accesskey file, got %s from live contract",
			ak.Contract.PublicKey,
			base64.RawURLEncoding.EncodeToString(ci.Pubkey),
		)
	}

	if err = clientlib.SaveContractInfo(t.Fd, ci, d); err != nil {
		return nil, fmt.Errorf(
			"could not save contract info for %s: %s",
			ak.Contract.Endpoint,
			err,
		)
	}
	sc := ci.Endpoint
	for _, p := range ak.Pofs {
		if p.Expiration <= time.Now().Unix() {
			t.l.Printf("skipping expired accesskey %s", p.Digest())
			continue
		}
		dup := false
		for _, p0 := range t.pofs {
			if p0.Digest() == p.Digest() {
				t.l.Printf("skipping duplicate accesskey %s", p.Digest())
				dup = true
				break
			}
		}
		if !dup {
			t.pofs = append(t.pofs, p)
		}
	}

	if err = t.Fd.Set(t.pofs, filenames.Pofs); err != nil {
		return nil, fmt.Errorf(
			"could not save new pofs for %s: %s",
			sc.String(), err,
		)
	}
	di, err := consume.DirectoryInfo(t.cl, sc)
	if err != nil {
		return nil, fmt.Errorf("could not get contract directory info: %s", err)
	}
	// maybe there's an upgrade available?
	if di.UpgradeChannels.Client != nil {
		if v, ok := di.UpgradeChannels.Client[version.Channel]; ok && v.GT(version.VERSION) {
			t.SetUpgradeable(true)
			t.l.Printf(
				"Upgrade available to %s, current version is %s. Please run `wireleap upgrade`.",
				v, version.VERSION,
			)
		}
	}
	return
}

func (t *T) GetSK(fetch bool) (*servicekey.T, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sk == nil {
		t.Fd.Get(&t.sk, "servicekey.json")
	}
	if t.sk != nil && t.sk.Contract != nil && !t.sk.IsExpiredAt(time.Now().Unix()) {
		t.l.Printf(
			"found existing servicekey %s",
			t.sk.PublicKey,
		)
		return t.sk, nil
	}
	if !t.cfg.Broker.Accesskey.UseOnDemand {
		return nil, fmt.Errorf("no fresh servicekey available and accesskey.use_on_demand is false")
	}
	if !fetch {
		return nil, fmt.Errorf("no activated servicekey available")
	}
	// discard old servicekey & get a new one
	return t.RefreshSK()
}

func (t *T) NewSKFromPof(skurl string, p *pof.T) (*servicekey.T, error) {
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

func (t *T) RefreshSK() (sk *servicekey.T, err error) {
	ps := []*pof.T{}
	if err = t.Fd.Get(&ps, filenames.Pofs); err != nil {
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
			t.l.Printf(
				"generating new servicekey from pof %s...",
				p.Digest(),
			)
			if clientlib.ContractURL(t.Fd) == nil {
				return nil, fmt.Errorf("no contract defined")
			}
			sk, err = t.NewSKFromPof(
				clientlib.ContractURL(t.Fd).String()+"/servicekey/activate",
				p,
			)
			if err != nil {
				t.l.Printf(
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
	if err = t.Fd.Set(&newps, filenames.Pofs); err != nil {
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
	if err = t.Fd.Set(&sk, filenames.Servicekey); err != nil {
		return nil, fmt.Errorf(
			"could not write new %s: %s",
			filenames.Servicekey,
			err,
		)
	}
	return sk, nil
}

func (t *T) PickPofs() (r []*pof.T) {
	for _, p := range t.pofs {
		if !p.IsExpiredAt(time.Now().Unix()) {
			// this one has not expired yet
			r = append(r, p)
		}
	}
	return r
}

func (t *T) Activate() (err error) {
	switch {
	case clientlib.ContractURL(t.Fd) == nil:
		return fmt.Errorf("contract has to be set")
	case t.cfg.Broker.Accesskey.UseOnDemand:
		return fmt.Errorf("accesskey.use_on_demand is enabled in config.json; refusing to run")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sk == nil {
		if err = t.Fd.Get(t.sk, filenames.Servicekey); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrNotExist) {
				// this is fine
			} else {
				return fmt.Errorf(
					"error reading old %s",
					filenames.Servicekey,
				)
			}
		}
	}
	if t.sk != nil && t.sk.Contract != nil && !t.sk.IsExpiredAt(time.Now().Unix()) {
		return fmt.Errorf(
			"refusing to replace non-expired servicekey: %s expires at %s",
			filenames.Servicekey,
			time.Unix(t.sk.Contract.SettlementOpen, 0).String(),
		)
	}
	// discard old servicekey & get a new one
	if t.sk, err = t.RefreshSK(); err != nil {
		return fmt.Errorf("error while activating servicekey with pof: %s", err)
	}
	if err = t.Fd.Set(t.sk, filenames.Servicekey); err != nil {
		return fmt.Errorf("could not write new servicekey: %s", err)
	}
	t.reload()
	return
}

func (t *T) CurrentSK() *servicekey.T { return t.sk }
func (t *T) CurrentPofs() []*pof.T    { return t.pofs }
