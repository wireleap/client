// Copyright (c) 2022 Wireleap

package startcmd

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/wireleap/client/broker"
	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/clientlib"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/client/restapi"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli"
	"github.com/wireleap/common/cli/fsdir"
	"github.com/wireleap/common/cli/process"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var reloads, shutdowns []func()

func reload() bool {
	for _, f := range reloads {
		f()
	}
	return false
}

func shutdown() bool {
	for _, f := range shutdowns {
		f()
	}
	return true
}

func setupServer(l net.Listener, h http.Handler, tc *tls.Config) {
	h1s := &http.Server{
		Handler:           h,
		TLSConfig:         tc,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	h2s := &http2.Server{MaxHandlers: 0, MaxConcurrentStreams: 0}
	if err := http2.ConfigureServer(h1s, h2s); err != nil {
		log.Fatalf("could not configure h2 server: %s", err)
	}
	h1s.Handler = h2c.NewHandler(h1s.Handler, h2s)
	go func() {
		if err := h1s.Serve(l); err != nil {
			log.Fatalf("serving on h2c://%s failed: %s", l.Addr(), err)
		}
	}()
}

var (
	broklog = log.New(os.Stderr, "[broker] ", log.LstdFlags|log.Lmsgprefix)
	restlog = log.New(os.Stderr, "[restapi] ", log.LstdFlags|log.Lmsgprefix)
)

func Cmd(arg0 string) *cli.Subcmd {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	fg := fs.Bool("fg", false, "Run in foreground, don't detach")
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    fmt.Sprintf("Start %s controller daemon", arg0),
		Run: func(fm fsdir.T) {
			var err error
			c := clientcfg.Defaults()
			// try versioned config first
			if err := fm.Get(&c, filenames.Config+".next"); err != nil {
				if err = fm.Get(&c, filenames.Config); err != nil {
					log.Fatalf("could not load config file %s: %s", fm.Path(filenames.Config), err)
				}
			}
			if *fg == false {
				var pid int
				if err = fm.Get(&pid, arg0+".pid"); err == nil {
					if process.Exists(pid) {
						log.Fatalf("%s daemon is already running!", arg0)
					}
				}

				binary, err := exec.LookPath(os.Args[0])
				if err != nil {
					log.Fatalf("could not find own binary path: %s", err)
				}

				logpath := fm.Path(arg0 + ".log")
				logfile, err := os.OpenFile(logpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

				if err != nil {
					log.Fatalf("could not open logfile %s: %s", logpath, err)
				}
				defer logfile.Close()

				cmd := exec.Cmd{
					Path:   binary,
					Args:   []string{binary, "start", "--fg"},
					Stdout: logfile,
					Stderr: logfile,
				}
				if err = cmd.Start(); err != nil {
					log.Fatalf("could not spawn background %s process: %s", arg0, err)
				}
				// assuming failed by default
				str := restapi.StatusReply{
					Home:   fm.Path(),
					Pid:    -1,
					State:  "failed",
					Broker: restapi.StatusBroker{},
				}
				if err = clientlib.DefaultAPIClient.Perform(http.MethodGet, "http://"+*c.Address+"/api/status", nil, &str); err != nil {
					st := &status.T{}
					if errors.As(err, &st) {
						// error returned from API can be jsonized
						clientlib.JSONOrDie(os.Stdout, st)
						os.Exit(1)
					} else {
						// NOTE: special exit code for fail due to upgrade requirement.
						// TODO FIXME needs a better way to signal this
						if _ = cmd.Wait(); cmd.ProcessState.ExitCode() == 26 {
							str.Upgrade = &restapi.StatusUpgrade{Required: true}
							clientlib.JSONOrDie(os.Stdout, str)
							os.Exit(1)
						}
						log.Fatalf("error while executing API request: %s", err)
					}
				} else {
					clientlib.JSONOrDie(os.Stdout, str)
				}
				return
			}
			if c.Broker.Address == nil {
				log.Fatalf("broker.address not provided, refusing to start")
			}
			log.Default().SetFlags(log.LstdFlags | log.Lmsgprefix)
			log.Default().SetPrefix("[controller] ")
			brok := broker.New(fm, &c, broklog)
			shutdowns = append(shutdowns, brok.Shutdown)
			reloads = append(reloads, brok.Reload)
			defer brok.Shutdown()
			os.Mkdir(fm.Path("webroot"), 0755)

			mux := http.NewServeMux()
			mux.Handle("/broker", brok)
			mux.Handle("/broker/", http.NotFoundHandler())

			// combo socket?
			if *c.Address == *c.Broker.Address {
				mux.Handle("/api/", http.StripPrefix("/api", restapi.New(brok, restlog)))
				mux.Handle("/", http.FileServer(http.Dir(fm.Path("webroot"))))
				restlog.Printf("listening h2c on %s", *c.Address)
			} else {
				restmux := http.NewServeMux()
				restmux.Handle("/api/", http.StripPrefix("/api", restapi.New(brok, restlog)))
				restmux.Handle("/", http.FileServer(http.Dir(fm.Path("webroot"))))

				var restl net.Listener
				if strings.HasPrefix(*c.Address, "/") {
					if err = os.RemoveAll(*c.Address); err != nil {
						restlog.Fatalf("could not remove unix socket %s: %s", *c.Address, err)
					}
					restl, err = net.Listen("unix", *c.Address)
					if err != nil {
						restlog.Fatalf("listening h2c on unix:%s failed: %s", *c.Address, err)
					}
					restlog.Printf("listening h2c on unix:%s", *c.Address)
				} else {
					restl, err = net.Listen("tcp", *c.Address)
					if err != nil {
						restlog.Fatalf("listening h2c on http://%s failed: %s", *c.Address, err)
					}
					restlog.Printf("listening h2c on http://%s", *c.Address)
				}
				defer restl.Close()
				setupServer(restl, restmux, brok.T.TLSClientConfig)
			}

			var brokl net.Listener
			if strings.HasPrefix(*c.Broker.Address, "/") {
				if err = os.RemoveAll(*c.Broker.Address); err != nil {
					restlog.Fatalf("could not remove unix socket %s: %s", *c.Broker.Address, err)
				}
				brokl, err = net.Listen("unix", *c.Broker.Address)
				if err != nil {
					broklog.Fatalf("listening h2c on unix:%s failed: %s", *c.Broker.Address, err)
				}
				broklog.Printf("listening h2c on unix:%s, waiting for forwarders", *c.Broker.Address)
			} else {
				brokl, err = net.Listen("tcp", *c.Broker.Address)
				if err != nil {
					broklog.Fatalf("listening h2c on http://%s failed: %s", *c.Broker.Address, err)
				}
				broklog.Printf("listening h2c on http://%s, waiting for forwarders", *c.Broker.Address)
			}
			defer brokl.Close()
			setupServer(brokl, mux, brok.T.TLSClientConfig)
			if err = fm.Set(os.Getpid(), arg0+".pid"); err != nil {
				log.Fatalf("could not write pid: %s", err)
			}
			cli.SignalLoop(cli.SignalMap{
				process.ReloadSignal: reload,
				syscall.SIGINT:       shutdown,
				syscall.SIGTERM:      shutdown,
				syscall.SIGQUIT:      shutdown,
			})
		},
	}
	r.Writer = tabwriter.NewWriter(r.FlagSet.Output(), 6, 8, 1, ' ', 0)
	r.Sections = []cli.Section{
		{
			Title: "Signals",
			Entries: []cli.Entry{
				{
					Key:   "SIGUSR1\t(10)",
					Value: "Reload configuration, contract information and circuit",
				},
				{
					Key:   "SIGTERM\t(15)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
				{
					Key:   "SIGQUIT\t(3)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
				{
					Key:   "SIGINT\t(2)",
					Value: "Gracefully stop wireleap daemon and exit",
				},
			},
		},
		{
			Title: "Environment",
			Entries: []cli.Entry{{
				Key:   "WIRELEAP_TARGET_PROTOCOL",
				Value: "Resolve target IP via tcp4, tcp6 or tcp (default)",
			}},
		},
	}
	return r
}
