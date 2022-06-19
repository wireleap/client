// Copyright (c) 2022 Wireleap

package execcmd

import (
	"flag"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
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

		arg0 := fs.Arg(0)
		if runtime.GOOS == "windows" {
			arg0 += ".bat"
		}

		p := fm.Path("scripts", arg0)

		fi, err := os.Stat(p)

		if err != nil {
			p0 := fm.Path("scripts", "default", arg0)
			fi, err = os.Stat(p0)
			if err != nil {
				log.Fatalf("could not stat %s or %s: %s", p, p0, err)
			}
			p = p0
		}

		if runtime.GOOS != "windows" && fi.Mode()&0111 == 0 {
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

		conn, err := net.DialTimeout("tcp", c.Forwarders.Socks.Address, time.Second)

		if err != nil {
			log.Fatalf("could not connect to wireleap at address.socks %s: %s", c.Forwarders.Socks.Address, err)
		}

		conn.Close()

		host, port, err := net.SplitHostPort(c.Forwarders.Socks.Address)
		if err != nil {
			log.Fatalf("could not parse wireleap address.socks %s: %s", c.Forwarders.Socks.Address, err)
		}

		cmd := exec.Command(p, fs.Args()[1:]...)
		cmd.Env = append(os.Environ(),
			"WIRELEAP_HOME="+fm.Path(),
			"WIRELEAP_SOCKS="+c.Forwarders.Socks.Address,
			"WIRELEAP_SOCKS_HOST="+host,
			"WIRELEAP_SOCKS_PORT="+port,
		)
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		err = cmd.Run()

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
