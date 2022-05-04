// Copyright (c) 2021 Wireleap

package restapi

import "github.com/blang/semver"

const VERSION_STRING = "0.0.1"

var VERSION = semver.MustParse(VERSION_STRING)

type VersionReply struct {
	Version semver.Version `json:"version,omitempty"`
}
