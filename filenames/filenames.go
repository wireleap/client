// Copyright (c) 2021 Wireleap

package filenames

const (
	Config     = "config.json"
	Pid        = "wireleap.pid"
	Servicekey = "servicekey.json"
	Pofs       = "pofs.json"
	Log        = "wireleap.log"
	Bypass     = "bypass.json"
	Contract   = "contract.json"
	Relays     = "relays.json"
)

var InitFiles = [...]string{Config, Servicekey, Pofs}
