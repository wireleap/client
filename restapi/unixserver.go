// Copyright (c) 2022 Wireleap

package restapi

import (
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
	defer l.Close()
	mux := provide.NewMux(rts)
	h := &http.Server{Handler: mux}
	return h.Serve(l)
}
