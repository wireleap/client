// Copyright (c) 2022 Wireleap

package broker

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/wireleap/common/api/pof"
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
	Fd    fsdir.T
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
	// accesskey manager state
	sk   *servicekey.T
	pofs []*pof.T
	// contract info
	ci *contractinfo.T
	// need upgrading?
	upgrade bool
	// upgrade val lock (has to be separate from global)
	uMu sync.Mutex
}

func New(fd fsdir.T, cfg *clientcfg.C, l *log.Logger) *T {
	t := &T{
		Fd: fd,
		cl: client.New(nil, clientcontract.T, clientdir.T),
		// cache dns resolution in netstack transport
		cache: dnscachedial.New(),
		T:     transport.New(transport.Options{Timeout: time.Duration(cfg.Broker.Circuit.Timeout)}),
		cfg:   cfg,
		l:     l,
	}
	var err error
	if err = t.Fd.Get(&t.pofs, filenames.Pofs); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrNotExist) {
			t.l.Fatalf("could not get previous pofs to initialize accesskeys: %s", err)
		}
	}
	if err = t.Fd.Get(&t.sk, filenames.Servicekey); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrNotExist) {
			t.l.Fatalf(
				"could not get apparently existing %s to initialize accesskeys: %s",
				filenames.Servicekey, err,
			)
		}
	}
	if cfg.Broker.Address == nil {
		t.l.Fatal("broker.address is nil in config, please set it")
	}
	t.T.Transport.DialContext = t.cache.Cover(t.T.Transport.DialContext)
	t.T.Transport.DialTLSContext = t.cache.Cover(t.T.Transport.DialTLSContext)
	t.cl.Transport = t.T.Transport
	if clientlib.ContractURL(t.Fd) != nil {
		// cache dns, sc and directory data if we can
		var (
			di dirinfo.T
			rl relaylist.T
		)
		if di, rl, err = t.Sync(); err != nil {
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
				t.Fd.Path(filenames.Bypass), err,
			)
		}
	}
	t.cl.RetryOpt.Interval = 1 * time.Second
	return t
}

type DialFunc func(string, string) (net.Conn, error)

func (t *T) Relays() (rs relaylist.T, err error) {
	err = t.Fd.Get(&rs, "relays.json")
	return
}

func (t *T) Circuit() (r []*relayentry.T, err error) {
	// use existing if available
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.circ != nil {
		return t.circ, nil
	}
	var rl relaylist.T
	if _, rl, err = t.Sync(); err != nil {
		return nil, err
	}
	var all circuit.T
	haveWL := t.cfg.Broker.Circuit.Whitelist != nil && len(t.cfg.Broker.Circuit.Whitelist) > 0
	if haveWL {
		for _, addr := range t.cfg.Broker.Circuit.Whitelist {
			if rl[addr] != nil {
				all = append(all, rl[addr])
			}
		}
	} else {
		all = rl.All()
	}
	if r, err = circuit.Make(t.cfg.Broker.Circuit.Hops, all); err != nil {
		if haveWL {
			err = fmt.Errorf("%w (broker.circuit.whitelist is non-empty)", err)
		}
		return
	}
	t.circ = r
	// expose bypass for wireleap_tun
	err = t.writeBypass(t.cache.Get(r[0].Addr.Hostname())...)
	return
}

func (t *T) ActiveCircuit() (r circuit.T) { return t.circ }

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
		func() (*servicekey.T, error) { return t.GetSK(true) },
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
	sc := t.cache.Get(clientlib.ContractURL(t.Fd).Hostname())
	bypass := append(sc, extra...)
	return t.Fd.Set(bypass, filenames.Bypass)
}

func (t *T) Sync() (di dirinfo.T, rl relaylist.T, err error) {
	sc := clientlib.ContractURL(t.Fd)
	if sc == nil {
		err = fmt.Errorf("contract is not defined")
		return
	}
	if t.ci, err = consume.ContractInfo(t.cl, sc); err != nil {
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
			skipv := upgrade.NewConfig(t.Fd, "wireleap", false).SkippedVersion()
			if skipv != nil && skipv.EQ(v) {
				t.SetUpgradeable(true)
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
	if err = clientlib.SaveContractInfo(t.Fd, t.ci, rl); err != nil {
		err = fmt.Errorf("could not save contract info: %w", err)
		return
	}
	return
}

func (t *T) ContractInfo() *contractinfo.T { return t.ci }

func (t *T) reload() {
	t.l.Println("reloading config")
	if err := t.Fd.Get(t.cfg, filenames.Config); err != nil {
		t.l.Printf(
			"could not reload config: %s, aborting reload",
			err,
		)
		return
	}
	// refresh contract info
	if _, _, err := t.Sync(); err != nil {
		t.l.Printf(
			"could not refresh contract info: %s, aborting reload",
			err,
		)
		return
	}
	// reset circuit
	t.circ = nil
}

func (t *T) Reload() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reload()
}

func (t *T) Shutdown() {
	t.l.Println("gracefully shutting down...")
	t.Fd.Del(filenames.Pid)
}

func (t *T) Config() *clientcfg.C { return t.cfg }

func (t *T) SaveConfig() error { return t.Fd.Set(&t.cfg, filenames.Config) }

func (t *T) SetUpgradeable(val bool) {
	t.uMu.Lock()
	t.upgrade = val
	t.uMu.Unlock()
}

func (t *T) IsUpgradeable() (r bool) {
	t.uMu.Lock()
	r = t.upgrade
	t.uMu.Unlock()
	return
}
