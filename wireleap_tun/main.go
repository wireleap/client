// Copyright (c) 2021 Wireleap

package main

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/vishvananda/netlink"
	"github.com/wireleap/client/wireleap_tun/tun"

	"net/http"
	_ "net/http/pprof"
)

func main() {
	if err := syscall.Seteuid(0); err != nil {
		log.Fatal("could not gain privileges; check if setuid flag is set?")
	}
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
	routes, err := getroutes(sh)
	if err != nil {
		log.Fatalf("could not get routes: %s", err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("could not set up file watcher: %s", err)
	}
	defer watcher.Close()
	err = watcher.Add(path.Join(sh, "bypass.json"))
	if err != nil {
		log.Fatalf("could not add bypass.json to file watcher: %s", err)
	}

	link, err := netlink.LinkByName(t.Name())
	if err != nil {
		log.Fatalf("could not get link for %s: %s", t.Name(), err)
	}
	err = netlink.LinkSetTxQLen(link, 1000)
	if err != nil {
		log.Fatalf("could not set link txqueue length for %s to %d: %s", t.Name(), 1000, err)
	}
	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Fatalf("could not set %s up: %s", link, err)
	}
	tunhost, _, err := net.SplitHostPort(tunaddr)
	if err != nil {
		log.Fatalf("could not parse WIRELEAP_ADDR_TUN `%s`: %s", tunaddr, err)
	}
	addr, err := netlink.ParseAddr(tunhost + "/31")
	if err != nil {
		log.Fatalf("could not parse address of %s: %s", tunaddr, err)
	}
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		log.Fatalf("could not set address of %s to %s: %s", link, addr, err)
	}
	// avoid clobbering the default route by being just a _little_ bit more specific
	for _, r := range append([]netlink.Route{{
		// lower half of all v4 addresses
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(1, net.IPv4len*8)},
	}, {
		// upper half of all v4 addresses
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IPv4(128, 0, 0, 0), Mask: net.CIDRMask(1, net.IPv4len*8)},
	}, {
		// v6 global-adressable range
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.ParseIP("2000::"), Mask: net.CIDRMask(3, net.IPv6len*8)},
	}}, routes...) {
		log.Printf("adding route: %+v", r)
		err = netlink.RouteReplace(&r)
		if err != nil {
			log.Fatalf("could not add route to %s: %s", r.Dst, err)
		}
	}
	pidfile := path.Join(sh, "wireleap_tun.pid")
	finalize := func() {
		// don't need to delete catch-all routes via tun dev as they will be
		// removed when the device is down
		for _, r := range routes {
			netlink.RouteDel(&r)
		}
		os.Remove(pidfile)
	}
	defer finalize()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
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
	if err = tunsplice(t, h2caddr, tunaddr); err != nil {
		log.Fatal("tunsplice returned error:", err)
	}
	for {
		select {
		case s := <-sig:
			finalize()
			log.Fatalf("terminating on signal %s", s)
		case _, ok := <-watcher.Events:
			if !ok {
				return
			}
			routes2, err := getroutes(sh)
			if err != nil {
				log.Fatal(err)
			}
			for _, r := range routes {
				netlink.RouteDel(&r)
			}
			for _, r := range routes2 {
				err = netlink.RouteReplace(&r)
				if err != nil {
					log.Fatalf("could not remove route %s: %s", r, err)
				}
			}
			routes = routes2
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error while watching files:", err)
		}
	}
}
