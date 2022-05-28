// Copyright (c) 2022 Wireleap

package restapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/common/api/provide"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/api/texturl"
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
	t.mux.Handle("/accesskeys", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.reply(w, t.newAccesskeysReply())
		}),
	}))
	t.mux.Handle("/accesskeys/import", provide.MethodGate(provide.Routes{
		http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.l.Printf("error when reading accesskeys import request body: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
			air := AccesskeyImportRequest{}
			if err = json.Unmarshal(b, &air); err != nil || air.URL == nil {
				t.l.Printf("error when unmarshaling accesskeys import request: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
			if err = t.br.AKM.Import(air.URL.String()); err != nil {
				t.l.Printf("error when importing accesskeys: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
		}),
	}))
	// catch-all handler for unrouted paths
	t.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.l.Printf("%s just served %+v", r.URL.Path, r)
		http.NotFound(w, r)
	}))
	return
}

type AccesskeyImportRequest struct {
	URL *texturl.URL `json:"url"`
}

type accesskeyReply struct {
	Contract   *texturl.URL `json:"contract"`
	Duration   int64        `json:"duration"`
	State      string       `json:"state"`
	Expiration int64        `json:"expiration"`
}

func (t *T) newAccesskeysReply() (rs []*accesskeyReply) {
	ci, err := t.br.ContractInfo()
	if err != nil {
		return
	}
	// get contract info
	if sk := t.br.AKM.CurrentSK(); sk != nil {
		state := "active"
		if sk.IsExpiredAt(time.Now().Unix()) {
			state = "expired"
		}
		rs = append(rs, &accesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      state,
			Expiration: sk.Contract.SettlementOpen,
		})
	}
	for _, p := range t.br.AKM.CurrentPofs() {
		rs = append(rs, &accesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      "inactive",
			Expiration: p.Expiration,
		})
	}
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
