// Copyright (c) 2022 Wireleap

package clientlib

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/status"
)

var DefaultAPIClient = func() *client.Client {
	cl := client.New(nil)
	cl.RetryOpt.Tries = 100
	cl.RetryOpt.Interval = 150 * time.Millisecond
	cl.RetryOpt.Verbose = false
	return cl
}()

// NOTE:
// the intended usage of this function is that it should never fail
// if it does fail, that's an issue with the calling code
func JSONOrDie(w io.Writer, x interface{}) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(x); err != nil {
		log.Fatalf("could not marshal JSON output for %+v: %s", w, err)
	}
}

func APICallOrDie(method, url string, in interface{}, out interface{}) {
	if err := DefaultAPIClient.Perform(method, url, in, out); err != nil {
		st := &status.T{}
		if errors.As(err, &st) {
			// error can be jsonized
			JSONOrDie(os.Stdout, st)
			os.Exit(1)
		} else {
			log.Fatalf("error while executing API request: %s", err)
		}
	} else {
		JSONOrDie(os.Stdout, out)
	}
}
