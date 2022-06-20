// Copyright (c) 2022 Wireleap

package logcmd

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd(arg0 string) *cli.Subcmd {
	return &cli.Subcmd{
		FlagSet: flag.NewFlagSet("log", flag.ExitOnError),
		Desc:    fmt.Sprintf("Show %s controller daemon logs", arg0),
		Run: func(fm fsdir.T) {
			c := clientcfg.Defaults()
			err := fm.Get(&c, filenames.Config)
			if err != nil {
				log.Fatalf("could not read config: %s", err)
			}
			cl := client.New(nil)
			url := "http://" + *c.Address + "/api/log"
			req, err := cl.NewRequest(http.MethodGet, url, nil)
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
		},
	}
}
