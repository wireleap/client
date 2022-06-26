// Copyright (c) 2022 Wireleap

package statuscmd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

func Cmd(arg0 string) *cli.Subcmd {
	r := &cli.Subcmd{
		FlagSet: flag.NewFlagSet("status", flag.ExitOnError),
		Desc:    fmt.Sprintf("Report %s controller daemon status", arg0),
		Sections: []cli.Section{{
			Title: "Exit codes",
			Entries: []cli.Entry{
				{
					Key:   "0",
					Value: fmt.Sprintf("%s controller is active", arg0),
				},
				{
					Key:   "1",
					Value: fmt.Sprintf("%s controller is inactive", arg0),
				},
				{
					Key:   "2",
					Value: fmt.Sprintf("%s controller is activating or deactivating", arg0),
				},
				{
					Key:   "3",
					Value: fmt.Sprintf("%s controller failed or state is unknown", arg0),
				},
			},
		}},
		Run: func(fm fsdir.T) {
			var (
				o = restapi.StatusReply{
					Home:  fm.Path(),
					Pid:   -1,
					State: "unknown",
				}
				exit = 3
			)
			// set both state string & exit code
			setState := func(s string) {
				switch s {
				case "active":
					exit = 0
				case "inactive":
					exit = 1
				case "activating", "deactivating":
					exit = 2
				case "failed", "unknown":
					exit = 3
				default:
					panic(fmt.Errorf("unknown state: %s", s))
				}
				o.State = s
			}
			if err := fm.Get(&o.Pid, arg0+".pid"); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					setState("inactive")
				} else {
					setState("unknown")
				}
			} else {
				if process.Exists(o.Pid) {
					c := clientcfg.Defaults()
					err := fm.Get(&c, filenames.Config)
					if err != nil {
						log.Fatalf("could not read config: %s", err)
					}
					clientlib.APICallOrDie(http.MethodGet, "http://"+*c.Address+"/api/status", nil, &o)
					setState(o.State)
					return
				} else {
					// pidfile was not cleaned up ...
					setState("failed")
				}
			}
			clientlib.JSONOrDie(os.Stdout, o)
			os.Exit(exit)
		},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 5, ' ', 0)
	return r
}
