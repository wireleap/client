// Copyright (c) 2022 Wireleap

package accesskeyscmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

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
	fs := flag.NewFlagSet("accesskeys", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Manage accesskeys",
		Sections: []cli.Section{{
			Title: "Commands",
			Entries: []cli.Entry{
				{Key: "list", Value: "List accesskeys"},
				{Key: "import", Value: "Import accesskeys from URL and set up associated contract"},
				{Key: "activate", Value: "Trigger accesskey activation (accesskey.use_on_demand=false)"},
			},
		}},
	}
	r.Run = func(fm fsdir.T) {
		if (fs.Arg(0) == "import" && fs.NArg() != 2) || (fs.Arg(0) != "import" && fs.NArg() != 1) {
			r.Usage()
			os.Exit(1)
		}
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		cl := client.New(nil)
		var (
			meth  = http.MethodGet
			u     = "http://" + *c.Address + "/api/accesskeys"
			param interface{}
		)
		switch fs.Arg(0) {
		case "list":
			// no changes needed to what's defined above
		case "import":
			u += "/import"
			meth = http.MethodPost
			from, err := url.Parse(fs.Arg(1))
			if err != nil {
				log.Fatal(err)
			}
			param = restapi.AccesskeyImportRequest{URL: &texturl.URL{*from}}
		case "activate":
			u += "/activate"
			meth = http.MethodPost
		default:
			log.Fatalf("unknown command %s", fs.Arg(0))
		}
		var o json.RawMessage
		err = cl.Perform(
			meth,
			u,
			param,
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
	r.SetMinimalUsage("COMMAND")
	return r
}
