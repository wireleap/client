// Copyright (c) 2021 Wireleap

package restapi

import (
	"log"
	"net/http"

	"github.com/wireleap/client/broker"
)

// api server stub
type T struct {
	br *broker.T
	l  *log.Logger
}

func New(br *broker.T, l *log.Logger) *T {
	return &T{br: br, l: l}
}

func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/", "":
		w.Write([]byte("hello world"))
		t.l.Printf("just served %+v", r)
	default:
		t.l.Printf("%s just served %+v", r.URL.Path, r)
		http.NotFound(w, r)
	}
}
