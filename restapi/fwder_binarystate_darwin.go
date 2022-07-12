package restapi

import (
	"os"
	"syscall"
)

const fwderSuffix = ""

func (t *T) getBinaryState(bin string) (st binaryState) {
	fi, err := os.Stat(t.br.Fd.Path(bin))
	if err != nil {
		return
	}
	st.Exists = true
	st.ChmodX = fi.Mode()&0100 != 0
	if bin == fwderPrefix+"tun" {
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok && stat.Uid == 0 {
			st.Chown0 = boolptr(true)
		} else {
			st.Chown0 = boolptr(false)
		}
		st.ChmodUS = boolptr(fi.Mode()&os.ModeSetuid != 0)
	}
	return
}
