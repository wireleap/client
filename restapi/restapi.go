// Copyright (c) 2022 Wireleap

package restapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/common/api/provide"
	"github.com/wireleap/common/api/status"
)

// api server stub
type T struct {
	br  *broker.T
	l   *log.Logger
	mux *http.ServeMux
}

func New(br *broker.T, l *log.Logger) (t *T) {
	t = &T{br: br, l: l, mux: http.NewServeMux()}
	t.mux.Handle("/version", provide.MethodGate(provide.Routes{
		http.MethodGet: t.replyHandler(Version{VERSION}),
	}))
	t.mux.Handle("/config", provide.MethodGate(provide.Routes{
		http.MethodGet: t.replyHandler(t.br.Config()),
	}))
	t.mux.Handle("/runtime", provide.MethodGate(provide.Routes{
		http.MethodGet: t.replyHandler(RuntimeReply),
	}))
	t.mux.Handle("/contract", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ci, err := t.br.ContractInfo()
			if err != nil {
				t.l.Printf("could not obtain contract info: %s", err)
				status.ErrInternal.WriteTo(w)
				return
			}
			t.reply(w, ci)
		}),
	}))
	t.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.l.Printf("%s just served %+v", r.URL.Path, r)
		http.NotFound(w, r)
	}))
	return
}

func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mux.ServeHTTP(w, r)
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

func (t *T) replyHandler(x interface{}) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.reply(w, x)
	})
}
