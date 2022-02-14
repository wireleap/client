// Copyright (c) 2022 Wireleap

package startcmd

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/commonsub/startcmd"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var reloads, shutdowns []func()

func reload() bool {
	for _, f := range reloads {
		f()
	}
	return false
}

func shutdown() bool {
	for _, f := range shutdowns {
		f()
	}
	return true
}

func setupServer(l net.Listener, h http.Handler, tc *tls.Config) {
	h1s := &http.Server{Handler: h, TLSConfig: tc}
	h2s := &http2.Server{MaxHandlers: 0, MaxConcurrentStreams: 0}
	if err := http2.ConfigureServer(h1s, h2s); err != nil {
		log.Fatalf("could not configure h2 server: %s", err)
	}
	h1s.Handler = h2c.NewHandler(h1s.Handler, h2s)
	go func() {
		if err := h1s.Serve(l); err != nil {
			log.Fatalf("serving on h2c://%s failed: %s", l.Addr(), err)
		}
	}()
}

var (
	broklog = log.New(os.Stderr, "[broker] ", log.LstdFlags|log.Lmsgprefix)
	restlog = log.New(os.Stderr, "[restapi] ", log.LstdFlags|log.Lmsgprefix)
)

func Cmd() *cli.Subcmd {
	run := func(f fsdir.T) {
		c := clientcfg.Defaults()
		err := f.Get(&c, filenames.Config)
		if err != nil {
			log.Fatalf("could not read config: %s", err)
		}
		// common signal handler setup
		if c.Broker.Address == nil {
			log.Fatalf("broker.address not provided, refusing to start")
		}
		log.Default().SetFlags(log.LstdFlags | log.Lmsgprefix)
		log.Default().SetPrefix("[broker] ")
		brok := broker.New(f, &c, broklog)
		shutdowns = append(shutdowns, brok.Shutdown)
		reloads = append(reloads, brok.Reload)
		defer brok.Shutdown()

		mux := http.NewServeMux()
		mux.Handle("/broker", brok)

		// combo socket?
		if *c.Address == *c.Broker.Address {
			mux.Handle("/api/", http.StripPrefix("/api", restapi.New(restlog)))
			restlog.Printf("listening on h2c://%s", *c.Address)
		} else {
			restmux := http.NewServeMux()
			restmux.Handle("/api/", http.StripPrefix("/api", restapi.New(restlog)))

			restl, err := net.Listen("tcp", *c.Address)
			if err != nil {
				restlog.Fatalf("listening on h2c://%s failed: %s", *c.Address, err)
			}
			setupServer(restl, restmux, brok.T.TLSClientConfig)
			restlog.Printf("listening on h2c://%s", *c.Address)
		}

		brokl, err := net.Listen("tcp", *c.Broker.Address)
		if err != nil {
			broklog.Fatalf("listening on h2c://%s failed: %s", *c.Broker.Address, err)
		}
		setupServer(brokl, mux, brok.T.TLSClientConfig)
		broklog.Printf("listening on h2c://%s, waiting for forwarders to connect", *c.Broker.Address)

		cli.SignalLoop(cli.SignalMap{
			process.ReloadSignal: reload,
			syscall.SIGINT:       shutdown,
			syscall.SIGTERM:      shutdown,
			syscall.SIGQUIT:      shutdown,
		})
	}
	r := startcmd.Cmd("wireleap", run)
	r.Desc = fmt.Sprintf("%s %s", r.Desc, "(Wireleap connection broker)")
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
