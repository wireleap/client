// Copyright (c) 2021 Wireleap

package startcmd

import (
	"fmt"
	"syscall"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/commonsub/startcmd"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

func Cmd() *cli.Subcmd {
	run := func(fm fsdir.T) {
		brok := broker.New(fm)
		defer brok.Shutdown()

		cli.SignalLoop(cli.SignalMap{
			process.ReloadSignal: brok.Reload,
			syscall.SIGINT:       brok.Shutdown,
			syscall.SIGTERM:      brok.Shutdown,
			syscall.SIGQUIT:      brok.Shutdown,
		})

	}
	r := startcmd.Cmd("wireleap", run)
	r.Desc = fmt.Sprintf("%s %s", r.Desc, "(SOCKSv5/connection broker)")
	r.Sections = []cli.Section{
		{
			Title: "Signals",
			Entries: []cli.Entry{
				{
					Key:   "SIGUSR1\t(10)",
					Value: "Reload configuration, contract information and circuit",
				},
				{
					Key:   "SIGTERM\t(15)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
				{
					Key:   "SIGQUIT\t(3)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
				{
					Key:   "SIGINT\t(2)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
			},
		},
		{
			Title: "Environment",
			Entries: []cli.Entry{{
				Key:   "WIRELEAP_TARGET_PROTOCOL",
				Value: "Resolve target IP via tcp4, tcp6 or tcp (default)",
			}},
		},
	}
	return r
}
