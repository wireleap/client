// Copyright (c) 2022 Wireleap

package versioncmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/wireleap/client/clientlib"
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
				clientlib.JSONOrDie(os.Stdout, restapi.RuntimeReply)
			} else {
				fmt.Println(out)
			}
		},
	}
}
