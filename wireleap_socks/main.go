// Copyright (c) 2022 Wireleap

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/wireleap/client/socks"
	"github.com/wireleap/common/wlnet"
	"github.com/wireleap/common/wlnet/h2conn"
	"golang.org/x/net/http2"
)

const udpbufsize = 4096 // change if bigger datagrams are expected

var tt = &http2.Transport{
	AllowHTTP: true,
	DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
		return net.Dial(network, addr)
	},
	ReadIdleTimeout: 10 * time.Second,
	PingTimeout:     10 * time.Second,
}

type DialFunc func(string, string) (*h2conn.T, error)

func dialFuncTo(h2caddr string) DialFunc {
	return func(proto, addr string) (*h2conn.T, error) {
		return h2conn.New(tt, h2caddr, map[string]string{
			"Wl-Dial-Protocol": proto,
			"Wl-Dial-Target":   addr,
			"Wl-Forwarder":     "socks",
		})
	}
}

// handle everything SOCKSv5-related on the same address
func ListenSOCKS(addr string, dialer DialFunc) (err error) {
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
	go ProxyUDP(udpl, dialer)
	go ProxyTCP(tcpl, dialer, udpl.LocalAddr())
	return
}

// handle TCP socks connections
func ProxyTCP(l net.Listener, dialer DialFunc, udpaddr net.Addr) {
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
					return
				}
				socks.WriteStatus(c0, socks.StatusOK, socks.AddrAddr(c0.LocalAddr()))
				if err = wlnet.Splice(context.Background(), c0, c1, 0, 32768); err != nil {
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
func ProxyUDP(l net.PacketConn, dialer DialFunc) {
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
			dstaddr, data, err := socks.DissectUDP(ibuf[:n])
			if err != nil {
				log.Printf("SOCKSv5 failed dissecting UDP packet: %s", err)
				return
			}
			conn, err := dialer("udp", dstaddr.String())
			if err != nil {
				log.Printf(
					"error dialing udp %s->%s->%s through the circuit: %s",
					laddr, l.LocalAddr(), dstaddr, err,
				)
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
				b, err := socks.ComposeUDP(dstaddr, obuf[:n])
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

func main() {
	var ok bool
	var h2caddr, socksaddr string

	if h2caddr, ok = os.LookupEnv("WIRELEAP_ADDR_H2C"); !ok {
		log.Fatal("WIRELEAP_ADDR_H2C is not defined")
	}
	if socksaddr, ok = os.LookupEnv("WIRELEAP_ADDR_SOCKS"); !ok {
		log.Fatal("WIRELEAP_ADDR_SOCKS is not defined")
	}

	h2caddr = "http://" + h2caddr
	if err := ListenSOCKS(socksaddr, dialFuncTo(h2caddr)); err != nil {
		log.Fatalf("listening on socks5://%s failed: %s", socksaddr, err)
	}

	log.Printf("listening for SOCKSv5 connections on %s", socksaddr)
	select {}
}
