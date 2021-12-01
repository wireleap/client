// Copyright (c) 2021 Wireleap

package execcmd

import (
	"flag"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Execute script from scripts directory",
	}
	r.SetMinimalUsage("FILENAME")
	r.Run = func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)

		if err != nil {
			log.Fatal(err)
		}

		if fs.NArg() < 1 {
			r.Usage()
		}

		p := fm.Path("scripts", fs.Arg(0))
		fi, err := os.Stat(p)

		if err != nil {
			p0 := fm.Path("scripts", "default", fs.Arg(0))
			fi, err = os.Stat(p0)
			if err != nil {
				log.Fatalf("could not stat %s or %s: %s", p, p0, err)
			}
			p = p0
		}

		if fi.Mode()&0111 == 0 {
			log.Fatalf("could not execute %s: file is not executable (did you `chmod +x`?)", p)
		}

		var pid int
		err = fm.Get(&pid, filenames.Pid)

		if err != nil {
			log.Fatalf("it appears wireleap is not running: could not get wireleap PID from %s: %s", fm.Path(filenames.Pid), err)
		}

		if !process.Exists(pid) {
			log.Fatalf("it appears wireleap (pid %d) is not running!", pid)
		}

		conn, err := net.DialTimeout("tcp", *c.Address.Socks, time.Second)

		if err != nil {
			log.Fatalf("could not connect to wireleap at address.socks %s: %s", *c.Address.Socks, err)
		}

		conn.Close()

		host, port, err := net.SplitHostPort(*c.Address.Socks)
		if err != nil {
			log.Fatalf("could not parse wireleap address.socks %s: %s", *c.Address.Socks, err)
		}

		err = syscall.Exec(
			p,
			fs.Args(),
			append([]string{
				"WIRELEAP_HOME=" + fm.Path(),
				"WIRELEAP_SOCKS=" + *c.Address.Socks,
				"WIRELEAP_SOCKS_HOST=" + host,
				"WIRELEAP_SOCKS_PORT=" + port,
			}, os.Environ()...),
		)

		hint := ""

		if os.IsPermission(err) {
			hint = ", check permissions (ownership and executable bit/+x)?"
		}

		if err != nil {
			log.Fatalf("could not execute %s: %s%s", p, err, hint)
		}
	}
	return r
}
