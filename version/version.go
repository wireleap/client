// Copyright (c) 2022 Wireleap

// The release version is defined here.
package version

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/blang/semver"
	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/sub/tuncmd/tuncmd_platform"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/consume"
	"github.com/wireleap/common/api/duration"
	"github.com/wireleap/common/api/interfaces/clientdir"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
	"github.com/wireleap/common/cli/upgrade"
)

var BUILD_FLAGS string

var BUILD_TIME string

var GIT_COMMIT string

// old name compat
var GITREV string = "0.0.0-UNSET-VERSION"

// VERSION_STRING is the current version string, set by the linker via go build
// -X flag.
var VERSION_STRING = GITREV

// VERSION is the semver version struct of VERSION_STRING.
var VERSION = semver.MustParse(VERSION_STRING)

// Hardcoded (for now) channel value for wireleap client.
const Channel = "default"

// Post-upgrade hook for superviseupgradecmd.
func PostUpgradeHook(f fsdir.T) (err error) {
	if tuncmd_platform.Available {
		log.Println("moving wireleap_tun to wireleap_tun.prev for potential rollback...")
		if err = os.Rename(f.Path("wireleap_tun"), f.Path("wireleap_tun.prev")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			err = fmt.Errorf("error while attempting to move wireleap_tun to wireleap_tun.prev: %s", err)
			return
		} else {
			// if it does not exist, that is fine
			err = nil
		}
	}
	// force unpacking of files
	log.Println("unpacking new embedded files...")
	if err = cli.RunChild(f.Path("wireleap"), "init", "--force-unpack-only"); err != nil {
		return
	}
	log.Println("stopping running wireleap...")
	if err = cli.RunChild(f.Path("wireleap"), "stop"); err != nil {
		return
	}
	if tuncmd_platform.Available {
		fp := f.Path("wireleap_tun")
		fmt.Println("===================================")
		fmt.Println("NOTE: to enable wireleap_tun again:")
		fmt.Println("$ sudo chown 0:0", fp)
		fmt.Println("$ sudo chmod u+s", fp)
		fmt.Println("===================================")
		fmt.Println("(to return to your shell prompt just press Return)")
	}
	return
}

// Post-rollback hook for rollbackcmd.
func PostRollbackHook(f fsdir.T) (err error) {
	// do the same thing but with the old binary on rollback
	log.Println("unpacking old embedded files...")
	if err = cli.RunChild(f.Path("wireleap"), "init", "--force-unpack-only"); err != nil {
		return
	}
	if tuncmd_platform.Available {
		log.Println("moving wireleap_tun.prev to wireleap_tun...")
		if err = os.Rename(f.Path("wireleap_tun.prev"), f.Path("wireleap_tun")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fp := f.Path("wireleap_tun")
			fmt.Println("===================================")
			fmt.Println("no wireleap_tun.prev found")
			fmt.Println("NOTE: to enable wireleap_tun again:")
			fmt.Println("$ sudo chown 0:0", fp)
			fmt.Println("$ sudo chmod u+s", fp)
			fmt.Println("===================================")
			fmt.Println("(to return to your shell prompt just press Return)")
		} else {
			// if it does not exist, that is fine
			err = nil
		}
	}
	return
}

// MIGRATIONS is the slice of versioned migrations.
var MIGRATIONS = []*upgrade.Migration{
	{
		Name:    "restructuring_config",
		Version: semver.Version{Major: 0, Minor: 6, Patch: 0},
		Apply: func(f fsdir.T) error {
			var oldc map[string]interface{}
			if err := f.Get(&oldc, "config.json.next"); err != nil {
				return fmt.Errorf("could not load config.json.next: %s", err)
			}
			c := clientcfg.Defaults()
			// old contract field is ignored/obsolete
			// old accesskey field
			if ak, ok := oldc["accesskey"].(clientcfg.Accesskey); ok {
				c.Broker.Accesskey = ak
			}
			// old circuit field
			if circ, ok := oldc["circuit"].(clientcfg.Circuit); ok {
				c.Broker.Circuit = circ
			}
			// old timeout field
			if timeout, ok := oldc["timeout"].(duration.T); ok {
				c.Broker.Circuit.Timeout = timeout
			}
			// replace nil with empty list
			if c.Broker.Circuit.Whitelist == nil {
				c.Broker.Circuit.Whitelist = make([]string, 0)
			}
			// address/port changes are merged automatically
			if err := f.SetIndented(&c, "config.json.next"); err != nil {
				return fmt.Errorf("could not save config.json.next: %s", err)
			}
			log.Println("NOTE: ports used by Wireleap client have changed as follows:")
			log.Println("h2c:    13492 -> 13490")
			log.Println("socks5: 13491 (no change)")
			log.Println("tun:    13493 -> 13492")
			log.Println("If you have been depending on the old values please change the configuration accordingly.")
			return nil
		},
		Rollback: func(fsdir.T) error {
			// since we only modify config.next there is no rollback
			return nil
		},
	},
}

// LatestChannelVersion is a special function for wireleap which will obtain
// the latest version supported by the currently configured update channel from
// the directory.
// NOTE: since wireleap is not running, we need to check status out of band.
func LatestChannelVersion(f fsdir.T) (_ semver.Version, err error) {
	var pid int
	// check if running wireleap or wireleap_tun
	if tuncmd_platform.Available {
		if err = f.Get(&pid, "wireleap_tun.pid"); err == nil && process.Exists(pid) {
			err = fmt.Errorf("wireleap_tun appears to be running, please stop it to upgrade")
			return
		}
	}
	if err = f.Get(&pid, "wireleap.pid"); err == nil && process.Exists(pid) {
		err = fmt.Errorf("wireleap appears to be running, please stop it to upgrade")
		return
	}
	c := clientcfg.Defaults()
	if err = f.Get(&c, filenames.Config); err != nil {
		return
	}
	if clientlib.ContractURL(f) == nil {
		err = fmt.Errorf("`contract` field in config is empty, setup a contract with `wireleap import`")
		return
	}
	cl := client.New(nil, clientdir.T)
	dinfo, err := consume.DirectoryInfo(cl, clientlib.ContractURL(f))
	if err != nil {
		return
	}
	v, ok := dinfo.UpgradeChannels.Client[Channel]
	if !ok {
		err = fmt.Errorf("no version for client channel '%s' is provided by directory", Channel)
		return
	}
	return v, nil
}
