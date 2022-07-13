// Copyright (c) 2022 Wireleap

package interceptcmd

import (
	"flag"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("intercept", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Run executable and redirect connections (req. SOCKS forwarder)",
	}
	r.SetMinimalUsage("[ARGS]")
	r.Run = func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		if fs.NArg() == 0 {
			r.Usage()
			os.Exit(1)
		}
		switch runtime.GOOS {
		case "linux":
			conn, err := net.DialTimeout("tcp", *c.Broker.Address, time.Second)
			if err != nil {
				log.Fatalf("could not connect to wireleap broker at address %s: %s", *c.Broker.Address, err)
			}
			conn.Close()
			lib := fm.Path("wireleap_intercept.so")
			args := fs.Args()

			bin, err := exec.LookPath(args[0])

			if err != nil {
				log.Fatal(err)
			}

			err = syscall.Exec(
				bin,
				args,
				append([]string{
					"LD_PRELOAD=" + lib,
					"SOCKS5_PROXY=" + c.Forwarders.Socks.Address,
				}, os.Environ()...),
			)

			if err != nil {
				log.Fatal(err)
			}
		default:
			log.Fatal("unsupported OS:", runtime.GOOS)
		}
	}
	return r
}
