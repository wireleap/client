// Copyright (c) 2022 Wireleap

package sockscmd

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

const Available = true

const name = "socks"

const bin = "wireleap_" + name

func Cmd() (r *cli.Subcmd) {
	r = &cli.Subcmd{
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError),
		Desc:    "Control SOCKSv5 proxy forwarder",
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
	force := r.FlagSet.Bool("force", false, "Force shutdown (when using `stop`)")
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
			st   restapi.FwderReply
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
			if args := r.FlagSet.Args(); *force || args[len(args)-1] == "--force" {
				pidfile := bin + ".pid"
				var pid int
				if err := fm.Get(&pid, pidfile); err != nil {
					log.Fatalf("could not read %s pidfile %s: %s", bin, pidfile, err)
				}
				process.Term(pid)
				time.Sleep(500 * time.Millisecond)
				process.Kill(pid)
				log.Printf("successfully killed %s, pid %d", bin, pid)
				return
			}
			meth = http.MethodPost
			url += "/stop"
		case "restart":
			meth = http.MethodPost
			url += "/stop"
			// specially handled below
		case "log":
			url += "/log"
			req, err := cl.NewRequest(meth, url, nil)
			if err != nil {
				log.Fatalf("could not create request to %s: %s", url, err)
			}
			res, err := cl.PerformRequestNoParse(req)
			if err != nil {
				log.Fatalf("could not perform request to %s: %s", url, err)
			}
			b, err := io.ReadAll(res.Body)
			if err != nil {
				log.Fatalf("could not read %s request body: %s", url, err)
			}
			os.Stdout.Write(b)
			return
		default:
			log.Fatalf("unknown %s subcommand: %s", name, cmd)
		}
		clientlib.APICallOrDie(meth, url, nil, &st)
		switch cmd {
		case "restart":
			url = "http://" + *c.Address + "/api/forwarders/" + name + "/start"
			clientlib.APICallOrDie(meth, url, nil, &st)
		case "status":
			switch st.State {
			case "failed", "inactive", "unknown":
				os.Exit(1)
			}
		}
	}
	return
}
