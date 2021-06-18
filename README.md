# wireleap client

This is the [wireleap](https://wireleap.com) client. The binary name is
`wireleap`.

## Client configuration

> Client configuration (config.json)

```json
{
    "timeout": "5s",
    "contract": "https://contract1.example.com",
    "accesskey": {
        "use_on_demand": true
    },
    "circuit": {
        "hops": 2,
        "whitelist": []
    },
    "address": {
        "socks": "127.0.0.1:13491",
        "h2c": "127.0.0.1:13492",
        "tun": "10.13.49.0:13493"
    }
}
```

> Config usage examples

```shell
# display help related to the config command
wireleap help config

# set the address of the connection broker (requires daemon restart)
wireleap config address.socks 127.0.0.1:3434
```

> Accesskeys examples

```shell
# automatically generate and activate servicekeys as needed
wireleap config accesskey.use_on_demand true

# manually generate and activate a servicekey
wireleap config accesskey.use_on_demand false
wireleap servicekey
```

> Circuit examples

```shell
# set the number of circuit hops (will auto-generate a new circuit)
wireleap config circuit.hops 1

# set a whitelist of relays to use
wireleap config circuit.whitelist '["wireleap://relay1.example.com:13490"]'

# manually trigger new circuit generation
wireleap reload
```

The client configuration is stored in `config.json`. The `wireleap
config` command provides a convenient interface for both _setting_ and
_getting_ configuration variables. Currently supported variables:

Key | Type | Comment
--- | ---- | -------
timeout | `string` | Dial timeout duration
contract | `string` | Service contract associated with accesskeys
accesskey.use_on_demand | `bool` | Activate accesskeys as needed
circuit.hops | `int` | Number of relay hops to use in a circuit
circuit.whitelist | `list` | Whitelist of relays to use
address.socks | `string` | SOCKSv5 proxy server address of wireleap daemon
address.h2c | `string` | h2c proxy server address of wireleap daemon
address.tun | `string` | tun device address (not loopback)

After changing configuration options via `wireleap config`, the changes
will be applied immediately.

Note: after editing the `config.json` file manually with a text editor
while `wireleap` is already running, you will need to reload `wireleap`
for the changes to take effect via `wireleap reload`.

Note: a `config.json` will be automatically created upon `wireleap
init`, and the contract variable will be set when importing accesskeys.

### Circuit (hops and whitelist)

The circuit defines which relays will be used to transmit traffic. Each
relay enrolled into a contract assigns itself a role related to its
position in the connection circuit. A `fronting` relay provides an
on-ramp to the routing layer, while a `backing` relay provides an exit
from the routing layer. `entropic` relays add additional entropy to the
circuit in the routing layer.

Depending on requirements, `circuit.hops` may be any positive integer,
setting the amount of relays used in a circuit. The amount of hops
specified implicitly asserts the relay roles as well.

Hops | Fronting | Entropic | Backing
---- | -------- | -------- | -------
`1` | 0 | 0 | 1
`2` | 1 | 0 | 1
`3+` | 1 | N | 1

A circuit is generated by randomly selecting from the available relays
enrolled in a service contract. Additionally, a whitelist may be
specified allowing the creation of an exact circuit when coupled with a
specific amount of hops, or a more general *only use these relays*.

An initial circuit is generated upon launch and regenerated either
automatically if issues are encountered or when the wireleap daemon
receives the `SIGUSR1` signal (which also happens when settings are
modified via `wireleap config` or a reload is requested via `wireleap
reload`).


## Client daemon

> Symlink wireleap to $PATH

```shell
# symlink the wireleap binary to a $PATH folder, e.g.,
mkdir -p $HOME/bin
ln -s $HOME/wireleap/wireleap $HOME/bin/wireleap
```

> Setup

```shell
# initialize wireleap_home
wireleap init

# import accesskeys
wireleap import path/to/accesskeys.json

# start the wireleap daemon (default: 127.0.0.1:13491)
wireleap start

# verify it is running and show some useful info
wireleap status
wireleap info

# (at some later time) stop the wireleap daemon
wireleap stop

# or, optionally, start the wireleap daemon in the foreground
wireleap start --fg
```

[Install](#installation) or [build](#building) `wireleap`, and add a
symlink of `wireleap` to your `$PATH` (highly recommended, assumed in
the following examples).

Once `wireleap` is in your `$PATH`, perform initialization, import
accesskeys, and start the SOCKSv5 connection broker daemon.

An accesskey is required to use relays enrolled in a service contract.
Accesskeys are provided by contracts after obtaining access.

Once running, any application that supports the `SOCKS5` protocol can
be configured to route its traffic via the connection broker.

Note: The client stores its
[configuration](#client--client-configuration) and other [essential
files](#client-files) in the enclosing directory of the `wireleap`
binary. It can be arbitrary but `$HOME/wireleap` is a sensible default
value.

# Client usage

## manual configuration

> Examples: Manual configuration

```shell
# manually specifying the --proxy flag
curl --proxy socks5h://$(wireleap config address.socks) URL

# manually exporting environment variable
export ALL_PROXY="socks5h://$(wireleap config address.socks)"
curl URL

# manually specifying the --proxy-server flag
google-chrome \
    --proxy-server="socks5://$(wireleap config address.socks)" \
    --user-data-dir="$HOME/.config/google-chrome-wireleap" \
    --incognito
```

> Mozilla Firefox

```
- Start Firefox, navigate to about:preferences
- General page > Network Settings > Settings
- Connection Settings
    - Select: Manual proxy configuration
        - SOCKS Host: 127.0.0.1
        - Port: 13491 # default, unless manually changed
    - Select: SOCKS v5
    - Check: Proxy DNS when using SOCKS v5
    - Click: OK
```

Once the `wireleap` SOCKS5 connection broker is running, any application
that supports the `SOCKS5` protocol can be configured to route its
traffic via the connection broker.

Unfortunately, there is no standard for configuration so a few examples
are provided.

Tip: `wireleap config address.socks` will return the SOCKS5 address the
wireleap daemon is configured to use.

## wireleap exec

> Examples: wireleap exec

```shell
# list available exec commands
ls $HOME/wireleap/scripts/default

# example usage
wireleap exec curl URL
wireleap exec git clone URL
wireleap exec google-chrome [URL]
```

As mentioned above, there is no standard for proxy configuration among
applications, so a few wrapper scripts are included in
`scripts/default/` which can be executed by invoking `wireleap exec`.

Feel free to add your own to `scripts`. Same-named files in `scripts/`
will override the ones in `scripts/default/`. If you do, please consider
[submitting them](#hacking) for inclusion. Please note that all scripts
should be POSIX `sh` compliant and tested with
[shellcheck](https://www.shellcheck.net).

## wireleap intercept

> Examples: wireleap intercept (Linux only)

```shell
wireleap intercept curl URL
wireleap intercept ssh USER@HOST
```

For applications that do not support proxying via the `SOCKS5` protocol
natively (or even those that do), it may be possible to use `wireleap
intercept` (experimental: Linux only).

The `wireleap_intercept.so` library is used by `wireleap intercept` to
intercept network connections from arbitrary programs and tunnel them
through the configured circuit.

## wireleap tun

> Examples: wireleap tun (Linux only)

```shell
# start the wireleap broker (required for tun)
wireleap start

# setup tun device and configure routes
sudo wireleap tun start

# verify it is running
sudo wireleap tun status

# show the log (eg. $HOME/wireleap/wireleap_tun.log)
sudo wireleap tun log

# (at some later time) stop the wireleap tun daemon
sudo wireleap tun stop

# or, optionally, start in the foreground
sudo wireleap tun start --fg
```

To forward all traffic on a system (both TCP and UDP) through the
`wireleap` connection broker, it is possible to use `wireleap tun`
(experimental: Linux only).

The `wireleap tun` subcommand will use the bundled `wireleap_tun` binary
(unpacked on `wireleap init`) to set up a [tun
device](https://en.wikipedia.org/wiki/TUN/TAP) and configure default
routes through it so that all traffic from the local system goes through
the tun device, effectively meaning that it is routed through the
currently used wireleap broker circuit.

Note: `wireleap_tun` should usually not be started other than via the
`wireleap tun` subcommand. The `wireleap tun` subcommand provides a
familiar `start/stop/status` interface to the daemon.

Note: `wireleap_tun` needs sufficient privileges to create a tun device
and manage routes during the lifetime of the daemon. Currently, this
means that `wireleap tun` commands need to be run with `sudo` or `su`
(as root).

Note: `tun`-related functipnality cannot be tested with a circuit
which includes local relays (via Docker or localtestnet setup) yet.

# Client upgrades

> Manual upgrade

```shell
# perform interactive upgrade to latest channel version
wireleap upgrade

# rollback if needed
wireleap rollback
```

The [precompiled binary](#installation) of `wireleap` includes manual
upgrade functionality. Due to protocol [versioning](#versioning), it is
highly recommended to keep the client up to date. A client which is out
of date with regard to the directory's required client version will
refuse to run.

The upgrade process is interactive so you will have the possibility
to accept or decline based on the changelog for the new release version.

If the upgrade was successful, the old binary is not deleted but kept as
`wireleap.prev` (for rollback purposes in case issues manifest
post-upgrade).

The client update channels supported by the directory and the respective
latest version is exposed via the directory's `/info` endpoint in
`update_channels.client`.

> Skipping a faulty version explicitly

```shell
cd "$(dirname $(which wireleap))"
echo "1.2.3" > .skip-upgrade-version # where 1.2.3 is the version you need to skip
```

Note: normally, upgrading to the directory-required version is needed to start
`wireleap`. However, if the last upgrade failed to run the new binary, it is
allowed to run the older version of `wireleap` temporarily. Moreover, if the
upgrade failed before the files were downloaded due to prolonged network issues
or such, you can temporarily force running the older version.

# Client files

> wireleap directory tree

```shell
tree $HOME/wireleap
├── config.json
├── pofs.json
├── relays.json
├── contract.json
├── servicekey.json
├── wireleap
├── wireleap.pid
├── wireleap.log
├── wireleap_tun
├── wireleap_tun.pid
├── wireleap_tun.log
├── wireleap_intercept.so
└── scripts/default
    ├── git
    ├── curl
    ├── chromium-browser
    └── ...
```

> servicekey.json and pofs.json

```shell
# automatically generate and activate servicekeys as needed
wireleap config accesskey.use_on_demand true

# manually generate and activate a servicekey
wireleap config accesskey.use_on_demand false
wireleap servicekey
```

> wireleap.pid

```shell
# reload config file, contract info and generate new circuit
wireleap reload

# terminate gracefully
wireleap stop
```

The client stores its configuration and other essential files on the
filesystem in the same directory as the `wireleap` binary. It can be any
directory but `$HOME/wireleap` is a sensible value. Some of the files
are described below:

### contract.json

Contains a snapshot of the `/info` API endpoint contents of the
currently used service contract.

### servicekey.json

If present, contains the currently active servicekey for the currently
active service contract. If `accesskey.use_on_demand` is set to
`true`, it is generated automatically using the proofs of funding from
`pofs.json`. If `accesskey.use_on_demand` is set to `false` and an
expired servicekey is read from this file, `wireleap` will return an
error. In that case, a new key can be generated via `wireleap
servicekey`.

### pofs.json

Contains the list of proof-of-funding tokens for the currently active
service contract obtained from importing `accesskeys.json` files. It is
managed automatically by `wireleap` if `accesskey.use_on_demand` is set
to `true`.  Alternatively, it can be managed manually via the `wireleap
servicekey` command.

### relays.json

Contains the list of known relays of the currently active service
contract obtained from its relay directory. It is refreshed on startup,
reload or when `wireleap` receives the `SIGUSR1` signal.

### wireleap.pid

Contains the PID (process ID) of the currently running `wireleap start`
daemon (if any). It can be used to send control signals to the daemon
such as `SIGUSR1` to reload config file and contract info, and `SIGINT`,
`SIGTERM` or `SIGQUIT` to terminate gracefully.

### wireleap_intercept.so

This is the library which is used by `wireleap intercept` to intercept
connections from arbitrary programs and tunnel them through the
configured network circuit. The command `wireleap intercept`, this file
and their associated command line options and configuration variables
are only present on Linux.

### wireleap_tun

This is the binary used to provide `wireleap tun` functionality. It is
not intended to be executed directly.

### scripts/

This directory contains scripts to be ran via `wireleap exec`. On
execution, the `WIRELEAP_SOCKS` environmental variable will be available
inside the script containing the current `wireleap` SOCKSv5 listening
address.

This directory is for user-defined scripts which take preference over
the default scripts (described below).

### scripts/default/

This directory contains scripts to be ran via `wireleap exec`. On
execution, the `WIRELEAP_SOCKS` environmental variable will be available
inside the script containing the current `wireleap` SOCKSv5 listening
address.

This directory contains default `wireleap`-supplied scripts. If
modifications are required, just save your version of the script under
the same name in `scripts/` as the `scripts/default/` script you wish to
alter. This ensures that updates will not overwrite user changes to
scripts.

