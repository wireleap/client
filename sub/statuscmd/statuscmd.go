// Copyright (c) 2022 Wireleap

package statuscmd

import (
	"encoding/json"
	"errors"
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
	"github.com/wireleap/common/cli/process"
)

func Cmd(arg0 string) *cli.Subcmd {
	r := &cli.Subcmd{
		FlagSet: flag.NewFlagSet("status", flag.ExitOnError),
		Desc:    fmt.Sprintf("Report %s daemon status", arg0),
		Sections: []cli.Section{{
			Title: "Exit codes",
			Entries: []cli.Entry{
				{
					Key:   "0",
					Value: fmt.Sprintf("%s is running", arg0),
				},
				{
					Key:   "1",
					Value: fmt.Sprintf("%s is not running", arg0),
				},
				{
					Key:   "2",
					Value: fmt.Sprintf("could not tell if %s is running or not", arg0),
				},
			},
		}},
		Run: func(fm fsdir.T) {
			var (
				pid    int
				status int
				text   string

				err = fm.Get(&pid, arg0+".pid")
			)

			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					text, status = arg0+" is not running", 1
				} else {
					text, status = fmt.Sprintf("could not read %s status", arg0), 2
				}
			} else {
				if process.Exists(pid) {
					c := clientcfg.Defaults()
					err := fm.Get(&c, filenames.Config)
					if err != nil {
						log.Fatalf("could not read config: %s", err)
					}

					cl := client.New(nil)
					var st json.RawMessage
					err = cl.Perform(
						http.MethodGet,
						"http://"+*c.Address+"/api/status",
						nil,
						&st,
					)
					if err != nil {
						log.Fatalf("could not get process status via API: %s", err)
					}
					fmt.Printf(string(st))
				} else {
					// pidfile was not cleaned up ...
					text, status = fmt.Sprintf(
						"%s is not running (might have crashed, see %s)",
						arg0,
						arg0+".log",
					), 1
				}
			}

			fmt.Println(text)
			os.Exit(status)
		},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 5, ' ', 0)
	return r
}
