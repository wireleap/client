package embedded

import "embed"

//go:embed wireleap_intercept.so wireleap_tun scripts DISCLAIMER
var FS embed.FS
