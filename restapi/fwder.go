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

type FwderReply struct {
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
		bin     = fwderPrefix + name
		pidfile = bin + ".pid"
		o       = FwderReply{
			Pid:     -1,
			State:   "inactive",
			Address: "0.0.0.0:0",
			Binary:  binaryReply{},
		}
		mu              sync.Mutex
		cl              = client.New(nil)
		syncBinaryState = func() {
			// NOTE this does not do any locking. locking by the caller is expected
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
		}
	)
	cl.SetTransport(&http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", bin+".sock")
		},
	})
	t.mux.Handle("/forwarders/"+name, provide.MethodGate(provide.Routes{http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		t.br.Fd.Get(&o.Pid, pidfile)
		var fst FwderState
		for i := 0; i < 10; i++ {
			if err := cl.PerformOnce(http.MethodGet, "http://localhost/state", nil, &fst); err == nil && fst.State != "unknown" {
				o.State = fst.State
				break
			} else if o.Pid != -1 && !process.Exists(o.Pid) && o.State != "failed" {
				o.State = "inactive"
			} else {
				o.State = "unknown"
			}
			time.Sleep(100 * time.Millisecond)
		}
		syncBinaryState()
		t.reply(w, o)
	})}))
	t.mux.Handle("/forwarders/"+name+"/start", provide.MethodGate(provide.Routes{http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer func() {
			if err == nil {
				t.reply(w, o)
			} else {
				status.ErrRequest.Wrap(err).WriteTo(w)
			}
		}()
		binpath := t.br.Fd.Path(bin)
		syncBinaryState()
		st := o.Binary.State
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
		if err = t.br.Fd.Get(&o.Pid, pidfile); err == nil && process.Exists(o.Pid) {
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
		t.br.Fd.Set(cmd.Process.Pid, pidfile)
		mu.Lock()
		o.Pid = cmd.Process.Pid
		mu.Unlock()
		// poll state until it's conclusive
		var fst FwderState
		for i := 0; i < 10; i++ {
			if err = cl.PerformOnce(http.MethodGet, "http://localhost/state", nil, &fst); err == nil && fst.State != "unknown" {
				mu.Lock()
				o.State = fst.State
				mu.Unlock()
				break
			} else {
				mu.Lock()
				o.State = "failed"
				mu.Unlock()
			}
			time.Sleep(100 * time.Millisecond)
		}
		go func() {
			// reap process so it doesn't turn zombie
			if err = cmd.Wait(); err != nil {
				mu.Lock()
				o.Pid = -1
				o.State = "failed"
				mu.Unlock()
			} else {
				mu.Lock()
				o.Pid = -1
				o.State = "inactive"
				mu.Unlock()
			}
		}()
	})}))
	t.mux.Handle("/forwarders/"+name+"/stop", provide.MethodGate(provide.Routes{http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer func() {
			if err == nil {
				mu.Lock()
				o.Pid = -1
				o.State = "inactive"
				mu.Unlock()
				t.reply(w, o)
			} else {
				status.ErrRequest.Wrap(err).WriteTo(w)
			}
		}()
		syncBinaryState()
		if err = t.br.Fd.Get(&o.Pid, pidfile); err != nil {
			err = fmt.Errorf(
				"could not get pid of %s from %s: %s",
				bin, t.br.Fd.Path(pidfile), err,
			)
			return
		}
		if process.Exists(o.Pid) {
			log.Println(o.Pid, "exists")
			if err = process.Term(o.Pid); err != nil {
				err = fmt.Errorf("could not terminate %s pid %d: %s", bin, o.Pid, err)
				return
			}
		}
		mu.Lock()
		o.State = "deactivating"
		mu.Unlock()
		for i := 0; i < 30; i++ {
			if !process.Exists(o.Pid) {
				log.Printf("stopped %s daemon (was pid %d)", bin, o.Pid)
				t.br.Fd.Del(pidfile)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		process.Kill(o.Pid)
		time.Sleep(100 * time.Millisecond)
		if process.Exists(o.Pid) {
			err = fmt.Errorf("timed out waiting for %s (pid %d) to shut down -- process still alive!", bin, o.Pid)
			return
		}
		log.Printf("stopped %s daemon (was pid %d)", bin, o.Pid)
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
