// Copyright (c) 2021 Wireleap

package configcmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	"text/tabwriter"

	"github.com/wireleap/client/clientcfg"
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

		if key == "" || (key != "circuit.whitelist" && fs.NArg() > 2) {
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
	err := fm.Get(&c, filenames.Config)

	if err != nil {
		log.Fatalf("error when loading config: %s", err)
	}

	for _, meta := range c.Metadata() {
		if meta.Name == key {
			if len(val) == 0 {
				b, err := json.Marshal(meta.Val)

				if err != nil {
					b = []byte("unknown")
				} else if meta.Quote && !bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
					b = b[1 : len(b)-1]
				}

				fmt.Println(string(b))
				return
			}

			if meta.Quote {
				val = "\"" + val + "\""
			}

			err = json.Unmarshal([]byte(val), meta.Val)

			if err != nil {
				log.Fatalf(
					"could not set %s value to %s: %s",
					key,
					val,
					err,
				)
			}

			err = fm.Set(&c, filenames.Config)

			if err != nil {
				log.Fatal(err)
			}

			var pid int
			err = fm.Get(&pid, filenames.Pid)

			if err != nil {
				return
			}

			syscall.Kill(pid, syscall.SIGUSR1)

			if key == "address.socks" {
				log.Printf("Note: address.socks changes will take effect on restart.")
			}

			return
		}
	}

	log.Fatalf("no such config key: %s", key)
}
