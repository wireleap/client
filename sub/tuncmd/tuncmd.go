// Copyright (c) 2022 Wireleap

package tuncmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

const Available = true

const name = "tun"

const bin = "wireleap_" + name

func Cmd() (r *cli.Subcmd) {
	r = &cli.Subcmd{
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError),
		Desc:    "Control tun device",
		Sections: []cli.Section{{
			Title: "Commands",
			Entries: []cli.Entry{
				{"start", fmt.Sprintf("Start %s daemon", bin)},
				{"stop", fmt.Sprintf("Stop %s daemon", bin)},
				{"status", fmt.Sprintf("Report %s daemon status", bin)},
				{"restart", fmt.Sprintf("Restart %s daemon", bin)},
				{"log", fmt.Sprintf("Show %s logs", bin)},
			},
		}},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 7, ' ', 0)
	r.SetMinimalUsage("COMMAND [OPTIONS]")
	r.Run = func(fm fsdir.T) {
		if r.FlagSet.NArg() < 1 {
			r.Usage()
		}
		cmd := r.FlagSet.Arg(0)
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		cl := client.New(nil)
		var (
			st   json.RawMessage
			meth = http.MethodGet
			url  = "http://" + *c.Address + "/api/forwarders/" + name
		)
		switch cmd {
		case "status":
			// url defined above is usable as-is
		case "start":
			meth = http.MethodPost
			url += "/start"
		case "stop":
			meth = http.MethodPost
			url += "/stop"
		case "restart":
			meth = http.MethodPost
			url += "/start"
			// specially handled below
		case "log":
			url += "/log"
		default:
			log.Fatalf("unknown %s subcommand: %s", name, cmd)
		}
		if err = cl.Perform(meth, url, nil, &st); err == nil {
			if cmd == "restart" {
				url = "http://" + *c.Address + "/api/forwarders/" + name + "/stop"
				if err = cl.Perform(meth, url, nil, &st); err == nil {
					fmt.Println(string(st))
					return
				}
			}
			fmt.Println(string(st))
			return
		}
		fmt.Printf("error while calling %s: %s", url, err)
		os.Exit(1)
	}
	return
}
