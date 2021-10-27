// Copyright (c) 2021 Wireleap

package netsetup

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/wireleap/client/wireleap_tun/tun"
	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

func sockwrite(rms []route.RouteMessage) error {
	fd, err := syscall.Socket(syscall.AF_ROUTE, syscall.SOCK_RAW, syscall.AF_UNSPEC)
	if err != nil {
		return fmt.Errorf("could not create raw routing socket: %s", err)
	}
	defer syscall.Close(fd)
	for _, rm := range rms {
		// generate human-readable debug output
		o := []string{"writing route message:"}
		switch rm.Type {
		case syscall.RTM_ADD:
			o = append(o, "add")
		case syscall.RTM_DELETE:
			o = append(o, "delete")
		}
		if aa := rm.Addrs[syscall.RTAX_DST]; aa != nil {
			o = append(o, "dst")
			switch a := aa.(type) {
			case *route.Inet4Addr:
				o = append(o, "v4", net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3]).String())
			case *route.Inet6Addr:
				o = append(o, "v6", net.IP(a.IP[:]).String())
			case *route.LinkAddr:
				o = append(o, "link#"+strconv.Itoa(a.Index))
			default:
				o = append(o, "<weirdness>")
			}
		}
		if aa := rm.Addrs[syscall.RTAX_GATEWAY]; aa != nil {
			o = append(o, "gw")
			switch a := aa.(type) {
			case *route.Inet4Addr:
				o = append(o, "v4", net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3]).String())
			case *route.Inet6Addr:
				o = append(o, "v6", net.IP(a.IP[:]).String())
			case *route.LinkAddr:
				o = append(o, "link#"+strconv.Itoa(a.Index))
			default:
				o = append(o, "<weirdness>")
			}
		}
		log.Println(o)

		b, err := rm.Marshal()
		if err != nil {
			return fmt.Errorf("could not marshal routemessage %+v: %s", rm, err)
		}
		n, err := syscall.Write(fd, b)
		// if route being added already exists or route being deleted is already gone
		if errors.Is(err, syscall.EEXIST) || errors.Is(err, syscall.ENOENT) {
			// do nothing
		} else {
			// only log failures here for now for debugging
			log.Printf("%d bytes written to AF_ROUTE, error is %s", n, err)
		}
	}
	return nil
}

func mkrms(t int, rts [][]route.Addr) (r []route.RouteMessage) {
	for i, addrs := range rts {
		r = append(r, route.RouteMessage{
			Version: syscall.RTM_VERSION,
			Seq:     i,
			Type:    t,
			Flags: syscall.RTF_STATIC |
				syscall.RTF_UP |
				syscall.RTF_GATEWAY |
				unix.RTF_GLOBAL,
			ID:    uintptr(os.Getpid()),
			Addrs: addrs,
		})
	}
	return
}

func getgws() (gw4 route.Addr, gw6 route.Addr, err error) {
	// get default route(s)
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, nil, err
	}
	for _, m := range msgs {
		switch m := m.(type) {
		case *route.RouteMessage:
			// looking for a destination of all zeroes (default route sign)
			switch a := m.Addrs[syscall.RTAX_DST].(type) {
			case *route.Inet4Addr:
				if !net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3]).Equal(net.IPv4zero) {
					// not default route
					continue
				}
			case *route.Inet6Addr:
				if !net.IP(a.IP[:]).Equal(net.IPv6zero) {
					// not default route
					continue
				}
			default:
				continue
			}

			// getting the gateway to use for v4/v6 bypass routes
			switch a := m.Addrs[syscall.RTAX_GATEWAY].(type) {
			case *route.Inet4Addr:
				if gw4 != nil {
					// already have one
					continue
				}
				if ip := net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3]); ip.IsLinkLocalUnicast() {
					// skip link-local gateways present on darwin
					continue
				} else {
					log.Printf("found default v4 route, gateway %s", ip)
				}
				gw4 = a
			case *route.Inet6Addr:
				if gw6 != nil {
					// already have one
					continue
				}
				if ip := net.IP(a.IP[:]); ip.IsLinkLocalUnicast() {
					// skip link-local gateways present on darwin
					continue
				} else {
					log.Printf("found default v6 route, gateway %s", ip)
				}
				gw6 = a
			default:
				continue
			}
		}
	}
	if gw4 == nil && gw6 == nil {
		return nil, nil, fmt.Errorf("could not obtain any default v4/v6 gateways")
	}
	return
}

// mkroutes returns the routes we need for wireleap to function (contract,
// directory, fronting relay).
func mkroutes(ips []net.IP) (routes []route.RouteMessage, err error) {
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsUnspecified() {
			// don't need routes for these...
			continue
		}
		gw4, gw6, err := getgws()
		if err != nil {
			return nil, err
		}
		// route bypass ips as default route using default gateway
		var addrs [][]route.Addr
		for _, ip := range ips {
			if ip4 := ip.To4(); ip4 != nil && gw4 != nil {
				// v4
				addrs = append(addrs, []route.Addr{
					syscall.RTAX_DST:     &route.Inet4Addr{IP: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]}},
					syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{255, 255, 255, 255}},
					syscall.RTAX_GATEWAY: gw4,
				})
			} else if gw6 != nil {
				// v6
				// TODO go 1.17:
				// use https://tip.golang.org/ref/spec#Conversions_from_slice_to_array_pointer
				ip6 := [16]byte{}
				copy(ip6[:], ip)
				addrs = append(addrs, []route.Addr{
					syscall.RTAX_DST:     &route.Inet6Addr{IP: ip6},
					syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{255, 255, 255, 255}},
					syscall.RTAX_GATEWAY: gw6,
				})
			}
		}
		routes = mkrms(syscall.RTM_ADD, addrs)
	}
	return
}

func Init(t *tun.T, tunaddr string) error {
	tunhost, _, err := net.SplitHostPort(tunaddr)
	if err != nil {
		return fmt.Errorf("could not parse WIRELEAP_ADDR_TUN `%s`: %s", tunaddr, err)
	}
	// FIXME unhardcode 2nd peer address
	if err = exec.Command("ifconfig", t.Name(), tunhost, "10.13.49.1", "netmask", "0xffffffff").Run(); err != nil {
		return fmt.Errorf("tun device %s configuration failed: %s", t.Name(), err)
	}
	gw4, gw6, err := getgws()
	if err != nil {
		return err
	}
	var addrs [][]route.Addr
	if gw4 != nil {
		addrs = append(addrs, []route.Addr{
			// lower half of all ipv4 addresses
			syscall.RTAX_DST:     &route.Inet4Addr{IP: [4]byte{}},       // 0.0.0.0
			syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{128}},    // /1
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: t.NetIf.Index}, // via utunX
		}, []route.Addr{
			// upper half of all ipv4 addresses
			syscall.RTAX_DST:     &route.Inet4Addr{IP: [4]byte{128}},    // 128.0.0.0
			syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{128}},    // /1
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: t.NetIf.Index}, // via utunX
		})
	}
	if gw6 != nil {
		addrs = append(addrs, []route.Addr{
			// global-adressable ipv6
			syscall.RTAX_DST:     &route.Inet6Addr{IP: [16]byte{32}},    // 2000::
			syscall.RTAX_NETMASK: &route.Inet6Addr{IP: [16]byte{224}},   // /3
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: t.NetIf.Index}, // via utunX
		})
	}
	return sockwrite(mkrms(syscall.RTM_ADD, addrs))
}

type darwinRoutes struct{ rts []route.RouteMessage }

func RoutesUp(sh string) (Routes, error) {
	log.Printf("bringing up bypass routes...")
	ips, err := ReadBypass(sh)
	if err != nil {
		return nil, fmt.Errorf("could not read bypass file: %s", err)
	}
	bypassrts, err := mkroutes(ips)
	if err != nil {
		return nil, fmt.Errorf("could not create routes to bypass IPs: %s", err)
	}
	if err = sockwrite(bypassrts); err != nil {
		return nil, fmt.Errorf("could not setup bypass routes: %s", err)
	}
	return darwinRoutes{bypassrts}, nil
}

func (t darwinRoutes) Down() error {
	log.Printf("bringing down bypass routes...")
	for _, rt := range t.rts {
		// mutate in place, this struct is being discarded anyway
		rt.Type = syscall.RTM_DELETE
	}
	return sockwrite(t.rts)
}
