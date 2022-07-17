# Wireleap client command line reference

## Table of contents

- [wireleap](#wireleap)
- [wireleap init](#wireleap-init)
- [wireleap config](#wireleap-config)
- [wireleap accesskeys](#wireleap-accesskeys)
- [wireleap start](#wireleap-start)
- [wireleap status](#wireleap-status)
- [wireleap reload](#wireleap-reload)
- [wireleap restart](#wireleap-restart)
- [wireleap stop](#wireleap-stop)
- [wireleap log](#wireleap-log)
- [wireleap tun](#wireleap-tun)
- [wireleap socks](#wireleap-socks)
- [wireleap intercept](#wireleap-intercept)
- [wireleap exec](#wireleap-exec)
- [wireleap upgrade](#wireleap-upgrade)
- [wireleap rollback](#wireleap-rollback)
- [wireleap version](#wireleap-version)

## wireleap 

```
$ wireleap help
Usage: wireleap COMMAND [OPTIONS]

Commands:
  help          Display this help message or help on a command
  init          Initialize wireleap home directory
  config        Get or set wireleap configuration settings
  accesskeys    Manage accesskeys
  start         Start wireleap controller daemon
  status        Report wireleap controller daemon status
  reload        Reload wireleap controller daemon configuration
  restart       Restart wireleap controller daemon
  stop          Stop wireleap controller daemon
  log           Show wireleap controller daemon logs
  tun           Control TUN device forwarder
  socks         Control SOCKSv5 proxy forwarder
  intercept     Run executable and redirect connections (req. SOCKS forwarder)
  exec          Execute script from scripts directory (req. SOCKS forwarder)
  upgrade       Upgrade wireleap to the latest version per directory
  rollback      Undo a partially completed upgrade
  version       Show version and exit

Run 'wireleap help COMMAND' for more information on a command.
```

## wireleap init

```
$ wireleap help init
Usage: wireleap init [OPTIONS]

Initialize wireleap home directory

Options:
  --force-unpack-only       Overwrite embedded files only
```

## wireleap config

```
$ wireleap help config
Usage: wireleap config [KEY [VALUE]]

Get or set wireleap configuration settings

Keys:
  address                        (str)  Controller address
  broker.address                 (str)  Override default broker address
  broker.accesskey.use_on_demand (bool) Activate accesskeys as needed
  broker.circuit.timeout         (str)  Dial timeout duration
  broker.circuit.hops            (int)  Number of relays to use in a circuit
  broker.circuit.whitelist       (list) Relay addresses to use in circuit
  forwarders.socks.address       (str)  SOCKSv5 proxy address
  forwarders.tun.address         (str)  TUN device address (not loopback)

To unset a key, specify `null` as the value
```

## wireleap accesskeys

```
$ wireleap help accesskeys
Usage: wireleap accesskeys COMMAND

Manage accesskeys

Commands:
  list      List accesskeys
  import    Import accesskeys from URL and set up associated contract
  activate  Trigger accesskey activation (accesskey.use_on_demand=false)
```

## wireleap start

```
$ wireleap help start
Usage: wireleap start [OPTIONS]

Start wireleap controller daemon

Options:
  --fg  Run in foreground, don't detach

Signals:
  SIGUSR1 (10)  Reload configuration, contract information and circuit
  SIGTERM (15)  Gracefully stop wireleap daemon and exit
  SIGQUIT (3)   Gracefully stop wireleap daemon and exit
  SIGINT  (2)   Gracefully stop wireleap daemon and exit

Environment:
  WIRELEAP_TARGET_PROTOCOL Resolve target IP via tcp4, tcp6 or tcp (default)
```

## wireleap status

```
$ wireleap help status
Usage: wireleap status

Report wireleap controller daemon status

Exit codes:
  0     wireleap controller is active
  1     wireleap controller is inactive
  2     wireleap controller is activating or deactivating
  3     wireleap controller failed or state is unknown
```

## wireleap reload

```
$ wireleap help reload
Usage: wireleap reload

Reload wireleap controller daemon configuration
```

## wireleap restart

```
$ wireleap help restart
Usage: wireleap restart

Restart wireleap controller daemon
```

## wireleap stop

```
$ wireleap help stop
Usage: wireleap stop

Stop wireleap controller daemon
```

## wireleap log

```
$ wireleap help log
Usage: wireleap log

Show wireleap controller daemon logs
```

## wireleap tun

```
$ wireleap help tun
Usage: wireleap tun COMMAND [OPTIONS]

Control TUN device forwarder

Commands:
  start         Start wireleap_tun daemon
  stop          Stop wireleap_tun daemon
  status        Report wireleap_tun daemon status
  restart       Restart wireleap_tun daemon
  log           Show wireleap_tun logs
```

## wireleap socks

```
$ wireleap help socks
Usage: wireleap socks COMMAND [OPTIONS]

Control SOCKSv5 proxy forwarder

Commands:
  start         Start wireleap_socks daemon
  stop          Stop wireleap_socks daemon
  status        Report wireleap_socks daemon status
  restart       Restart wireleap_socks daemon
  log           Show wireleap_socks logs
```

## wireleap intercept

```
$ wireleap help intercept
Usage: wireleap intercept [ARGS]

Run executable and redirect connections (req. SOCKS forwarder)
```

## wireleap exec

```
$ wireleap help exec
Usage: wireleap exec COMMAND|FILENAME

Execute script from scripts directory (req. SOCKS forwarder)

Commands:
  list  List available scripts in scripts directory
```

## wireleap upgrade

```
$ wireleap help upgrade
Usage: wireleap upgrade

Upgrade wireleap to the latest version per directory
```

## wireleap rollback

```
$ wireleap help rollback
Usage: wireleap rollback

Undo a partially completed upgrade
```

## wireleap version

```
$ wireleap help version
Usage: wireleap version [OPTIONS]

Show version and exit

Options:
  -v   show verbose output
```

