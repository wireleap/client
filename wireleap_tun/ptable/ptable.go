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

	sync.Mutex // this mutex guards Conn only
	Conn       net.Conn
}

type T [nfamilies * nports]atomic.Value

func (t *T) Get(f Family, port int) (e *Entry) {
	v := t[int(f*nports)+port].Load()
	if v == nil {
		return (*Entry)(nil)
	}
	return v.(*Entry)
}
func (t *T) Set(f Family, port int, e *Entry) { t[int(f*nports)+port].Store(e) }
func (t *T) Del(f Family, port int)           { t[int(f*nports)+port].Store((*Entry)(nil)) }
