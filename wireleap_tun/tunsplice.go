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
	"github.com/wireleap/client/wireleap_tun/netsetup"
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
	sync := make(chan error)
	upstream := pe.Conn()
	go func() { _, err := io.Copy(c, upstream); sync <- err }()
	go func() { _, err := io.Copy(upstream, c); sync <- err }()
	e := <-sync // wait until EOF or error
	c.Close()   // clean up
	upstream.Close()
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

// listenDual sets up TCP listening sockets for IPv4 and IPv6 and return the
// addresses.
func listenDual(tunif *tun.T, tunaddr string) (if4, if6 *net.TCPAddr, err error) {
	// setup addresses of tunside tcp forwarder
	addrs, err := tunif.NetIf.Addrs()
	if err != nil {
		return nil, nil, err
	}
	if len(addrs) > 2 {
		err = fmt.Errorf("interface %s has more addresses than required, misconfiguration?", tunif.Name())
		return
	}
	_, tunportstr, err := net.SplitHostPort(tunaddr)
	if err != nil {
		err = fmt.Errorf("could not parse tunaddr `%s`: %s", tunaddr, err)
		return
	}
	tunport, err := strconv.Atoi(tunportstr)
	if err != nil {
		err = fmt.Errorf("could not parse tunaddr port `%s`: %s", tunportstr, err)
		return
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.To4() == nil {
				if6 = &net.TCPAddr{IP: ipnet.IP, Port: tunport, Zone: tunif.Name()}
			} else {
				if4 = &net.TCPAddr{IP: ipnet.IP, Port: tunport, Zone: tunif.Name()}
			}
		}
	}
	if if4 != nil {
		l4, err := net.ListenTCP("tcp4", if4)
		if err != nil {
			return nil, nil, fmt.Errorf("could not listen v4 on %s: %s", if4, err)
		}
		log.Printf("listening on tcp4 socket %s", l4.Addr())
		go tcpfwd(l4)
	}
	if if6 != nil {
		l6, err := net.ListenTCP("tcp6", if6)
		if err != nil {
			return nil, nil, fmt.Errorf("could not listen v6 on %s: %s", if6, err)
		}
		log.Printf("listening on tcp6 socket %s", l6.Addr())
		go tcpfwd(l6)
	}
	return
}

type dialFunc func(string, string) (*h2conn.T, error)

func mutateLoop(if4, if6 *net.TCPAddr, r *tun.Reader, w *tun.Writer, dialf dialFunc) {
	var (
		buf  = gopacket.NewSerializeBuffer()
		opts = gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

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
		tunaddr      *net.TCPAddr
		srcip, dstip *net.IP
		err          error
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
							newsrc = netsetup.CopyIP(nat.DstIP)
							newdst = netsetup.CopyIP(tunaddr.IP)
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
						dstaddr := net.JoinHostPort(
							ipl.NetworkFlow().Dst().String(),
							tcp.TransportFlow().Dst().String(),
						)
						pt.Set(ptable.TCP, natport, &ptable.Entry{
							SrcIP:   netsetup.CopyIP(*srcip),
							DstIP:   netsetup.CopyIP(*dstip),
							SrcPort: natport,
							DstPort: int(tcp.DstPort),
						}, func() (c net.Conn, err error) {
							if c, err = dialf("tcp", dstaddr); err != nil {
								log.Printf("error wireleap-dialing tcp %s: %s", dstaddr, err)
							}
							return
						})
					}
					*srcip = netsetup.NextIP(tunaddr.IP)
					*dstip = netsetup.CopyIP(tunaddr.IP)
					tcp.DstPort = layers.TCPPort(tunaddr.Port)
				}
				err = gopacket.SerializeLayers(buf, opts, ipl, &tcp, gopacket.Payload(tcp.Payload))
				if err != nil {
					log.Printf("could not serialize tcp: %s %+v %+v", err, srcip, dstip)
					continue
				}
				// copy bytes
				out := buf.Bytes()
				dup := make([]byte, len(out))
				copy(dup, out)
				w.Send(dup)
			case layers.LayerTypeUDP:
				udp.SetNetworkLayerForChecksum(ipl)
				natport := int(udp.SrcPort)
				if nat := pt.Get(ptable.UDP, natport); nat == nil {
					// copy stored variables
					srcip, dstip, srcport, dstport := netsetup.CopyIP(*srcip), netsetup.CopyIP(*dstip), udp.SrcPort, udp.DstPort
					nat = &ptable.Entry{
						SrcIP:   srcip,
						DstIP:   dstip,
						SrcPort: natport,
						DstPort: int(dstport),
					}
					dstaddr := net.JoinHostPort(
						ipl.NetworkFlow().Dst().String(),
						udp.TransportFlow().Dst().String(),
					)
					// copy payload for async usage
					p2 := make([]byte, len(udp.Payload))
					copy(p2, udp.Payload)
					pt.Set(ptable.UDP, natport, nat, func() (c net.Conn, err error) {
						if c, err = dialf("udp", dstaddr); err != nil {
							log.Printf("error wireleap-dialing udp %s: %s", dstaddr, err)
							return
						}
						go func() {
							// handle errors by cleaning up nat entry
							defer pt.Del(ptable.UDP, natport)
							if _, err := c.Write(p2); err != nil {
								log.Printf("error udp writing initial data to %s: %s", dstaddr, err)
								return
							}
							var (
								nl interface {
									gopacket.NetworkLayer
									gopacket.SerializableLayer
								}
								sbuf = gopacket.NewSerializeBuffer()
								rbuf = make([]byte, 4096) // is this enough?
								v4l  = layers.IPv4{Version: 4, Protocol: layers.IPProtocolUDP, TTL: 64}
								v6l  = layers.IPv6{Version: 6, NextHeader: layers.IPProtocolUDP, HopLimit: 64}
								udp  = layers.UDP{}
							)
							for {
								c.SetDeadline(time.Now().Add(time.Second * 10))
								n, err := c.Read(rbuf)
								if err != nil {
									if DEBUG {
										log.Printf("could not read from udp conn: %s", err)
									}
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
									if DEBUG {
										log.Printf("could not serialize udp: %s %+v %+v", err, v4l, v6l)
									}
									return
								}
								// copy bytes
								out := sbuf.Bytes()
								dup := make([]byte, len(out))
								copy(dup, out)
								w.Send(dup)
							}
						}()
						return
					})
				} else {
					c := nat.Conn()
					c.SetDeadline(time.Now().Add(time.Second * 10))
					if _, err = c.Write(udp.Payload); err != nil {
						if DEBUG {
							log.Printf("error udp writing to %s: %s", *dstip, err)
						}
						return
					}
				}
			}
		}
	}
}

// tunsplice reads packets on the tun device and forwards them to wireleap in
// appropriate form.
func tunsplice(t *tun.T, h2caddr, tunaddr string) error {
	log.Printf("capturing packets from %s and proxying via h2c://%s", t.Name(), h2caddr)
	if4, if6, err := listenDual(t, tunaddr)
	if err != nil {
		return fmt.Errorf("couldn't listen on v4/v6 tcp socket: %s", err)
	}

	h2caddr = "http://" + h2caddr
	// h2c-enabled transport
	tt := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
		ReadIdleTimeout: 10 * time.Second,
		PingTimeout:     10 * time.Second,
	}
	dialf := func(proto, addr string) (*h2conn.T, error) {
		return h2conn.New(tt, h2caddr, map[string]string{
			"Wl-Dial-Protocol": proto,
			"Wl-Dial-Target":   addr,
		})
	}
	go mutateLoop(if4, if6, tun.NewReader(t), tun.NewWriter(t), dialf)
	return nil
}
