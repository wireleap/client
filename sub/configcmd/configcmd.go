// Copyright (c) 2021 Wireleap

package configcmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd(fm0 fsdir.T) *cli.Subcmd {
	fs := flag.NewFlagSet("config", flag.ExitOnError)

	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Get or set wireleap configuration settings",
	}

	r.Run = func(fm fsdir.T) {
		if fs.NArg() < 1 {
			r.Usage()
		}

		key := fs.Arg(0)
		val := fs.Arg(1)
		vals := fs.Args()[1:]

		if key == "" {
			r.Usage()
		}

		// pre-processing val list for all "list" type config items to be in-line
		// with all other arguments
		var val_type string
		c := clientcfg.Defaults()
		for _, meta := range c.Metadata() {
			if meta.Name == key {
				val_type = meta.Type
				break
			}
		}

		if val_type != "list" && fs.NArg() > 2 {
			r.Usage()
		}

		if val_type == "list" && len(vals) > 0 {
			if vals[0] == "null" {
				val = vals[0]
			} else {
				val_bytes, err := json.Marshal(&vals)
				if err != nil {
					log.Fatalf(
						"could not marshal values for `circuit.whitelist`: %s",
						err)
				}
				val = string(val_bytes)
			}
		}
		Run(fm, key, val)
	}

	r.Usage = func() {
		w := tabwriter.NewWriter(r.Output(), 0, 8, 1, ' ', 0)

		fmt.Fprintf(w, "Usage: wireleap %s [KEY [VALUE]]\n\n", r.Name())
		fmt.Fprintf(w, "%s\n\n", r.Desc)
		fmt.Fprint(w, "Keys:\n")

		c := clientcfg.Defaults()
		fm0.Get(&c, filenames.Config)

		for _, meta := range c.Metadata() {
			fmt.Fprintf(w, "  %s\t(%s)\t%s\n", meta.Name, meta.Type, meta.Desc)
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, "To unset a key, specify `null` as the value")

		w.Flush()
		os.Exit(2)
	}

	return r
}

func Run(fm fsdir.T, key, val string) {
	c := clientcfg.Defaults()
	if err := fm.Get(&c, filenames.Config); err != nil {
		log.Fatalf("error when loading config: %s", err)
	}
	var match *clientcfg.Meta
	for _, meta := range c.Metadata() {
		if meta.Name == key {
			match = meta
			break
		}
	}
	if match == nil {
		log.Fatalf("no such config key: %s", key)
	}
	if len(val) == 0 {
		// show defined value
		b, err := json.Marshal(match.Val)
		if err != nil {
			// this shouldn't really happen
			b = []byte("unknown")
		} else if match.Quote && !bytes.Equal(b, []byte("null")) {
			b = b[1 : len(b)-1]
		}
		fmt.Println(string(b))
		return
	}
	if match.Quote {
		val = "\"" + val + "\""
	}
	if err := json.Unmarshal([]byte(val), match.Val); err != nil {
		log.Fatalf("could not set %s value to %s: %s", key, val, err)
	}
	if key == "address.socks" {
		log.Printf("Note: address.socks changes will take effect on restart.")
	}
	clientlib.APICallOrDie(http.MethodPost, "http://"+*c.Address+"/api/config", &c, &c)
}
