// Copyright (c) 2021 Wireleap

package clientlib

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/wireleap/client/socks"
	"github.com/wireleap/common/wlnet"
)

const udpbufsize = 4096 // change if bigger datagrams are expected

type DialFunc func(string, string) (net.Conn, error)

// handle everything SOCKSv5-related on the same address
func ListenSOCKS(addr string, dialer DialFunc, errf func(error)) (err error) {
	var udpl net.PacketConn
	var tcpl net.Listener
	udpl, err = net.ListenPacket("udp", addr)
	if err != nil {
		err = fmt.Errorf("could not listen on requested udp address %s: %w", addr, err)
		return
	}
	tcpl, err = net.Listen("tcp", addr)
	if err != nil {
		err = fmt.Errorf("could not listen on requested tcp address %s: %w", addr, err)
		return
	}
	go ProxyUDP(udpl, dialer, errf)
	go ProxyTCP(tcpl, dialer, errf, udpl.LocalAddr())
	return
}

// handle TCP socks connections
func ProxyTCP(l net.Listener, dialer DialFunc, errf func(error), udpaddr net.Addr) {
	pause := 1 * time.Second
	for {
		c0, err := l.Accept()
		if err != nil {
			log.Printf("SOCKSv5 tcp socket accept error: %s, pausing for %s", err, pause)
			time.Sleep(pause)
			continue
		}
		go func() {
			log.Printf("SOCKSv5 tcp socket accepted: %s -> %s", c0.RemoteAddr(), c0.LocalAddr())
			cmd, addr, err := socks.Handshake(c0)
			if err != nil {
				log.Printf("SOCKSv5 tcp socket handshake error: %s", err)
				c0.Close()
				return
			}
			switch cmd {
			case socks.CONNECT:
				defer c0.Close()
				c1, err := dialer("tcp", addr)
				if err != nil {
					log.Printf("error dialing tcp through the circuit: %s", err)
					socks.WriteStatus(c0, socks.StatusGeneralFailure, socks.AddrAddr(c0.LocalAddr()))
					errf(err)
					return
				}
				socks.WriteStatus(c0, socks.StatusOK, socks.AddrAddr(c0.LocalAddr()))
				if err = wlnet.Splice(c0, c1, 0, 32768); err != nil {
					log.Printf("error splicing initial connection: %s", err)
				}
			case socks.UDP_ASSOC:
				socks.WriteStatus(c0, socks.StatusOK, socks.AddrAddr(udpaddr))
			default:
				socks.WriteStatus(c0, socks.StatusCommandNotSupported, socks.AddrAddr(l.Addr()))
				c0.Close()
			}
		}()
	}
}

// handle UDP packets
func ProxyUDP(l net.PacketConn, dialer DialFunc, errf func(error)) {
	l.(*net.UDPConn).SetWriteBuffer(2147483647)
	l.(*net.UDPConn).SetReadBuffer(2147483647)
	for {
		ibuf, obuf := make([]byte, udpbufsize), make([]byte, udpbufsize)
		n, laddr, err := l.ReadFrom(ibuf)
		if err != nil {
			log.Printf("error while reading udp packet from %s: %s", laddr, err)
			continue
		}
		go func() {
			srcaddr, dstaddr, data := socks.DissectUDP(ibuf[:n])
			conn, err := dialer("udp", dstaddr.String())
			if err != nil {
				log.Printf(
					"error dialing udp %s->%s->%s through the circuit: %s",
					laddr, l.LocalAddr(), dstaddr, err,
				)
				errf(err)
				return
			}
			_, err = conn.Write(data)
			if err != nil {
				log.Printf(
					"error when writing initial data to %s->%s->%s udp tunnel: %s",
					laddr, l.LocalAddr(), dstaddr, err,
				)
				return
			}
			for {
				n, err := conn.Read(obuf)
				if err != nil {
					if err != io.EOF {
						log.Printf("error reading %s<-%s<-%s via udp: %s", laddr, l.LocalAddr(), dstaddr, err)
					}
					break
				}
				b, err := socks.ComposeUDP(srcaddr, dstaddr, obuf[:n])
				if err != nil {
					log.Printf("error writing %s<-%s<-%s via udp: %s", laddr, l.LocalAddr(), dstaddr, err)
					break
				}
				_, err = l.WriteTo(b, laddr)
				if err != nil {
					log.Printf("error writing %s<-%s<-%s via udp: %s", laddr, l.LocalAddr(), dstaddr, err)
					break
				}
			}
		}()
	}
}
