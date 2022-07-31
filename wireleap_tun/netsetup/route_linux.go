// Copyright (c) 2022 Wireleap

package netsetup

import (
	"fmt"
	"log"
	"net"

	"github.com/vishvananda/netlink"
	"github.com/wireleap/client/wireleap_tun/tun"
)

// default route filter
var filter = &netlink.Route{Dst: nil}

// mkroutes returns the routes we need for wireleap to function (contract,
// directory, fronting relay).
// NOTE: routes returned by filter can be duplicate. therefore, when iterating
// do not add but replace
func mkroutes(ips []net.IP) (routes []netlink.Route, err error) {
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.To4() == nil {
			// don't need routes for these... TODO FIXME ipv6
			continue
		}
		var tmp []netlink.Route
		// get default route(s)
		if tmp, err = netlink.RouteListFiltered(netlink.FAMILY_V4, filter, netlink.RT_FILTER_DST); err != nil {
			err = fmt.Errorf("could not get route(s) to %s: %s", ip, err)
			return
		}
		// route bypass ips as default route
		for _, r := range tmp {
			if r.Gw != nil {
				r.Dst = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
				routes = append(routes, r)
			}
		}
	}
	return
}

func Init(t *tun.T, tunaddr string) error {
	// set tun device up & add defined address
	tunhost, _, err := net.SplitHostPort(tunaddr)
	if err != nil {
		return fmt.Errorf("could not parse WIRELEAP_ADDR_TUN `%s`: %s", tunaddr, err)
	}
	addr, err := netlink.ParseAddr(tunhost + "/31")
	if err != nil {
		return fmt.Errorf("could not parse address of %s: %s", tunaddr, err)
	}
	link, err := netlink.LinkByName(t.Name())
	if err != nil {
		return fmt.Errorf("could not get link for %s: %s", t.Name(), err)
	}
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return fmt.Errorf("could not set address of %s to %s: %s", link, addr, err)
	}
	err = netlink.LinkSetTxQLen(link, 1000)
	if err != nil {
		return fmt.Errorf("could not set link txqueue length for %s to %d: %s", t.Name(), 1000, err)
	}
	err = netlink.LinkSetUp(link)
	if err != nil {
		return fmt.Errorf("could not set %s up: %s", link, err)
	}
	// avoid clobbering the default route by being just a _little_ bit more specific
	initrts := []netlink.Route{{
		// lower half of all v4 addresses
		LinkIndex: t.NetIf.Index,
		Dst:       &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(1, net.IPv4len*8)},
	}, {
		// upper half of all v4 addresses
		LinkIndex: t.NetIf.Index,
		Dst:       &net.IPNet{IP: net.IPv4(128, 0, 0, 0), Mask: net.CIDRMask(1, net.IPv4len*8)},
	}, {
		// v6 global-adressable range
		LinkIndex: t.NetIf.Index,
		Dst:       &net.IPNet{IP: net.ParseIP("2000::"), Mask: net.CIDRMask(3, net.IPv6len*8)},
	}}
	for _, rt := range initrts {
		log.Printf("adding catch-all route: %+v", rt)
		if err = netlink.RouteReplace(&rt); err != nil {
			return fmt.Errorf("could not add catch-all route to %s: %s", rt.Dst, err)
		}
		log.Printf("added catch-all route to %s via %s", rt.Dst, rt.Gw)
	}
	return nil
}

type linuxRoutes struct{ rts []netlink.Route }

func RoutesUp(ips ...net.IP) (Routes, error) {
	bypassrts, err := mkroutes(ips)
	if err != nil {
		return nil, fmt.Errorf("could not create routes to bypass IPs: %s", err)
	}
	for _, rt := range bypassrts {
		log.Printf("adding bypass route: %+v", rt)
		if err = netlink.RouteReplace(&rt); err != nil {
			return nil, fmt.Errorf("could not add bypass route to %s: %s", rt.Dst, err)
		}
		log.Printf("added bypass route to %s via %s", rt.Dst, rt.Gw)
	}
	return linuxRoutes{bypassrts}, nil
}

func (t linuxRoutes) Down() error {
	for _, rt := range t.rts {
		if err := netlink.RouteDel(&rt); err != nil {
			return fmt.Errorf("error bringing route %s via %s down: %s", rt.Dst, rt.Gw, err)
		}
	}
	return nil
}
