// Copyright (c) 2021 Wireleap

package main

import (
	"os"

	"github.com/wireleap/common/api/interfaces/clientcontract"
	"github.com/wireleap/common/api/interfaces/clientdir"
	"github.com/wireleap/common/api/interfaces/clientrelay"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/commonsub/commonlib"
	"github.com/wireleap/common/cli/commonsub/logcmd"
	"github.com/wireleap/common/cli/commonsub/migratecmd"
	"github.com/wireleap/common/cli/commonsub/rollbackcmd"
	"github.com/wireleap/common/cli/commonsub/superviseupgradecmd"
	"github.com/wireleap/common/cli/commonsub/upgradecmd"
	"github.com/wireleap/common/cli/commonsub/versioncmd"
	"github.com/wireleap/common/cli/upgrade"

	"github.com/wireleap/client/sub/configcmd"
	"github.com/wireleap/client/sub/execcmd"
	"github.com/wireleap/client/sub/importcmd"
	"github.com/wireleap/client/sub/infocmd"
	"github.com/wireleap/client/sub/initcmd"
	"github.com/wireleap/client/sub/interceptcmd"
	"github.com/wireleap/client/sub/servicekeycmd"
	"github.com/wireleap/client/sub/startcmd"
	"github.com/wireleap/client/sub/tuncmd"
	"github.com/wireleap/client/version"
	"github.com/wireleap/common/cli/commonsub/reloadcmd"
	"github.com/wireleap/common/cli/commonsub/restartcmd"
	"github.com/wireleap/common/cli/commonsub/statuscmd"
	"github.com/wireleap/common/cli/commonsub/stopcmd"
)

const binname = "wireleap"

func main() {
	fm := cli.Home()

	cli.CLI{
		Subcmds: []*cli.Subcmd{
			initcmd.Cmd(),
			configcmd.Cmd(fm),
			importcmd.Cmd(),
			servicekeycmd.Cmd(),
			startcmd.Cmd(),
			statuscmd.Cmd(binname),
			reloadcmd.Cmd(binname),
			restartcmd.Cmd(binname, startcmd.Cmd().Run, stopcmd.Cmd(binname).Run),
			stopcmd.Cmd(binname),
			execcmd.Cmd(),
			interceptcmd.Cmd(),
			tuncmd.Cmd(),
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
			infocmd.Cmd(),
			logcmd.Cmd(binname),
			versioncmd.Cmd(
				&version.VERSION,
				clientdir.T,
				clientcontract.T,
				clientrelay.T,
			),
		},
	}.Parse(os.Args).Run(fm)
}
