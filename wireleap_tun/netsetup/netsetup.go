// Copyright (c) 2021 Wireleap

package netsetup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path"
)

// CopyIP copies an IP and returns the copy.
func CopyIP(i1 net.IP) (i2 net.IP) {
	i2 = make([]byte, len(i1))
	copy(i2, i1)
	return
}

// NextIP returns a new IP from the passed one with the last octet incremented
// by 1. Normally, this should be its /31 "neighbor" if the original IP is the
// lowest /31 address.
func NextIP(i1 net.IP) (i2 net.IP) {
	i2 = CopyIP(i1)
	i2[len(i2)-1]++
	return
}

// route table storage to keep track of bypass routes
type Routes interface{ Down() error }

// helper function: read IPs from bypass file
func ReadBypass(sh string) ([]net.IP, error) {
	p := path.Join(sh, "bypass.json")
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("could not read wireleap bypass file %s: %s", p, err)
	}
	var ips []net.IP
	if err = json.Unmarshal(b, &ips); err != nil {
		return nil, fmt.Errorf("could not unmarshal wireleap bypass file %s: %s", p, err)
	}
	return ips, nil
}
