// Copyright (c) 2022 Wireleap

package restapi

import (
	"runtime"

	"github.com/blang/semver"
	"github.com/wireleap/client/version"
	"github.com/wireleap/common/api/interfaces/clientcontract"
	"github.com/wireleap/common/api/interfaces/clientdir"
	"github.com/wireleap/common/api/interfaces/clientrelay"
	"github.com/wireleap/common/cli/upgrade"
)

type Versions struct {
	Software       *semver.Version `json:"software"`
	Api            *semver.Version `json:"api"`
	ClientRelay    *semver.Version `json:"client-relay"`
	ClientDir      *semver.Version `json:"client-dir"`
	ClientContract *semver.Version `json:"client-contract"`
}

type Upgrade struct {
	Supported bool `json:"supported"`
}

type Build struct {
	GitCommit string `json:"gitcommit"`
	GoVersion string `json:"goversion"`
	Time      string `json:"time"`
	Flags     string `json:"flags"`
}

type Platform struct {
	Os   string `json:"linux"`
	Arch string `json:"arch"`
}

type Runtime struct {
	Versions Versions `json:"versions"`
	Upgrade  Upgrade  `json:"upgrade"`
	Build    Build    `json:"build"`
	Platform Platform `json:"platform"`
}

var RuntimeReply = Runtime{
	Versions: Versions{
		Software:       &version.VERSION,
		Api:            &VERSION,
		ClientRelay:    &clientrelay.T.Version,
		ClientDir:      &clientdir.T.Version,
		ClientContract: &clientcontract.T.Version,
	},
	Upgrade: Upgrade{Supported: upgrade.Supported},
	Build: Build{
		GitCommit: version.GIT_COMMIT,
		GoVersion: runtime.Version(),
		Time:      version.BUILD_TIME,
		Flags:     version.BUILD_FLAGS,
	},
	Platform: Platform{
		Os:   runtime.GOOS,
		Arch: runtime.GOARCH,
	},
}
