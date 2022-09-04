// Copyright (c) 2022 Wireleap

package stopcmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
)

func Cmd(arg0 string) *cli.Subcmd {
	return &cli.Subcmd{
		FlagSet: flag.NewFlagSet("stop", flag.ExitOnError),
		Desc:    fmt.Sprintf("Stop %s controller daemon", arg0),
		Run: func(fm fsdir.T) {
			var (
				pid      int
				err      error
				pidfile  = arg0 + ".pid"
				sockspid = "wireleap_socks.pid"
				tunpid   = "wireleap_tun.pid"
			)
			if err = fm.Get(&pid, sockspid); err == nil && process.Exists(pid) {
				log.Fatalf("`wireleap_socks` appears to be running, stop it before stopping `wireleap`")
			}
			if err = fm.Get(&pid, tunpid); err == nil && process.Exists(pid) {
				log.Fatalf("`wireleap_tun` appears to be running, stop it before stopping `wireleap`")
			}
			if err = fm.Get(&pid, pidfile); err != nil {
				log.Fatalf(
					"could not get pid of %s from %s: %s",
					arg0, fm.Path(pidfile), err,
				)
			}
			if process.Exists(pid) {
				if err = process.Term(pid); err != nil {
					log.Fatalf("could not terminate %s pid %d: %s", arg0, pid, err)
				}
			}
			o := restapi.StatusReply{
				Home:   fm.Path(),
				Pid:    -1,
				State:  "inactive",
				Broker: restapi.StatusBroker{},
			}
			for i := 0; i < 30; i++ {
				if !process.Exists(pid) {
					clientlib.JSONOrDie(os.Stdout, o)
					fm.Del(pidfile)
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			process.Kill(pid)
			time.Sleep(100 * time.Millisecond)
			if process.Exists(pid) {
				log.Fatalf("timed out waiting for %s (pid %d) to shut down -- process still alive!", arg0, pid)
			}
			clientlib.JSONOrDie(os.Stdout, o)
			fm.Del(pidfile)
		},
	}
}
