package broker

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/wireleap/client/circuit"
	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/dnscachedial"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/version"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/consume"
	"github.com/wireleap/common/api/contractinfo"
	"github.com/wireleap/common/api/dirinfo"
	"github.com/wireleap/common/api/interfaces/clientcontract"
	"github.com/wireleap/common/api/interfaces/clientdir"
	"github.com/wireleap/common/api/relayentry"
	"github.com/wireleap/common/api/relaylist"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/upgrade"
	"github.com/wireleap/common/wlnet"
	"github.com/wireleap/common/wlnet/flushwriter"
	"github.com/wireleap/common/wlnet/h2rwc"
	"github.com/wireleap/common/wlnet/transport"
)

type T struct {
	fd    fsdir.T
	cfg   *clientcfg.C
	cl    *client.Client
	cache *dnscachedial.Control
	// global broker lock
	mu sync.Mutex
	// currently active circuit
	// only one, should be mutex-protected
	circ circuit.T
	// transport
	*transport.T
	// broker prefix logger
	l *log.Logger
	// accesskey manager
	AKM *AKManager
}

func New(fd fsdir.T, cfg *clientcfg.C, l *log.Logger) *T {
	t := &T{
		fd: fd,
		cl: client.New(nil, clientcontract.T, clientdir.T),
		// cache dns resolution in netstack transport
		cache: dnscachedial.New(),
		T:     transport.New(transport.Options{Timeout: time.Duration(cfg.Broker.Timeout)}),
		cfg:   cfg,
		l:     l,
	}
	var err error
	t.AKM, err = NewAKManager(t.fd, t.cfg, t.cl)
	if err != nil {
		t.l.Fatal("could not initialize accesskey manager: %s", err)
	}
	if cfg.Broker.Address == nil {
		t.l.Fatal("broker.address is nil in config, please set it")
	}
	t.T.Transport.DialContext = t.cache.Cover(t.T.Transport.DialContext)
	t.T.Transport.DialTLSContext = t.cache.Cover(t.T.Transport.DialTLSContext)
	t.cl.Transport = t.T.Transport
	if clientlib.ContractURL(t.fd) != nil {
		// cache dns, sc and directory data if we can
		var (
			di dirinfo.T
			rl relaylist.T
		)
		if _, di, rl, err = t.Sync(); err != nil {
			t.l.Fatalf("could not get contract info: %s", err)
		}
		// cache relay ip addresses for tun
		if rl != nil {
			for _, r := range rl.All() {
				if err = t.cache.Cache(context.Background(), r.Addr.Hostname()); err != nil {
					t.l.Printf("could not cache %s: %s", r.Addr.Hostname(), err)
				}
			}
		}
		// write bypass for tun
		if err = t.writeBypass(t.cache.Get(di.Endpoint.Hostname())...); err != nil {
			t.l.Fatalf(
				"could not write first bypass file %s: %s",
				t.fd.Path(filenames.Bypass), err,
			)
		}
	}
	return t
}

type DialFunc func(string, string) (net.Conn, error)

func (t *T) Circuit() (r []*relayentry.T, err error) {
	// use existing if available
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.circ != nil {
		return t.circ, nil
	}
	var (
		rl relaylist.T
	)
	if _, _, rl, err = t.Sync(); err != nil {
		return nil, err
	}
	var all circuit.T
	if t.cfg.Broker.Circuit.Whitelist != nil {
		if len(*t.cfg.Broker.Circuit.Whitelist) > 0 {
			for _, addr := range *t.cfg.Broker.Circuit.Whitelist {
				if rl[addr] != nil {
					all = append(all, rl[addr])
				}
			}
		}
	} else {
		all = rl.All()
	}
	if r, err = circuit.Make(t.cfg.Broker.Circuit.Hops, all); err != nil {
		return
	}
	t.circ = r
	// expose bypass for wireleap_tun
	err = t.writeBypass(t.cache.Get(r[0].Addr.Hostname())...)
	return
}

func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		status.ErrMethod.WriteTo(w)
		return
	}
	protocol := r.Header.Get("Wl-Dial-Protocol")
	target := r.Header.Get("Wl-Dial-Target")
	fwdr := r.Header.Get("Wl-Forwarder")
	if fwdr == "" {
		fwdr = "unnamed_forwarder"
	}
	t.l.Printf("%s forwarder connected", fwdr)

	dialf := t.T.DialWL
	// force target protocol if needed
	tproto, ok := os.LookupEnv("WIRELEAP_TARGET_PROTOCOL")
	if ok {
		dialf = func(c net.Conn, proto string, remote *url.URL, p *wlnet.Init) (net.Conn, error) {
			if remote.Scheme == "target" {
				proto = tproto
			}
			return dialf(c, proto, remote, p)
		}
	}

	dialer := clientlib.CircuitDialer(
		func() (*servicekey.T, error) { return t.AKM.Get(true) },
		t.Circuit,
		dialf,
	)
	cc, err := dialer(protocol, target)
	if err != nil {
		t.l.Printf("%s->h2->circuit dial failure: %s", fwdr, err)
		return
	}
	rwc := h2rwc.T{flushwriter.T{w}, r.Body}
	err = wlnet.Splice(context.Background(), rwc, cc, 0, 32*1024)
	if err != nil {
		if o := clientlib.TraceOrigin(err, t.circ); o != nil {
			if status.IsCircuitError(err) {
				// reset on circuit errors
				t.l.Printf(
					"relay-originated circuit error from %s: %s, resetting circuit",
					o.Pubkey,
					err,
				)
				t.mu.Lock()
				t.circ = nil
				t.mu.Unlock()
			} else {
				// not reset-worthy
				t.l.Printf("error from %s: %s", o.Pubkey, err)
			}
		} else {
			t.l.Printf("circuit dial error: %s", err)
		}
		status.ErrGateway.WriteTo(w)
	}
	cc.Close()
	rwc.Close()
}

// write bypass.json file
func (t *T) writeBypass(extra ...string) error {
	// expose bypass for wireleap_tun
	sc := t.cache.Get(clientlib.ContractURL(t.fd).Hostname())
	bypass := append(sc, extra...)
	return t.fd.Set(bypass, filenames.Bypass)
}

func (t *T) Sync() (ci *contractinfo.T, di dirinfo.T, rl relaylist.T, err error) {
	sc := clientlib.ContractURL(t.fd)
	if sc == nil {
		err = fmt.Errorf("contract is not defined")
		return
	}
	if ci, err = consume.ContractInfo(t.cl, sc); err != nil {
		err = fmt.Errorf(
			"could not get contract info for %s: %s",
			sc.String(), err,
		)
		return
	}
	if di, err = consume.DirectoryInfo(t.cl, sc); err != nil {
		err = fmt.Errorf("could not get contract directory info: %w", err)
		return
	}
	// maybe there's an upgrade available?
	if di.UpgradeChannels.Client != nil {
		if v, ok := di.UpgradeChannels.Client[version.Channel]; ok && v.GT(version.VERSION) {
			skipv := upgrade.NewConfig(t.fd, "wireleap", false).SkippedVersion()
			if skipv != nil && skipv.EQ(v) {
				t.l.Printf("Upgrade available to %s, current version is %s. ", v, version.VERSION)
				t.l.Printf("Last upgrade attempt to %s failed! Keeping current version; please upgrade when possible.", skipv)
			} else {
				t.l.Fatalf(
					"Upgrade available to %s, current version is %s. Please run `wireleap upgrade`.",
					v, version.VERSION,
				)
			}
		}
	}
	if rl, err = consume.ContractRelays(t.cl, sc); err != nil {
		err = fmt.Errorf(
			"could not get contract relays for %s: %s",
			sc.String(), err,
		)
		return
	}
	if err = clientlib.SaveContractInfo(t.fd, ci, rl); err != nil {
		err = fmt.Errorf("could not save contract info: %w", err)
		return
	}
	return
}

func (t *T) ContractInfo() (ci *contractinfo.T, err error) {
	err = t.fd.Get(&ci, filenames.Contract)
	return
}

func (t *T) Reload() {
	t.l.Println("reloading config")
	t.mu.Lock()
	defer t.mu.Unlock()

	cfg := clientcfg.Defaults()
	err := t.fd.Get(&cfg, filenames.Config)
	if err != nil {
		t.l.Printf(
			"could not reload config: %s, aborting reload",
			err,
		)
		return
	}
	t.cfg = &cfg
	// refresh contract info
	if _, _, _, err := t.Sync(); err != nil {
		t.l.Printf(
			"could not refresh contract info: %s, aborting reload",
			err,
		)
		return
	}
	// reset circuit
	t.circ = nil
}

func (t *T) Shutdown() {
	t.l.Println("gracefully shutting down...")
	t.fd.Del(filenames.Pid)
}

func (t *T) Config() *clientcfg.C { return t.cfg }
