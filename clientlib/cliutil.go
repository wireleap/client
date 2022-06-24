// Copyright (c) 2022 Wireleap

package clientlib

import (
	"encoding/json"
	"io"
	"log"
)

// NOTE:
// the intended usage of this function is that it should never fail
// if it does fail, that's an issue with the calling code
func JSONOrDie(w io.Writer, x interface{}) {
	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		log.Fatalf("could not marshal JSON output for %+v: %s", w, err)
	}
	w.Write(b)
	w.Write([]byte{'\n'})
}
