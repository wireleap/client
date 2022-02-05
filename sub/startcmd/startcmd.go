// Copyright (c) 2022 Wireleap

package startcmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"sync"
	"syscall"
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
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/commonsub/startcmd"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
	"github.com/wireleap/common/cli/upgrade"
	"github.com/wireleap/common/wlnet"
	"github.com/wireleap/common/wlnet/transport"
)

func Cmd() *cli.Subcmd {
	run := func(fm fsdir.T) {
		var (
			cl   = client.New(nil, clientcontract.T, clientdir.T)
			ci   *contractinfo.T
			rl   relaylist.T
			di   dirinfo.T
			circ circuit.T
			mu   sync.Mutex
		)
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		if c.Forwarders.Socks == nil && c.Broker.Address == nil {
			log.Fatal("both address.socks and address.h2c are nil in config, please set one or both")
		}
		tt := transport.New(transport.Options{Timeout: time.Duration(c.Broker.Timeout)})
		// cache dns resolution in netstack transport
		cache := dnscachedial.New()
		tt.Transport.DialContext = cache.Cover(tt.Transport.DialContext)
		tt.Transport.DialTLSContext = cache.Cover(tt.Transport.DialTLSContext)
		cl.Transport = tt.Transport
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
		// write bypass
		writeBypass := func(extra ...string) error {
			// expose bypass for wireleap_tun
			sc := cache.Get(clientlib.ContractURL(fm).Hostname())
			dir := cache.Get(di.Endpoint.Hostname())
			bypass := append(sc, dir...)
			bypass = append(bypass, extra...)
			return fm.Set(bypass, filenames.Bypass)
		}
		// make circuit
		syncinfo := func() error {
			sc := clientlib.ContractURL(fm)
			if sc == nil {
				return fmt.Errorf("contract is not defined")
			}
			if ci, err = consume.ContractInfo(cl, sc); err != nil {
				return fmt.Errorf(
					"could not get contract info for %s: %s",
					sc.String(), err,
				)
			}
			if di, err = consume.DirectoryInfo(cl, sc); err != nil {
				return fmt.Errorf("could not get contract directory info: %w", err)
			}
			// maybe there's an upgrade available?
			if di.UpgradeChannels.Client != nil {
				if v, ok := di.UpgradeChannels.Client[version.Channel]; ok && v.GT(version.VERSION) {
					skipv := upgrade.NewConfig(fm, "wireleap", false).SkippedVersion()
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
			if rl, err = consume.ContractRelays(cl, sc); err != nil {
				return fmt.Errorf(
					"could not get contract relays for %s: %s",
					sc.String(), err,
				)
			}
			if err = clientlib.SaveContractInfo(fm, ci, rl); err != nil {
				return fmt.Errorf("could not save contract info: %w", err)
			}
			return nil
		}
		if clientlib.ContractURL(fm) != nil {
			// cache dns, sc and directory data if we can
			if err = syncinfo(); err != nil {
				log.Fatalf("could not get contract info: %s", err)
			}
			// cache relay ip addresses for tun
			if rl != nil {
				for _, r := range rl.All() {
					if err = cache.Cache(context.Background(), r.Addr.Hostname()); err != nil {
						log.Printf("could not cache %s: %s", r.Addr.Hostname(), err)
					}
				}
			}
			// write bypass for tun
			if err = writeBypass(); err != nil {
				log.Fatalf(
					"could not write first bypass file %s: %s",
					fm.Path(filenames.Bypass), err,
				)
			}
		}
		circuitf := func() (r []*relayentry.T, err error) {
			// use existing if available
			mu.Lock()
			defer mu.Unlock()
			if circ != nil {
				return circ, nil
			}
			if err = syncinfo(); err != nil {
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
			circ = r
			// expose bypass for wireleap_tun
			err = writeBypass(cache.Get(r[0].Addr.Hostname())...)
			return
		}
		sks := clientlib.SKSource(fm, &c, cl)
		// set up local listening functions
		var (
			listening = []string{}
			dialer    = clientlib.CircuitDialer(
				clientlib.AlwaysFetch(sks),
				circuitf,
				dialf,
			)
			errf = func(e error) {
				if err != nil {
					if o := clientlib.TraceOrigin(err, circ); o != nil {
						if status.IsCircuitError(err) {
							// reset on circuit errors
							log.Printf(
								"relay-originated circuit error from %s: %s, resetting circuit",
								o.Pubkey,
								err,
							)
							mu.Lock()
							circ = nil
							mu.Unlock()
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
		if c.Forwarders.Socks != nil {
			// TODO launch wireleap_socks forwarder
			// err = clientlib.ListenSOCKS(*c.Forwarders.Socks, dialer, errf)
			// if err != nil {
			// log.Fatalf("listening on socks5://%s and udp://%s failed: %s", *c.Forwarders.Socks, *c.Forwarders.Socks, err)
			// }
			// listening = append(listening, "socksv5://"+*c.Forwarders.Socks, "udp://"+*c.Forwarders.Socks)
		}
		if c.Broker.Address != nil {
			err = clientlib.ListenH2C(*c.Broker.Address, tt.TLSClientConfig, dialer, errf)
			if err != nil {
				log.Fatalf("listening on h2c://%s failed: %s", *c.Broker.Address, err)
			}
			listening = append(listening, "h2c://"+*c.Broker.Address)
		}
		log.Printf("listening on: %v", listening)
		shutdown := func() bool {
			log.Println("gracefully shutting down...")
			fm.Del(filenames.Pid)
			return true
		}
		defer shutdown()
		cli.SignalLoop(cli.SignalMap{
			process.ReloadSignal: func() (_ bool) {
				log.Println("reloading config")
				mu.Lock()
				defer mu.Unlock()
				// reload config

				c = clientcfg.Defaults()
				err = fm.Get(&c, filenames.Config)
				if err != nil {
					log.Printf(
						"could not reload config: %s, aborting reload",
						err,
					)
					return
				}
				// refresh contract info
				if err = syncinfo(); err != nil {
					log.Printf(
						"could not refresh contract info: %s, aborting reload",
						err,
					)
					return
				}
				// reset circuit
				circ = nil
				return
			},
			syscall.SIGINT:  shutdown,
			syscall.SIGTERM: shutdown,
			syscall.SIGQUIT: shutdown,
		})
	}
	r := startcmd.Cmd("wireleap", run)
	r.Desc = fmt.Sprintf("%s %s", r.Desc, "(SOCKSv5/connection broker)")
	r.Sections = []cli.Section{
		{
			Title: "Signals",
			Entries: []cli.Entry{
				{
					Key:   "SIGUSR1\t(10)",
					Value: "Reload configuration, contract information and circuit",
				},
				{
					Key:   "SIGTERM\t(15)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
				{
					Key:   "SIGQUIT\t(3)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
				{
					Key:   "SIGINT\t(2)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
			},
		},
		{
			Title: "Environment",
			Entries: []cli.Entry{{
				Key:   "WIRELEAP_TARGET_PROTOCOL",
				Value: "Resolve target IP via tcp4, tcp6 or tcp (default)",
			}},
		},
	}
	return r
}
