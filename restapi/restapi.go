// Copyright (c) 2022 Wireleap

package restapi

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/provide"
	"github.com/wireleap/common/api/relayentry"
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
			go t.br.Reload()
			t.reply(w, t.br.Config())
		}),
	}))
	t.mux.Handle("/runtime", provide.MethodGate(provide.Routes{
		http.MethodGet: t.replyHandler(RuntimeReply),
	}))
	t.mux.Handle("/contract", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ci := t.br.ContractInfo(); ci != nil {
				t.reply(w, t.br.ContractInfo())
			} else {
				status.ErrNotFound.Wrap(fmt.Errorf("contract info is not initialized")).WriteTo(w)
			}
		}),
	}))
	t.mux.Handle("/relays", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rs, err := t.br.Relays()
			if err != nil {
				t.l.Printf("error %s while serving relays", err)
				status.ErrNotFound.Wrap(err).WriteTo(w)
				return
			}
			type selectableRelay struct {
				*relayentry.T
				Selectable bool `json:"selectable"`
			}
			var ors []selectableRelay
			// selectable by default
			sel := true
			wl := t.br.Config().Broker.Circuit.Whitelist
			for _, r := range rs {
				if wl != nil && len(*wl) > 0 {
					// non-selectable by default if whitelist is set
					sel = false
					for _, wlr := range *wl {
						if wlr == r.Addr.String() {
							// found in whitelist = selectable
							sel = true
							break
						}
					}
				}
				ors = append(ors, selectableRelay{
					T:          r,
					Selectable: sel,
				})
			}
			t.reply(w, ors)
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
				status.ErrRequest.Wrap(err).WriteTo(w)
				return
			}
			air := AccesskeyImportRequest{}
			if err = json.Unmarshal(b, &air); err != nil || air.URL == nil {
				t.l.Printf("error when unmarshaling accesskeys import request: %s", err)
				status.ErrRequest.WriteTo(w)
				return
			}
			aks, err := t.br.Import(air.URL.URL)
			if err != nil {
				t.l.Printf("error when importing accesskeys: %s", err)
				status.ErrRequest.Wrap(err).WriteTo(w)
				return
			}
			go t.br.Reload()
			t.reply(w, t.accesskeysFromPofs(aks.Pofs...))
		}),
	}))
	t.mux.Handle("/accesskeys/activate", provide.MethodGate(provide.Routes{
		http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := t.br.Activate(); err != nil {
				t.l.Printf("error when activating new accesskey: %s", err)
				status.ErrRequest.Wrap(err).WriteTo(w)
				return
			}
			t.reply(w, t.accesskeysFromSks(t.br.CurrentSK()))
		}),
	}))
	t.mux.Handle("/status", provide.MethodGate(provide.Routes{
		http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			circList := []string{}
			for _, r := range t.br.ActiveCircuit() {
				circList = append(circList, r.Addr.String())
			}
			t.reply(w, StatusReply{
				Home:    t.br.Fd.Path(),
				Pid:     os.Getpid(),
				State:   "active",
				Broker:  StatusBroker{ActiveCircuit: circList},
				Upgrade: StatusUpgrade{Required: t.br.IsUpgradeable()},
			})
		}),
	}))
	t.mux.Handle("/reload", provide.MethodGate(provide.Routes{
		http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.br.Reload()
			circList := []string{}
			for _, r := range t.br.ActiveCircuit() {
				circList = append(circList, r.Addr.String())
			}
			t.reply(w, StatusReply{
				Home:    t.br.Fd.Path(),
				Pid:     os.Getpid(),
				State:   "active",
				Broker:  StatusBroker{ActiveCircuit: circList},
				Upgrade: StatusUpgrade{Required: t.br.IsUpgradeable()},
			})
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
