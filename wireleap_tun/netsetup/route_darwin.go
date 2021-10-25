// Copyright (c) 2021 Wireleap

package netsetup

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"

	"github.com/wireleap/client/wireleap_tun/tun"
	"golang.org/x/net/route"
)

func sockwrite(rms []route.RouteMessage) error {
	fd, err := syscall.Socket(syscall.AF_ROUTE, syscall.SOCK_RAW, syscall.AF_UNSPEC)
	if err != nil {
		return fmt.Errorf("could not create raw routing socket: %s", err)
	}
	defer syscall.Close(fd)
	for _, rm := range rms {
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
			Flags:   syscall.RTF_STATIC | syscall.RTF_UP | syscall.RTF_GATEWAY,
			ID:      uintptr(os.Getpid()),
			Addrs:   addrs,
		})
	}
	return
}

// mkroutes returns the routes we need for wireleap to function (contract,
// directory, fronting relay).
// NOTE: routes returned by filter can be duplicate. therefore, when iterating
// do not add but replace
func mkroutes(ips []net.IP) (routes []route.RouteMessage, err error) {
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.To4() == nil {
			// don't need routes for these... TODO FIXME ipv6
			continue
		}
		// get default route(s)
		rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
		if err != nil {
			log.Fatal(err)
		}
		msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
		if err != nil {
			log.Fatal(err)
		}
		var gwaddr route.Addr
		var dst, gw net.IP
		for _, m := range msgs {
			switch m := m.(type) {
			case *route.RouteMessage:
				switch a := m.Addrs[syscall.RTAX_DST].(type) {
				case *route.Inet4Addr:
					dst = net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3])
				}

				if !dst.Equal(net.IPv4zero) {
					// not default route
					continue
				}

				switch a := m.Addrs[syscall.RTAX_GATEWAY].(type) {
				case *route.Inet4Addr:
					gwaddr = a
					gw = net.IPv4(a.IP[0], a.IP[1], a.IP[2], a.IP[3])
				default:
					continue
				}

				log.Printf("found default v4 route dst = %s, gw = %s", dst, gw)
				break
			}
		}
		if gwaddr == nil || dst == nil || gw == nil {
			return nil, fmt.Errorf("could not obtain default route")
		}
		// route bypass ips as default route using default gateway
		var addrs [][]route.Addr
		for _, ip := range ips {
			ip4 := ip.To4()
			if ip4 == nil {
				// TODO FIXME ipv6?
				continue
			}
			addrs = append(addrs, []route.Addr{
				syscall.RTAX_DST:     &route.Inet4Addr{IP: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]}},
				syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{255, 255, 255, 255}},
				syscall.RTAX_GATEWAY: gwaddr,
			})
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
	return sockwrite(mkrms(syscall.RTM_ADD, [][]route.Addr{
		{
			// lower half of all ipv4 addresses
			syscall.RTAX_DST:     &route.Inet4Addr{IP: [4]byte{}},       // 0.0.0.0
			syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{128}},    // /1
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: t.NetIf.Index}, // via utunX
		},
		{
			// upper half of all ipv4 addresses
			syscall.RTAX_DST:     &route.Inet4Addr{IP: [4]byte{128}},    // 128.0.0.0
			syscall.RTAX_NETMASK: &route.Inet4Addr{IP: [4]byte{128}},    // /1
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: t.NetIf.Index}, // via utunX
		},
		{
			// global-adressable ipv6
			syscall.RTAX_DST:     &route.Inet6Addr{IP: [16]byte{32}},    // 2000::
			syscall.RTAX_NETMASK: &route.Inet6Addr{IP: [16]byte{224}},   // /3
			syscall.RTAX_GATEWAY: &route.LinkAddr{Index: t.NetIf.Index}, // via utunX
		},
	}))
}

type darwinRoutes struct{ rts []route.RouteMessage }

func RoutesUp(sh string) (Routes, error) {
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
	for _, rt := range t.rts {
		// mutate in place, this struct is being discarded anyway
		rt.Type = syscall.RTM_DELETE
	}
	return sockwrite(t.rts)
}
