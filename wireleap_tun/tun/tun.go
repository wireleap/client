// Copyright (c) 2022 Wireleap

package tun

import (
	"net"

	"github.com/songgao/water"
)

// T is the type of a tun device.
type T struct {
	*water.Interface
	NetIf *net.Interface
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
	}
	return
}
