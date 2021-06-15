// Copyright (c) 2021 Wireleap

package clientlib

import (
	"crypto/tls"
	"log"
	"net/http"

	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/wlnet"
	"github.com/wireleap/common/wlnet/flushwriter"
	"github.com/wireleap/common/wlnet/h2rwc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ListenH2C listens on the given address, waiting for h2c connection requests
// to dial through the circuit. The target protocol and address are supplied in
// the headers which allows using HPACK compression and immediate status
// feedback.
func ListenH2C(addr string, tc *tls.Config, dialer DialFunc, errf func(error)) error {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			status.ErrMethod.WriteTo(w)
			return
		}
		protocol := r.Header.Get("Sm-Dial-Protocol")
		target := r.Header.Get("Sm-Dial-Target")
		cc, err := dialer(protocol, target)
		if err != nil {
			log.Printf("h2->circuit dial failure: %s", err)
			return
		}
		rwc := h2rwc.T{flushwriter.T{w}, r.Body}
		err = wlnet.Splice(rwc, cc, 0, 32*1024)
		if err != nil {
			status.ErrGateway.WriteTo(w)
		}
		cc.Close()
		rwc.Close()
	}
	h1s := &http.Server{Addr: addr, Handler: http.HandlerFunc(h), TLSConfig: tc}
	h2s := &http2.Server{MaxHandlers: 0, MaxConcurrentStreams: 0}
	if err := http2.ConfigureServer(h1s, h2s); err != nil {
		return err
	}
	h1s.Handler = h2c.NewHandler(h1s.Handler, h2s)
	go func() { log.Fatal(h1s.ListenAndServe()) }()
	return nil
}
