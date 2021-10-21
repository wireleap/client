// Copyright (c) 2021 Wireleap

package netsetup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path"
)

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
