// Copyright (c) 2022 Wireleap

package versioncmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/blang/semver"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/api/interfaces"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd(swversion *semver.Version, is ...interfaces.T) *cli.Subcmd {
	out := swversion.String()
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	verbose := fs.Bool("v", false, "show verbose output")

	return &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Show version and exit",
		Run: func(_ fsdir.T) {
			if *verbose {
				b, err := json.Marshal(restapi.RuntimeReply)
				if err != nil {
					// shouldn't ever happen...
					log.Printf("%+v", restapi.RuntimeReply)
					log.Fatalf("could not marshal runtime struct: %s", err)
				}
				os.Stdout.Write(b)
			} else {
				fmt.Println(out)
			}
		},
	}
}
