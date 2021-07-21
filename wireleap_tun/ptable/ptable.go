// Copyright (c) 2021 Wireleap

package ptable

import (
	"net"
	"sync"
	"sync/atomic"
)

type Family int

const (
	TCP Family = iota
	UDP
	nfamilies
	nports = 65535
)

type Entry struct {
	SrcIP, DstIP     net.IP
	SrcPort, DstPort int

	mu   sync.Mutex // for initializing conn
	conn net.Conn
}

type T [nfamilies * nports]atomic.Value

func (t *T) Get(f Family, port int) (e *Entry) {
	v := t[int(f*nports)+port].Load()
	if v == nil || v.(*Entry) == nil {
		return (*Entry)(nil)
	}
	e = v.(*Entry)
	return
}

func (t *T) Set(f Family, port int, e *Entry, init func() (net.Conn, error)) {
	e.mu.Lock()
	t[int(f*nports)+port].Store(e)
	go func() {
		defer e.mu.Unlock()
		if c, err := init(); err == nil {
			e.conn = c
		} else {
			t.Del(f, port)
		}
	}()
}

func (t *T) Del(f Family, port int) {
	// remove reference
	// it will get garbage collected after it goes out of scope
	t[int(f*nports)+port].Store((*Entry)(nil))
}

func (e *Entry) Conn() net.Conn {
	// wait until init unlocks the connection
	e.mu.Lock()
	e.mu.Unlock()
	return e.conn
}
