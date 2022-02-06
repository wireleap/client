package broker

import (
	"context"
	"fmt"
	"log"
	"net"
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
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/upgrade"
	"github.com/wireleap/common/wlnet"
	"github.com/wireleap/common/wlnet/transport"
)

type T struct {
	fd    fsdir.T
	cl    *client.Client
	cache *dnscachedial.Control
	// global broker lock
	mu sync.Mutex
	// currently active circuit
	// only one, should be mutex-protected
	circ circuit.T
}

func New(fd fsdir.T) *T {
	t := &T{
		fd: fd,
		cl: client.New(nil, clientcontract.T, clientdir.T),
		// cache dns resolution in netstack transport
		cache: dnscachedial.New(),
	}
	c := clientcfg.Defaults()
	err := t.fd.Get(&c, filenames.Config)
	if err != nil {
		log.Fatal(err)
	}
	if c.Forwarders.Socks == nil && c.Broker.Address == nil {
		log.Fatal("both address.socks and address.h2c are nil in config, please set one or both")
	}
	tt := transport.New(transport.Options{Timeout: time.Duration(c.Broker.Timeout)})
	tt.Transport.DialContext = t.cache.Cover(tt.Transport.DialContext)
	tt.Transport.DialTLSContext = t.cache.Cover(tt.Transport.DialTLSContext)
	t.cl.Transport = tt.Transport
	dialf := tt.DialWL
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
	if clientlib.ContractURL(t.fd) != nil {
		// cache dns, sc and directory data if we can
		var (
			di dirinfo.T
			rl relaylist.T
		)
		if _, di, rl, err = t.Sync(); err != nil {
			log.Fatalf("could not get contract info: %s", err)
		}
		// cache relay ip addresses for tun
		if rl != nil {
			for _, r := range rl.All() {
				if err = t.cache.Cache(context.Background(), r.Addr.Hostname()); err != nil {
					log.Printf("could not cache %s: %s", r.Addr.Hostname(), err)
				}
			}
		}
		// write bypass for tun
		if err = t.writeBypass(t.cache.Get(di.Endpoint.Hostname())...); err != nil {
			log.Fatalf(
				"could not write first bypass file %s: %s",
				t.fd.Path(filenames.Bypass), err,
			)
		}
	}
	circuitf := func() (r []*relayentry.T, err error) {
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
		if c.Broker.Circuit.Whitelist != nil {
			if len(*c.Broker.Circuit.Whitelist) > 0 {
				for _, addr := range *c.Broker.Circuit.Whitelist {
					if rl[addr] != nil {
						all = append(all, rl[addr])
					}
				}
			}
		} else {
			all = rl.All()
		}
		if r, err = circuit.Make(c.Broker.Circuit.Hops, all); err != nil {
			return
		}
		t.circ = r
		// expose bypass for wireleap_tun
		err = t.writeBypass(t.cache.Get(r[0].Addr.Hostname())...)
		return
	}
	sks := clientlib.SKSource(t.fd, &c, t.cl)
	// set up local listening functions
	var (
		dialer = clientlib.CircuitDialer(
			clientlib.AlwaysFetch(sks),
			circuitf,
			dialf,
		)
		errf = func(e error) {
			if err != nil {
				if o := clientlib.TraceOrigin(err, t.circ); o != nil {
					if status.IsCircuitError(err) {
						// reset on circuit errors
						log.Printf(
							"relay-originated circuit error from %s: %s, resetting circuit",
							o.Pubkey,
							err,
						)
						t.mu.Lock()
						t.circ = nil
						t.mu.Unlock()
					} else {
						// not reset-worthy
						log.Printf("error from %s: %s", o.Pubkey, err)
					}
				} else {
					log.Printf("circuit dial error: %s", err)
				}
			}
		}
	)
	if c.Broker.Address != nil {
		err = clientlib.ListenH2C(*c.Broker.Address, tt.TLSClientConfig, dialer, errf)
		if err != nil {
			log.Fatalf("listening on h2c://%s failed: %s", *c.Broker.Address, err)
		}
		log.Printf("listening on h2c://%s, waiting for forwarders to connect", *c.Broker.Address)
	}
	return &T{}
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
				log.Printf("Upgrade available to %s, current version is %s. ", v, version.VERSION)
				log.Printf("Last upgrade attempt to %s failed! Keeping current version; please upgrade when possible.", skipv)
			} else {
				log.Fatalf(
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

func (t *T) Reload() (_ bool) {
	log.Println("reloading config")
	t.mu.Lock()
	defer t.mu.Unlock()

	c := clientcfg.Defaults()
	err := t.fd.Get(&c, filenames.Config)
	if err != nil {
		log.Printf(
			"could not reload config: %s, aborting reload",
			err,
		)
		return
	}
	// refresh contract info
	if _, _, _, err := t.Sync(); err != nil {
		log.Printf(
			"could not refresh contract info: %s, aborting reload",
			err,
		)
		return
	}
	// reset circuit
	t.circ = nil
	return
}

func (t *T) Shutdown() bool {
	log.Println("gracefully shutting down...")
	t.fd.Del(filenames.Pid)
	return true
}
