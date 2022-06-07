// Copyright (c) 2022 Wireleap

package restapi

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/provide"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli/process"
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
		http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.l.Printf("could not read POST /config request body: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
			if err = json.Unmarshal(b, t.br.Config()); err != nil {
				t.l.Printf("could not unmarshal POST /config request body: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
			if err = t.br.SaveConfig(); err != nil {
				t.l.Printf("could not save config changes: %s", err)
				status.ErrInternal.WriteTo(w)
				return
			}
			t.br.Reload()
			status.OK.WriteTo(w)
		}),
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
	t.mux.Handle("/relays", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rs, err := t.br.Relays()
			if err != nil {
				t.l.Printf("error %s while serving relays", err)
				status.ErrInternal.WriteTo(w)
				return
			}
			t.reply(w, rs)
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
			status.OK.WriteTo(w)
		}),
	}))
	t.mux.Handle("/accesskeys/activate", provide.MethodGate(provide.Routes{
		http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := t.br.AKM.Activate(); err != nil {
				t.l.Printf("error when activating new accesskey: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
			status.OK.WriteTo(w)
		}),
	}))
	t.mux.Handle("/status", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var (
				err   error
				pid   int
				state string
			)

			// TODO increase number of detectable states
			if err = t.br.Fd.Get(&pid, filenames.Pid); err != nil {
				pid = 0
				state = "inactive"
			} else {
				if process.Exists(pid) {
					state = "active"
				} else {
					state = "inactive"
				}
			}

			circList := []string{}
			for _, r := range t.br.ActiveCircuit() {
				circList = append(circList, r.Addr.String())
			}

			t.reply(w, statusReply{
				Home:   "/",
				Pid:    pid,
				State:  state,
				Broker: statusBroker{ActiveCircuit: circList},
				// TODO FIXME
				Upgrade: statusUpgrade{Required: false},
			})
		}),
	}))
	t.mux.Handle("/reload", provide.MethodGate(provide.Routes{
		http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.br.Reload()
			status.OK.WriteTo(w)
		}),
	}))
	t.mux.Handle("/log", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logfile := t.br.Fd.Path(filenames.Log)
			b, err := ioutil.ReadFile(logfile)
			if err != nil {
				status.ErrRequest.WriteTo(w)
				return
			}
			w.Write(b)
		}),
	}))
	t.registerForwarder("socks")
	t.registerForwarder("tun")
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
