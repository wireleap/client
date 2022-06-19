// Copyright (c) 2021 Wireleap

package importcmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/api/texturl"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Import accesskeys JSON and set up associated contract",
		Sections: []cli.Section{{
			Title: "Arguments",
			Entries: []cli.Entry{
				{Key: "URL", Value: "URL to download accesskeys (scheme: https or file)"},
			},
		}},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 8, ' ', 0)
	r.Run = func(fm fsdir.T) {
		if fs.NArg() != 1 {
			r.Usage()
			os.Exit(1)
		}
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		u, err := url.Parse(fs.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		cl := client.New(nil)
		var o json.RawMessage
		err = cl.Perform(
			http.MethodPost,
			"http://"+*c.Address+"/api/accesskeys/import",
			restapi.AccesskeyImportRequest{
				URL: &texturl.URL{*u},
			},
			&o,
		)
		if err != nil {
			st := &status.T{}
			if errors.As(err, &st) {
				fmt.Println(st)
				return
			} else {
				log.Printf("error while executing request: %s", err)
			}
		} else {
			fmt.Println(string(o))
		}
	}
	r.SetMinimalUsage("URL")
	return r
}
