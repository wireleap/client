// Copyright (c) 2021 Wireleap

package servicekeycmd

import (
	"flag"
	"log"
	"net/http"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("servicekey", flag.ExitOnError)

	run := func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		cl := client.New(nil)
		var st status.T
		cl.Perform(
			http.MethodPost,
			"http://"+*c.Address+"/api/accesskeys/activate",
			nil,
			&st,
		)
		log.Println(st)
	}

	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Trigger accesskey activation (accesskey.use_on_demand=false)",
		Run:     run,
	}

	r.SetMinimalUsage("")

	return r
}
