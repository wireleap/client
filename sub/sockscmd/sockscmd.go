// Copyright (c) 2021 Wireleap

package sockscmd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/commonsub/logcmd"
	"github.com/wireleap/common/cli/commonsub/statuscmd"
	"github.com/wireleap/common/cli/commonsub/stopcmd"
	"github.com/wireleap/common/cli/fsdir"
)

const bin = "wireleap_socks"

const pidfile = bin + ".pid"

func Cmd() (r *cli.Subcmd) {
	r = &cli.Subcmd{
		FlagSet: flag.NewFlagSet("socks", flag.ExitOnError),
		Desc:    "Control SOCKSv5 forwarder",
		Sections: []cli.Section{{
			Title: "Commands",
			Entries: []cli.Entry{
				{"start", fmt.Sprintf("Start %s daemon", bin)},
				{"stop", fmt.Sprintf("Stop %s daemon", bin)},
				{"status", fmt.Sprintf("Report %s daemon status", bin)},
				{"restart", fmt.Sprintf("Restart %s daemon", bin)},
				{"log", fmt.Sprintf("Show %s logs", bin)},
			},
		}},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 0, 8, 7, ' ', 0)
	r.SetMinimalUsage("COMMAND [OPTIONS]")
	r.Run = func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}
		if c.Broker.Address == nil {
			log.Fatal("nil address.h2c in config.json")
		}
		if r.FlagSet.NArg() < 1 {
			r.Usage()
		}
		var pid int
		cmd := r.FlagSet.Arg(0)
		binpath := fm.Path(bin)
		switch cmd {
		case "start":
			fi, err := os.Stat(binpath)
			if err != nil {
				log.Fatalf("could not stat %s: %s", binpath, err)
			}
			switch {
			case fi.Mode()&0111 == 0:
				log.Fatalf("could not execute %s: file is not executable (did you `chmod +x %s`?)", binpath, binpath)
			}
			if err = fm.Get(&pid, filenames.Pid); err != nil {
				log.Fatalf("it appears wireleap is not running: could not get wireleap PID from %s: %s", fm.Path(filenames.Pid), err)
			}
			if err = syscall.Kill(pid, 0); err != nil {
				log.Fatalf("it appears wireleap is not running: %s", err)
			}
			if c.Broker.Address == nil {
				log.Fatal("`address.h2c` in config is null, please define one for this command to work")
			}
			if c.Forwarders.Socks == nil {
				log.Fatal("`address.socks` in config is null, please define one for this command to work")
			}
			conn, err := net.DialTimeout("tcp", *c.Broker.Address, time.Second)
			if err != nil {
				log.Fatalf("could not connect to wireleap at address.h2c %s: %s", *c.Broker.Address, err)
			}
			conn.Close()
			env := append(
				os.Environ(),
				"WIRELEAP_HOME="+fm.Path(),
				"WIRELEAP_ADDR_H2C="+*c.Broker.Address+"/broker",
				"WIRELEAP_ADDR_SOCKS="+*c.Forwarders.Socks,
			)
			if r.FlagSet.Arg(1) != "--fg" {
				err = fm.Get(&pid, pidfile)
				if err == nil {
					err = syscall.Kill(pid, 0)
					if err == nil {
						log.Fatalf("%s daemon is already running!", bin)
					}
				}
				logpath := fm.Path(bin + ".log")
				logfile, err := os.OpenFile(logpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
				if err != nil {
					log.Fatalf("could not open %s logfile %s: %s", bin, logpath, err)
				}
				defer logfile.Close()
				cmd := exec.Cmd{
					Path:   binpath,
					Args:   []string{bin},
					Env:    env,
					Stdout: logfile,
					Stderr: logfile,
				}
				if err = cmd.Start(); err != nil {
					log.Fatalf("could not spawn background %s process: %s", bin, err)
				}
				log.Printf(
					"starting %s with pid %d, writing to %s...",
					bin, cmd.Process.Pid, logpath,
				)
				// wait for 2s and see if it's still alive
				e := make(chan error)
				go func() { e <- cmd.Wait() }()
				select {
				case <-e:
					log.Printf("%s is not running, %s follows:", bin, logpath)
					b, err := ioutil.ReadFile(logpath)
					if err != nil {
						log.Fatalf("could not get %s contents!", logpath)
					}
					os.Stdout.Write(b)
					os.Exit(1)
				case <-time.NewTimer(time.Second * 2).C:
					fm.Del(pidfile)
					pidtext := []byte(strconv.Itoa(cmd.Process.Pid))
					if err = ioutil.WriteFile(fm.Path(pidfile), pidtext, 0644); err != nil {
						log.Fatalf("could not write pidfile %s: %s", pidfile, err)
					}
					log.Printf("%s spawned succesfully", bin)
				}
				return
			}
			err = syscall.Exec(binpath, nil, env)
			hint := ""
			if os.IsPermission(err) {
				hint = ", check permissions (executable bit/+x)?"
			}
			if err != nil {
				log.Fatalf("could not execute %s: %s%s", binpath, err, hint)
			}
		case "stop":
			stopcmd.Cmd(bin).Run(fm)
		case "restart":
			if err = fm.Get(&pid, bin+".pid"); err == nil {
				stopcmd.Cmd(bin).Run(fm)
				i := 0
				for ; i < 10; i++ {
					err = syscall.Kill(pid, 0)
					if err != nil {
						break
					}
					time.Sleep(500 * time.Millisecond)
				}
				if i == 10 {
					log.Fatalf("timed out waiting for %s to stop", bin)
				}
			}
			r.FlagSet.Args()[0] = "start"
			r.Run(fm)
		case "status":
			statuscmd.Cmd(bin).Run(fm)
		case "log":
			logcmd.Cmd(bin).Run(fm)
		default:
			log.Fatalf("unknown socks subcommand: %s", cmd)
		}
	}
	return
}