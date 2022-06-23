// Copyright (c) 2022 Wireleap

package reloadcmd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd(arg0 string) *cli.Subcmd {
	return &cli.Subcmd{
		FlagSet: flag.NewFlagSet("reload", flag.ExitOnError),
		Desc:    fmt.Sprintf("Reload %s controller daemon configuration", arg0),
		Run: func(fm fsdir.T) {
			c := clientcfg.Defaults()
			err := fm.Get(&c, filenames.Config)
			if err != nil {
				log.Fatalf("could not read config: %s", err)
			}
			cl := client.New(nil)
			var st restapi.StatusReply
			err = cl.Perform(http.MethodPost, "http://"+*c.Address+"/api/reload", nil, &st)
			if err != nil {
				e := &status.T{}
				if errors.As(err, &e) {
					// error can be jsonized
					clientlib.JSONOrDie(os.Stdout, e)
					return
				} else {
					log.Printf("error while executing request: %s", err)
				}
			} else {
				clientlib.JSONOrDie(os.Stdout, st)
			}
		},
	}
}
