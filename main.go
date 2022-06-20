// Copyright (c) 2021 Wireleap

package main

import (
	"os"

	"github.com/wireleap/common/api/interfaces/clientcontract"
	"github.com/wireleap/common/api/interfaces/clientdir"
	"github.com/wireleap/common/api/interfaces/clientrelay"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/commonsub/commonlib"
	"github.com/wireleap/common/cli/commonsub/migratecmd"
	"github.com/wireleap/common/cli/commonsub/rollbackcmd"
	"github.com/wireleap/common/cli/commonsub/superviseupgradecmd"
	"github.com/wireleap/common/cli/commonsub/upgradecmd"
	"github.com/wireleap/common/cli/commonsub/versioncmd"
	"github.com/wireleap/common/cli/upgrade"

	"github.com/wireleap/client/sub/accesskeyscmd"
	"github.com/wireleap/client/sub/configcmd"
	"github.com/wireleap/client/sub/execcmd"
	"github.com/wireleap/client/sub/initcmd"
	"github.com/wireleap/client/sub/interceptcmd"
	"github.com/wireleap/client/sub/logcmd"
	"github.com/wireleap/client/sub/reloadcmd"
	"github.com/wireleap/client/sub/restartcmd"
	"github.com/wireleap/client/sub/sockscmd"
	"github.com/wireleap/client/sub/startcmd"
	"github.com/wireleap/client/sub/statuscmd"
	"github.com/wireleap/client/sub/stopcmd"
	"github.com/wireleap/client/sub/tuncmd"
	"github.com/wireleap/client/version"
)

const binname = "wireleap"

func main() {
	fm := cli.Home()

	cli.CLI{
		Subcmds: []*cli.Subcmd{
			initcmd.Cmd(),
			configcmd.Cmd(fm),
			accesskeyscmd.Cmd(),
			startcmd.Cmd(binname),
			statuscmd.Cmd(binname),
			reloadcmd.Cmd(binname),
			restartcmd.Cmd(binname, startcmd.Cmd(binname).Run, stopcmd.Cmd(binname).Run),
			stopcmd.Cmd(binname),
			logcmd.Cmd(binname),
			tuncmd.Cmd(),
			sockscmd.Cmd(),
			interceptcmd.Cmd(),
			execcmd.Cmd(),
			upgradecmd.Cmd(
				binname,
				upgrade.ExecutorSupervised,
				version.VERSION,
				version.LatestChannelVersion,
			),
			rollbackcmd.Cmd(commonlib.Context{
				BinName:  binname,
				PostHook: version.PostRollbackHook,
			}),
			superviseupgradecmd.Cmd(commonlib.Context{
				BinName:    binname,
				NewVersion: version.VERSION,
				PostHook:   version.PostUpgradeHook,
			}),
			migratecmd.Cmd(binname, version.MIGRATIONS, version.VERSION),
			versioncmd.Cmd(
				&version.VERSION,
				clientdir.T,
				clientcontract.T,
				clientrelay.T,
			),
		},
	}.Parse(os.Args).Run(fm)
}
