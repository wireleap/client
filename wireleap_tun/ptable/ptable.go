// Copyright (c) 2021 Wireleap

package ptable

import (
	"net"
	"sync"
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

// WARNING: not concurrency-safe. Write from one thread only.
type T [nfamilies * nports]*Entry

func (t *T) Get(f Family, port int) (e *Entry) { return t[int(f*nports)+port] }
func (t *T) Set(f Family, port int, e *Entry)  { t[int(f*nports)+port] = e }
func (t *T) Del(f Family, port int)            { t[int(f*nports)+port] = nil }
