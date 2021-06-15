// Copyright (c) 2021 Wireleap

// The release version is defined here.
package version

import (
	"fmt"
	"log"

	"github.com/blang/semver"
	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/auth"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/consume"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/upgrade"
)

// old name compat
var GITREV string = "<unset>"

// VERSION_STRING is the current version string, set by the linker via go build
// -X flag.
var VERSION_STRING = GITREV

// VERSION is the semver version struct of VERSION_STRING.
var VERSION = semver.MustParse(VERSION_STRING)

// Hardcoded (for now) channel value for wireleap client.
const Channel = "client"

// Post-upgrade hook for superviseupgradecmd.
func PostUpgradeHook(f fsdir.T) (err error) {
	// force unpacking of files
	log.Println("unpacking new embedded files...")
	if err = cli.RunChild(f.Path("wireleap"), "init", "--force-unpack-only"); err != nil {
		return
	}
	log.Println("stopping running wireleap...")
	if err = cli.RunChild(f.Path("wireleap"), "stop"); err != nil {
		return
	}
	fp := f.Path("wireleap_tun")
	fmt.Println("===================================")
	fmt.Println("NOTE: to enable wireleap_tun again:")
	fmt.Println("$ sudo chown root:root", fp)
	fmt.Println("$ sudo chmod u+s", fp)
	fmt.Println("===================================")
	fmt.Println("(to return to your shell prompt just press Return)")
	return
}

// Post-rollback hook for rollbackcmd.
func PostRollbackHook(f fsdir.T) (err error) {
	// do the same thing but with the old binary on rollback
	log.Println("unpacking old embedded files...")
	if err = cli.RunChild(f.Path("wireleap"), "init", "--force-unpack-only"); err != nil {
		return
	}
	fp := f.Path("wireleap_tun")
	fmt.Println("===================================")
	fmt.Println("NOTE: to enable wireleap_tun again:")
	fmt.Println("$ sudo chown root:root", fp)
	fmt.Println("$ sudo chmod u+s", fp)
	fmt.Println("===================================")
	fmt.Println("(to return to your shell prompt just press Return)")
	return
}

// MIGRATIONS is the slice of versioned migrations.
var MIGRATIONS = []*upgrade.Migration{}

// LatestChannelVersion is a special function for wireleap which will obtain
// the latest version supported by the currently configured update channel from
// the directory.
func LatestChannelVersion(f fsdir.T) (_ semver.Version, err error) {
	// check if running wireleap or wireleap_tun
	if err = cli.RunChild(f.Path("wireleap"), "tun", "status"); err == nil {
		err = fmt.Errorf("wireleap_tun appears to be running, please stop it to upgrade")
		return
	}
	if err = cli.RunChild(f.Path("wireleap"), "status"); err == nil {
		err = fmt.Errorf("wireleap appears to be running, please stop it to upgrade")
		return
	}
	c := clientcfg.Defaults()
	if err = f.Get(&c, filenames.Config); err != nil {
		return
	}
	if c.Contract == nil {
		err = fmt.Errorf("`contract` field in config is empty, setup a contract with `wireleap import`")
		return
	}
	cl := client.New(nil, auth.Client)
	dinfo, err := consume.DirectoryInfo(cl, c.Contract)
	if err != nil {
		return
	}
	v, ok := dinfo.Channels[Channel]
	if !ok {
		err = fmt.Errorf("no version for channel '%s' is provided by directory", Channel)
		return
	}
	return v, nil
}
