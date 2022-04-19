// Copyright (c) 2021 Wireleap

// Package socks provides a barebones SOCKSv5 server handshake protocol
// implementation according to RFC1928.
// https://datatracker.ietf.org/doc/html/rfc1928
package socks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	SOCKSv5 = 0x05

	CONNECT   = 0x01
	BIND      = 0x02
	UDP_ASSOC = 0x03

	ADDR_IPV4 = 0x01
	ADDR_FQDN = 0x03
	ADDR_IPV6 = 0x04

	RSV = 0x00
)

type SocksStatus byte

func (e SocksStatus) Error() string {
	return [...]string{
		"OK",
		"general failure",
		"not allowed",
		"network unreachable",
		"host unreachable",
		"connection refused",
		"TTL expired",
		"command not supported",
		"address type not supported",
	}[e]
}

const (
	StatusOK SocksStatus = iota
	StatusGeneralFailure
	StatusNotAllowed
	StatusNetworkUnreachable
	StatusHostUnreachable
	StatusConnRefused
	StatusTTLExpired
	StatusCommandNotSupported
	StatusAddressNotSupported
)

func WriteStatus(c net.Conn, status SocksStatus, addr Addr) (int, error) {
	return c.Write(append([]byte{SOCKSv5, byte(status), RSV}, addr...))
}

type Addr []byte

func AddrIPPort(ip net.IP, port int) (r Addr) {
	if ip4 := ip.To4(); ip4 == nil {
		r = append(r, ADDR_IPV6)
		r = append(r, ip...)
	} else {
		r = append(r, ADDR_IPV4)
		r = append(r, ip4...)
	}
	r = append(r, byte(port>>8), byte(port))
	return
}

func AddrString(addr string) (r Addr, err error) {
	host, portstr, err := net.SplitHostPort(addr)
	if err != nil {
		return
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// probably fqdn
		r = append(r, ADDR_FQDN)
		r = append(r, byte(len(host)))
		r = append(r, []byte(host)...)
	} else {
		if ip4 := ip.To4(); ip4 != nil {
			// v4
			r = append(r, ADDR_IPV4)
			r = append(r, []byte(ip4)...)
		} else {
			// v6
			r = append(r, ADDR_IPV6)
			r = append(r, []byte(ip)...)
		}
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		return
	}
	r = append(r, []byte{byte(port >> 8), byte(port)}...)
	return
}

func AddrAddr(orig net.Addr) (r Addr) {
	switch v := orig.(type) {
	case *net.TCPAddr:
		r = AddrIPPort(v.IP, v.Port)
	case *net.UDPAddr:
		r = AddrIPPort(v.IP, v.Port)
	}
	return
}

func (t Addr) String() string {
	if ip, port := t.IPPort(); ip != nil {
		// ip:port
		return net.JoinHostPort(ip.String(), strconv.Itoa(port))
	}
	if len(t) < 2 || len(t) < int(2+t[1]+2) {
		// ???
		return ""
	}
	// fqdn:port
	port := int(t[1]+2)<<8 | int(t[1]+2+1)
	return net.JoinHostPort(string(t[2:t[1]+2]), strconv.Itoa(port))
}

func (t Addr) IPPort() (ip net.IP, port int) {
	if len(t) < 1 {
		return
	}
	switch t[0] {
	case ADDR_IPV4, ADDR_IPV6:
		mult := int(t[0])
		if len(t) < 1+mult*4+2 {
			return
		}
		ip = net.IP(t[1 : mult*4+1])
		port = int(t[1+mult*4])<<8 | int(t[1+mult*4+1])
	}
	return
}

func ComposeUDP(dstaddr Addr, p []byte) (r []byte, err error) {
	// RSV, RSV, FRAG
	r = []byte{0, 0, 0}
	// ATYP, ADDR, PORT
	r = append(r, dstaddr...)
	// DATA
	r = append(r, p...)
	return
}

var ErrFragment = errors.New("received UDP message is fragmented, fragmentation is not supported")

func DissectUDP(p []byte) (dstaddr Addr, data []byte, err error) {
	// RSV, RSV ignored
	// FRAG -- do not process fragmentation
	if p[2] != 0 {
		err = ErrFragment
		return
	}
	// ATYP, ADDR, DATA
	switch p[3] {
	case ADDR_IPV4, ADDR_IPV6:
		mult := p[3]
		// ATYP + (1 or 4) * 4 + PORT
		dstaddr = make([]byte, 1+mult*4+2)
		copy(dstaddr, p[3:])
		data = p[3+len(dstaddr):]
	case ADDR_FQDN:
		// SIZE
		size := p[4]
		dstaddr = make([]byte, 2+size+2)
		copy(dstaddr, p[3:])
		data = p[2+len(dstaddr):]
	}
	return
}

func Handshake(c net.Conn) (cmd byte, address string, err error) {
	b := make([]byte, 1)
	// read auth methods
	// SOCKS version
	_, err = io.ReadFull(c, b)
	if err != nil {
		return
	}
	if b[0] != SOCKSv5 {
		WriteStatus(c, StatusGeneralFailure, AddrAddr(c.LocalAddr()))
		err = fmt.Errorf("unknown SOCKS auth version: 0x%x", b)
		return
	}
	// number of auth methods
	_, err = io.ReadFull(c, b)
	if err != nil {
		return
	}
	methods := make([]byte, b[0])
	// auth methods -- don't care which ones as no auth is required
	_, err = io.ReadFull(c, methods)
	if err != nil {
		return
	}
	// tell the client no auth is needed
	_, err = c.Write([]byte{0x05, 0x00})
	if err != nil {
		return
	}
	// read request
	// SOCKS version
	_, err = io.ReadFull(c, b)
	if err != nil {
		return
	}
	if b[0] != SOCKSv5 {
		WriteStatus(c, StatusGeneralFailure, AddrAddr(c.LocalAddr()))
		err = fmt.Errorf("unknown SOCKS request version: 0x%x", b)
		return
	}
	// command
	_, err = io.ReadFull(c, b)
	if err != nil {
		return
	}
	switch b[0] {
	case CONNECT, UDP_ASSOC:
		cmd = b[0]
	default:
		WriteStatus(c, StatusCommandNotSupported, AddrAddr(c.LocalAddr()))
		err = fmt.Errorf("unsupported SOCKS command %d", b)
		return
	}
	// RSV 0x0
	_, err = io.ReadFull(c, b)
	if err != nil {
		return
	}
	// address type
	_, err = io.ReadFull(c, b)
	if err != nil {
		return
	}
	var addr, nulladdr []byte
	switch b[0] {
	case ADDR_IPV4:
		// 4 bytes long
		nulladdr = make([]byte, net.IPv4len)
		addr = make([]byte, net.IPv4len)
		_, err = io.ReadFull(c, addr)
		if err != nil {
			return
		}
		address = net.IP(addr).String()
	case ADDR_IPV6:
		// 16 bytes long
		nulladdr = make([]byte, 16)
		addr = make([]byte, 16)
		_, err = io.ReadFull(c, addr)
		if err != nil {
			return
		}
		address = net.IP(addr).String()
	case ADDR_FQDN:
		// fqdn length in bytes
		_, err = io.ReadFull(c, b)
		if err != nil {
			return
		}
		// fqdn
		addr = make([]byte, b[0])
		_, err = io.ReadFull(c, addr)
		if err != nil {
			return
		}
		address = string(addr)
	}

	// port number
	portb := make([]byte, 2)
	_, err = io.ReadFull(c, portb)
	if err != nil {
		return
	}
	port := int(portb[0])<<8 | int(portb[1])
	if cmd == UDP_ASSOC && bytes.Equal(addr, nulladdr) && port == 0 {
		address = ""
	} else {
		address = net.JoinHostPort(address, strconv.Itoa(port))
	}
	return
}
