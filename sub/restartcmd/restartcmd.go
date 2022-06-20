// Copyright (c) 2022 Wireleap

package restartcmd

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

func Cmd(arg0 string, start func(fsdir.T), stop func(fsdir.T)) *cli.Subcmd {
	return &cli.Subcmd{
		FlagSet: flag.NewFlagSet("restart", flag.ExitOnError),
		Desc:    fmt.Sprintf("Restart %s controller daemon", arg0),
		Run: func(fm fsdir.T) {
			var pid int
			if fm.Get(&pid, arg0+".pid") == nil {
				stop(fm)
				i := 0
				for ; i < 10 && process.Exists(pid); i++ {
					time.Sleep(500 * time.Millisecond)
				}
				if i == 10 {
					log.Fatalf("timed out waiting for %s to stop", arg0)
				}
			}
			start(fm)
		},
	}
}
