// Copyright (c) 2022 Wireleap

package main

import (
	"net"
	"sync"

	"github.com/wireleap/client/wireleap_tun/netsetup"
)

// a bypassList holds the bypassed IPs and routes
type bypassList struct {
	m   []net.IP
	mu  sync.RWMutex
	rts netsetup.Routes
}

func (t *bypassList) Set(ips ...net.IP) (err error) {
	t.mu.Lock()
	t.m = ips
	if t.rts != nil {
		t.rts.Down()
		t.rts = nil
	}
	t.rts, err = netsetup.RoutesUp(ips...)
	t.mu.Unlock()
	return
}

func (t *bypassList) Get() []net.IP {
	t.mu.RLock()
	r := make([]net.IP, len(t.m))
	copy(r, t.m)
	t.mu.RUnlock()
	return r
}

func (t *bypassList) Clear() {
	t.mu.Lock()
	t.m = []net.IP{}
	if t.rts != nil {
		t.rts.Down()
		t.rts = nil
	}
	t.mu.Unlock()
}
