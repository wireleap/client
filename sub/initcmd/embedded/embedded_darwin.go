// Copyright (c) 2021 Wireleap

package embedded

import "embed"

//go:embed wireleap_tun scripts_darwin LICENSE completion.bash
var FS embed.FS
