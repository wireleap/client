// Copyright (c) 2021 Wireleap

package embedded

import "embed"

//go:embed wireleap_tun scripts LICENSE completion_darwin.bash
var FS embed.FS
