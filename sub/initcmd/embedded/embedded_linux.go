// Copyright (c) 2021 Wireleap

package embedded

import "embed"

//go:embed wireleap_intercept.so wireleap_tun scripts LICENSE completion_linux.bash
var FS embed.FS
