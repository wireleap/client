// Copyright (c) 2021 Wireleap

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path"

	"github.com/vishvananda/netlink"
)

var filter = &netlink.Route{Dst: nil} // default route filter

// getroutes gets the routes we need for wireleap to function (contract,
// directory, fronting relay).
// NOTE: returned routes can be duplicate. therefore, when iterating do not add
// but replace
func getroutes(sh string) (routes []netlink.Route, err error) {
	p := path.Join(sh, "bypass.json")
	b, err := ioutil.ReadFile(p)
	if err != nil {
		err = fmt.Errorf("could not read wireleap bypass file %s: %s", p, err)
		return
	}
	var ips []net.IP
	if err = json.Unmarshal(b, &ips); err != nil {
		err = fmt.Errorf("could not unmarshal wireleap bypass file %s: %s", p, err)
		return
	}
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
