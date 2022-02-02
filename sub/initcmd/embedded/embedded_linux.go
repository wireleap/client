// Copyright (c) 2022 Wireleap

package embedded

import "embed"

//go:embed wireleap_intercept.so wireleap_tun wireleap_socks scripts_linux LICENSE completion.bash
var FS embed.FS
