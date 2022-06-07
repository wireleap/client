package restapi

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

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
	Exists  bool `json:"exists"`
	ChmodX  bool `json:"chmod_x"`
	Chown0  bool `json:"chown_0"`
	ChmodUS bool `json:"chmod_us"`
}

func (t *T) getBinaryState(bin string) (st binaryState) {
	fi, err := os.Stat(t.br.Fd.Path(bin))
	if err != nil {
		return
	}
	st.Exists = true
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok && stat.Uid == 0 {
		st.Chown0 = true
	}
	st.ChmodX = fi.Mode()&0100 != 0
	st.ChmodUS = fi.Mode()&os.ModeSetuid != 0
	return
}

func (t *T) registerForwarder(name string) {
	bin := fwderPrefix + name
	t.mux.Handle("/forwarders/"+name, provide.MethodGate(provide.Routes{http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := fwderReply{
			Pid:     -1,
			State:   "inactive",
			Address: "0.0.0.0:0",
			Binary: binaryReply{
				Ok: false,
			},
		}
		if t.br.Fd.Get(&o.Pid, bin+".pid"); o.Pid != -1 && process.Exists(o.Pid) {
			o.State = "active"
		}
		st := t.getBinaryState(bin)
		// TODO can this be handled in a better way?
		switch name {
		case "socks":
			o.Address = *t.br.Config().Forwarders.Socks
			o.Binary.Ok = st.Exists && st.ChmodX
		case "tun":
			o.Address = *t.br.Config().Forwarders.Tun
			o.Binary.Ok = st.Exists && st.ChmodX && st.Chown0 && st.ChmodUS
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
		case name == "tun" && !st.Chown0:
			err = fmt.Errorf(
				"could not execute %s: file is not owned by root (did you `chown 0:0 %s && chmod u+s %s`?)",
				binpath, binpath, binpath,
			)
			return
		case name == "tun" && !st.ChmodUS:
			err = fmt.Errorf("could not execute %s: file is not setuid (did you `chmod u+s %s`?)", binpath, binpath)
			return
		}
		env := append(
			os.Environ(),
			"WIRELEAP_HOME="+t.br.Fd.Path(),
			"WIRELEAP_ADDR_H2C="+*t.br.Config().Broker.Address+"/broker",
			"WIRELEAP_ADDR_TUN="+*t.br.Config().Forwarders.Tun,
			"WIRELEAP_ADDR_SOCKS="+*t.br.Config().Forwarders.Socks,
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
			err = fmt.Errorf("could not spawn background %s process: %s", bin, err)
			return
		}
		log.Printf(
			"starting %s with pid %d, writing to %s...",
			bin, cmd.Process.Pid, logpath,
		)
		t.br.Fd.Set(cmd.Process.Pid, bin+".pid")
		// wait for 2s and see if it's still alive
		e := make(chan error)
		go func() { e <- cmd.Wait() }()
		select {
		case <-e:
			log.Printf("%s is not running, %s follows:", bin, logpath)
		case <-time.NewTimer(time.Second * 2).C:
			log.Printf("%s spawned succesfully", bin)
		}
		return
		err = syscall.Exec(binpath, nil, env)
		hint := ""
		if os.IsPermission(err) {
			hint = ", check permissions (owned by 0:0, executable bit/+x, setuid/+s)?"
		}
		if err != nil {
			err = fmt.Errorf("could not execute %s: %s%s", binpath, err, hint)
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
