// Copyright (c) 2021 Wireleap

package snatmap

import (
	"net"
	"strconv"
	"sync"
)

// T is the type of a concurrent string source -> destination address map.
type T struct {
	mu sync.RWMutex
	m  map[string]*Addr
}

type Addr struct {
	// optional tcpconn chan for dial priming
	Conn chan *net.TCPConn

	IP   net.IP
	Port int
}

func (t *Addr) String() string {
	return net.JoinHostPort(t.IP.String(), strconv.Itoa(t.Port))
}

func New() *T { return &T{m: map[string]*Addr{}} }

func (t *T) Add(k string, v *Addr) {
	t.mu.Lock()
	t.m[k] = v
	t.mu.Unlock()
}

func (t *T) Get(k string) (v *Addr) {
	t.mu.RLock()
	v = t.m[k]
	t.mu.RUnlock()
	return
}

func (t *T) Del(k string) {
	t.mu.Lock()
	delete(t.m, k)
	t.mu.Unlock()
}
