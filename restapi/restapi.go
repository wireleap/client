// Copyright (c) 2021 Wireleap

package restapi

import (
	"log"
	"net/http"
)

// api server stub
type T struct {
	l *log.Logger
}

func New(_ *log.Logger) *T {
	return &T{}
}

func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/", "":
		w.Write([]byte("hello world"))
	default:
		http.NotFound(w, r)
	}
}
