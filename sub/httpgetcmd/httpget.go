// Copyright (c) 2022 Wireleap

package httpgetcmd

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/wlnet/h2conn"
	"golang.org/x/net/http2"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("httpget", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Perform a HTTP GET through the circuit (experimental)",
	}
	r.SetMinimalUsage("[URL]")
	r.Run = func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		if fs.NArg() == 0 || fs.NArg() > 1 {
			r.Usage()
			os.Exit(1)
		}
		conn, err := net.DialTimeout("tcp", *c.Broker.Address, time.Second)
		if err != nil {
			log.Fatalf("could not connect to wireleap broker at address %s: %s", *c.Broker.Address, err)
		}
		conn.Close()
		// h2c transport for non-TLS dial to broker
		var h2ct = &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
			ReadIdleTimeout: 10 * time.Second,
			PingTimeout:     10 * time.Second,
		}
		// high-level transport which connects through the broker
		var wlt = &http.Transport{
			Dial: func(proto string, addr string) (net.Conn, error) {
				c, err := h2conn.New(h2ct, "https://"+*c.Broker.Address+"/broker", map[string]string{
					"Wl-Dial-Protocol": proto,
					"Wl-Dial-Target":   addr,
					"Wl-Forwarder":     "httpget",
				})

				if err != nil {
					log.Fatalf("error when h2conn: %s", err)
				}
				return c, nil
			},
		}
		cl := &http.Client{Transport: wlt}
		res, err := cl.Get(fs.Arg(0))
		if err != nil {
			log.Fatalf("error while performing get request: %s", err)
		}
		defer res.Body.Close()
		if _, err = io.Copy(os.Stdout, res.Body); err != nil {
			log.Fatalf("error while reading response body: %s", err)
		}
	}
	return r
}
