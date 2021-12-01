// Copyright (c) 2021 Wireleap

package embedded

import "embed"

//go:embed wireleap_intercept.so wireleap_tun scripts_linux LICENSE completion.bash
var FS embed.FS
