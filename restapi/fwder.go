package restapi

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/provide"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli/process"
)

const fwderPrefix = "wireleap_"

type fwderReply struct {
	Pid     int         `json:"pid"`
	State   string      `json:"state"`
	Address string      `json:"address"`
	Binary  binaryReply `json:"binary"`
}

type binaryReply struct {
	Ok    bool        `json:"ok"`
	State binaryState `json:"state"`
}

type binaryState struct {
	// those are sensible for any forwarder
	Exists bool `json:"exists"`
	ChmodX bool `json:"chmod_x"`
	// those are specific to (currently) only tun
	Chown0  *bool `json:"chown_0,omitempty"`
	ChmodUS *bool `json:"chmod_us,omitempty"`
}

type FwderState struct {
	State string `json:"state"`
}

func boolptr(x bool) *bool { return &x }

func (t *T) getBinaryState(bin string) (st binaryState) {
	fi, err := os.Stat(t.br.Fd.Path(bin))
	if err != nil {
		return
	}
	st.Exists = true
	st.ChmodX = fi.Mode()&0100 != 0
	if bin == fwderPrefix+"tun" {
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok && stat.Uid == 0 {
			st.Chown0 = boolptr(true)
		} else {
			st.Chown0 = boolptr(false)
		}
		st.ChmodUS = boolptr(fi.Mode()&os.ModeSetuid != 0)
	}
	return
}

func (t *T) registerForwarder(name string) {
	var (
		bin = fwderPrefix + name
		o   = fwderReply{
			Pid:     -1,
			State:   "unknown",
			Address: "0.0.0.0:0",
			Binary: binaryReply{
				Ok: false,
			},
		}
		mu sync.Mutex
		cl = client.New(nil)
	)
	cl.SetTransport(&http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", bin+".sock")
		},
	})
	t.mux.Handle("/forwarders/"+name, provide.MethodGate(provide.Routes{http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		t.br.Fd.Get(&o.Pid, bin+".pid")
		var fst FwderState
		if err := cl.Perform(http.MethodGet, "http://localhost/state", nil, &fst); err == nil && fst.State != "unknown" {
			o.State = fst.State
		} else {
			o.State = "unknown"
		}
		st := t.getBinaryState(bin)
		// TODO can this be handled in a better way?
		switch name {
		case "socks":
			o.Address = t.br.Config().Forwarders.Socks.Address
			o.Binary.Ok = st.Exists && st.ChmodX
		case "tun":
			o.Address = t.br.Config().Forwarders.Tun.Address
			o.Binary.Ok = st.Exists && st.ChmodX && *st.Chown0 && *st.ChmodUS
		}
		o.Binary.State = st
		t.reply(w, o)
	})}))
	t.mux.Handle("/forwarders/"+name+"/start", provide.MethodGate(provide.Routes{http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pid int
		var err error
		defer func() {
			if err == nil {
				status.OK.WriteTo(w)
			} else {
				status.ErrRequest.Wrap(err).WriteTo(w)
			}
		}()
		binpath := t.br.Fd.Path(bin)
		st := t.getBinaryState(bin)
		switch {
		case !st.Exists:
			err = fmt.Errorf("forwarder %s does not exist", bin)
			return
		case !st.ChmodX:
			err = fmt.Errorf("could not execute %s: file is not executable (did you `chmod +x %s`?)", binpath, binpath)
			return
		case name == "tun" && !*st.Chown0:
			err = fmt.Errorf(
				"could not execute %s: file is not owned by root (did you `chown 0:0 %s && chmod u+s %s`?)",
				binpath, binpath, binpath,
			)
			return
		case name == "tun" && !*st.ChmodUS:
			err = fmt.Errorf("could not execute %s: file is not setuid (did you `chmod u+s %s`?)", binpath, binpath)
			return
		}
		env := append(
			os.Environ(),
			"WIRELEAP_HOME="+t.br.Fd.Path(),
			"WIRELEAP_ADDR_H2C="+*t.br.Config().Broker.Address+"/broker",
			"WIRELEAP_ADDR_TUN="+t.br.Config().Forwarders.Tun.Address,
			"WIRELEAP_ADDR_SOCKS="+t.br.Config().Forwarders.Socks.Address,
		)
		if err = t.br.Fd.Get(&pid, bin+".pid"); err == nil && process.Exists(pid) {
			err = fmt.Errorf("%s daemon is already running!", bin)
			return
		}
		logpath := t.br.Fd.Path(bin + ".log")
		logfile, err := os.OpenFile(logpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			err = fmt.Errorf("could not open %s logfile %s: %s", bin, logpath, err)
			return
		}
		defer logfile.Close()
		cmd := exec.Cmd{
			Path:   binpath,
			Args:   []string{bin},
			Env:    env,
			Stdout: logfile,
			Stderr: logfile,
		}
		if err = cmd.Start(); err != nil {
			mu.Lock()
			o.State = "failed"
			mu.Unlock()
			err = fmt.Errorf("could not spawn background %s process: %s", bin, err)
			return
		}
		log.Printf(
			"starting %s with pid %d, writing to %s...",
			bin, cmd.Process.Pid, logpath,
		)
		t.br.Fd.Set(cmd.Process.Pid, bin+".pid")
		// poll state until it's conclusive
		var fst FwderState
		for i := 0; i < 10; i++ {
			if err = cl.Perform(http.MethodGet, "http://localhost/state", nil, &fst); err == nil && fst.State != "unknown" {
				mu.Lock()
				o.State = fst.State
				mu.Unlock()
			} else {
				mu.Lock()
				o.State = "failed"
				mu.Unlock()
			}
		}
	})}))
	t.mux.Handle("/forwarders/"+name+"/stop", provide.MethodGate(provide.Routes{http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			pid     int
			err     error
			pidfile = bin + ".pid"
		)
		defer func() {
			if err == nil {
				status.OK.WriteTo(w)
			} else {
				status.ErrRequest.Wrap(err).WriteTo(w)
			}
		}()
		if err = t.br.Fd.Get(&pid, pidfile); err != nil {
			err = fmt.Errorf(
				"could not get pid of %s from %s: %s",
				bin, t.br.Fd.Path(pidfile), err,
			)
			return
		}
		if process.Exists(pid) {
			if err = process.Term(pid); err != nil {
				err = fmt.Errorf("could not terminate %s pid %d: %s", bin, pid, err)
				return
			}
		}
		for i := 0; i < 30; i++ {
			if !process.Exists(pid) {
				log.Printf("stopped %s daemon (was pid %d)", bin, pid)
				t.br.Fd.Del(pidfile)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		process.Kill(pid)
		time.Sleep(100 * time.Millisecond)
		if process.Exists(pid) {
			err = fmt.Errorf("timed out waiting for %s (pid %d) to shut down -- process still alive!", bin, pid)
			return
		}
		log.Printf("stopped %s daemon (was pid %d)", bin, pid)
		t.br.Fd.Del(pidfile)
	})}))
	t.mux.Handle("/forwarders/"+name+"/log", provide.MethodGate(provide.Routes{http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logfile := t.br.Fd.Path(bin + ".log")
		b, err := ioutil.ReadFile(logfile)
		if err != nil {
			status.ErrRequest.WriteTo(w)
			return
		}
		w.Write(b)
	})}))
}
