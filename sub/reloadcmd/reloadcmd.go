// Copyright (c) 2022 Wireleap

package reloadcmd

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
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
			var st restapi.StatusReply
			clientlib.APICallOrDie(http.MethodPost, "http://"+*c.Address+"/api/reload", nil, &st)
		},
	}
}
