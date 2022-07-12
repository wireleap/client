package restapi

import "os"

const fwderSuffix = ".exe"

func (t *T) getBinaryState(bin string) (st binaryState) {
	if _, err := os.Stat(t.br.Fd.Path(bin)); err != nil {
		return
	}
	st.Exists = true
	// NOTE not implemented on windows, just pretend everything is ok
	st.ChmodX = true
	if bin == fwderPrefix+"tun" {
		st.Chown0 = boolptr(true)
		st.ChmodUS = boolptr(true)
	}
	return
}
