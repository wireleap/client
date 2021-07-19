// Copyright (c) 2021 Wireleap

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/wireleap/client/wireleap_tun/ptable"
	"github.com/wireleap/client/wireleap_tun/tun"
	"github.com/wireleap/common/wlnet/h2conn"
	"golang.org/x/net/http2"
)

var pt = &ptable.T{}
var DEBUG = false

// spliceconn copies one accepted TCP connection's i/o to the stored connection
// for this port table entry.
func spliceconn(c net.Conn) {
	defer c.Close()
	p := c.RemoteAddr().(*net.TCPAddr).Port
	pe := pt.Get(ptable.TCP, p)
	if pe == nil {
		if DEBUG {
			log.Printf("no destination known for source port %d, ignoring", p)
		}
		return
	}
	// wait on available connection
	pe.Lock()
	defer pe.Unlock()
	if pe.Conn == nil {
		if DEBUG {
			log.Printf("no connection found for source port %d, ignoring", p)
		}
		return
	}
	sync := make(chan error)
	go func() { _, err := io.Copy(c, pe.Conn); sync <- err }()
	go func() { _, err := io.Copy(pe.Conn, c); sync <- err }()
	e := <-sync // wait until EOF or error
	c.Close()   // clean up
	pe.Conn.Close()
	<-sync // ignore 2nd error, it's caused by close
	if DEBUG {
		log.Println("tcp splice terminated, error =", e)
	}
}

// tcpfwd mediates between routed raw packets on the tun device and TCP
// connections to wireleap.
func tcpfwd(l *net.TCPListener) {
	pause := 1 * time.Second // avoid spam if ulimit is exhausted
	for {
		c, err := l.AcceptTCP()
		if err != nil {
			log.Printf("%s tcp accept failed: %s, pausing for %s", l.Addr(), err, pause)
			time.Sleep(pause)
			continue
		}
		// clean up connections immediately after they're done with
		c.SetLinger(0)
		c.SetKeepAlive(false)
		go spliceconn(c)
	}
}

// copyip copies an IP.
func copyip(i1 net.IP) (i2 net.IP) {
	i2 = make([]byte, len(i1))
	copy(i2, i1)
	return
}

// nextip returns a new IP from the passed one with the last octet incremented
// by 1. Normally, this should be its /31 "neighbor".
func nextip(i1 net.IP) (i2 net.IP) {
	i2 = copyip(i1)
	i2[len(i2)-1]++
	return
}

// tunsplice reads packets on the tun device and forwards them to wireleap in
// appropriate form.
func tunsplice(t *tun.T, h2caddr, tunaddr string) error {
	var (
		buf      = gopacket.NewSerializeBuffer()
		opts     = gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
		if4, if6 *net.TCPAddr
	)
	log.Printf("capturing packets from %s and proxying via h2c://%s", t.Name(), h2caddr)
	h2caddr = "http://" + h2caddr
	// setup addresses of tunside tcp forwarder
	addrs, err := t.NetIf.Addrs()
	if err != nil {
		return err
	}
	if len(addrs) > 2 {
		return fmt.Errorf("interface %s has more addresses than required, misconfiguration?", t.Name())
	}
	_, tunportstr, err := net.SplitHostPort(tunaddr)
	if err != nil {
		return fmt.Errorf("could not parse tunaddr `%s`: %s", tunaddr, err)
	}
	tunport, err := strconv.Atoi(tunportstr)
	if err != nil {
		return fmt.Errorf("could not parse tunaddr port `%s`: %s", tunportstr, err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.To4() == nil {
				if6 = &net.TCPAddr{IP: ipnet.IP, Port: tunport, Zone: t.Name()}
			} else {
				if4 = &net.TCPAddr{IP: ipnet.IP, Port: tunport, Zone: t.Name()}
			}
		}
	}
	l4, err := net.ListenTCP("tcp4", if4)
	if err != nil {
		return fmt.Errorf("could not listen v4 on %s: %s", if4, err)
	}
	log.Printf("listening on tcp4 socket %s", l4.Addr())
	l6, err := net.ListenTCP("tcp6", if6)
	if err != nil {
		return fmt.Errorf("could not listen v6 on %s: %s", if6, err)
	}
	log.Printf("listening on tcp6 socket %s", l6.Addr())
	// h2c-enabled transport
	tt := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	go tcpfwd(l4)
	go tcpfwd(l6)
	r := tun.NewReader(t)
	go func() {
		var (
			ip4     layers.IPv4
			ip6     layers.IPv6
			tcp     layers.TCP
			udp     layers.UDP
			v4p     = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4, &tcp, &udp)
			v6p     = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv6, &ip6, &tcp, &udp)
			decoded = make([]gopacket.LayerType, 0, 3)

			ipl interface {
				gopacket.NetworkLayer
				gopacket.SerializableLayer
			}
			trl interface {
				gopacket.TransportLayer
				gopacket.SerializableLayer
			}
			tunaddr      *net.TCPAddr
			srcip, dstip *net.IP
		)
		v4p.DecodingLayerParserOptions.IgnoreUnsupported = true
		v6p.DecodingLayerParserOptions.IgnoreUnsupported = true
		for {
			data := r.Recv()
			switch data[0] >> 4 {
			case 4:
				err = v4p.DecodeLayers(data, &decoded)
			case 6:
				err = v6p.DecodeLayers(data, &decoded)
			}
			if err != nil {
				log.Println("error while decoding packet:", err)
				continue
			}
			if len(decoded) != 2 {
				continue
			}
			for _, typ := range decoded {
				switch typ {
				case layers.LayerTypeIPv4:
					tunaddr, ipl, srcip, dstip = if4, &ip4, &ip4.SrcIP, &ip4.DstIP
				case layers.LayerTypeIPv6:
					tunaddr, ipl, srcip, dstip = if6, &ip6, &ip6.SrcIP, &ip6.DstIP
				case layers.LayerTypeTCP:
					trl = &tcp
					tcp.SetNetworkLayerForChecksum(ipl)

					if !srcip.Equal(tunaddr.IP) {
						// not interested
						continue
					}
					if tcp.SrcPort == layers.TCPPort(tunaddr.Port) {
						// packet from tcp socket to virtual nexthop
						if nat := pt.Get(ptable.TCP, int(tcp.DstPort)); nat != nil {
							// redirect to client
							var (
								newsrc = copyip(nat.DstIP)
								newdst = copyip(tunaddr.IP)
							)
							if (newsrc.To4() == nil) != (newdst.To4() == nil) {
								log.Printf(
									"IP family mismatch after NAT: nat entry %+v old src %s:%d new src %s:%d but old dst %s:%d new dst %s:%d",
									nat,
									srcip, tcp.SrcPort,
									newsrc, nat.DstPort,
									dstip, tcp.DstPort,
									newdst, tcp.DstPort,
								)
							}
							*srcip, *dstip, tcp.SrcPort = newsrc, newdst, layers.TCPPort(nat.DstPort)
							if tcp.FIN || tcp.RST {
								// clean up finished connection
								pt.Del(ptable.TCP, int(tcp.DstPort))
							}
						} else {
							continue
						}
					} else {
						// original packet from client to destination
						// redirect to tcp socket with spoofed nexthop srcaddr
						natport := int(tcp.SrcPort)
						if nat := pt.Get(ptable.TCP, natport); nat == nil {
							nat = &ptable.Entry{
								SrcIP:   copyip(*srcip),
								DstIP:   copyip(*dstip),
								SrcPort: natport,
								DstPort: int(tcp.DstPort),
							}
							pt.Set(ptable.TCP, natport, nat)
							nat.Lock()
							dstaddr := net.JoinHostPort(
								ipl.NetworkFlow().Dst().String(),
								trl.TransportFlow().Dst().String(),
							)
							go func() {
								defer nat.Unlock()
								c, err := h2conn.New(tt, h2caddr, map[string]string{
									"Wl-Dial-Protocol": "tcp",
									"Wl-Dial-Target":   dstaddr,
								})
								if err != nil {
									pt.Del(ptable.TCP, natport)
									log.Printf("error wireleap-dialing %s: %s", dstaddr, err)
									return
								}
								nat.Conn = c
							}()
						}
						*srcip = nextip(tunaddr.IP)
						*dstip = copyip(tunaddr.IP)
						tcp.DstPort = layers.TCPPort(tunaddr.Port)
					}
					err = gopacket.SerializeLayers(buf, opts, ipl, trl, gopacket.Payload(tcp.Payload))
					if err != nil {
						log.Printf("could not serialize tcp: %s %+v %+v", err, srcip, dstip)
						continue
					}
					_, err = t.Write(buf.Bytes())
					if err != nil {
						log.Printf("could not write tcp packet to tun: %s %+v %+v", err, srcip, dstip)
						continue
					}
				case layers.LayerTypeUDP:
					trl = &udp
					udp.SetNetworkLayerForChecksum(ipl)
					natport := int(udp.SrcPort)
					nat := pt.Get(ptable.UDP, natport)
					if nat == nil {
						nat = &ptable.Entry{
							SrcIP:   copyip(*srcip),
							DstIP:   copyip(*dstip),
							SrcPort: natport,
							DstPort: int(udp.DstPort),
						}
						pt.Set(ptable.UDP, natport, nat)
						// lock while establishing connection
						nat.Lock()
						// copy stored variables
						srcip, dstip, srcport, dstport := copyip(*srcip), copyip(*dstip), udp.SrcPort, udp.DstPort
						data := make([]byte, len(udp.Payload))
						copy(data, udp.Payload)
						go func() {
							defer pt.Del(ptable.UDP, natport)
							dstaddr := net.JoinHostPort(
								ipl.NetworkFlow().Dst().String(),
								trl.TransportFlow().Dst().String(),
							)
							c, err := h2conn.New(tt, h2caddr, map[string]string{
								"Wl-Dial-Protocol": "udp",
								"Wl-Dial-Target":   dstaddr,
							})
							if err != nil {
								log.Printf("error udp wireleap-dialing %s: %s", dstaddr, err)
								nat.Unlock()
								return
							}
							nat.Conn = c
							nat.Unlock()
							_, err = c.Write(data)
							if err != nil {
								log.Printf("error udp writing to %s: %s", dstaddr, err)
								return
							}
							var (
								nl interface {
									gopacket.NetworkLayer
									gopacket.SerializableLayer
								}
								sbuf = gopacket.NewSerializeBuffer()
								rbuf = make([]byte, 4096) // is this enough?
								v4l  = layers.IPv4{Version: 4, Protocol: layers.IPProtocolUDP}
								v6l  = layers.IPv6{Version: 6, NextHeader: layers.IPProtocolUDP}
								udp  = layers.UDP{}
							)
							for {
								nat.Conn.SetDeadline(time.Now().Add(time.Second * 5))
								n, err := nat.Conn.Read(rbuf)
								if err != nil {
									return
								}
								if ip4 := srcip.To4(); ip4 != nil {
									// v4
									v4l.SrcIP, v4l.DstIP, nl = dstip, ip4, &v4l
								} else {
									// v6
									v6l.SrcIP, v6l.DstIP, nl = dstip, srcip, &v6l
								}
								udp.SrcPort = layers.UDPPort(dstport)
								udp.DstPort = layers.UDPPort(srcport)
								udp.SetNetworkLayerForChecksum(nl)
								err = gopacket.SerializeLayers(sbuf, opts, nl, &udp, gopacket.Payload(rbuf[:n]))
								if err != nil {
									log.Printf("could not serialize udp: %s %+v %+v", err, v4l, v6l)
									return
								}
								_, err = t.Write(sbuf.Bytes())
								if err != nil {
									log.Printf("could not write udp packet: %s %+v %+v", err, v4l, v6l)
									return
								}
							}
						}()
					}
					nat.Lock()
					nat.Unlock()
					if nat.Conn != nil {
						nat.Conn.SetDeadline(time.Now().Add(time.Second * 5))
						_, err = nat.Conn.Write(data)
						if err != nil {
							log.Printf("error udp writing to %s: %s", *dstip, err)
							return
						}
					}
				}
			}
		}
	}()
	return nil
}
