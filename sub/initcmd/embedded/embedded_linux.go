package embedded

import "embed"

//go:embed wireleap_intercept.so wireleap_tun scripts LICENSE
var FS embed.FS
