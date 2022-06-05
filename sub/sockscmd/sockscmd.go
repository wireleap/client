// Copyright (c) 2022 Wireleap

package sockscmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

const Available = true

const bin = "wireleap_socks"

func Cmd() (r *cli.Subcmd) {
	r = &cli.Subcmd{
		FlagSet: flag.NewFlagSet("socks", flag.ExitOnError),
		Desc:    "Control SOCKSv5 forwarder",
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
		var st json.RawMessage

		switch cmd {
		case "status":
			cl.Perform(http.MethodGet, "http://"+*c.Address+"/api/forwarders/socks", nil, &st)
		case "start":
			cl.Perform(http.MethodPost, "http://"+*c.Address+"/api/forwarders/socks/start", nil, &st)
		case "stop":
			cl.Perform(http.MethodPost, "http://"+*c.Address+"/api/forwarders/socks/stop", nil, &st)
		case "restart":
			cl.Perform(http.MethodPost, "http://"+*c.Address+"/api/forwarders/socks/start", nil, &st)
			log.Println(st)
			cl.Perform(http.MethodPost, "http://"+*c.Address+"/api/forwarders/socks/stop", nil, &st)
			log.Println(st)
		case "log":
			cl.Perform(http.MethodGet, "http://"+*c.Address+"/api/forwarders/socks/log", nil, &st)
		default:
			log.Fatalf("unknown socks subcommand: %s", cmd)
		}

		log.Println(string(st))
	}
	return
}
