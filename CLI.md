# Wireleap client command line reference

## Table of contents

- [wireleap](#wireleap)
- [wireleap init](#wireleap-init)
- [wireleap config](#wireleap-config)
- [wireleap import](#wireleap-import)
- [wireleap servicekey](#wireleap-servicekey)
- [wireleap start](#wireleap-start)
- [wireleap status](#wireleap-status)
- [wireleap reload](#wireleap-reload)
- [wireleap restart](#wireleap-restart)
- [wireleap stop](#wireleap-stop)
- [wireleap exec](#wireleap-exec)
- [wireleap intercept](#wireleap-intercept)
- [wireleap tun](#wireleap-tun)
- [wireleap upgrade](#wireleap-upgrade)
- [wireleap rollback](#wireleap-rollback)
- [wireleap info](#wireleap-info)
- [wireleap log](#wireleap-log)
- [wireleap version](#wireleap-version)

## wireleap 

```
$ wireleap help
Usage: wireleap COMMAND [OPTIONS]

Commands:
  help          Display this help message or help on a command
  init          Initialize wireleap directory
  config        Get or set wireleap configuration settings
  import        Import accesskeys JSON and set up associated contract
  servicekey    Trigger accesskey activation (accesskey.use_on_demand=false)
  start         Start wireleap daemon (SOCKSv5/connection broker)
  status        Report wireleap daemon status
  reload        Reload wireleap daemon configuration
  restart       Restart wireleap daemon
  stop          Stop wireleap daemon
  exec          Execute script from scripts directory
  intercept     Run executable and redirect connections to wireleap daemon
  tun           Control tun device
  upgrade       Upgrade wireleap to the latest version per directory
  rollback      Undo a partially completed upgrade
  info          Display some info and stats
  log           Show wireleap logs
  version       Show version and exit

Run 'wireleap help COMMAND' for more information on a command.
```

## wireleap init

```
$ wireleap help init
Usage: wireleap init [options]

Initialize wireleap directory

Options:
  --force-unpack-only       Overwrite embedded files only
```

## wireleap config

```
$ wireleap help config
Usage: wireleap config [KEY [VALUE]]

Get or set wireleap configuration settings

Keys:
  timeout                 (str)  Dial timeout duration
  contract                (str)  Service contract associated with accesskeys
  address.socks           (str)  SOCKS5 proxy address of wireleap daemon
  address.h2c             (str)  H2C proxy address of wireleap daemon
  address.tun             (str)  TUN device address (not loopback)
  circuit.hops            (int)  Number of relay hops to use in a circuit
  circuit.whitelist       (list) Whitelist of relays to use
  accesskey.use_on_demand (bool) Activate accesskeys as needed

To unset a key, specify `null` as the value
```

## wireleap import

```
$ wireleap help import
Usage: wireleap import FILE|URL

Import accesskeys JSON and set up associated contract

Arguments:
  FILE        Path to accesskeys file, or - to read standard input
  URL         URL to download accesskeys (https required)
```

## wireleap servicekey

```
$ wireleap help servicekey
Usage: wireleap servicekey

Trigger accesskey activation (accesskey.use_on_demand=false)
```

## wireleap start

```
$ wireleap help start
Usage: wireleap start [options]

Start wireleap daemon (SOCKSv5/connection broker)

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

Report wireleap daemon status

Exit codes:
  0     wireleap is running
  1     wireleap is not running
  2     could not tell if wireleap is running or not
```

## wireleap reload

```
$ wireleap help reload
Usage: wireleap reload

Reload wireleap daemon configuration
```

## wireleap restart

```
$ wireleap help restart
Usage: wireleap restart

Restart wireleap daemon
```

## wireleap stop

```
$ wireleap help stop
Usage: wireleap stop

Stop wireleap daemon
```

## wireleap exec

```
$ wireleap help exec
Usage: wireleap exec FILENAME

Execute script from scripts directory
```

## wireleap intercept

```
$ wireleap help intercept
Usage: wireleap intercept [args]

Run executable and redirect connections to wireleap daemon
```

## wireleap tun

```
$ wireleap help tun
Usage: wireleap tun COMMAND [OPTIONS]

Control tun device

Commands:
  start         Start wireleap_tun daemon
  stop          Stop wireleap_tun daemon
  status        Report wireleap_tun daemon status
  restart       Restart wireleap_tun daemon
  log           Show wireleap_tun logs
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

## wireleap info

```
$ wireleap help info
Usage: wireleap info

Display some info and stats
```

## wireleap log

```
$ wireleap help log
Usage: wireleap log

Show wireleap logs
```

## wireleap version

```
$ wireleap help version
Usage: wireleap version [options]

Show version and exit

Options:
  -v   show verbose output
```

