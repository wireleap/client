// Copyright (c) 2021 Wireleap Ltd.

package embedded

import "embed"

//go:embed wireleap_intercept.so wireleap_tun scripts LICENSE
var FS embed.FS
