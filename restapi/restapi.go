// Copyright (c) 2022 Wireleap

package restapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/common/api/status"
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
	case "/version":
		t.reply(w, Version{VERSION})
	case "/runtime":
		t.reply(w, RuntimeReply)
	case "/", "":
		w.Write([]byte("hello world"))
		t.l.Printf("just served %+v", r)
	default:
		t.l.Printf("%s just served %+v", r.URL.Path, r)
		http.NotFound(w, r)
	}
}

func (t *T) reply(w http.ResponseWriter, x interface{}) {
	b, err := json.Marshal(x)
	if err != nil {
		t.l.Printf("error %s while serving reply", err)
		status.ErrInternal.WriteTo(w)
		return
	}
	w.Write(b)
}
