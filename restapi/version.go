// Copyright (c) 2022 Wireleap

package restapi

import "github.com/blang/semver"

const VERSION_STRING = "0.0.1"

var VERSION = semver.MustParse(VERSION_STRING)

type Version struct {
	Version semver.Version `json:"version,omitempty"`
}
