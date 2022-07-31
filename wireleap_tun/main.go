// Copyright (c) 2022 Wireleap

package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"syscall"

	"github.com/wireleap/client/restapi"
	"github.com/wireleap/client/wireleap_tun/netsetup"
	"github.com/wireleap/client/wireleap_tun/tun"
	"github.com/wireleap/common/api/provide"
	"github.com/wireleap/common/api/status"

	"net/http"
	_ "net/http/pprof"
)

func main() {
	// set up state API
	state := "activating"
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("could not find own executable path: %s", err)
	}
	bypass := bypassList{}
	err = restapi.UnixServer(exe+".sock", provide.Routes{
		"/state": provide.MethodGate(provide.Routes{
			http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				st := restapi.FwderState{State: state}
				b, err := json.Marshal(st)
				if err != nil {
					log.Printf("error while serving /state reply: %s", err)
					status.ErrInternal.WriteTo(w)
					return
				}
				w.Write(b)
			}),
		}),
		"/bypass": provide.MethodGate(provide.Routes{
			http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := json.Marshal(bypass.Get())
				if err != nil {
					log.Printf("error while serving /bypass GET reply: %s", err)
					status.ErrInternal.WriteTo(w)
					return
				}
				w.Write(b)
			}),
			http.MethodPost: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ips := []net.IP{}
				b, err := io.ReadAll(r.Body)
				if err != nil {
					log.Printf("error while reading /bypass POST request body: %s", err)
					status.ErrRequest.WriteTo(w)
					return
				}
				err = json.Unmarshal(b, &ips)
				if err != nil {
					log.Printf("error while unmarshaling /bypass POST request body: %s", err)
					status.ErrRequest.WriteTo(w)
					return
				}
				if err = bypass.Set(ips...); err != nil {
					// hard fail here to catch bugs & avoid inadvertent leaks
					// TODO maybe soft fail in the future
					log.Fatalf("could not configure routes: %s", err)
				}
				status.OK.WriteTo(w)
			}),
			http.MethodDelete: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bypass.Clear()
				status.OK.WriteTo(w)
			}),
		}),
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := syscall.Seteuid(0); err != nil {
		log.Fatal("could not gain privileges; check if setuid flag is set?")
	}
	os.Chmod(exe+".sock", 0660)
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sh := os.Getenv("WIRELEAP_HOME")
	h2caddr := os.Getenv("WIRELEAP_ADDR_H2C")
	tunaddr := os.Getenv("WIRELEAP_ADDR_TUN")
	if sh == "" || h2caddr == "" || tunaddr == "" {
		log.Fatal("Running wireleap_tun separately from wireleap is not supported. Please use `sudo wireleap tun start`.")
	}
	t, err := tun.New()
	if err != nil {
		log.Fatalf("could not create tun device: %s", err)
	}
	rlim := syscall.Rlimit{Cur: 65535, Max: 65535}
	if err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		log.Fatalf("could not set RLIMIT_NOFILE to %+v", rlim)
	}
	if err = netsetup.Init(t, tunaddr); err != nil {
		log.Fatalf("could not configure tun device %s as %s: %s", t.Name(), tunaddr, err)
	}
	pidfile := path.Join(sh, "wireleap_tun.pid")
	finalize := func() {
		// don't need to delete catch-all routes via tun dev as they will be
		// removed when the device is down
		bypass.Clear()
		os.Remove(pidfile)
	}
	defer finalize()
	os.Remove(pidfile)
	pidtext := []byte(strconv.Itoa(os.Getpid()))
	err = ioutil.WriteFile(pidfile, pidtext, 0644)
	if err != nil {
		finalize()
		log.Fatalf("could not write pidfile %s: %s", pidfile, err)
	}
	defer os.Remove(pidfile)
	// setup debugging & profiling
	if os.Getenv("WIRELEAP_TUN_DEBUG") != "" {
		DEBUG = true
	}
	if os.Getenv("WIRELEAP_TUN_PPROF") != "" {
		go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()
	}
	if bpr := os.Getenv("WIRELEAP_TUN_BLOCK_PROFILE_RATE"); bpr != "" {
		n, err := strconv.Atoi(bpr)
		if err != nil {
			log.Fatalf("invalid WIRELEAP_TUN_BLOCK_PROFILE_RATE value: %s", bpr)
		}
		runtime.SetBlockProfileRate(n)
	}
	if mpf := os.Getenv("WIRELEAP_TUN_MUTEX_PROFILE_FRACTION"); mpf != "" {
		n, err := strconv.Atoi(mpf)
		if err != nil {
			log.Fatalf("invalid WIRELEAP_TUN_MUTEX_PROFILE_FRACTION value: %s", mpf)
		}
		runtime.SetMutexProfileFraction(n)
	}
	log.Printf("listening for state queries on %s", exe+".sock")
	if err = tunsplice(t, h2caddr, tunaddr); err != nil {
		log.Fatal("tunsplice returned error:", err)
	}
	state = "active"
	for {
		select {
		case s := <-sig:
			state = "deactivating"
			log.Printf("terminating on signal %s", s)
			return
		}
	}
}
