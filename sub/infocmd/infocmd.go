// Copyright (c) 2021 Wireleap

package infocmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/accesskey"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("info", flag.ExitOnError)

	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Display some info and stats",
		Run:     func(fm fsdir.T) { Run(fm) },
	}

	return r
}

type output struct {
	WireleapState string  `json:"wireleap_state,omitempty"`
	WireleapPid   int     `json:"wireleap_pid,omitempty"`
	WireleapHome  string  `json:"wireleap_home,omitempty"`
	WireleapSocks *string `json:"wireleap_socks,omitempty"`
	AKAvailable   int64   `json:"accesskeys_available"`
	SKSeconds     int64   `json:"sk_seconds"`
}

func Run(fm fsdir.T) {
	var (
		sk   *servicekey.T
		left int64
		err  error
	)

	err = fm.Get(&sk, filenames.Servicekey)

	if err == nil {
		left = sk.Contract.SettlementOpen - time.Now().Unix()

		if left < 0 {
			left = 0
		}
	} else {
		// this is fine, just display 0
		left = 0
	}

	var c *clientcfg.C
	err = fm.Get(&c, filenames.Config)

	if err != nil {
		log.Fatal(err)
	}

	var (
		pid   int
		state string
	)

	err = fm.Get(&pid, filenames.Pid)

	if err != nil {
		pid = 0
		state = "not_running"
	} else {
		if process.Exists(pid) {
			state = "running"
		} else {
			state = "not_running"
		}
	}

	var aks []*accesskey.T
	err = fm.Get(&aks, filenames.Pofs)

	if err != nil {
		// this is fine, just display 0
	}

	b, err := json.MarshalIndent(output{
		WireleapState: state,
		WireleapPid:   pid,
		WireleapHome:  fm.Path(),
		WireleapSocks: &c.Forwarders.Socks.Address,
		AKAvailable:   int64(len(aks)),
		SKSeconds:     left,
	}, "", "    ")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(b))
}
