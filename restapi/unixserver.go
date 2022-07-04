// Copyright (c) 2022 Wireleap

package restapi

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/wireleap/common/api/provide"
)

func UnixServer(p string, rts provide.Routes) error {
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	l, err := net.Listen("unix", p)
	if err != nil {
		return err
	}
	mux := provide.NewMux(rts)
	h := &http.Server{Handler: mux}
	go func() {
		defer l.Close()
		if err = h.Serve(l); err != nil {
			log.Fatalf("error when serving unix socket: %s", err)
		}
	}()
	return nil
}
