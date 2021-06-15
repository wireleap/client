// Copyright (c) 2021 Wireleap

package tun

import (
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/songgao/water"
)

const bufsize = 65535 // maximum IP packet size just to be safe

// T is the type of a tun device.
type T struct {
	*water.Interface
	NetIf *net.Interface
	buf   []byte
}

// New() creates a new tun device.
func New() (s *T, err error) {
	var ifc *water.Interface
	ifc, err = water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		return
	}
	var nif *net.Interface
	nif, err = net.InterfaceByName(ifc.Name())
	if err != nil {
		return
	}
	s = &T{
		Interface: ifc,
		NetIf:     nif,
		buf:       make([]byte, bufsize),
	}
	return
}

// ReadPacketData() fulfills the gopacket.PacketSource interface.
func (s *T) ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	var n int
	n, err = s.Read(s.buf)
	if err != nil {
		return
	}
	data = s.buf[:n]
	ci.Timestamp = time.Now()
	ci.CaptureLength = n
	return
}
