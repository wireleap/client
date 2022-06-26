// Copyright (c) 2022 Wireleap

package initcmd

import (
	"flag"
	"log"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/sub/initcmd/embedded"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force-unpack-only", false, "Overwrite embedded files only")
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Initialize wireleap home directory",
		Run: func(fm fsdir.T) {
			if err := cli.UnpackEmbedded(embedded.FS, fm, *force); err != nil {
				log.Fatalf("error while unpacking embedded files: %s", err)
			}
			if !*force {
				if err := fm.Set(clientcfg.Defaults(), filenames.Config); err != nil {
					log.Fatalf("could not write initial config.json: %s", err)
				}
			}
		},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 6, ' ', 0)
	return r
}
