# Wireleap client

[Wireleap](https://wireleap.com) is a decentralized communications
protocol and open-source software designed with the goal of providing
unrestricted access to the internet from anywhere.

The Wireleap client software is used to tunnel traffic through
servers running Wireleap relay software.

This repository is for the Wireleap client.

## Table of contents

- [Installation](#installation)
    - [Shell completion](#shell-completion)
- [Controller](#controller)
    - [Configuration](#configuration)
    - [Accesskeys](#accesskeys)
    - [Circuit](#circuit)
- [Forwarders](#forwarders)
    - [Specific traffic (SOCKSv5)](#specific-traffic-socksv5)
    - [All traffic (TUN)](#all-traffic-tun)
- [Upgrade](#upgrade)
- [Files](#files)
- [Versioning](#versioning)
- [Building](#building)
- [Contributing](#contributing)
- [License](#license)

## Installation

The quickest way to install the Wireleap client is by using the
convenience script.

Linux:

```shell
curl -fsSL https://get.wireleap.com/linux -o get-wireleap.sh
sh get-wireleap.sh $HOME/wireleap --symlink=$HOME/.local/bin/wireleap
```

macOS:

```shell
curl -fsSL https://get.wireleap.com/darwin -o get-wireleap.sh
sh get-wireleap.sh $HOME/wireleap

# add wireleap to your $PATH. if unsure, this should work for most users
sudo ln -s $HOME/wireleap/wireleap /usr/local/bin/wireleap
```

Windows:

```powershell
# To see a description of the options accepted by the installation script:
# powershell -nop -c "iex(New-Object Net.WebClient).DownloadString('https://get.wireleap.com/windows'); Get-Help Get-Wireleap -Full"
powershell -nop -c "iex(New-Object Net.WebClient).DownloadString('https://get.wireleap.com/windows'); Get-Wireleap -Dir $env:USERPROFILE\wireleap"
```

The above will verify your environment's compatibility, download the
latest client binary as well as the associated hash file to
cryptographically verify its integrity via GPG signature (in a temporary
keyring, currently Linux and macOS only) and checksum hash. If all
checks pass, it will release the binary from quarantine, initialize the
client in the specified directory, and create a symlink if requested.

Alternatively, you can download the [latest release][releases] and
perform manual verification and installation, or [build from
source](#building).

[releases]: https://github.com/wireleap/client/releases

### Shell completion

#### bash

Bash completion is available for all `wireleap` commands, sub-commands,
option flags, as well as `exec` and `config broker.circuit.whitelist`.
Add the following line to your `$HOME/.bashrc` or similar location.

```shell
[ -e "$HOME/wireleap/completion.bash" ] && source "$HOME/wireleap/completion.bash"
```

#### zsh

The provided `completion.bash` script is compatible with Zsh by using the
`bashcompinit` compatibility layer. Add the following line to your
`$HOME/.zshrc` or similar location.

```shell
if [ -e "$HOME/wireleap/completion.bash" ]; then
    autoload compinit && compinit
    autoload bashcompinit && bashcompinit
    source "$HOME/wireleap/completion.bash"
fi
```

## Controller

Once `wireleap` has been initialized and is in your `$PATH`, the
controller daemon can be started.

```shell
# start the wireleap controller daemon
wireleap start
wireleap status

# (at some later time) stop the wireleap controller daemon
wireleap stop
```

Traffic can then be routed via wireleap either for specific applications
via the [SOCKSv5 forwarder](#specific-traffic-socksv5), or for all
traffic on the system via the [TUN forwarder](#all-traffic-tun).

### Configuration

The client configuration is stored in `config.json`. This file will
automatically be created upon `wireleap init`. Currently supported
variables:

Key                            | Type     | Comment
---                            | ----     | -------
address                        | `string` | Controller address
broker.address                 | `string` | Override default broker address
broker.accesskey.use_on_demand | `bool`   | Activate accesskeys as needed
broker.circuit.timeout         | `string` | Dial timeout duration
broker.circuit.hops            | `int`    | Number of relays to use in a circuit
broker.circuit.whitelist       | `list`   | Relay addresses to use in circuit
forwarders.socks.address       | `string` | SOCKSv5 proxy address
forwarders.tun.address         | `string` | TUN device address (not loopback)

```json
{
  "address": "127.0.0.1:13490",
  "broker": {
    "accesskey": {
      "use_on_demand": true
    },
    "circuit": {
      "timeout": "5s",
      "hops": 1,
      "whitelist": []
    }
  },
  "forwarders": {
    "socks": {
      "address": "127.0.0.1:13491"
    },
    "tun": {
      "address": "10.13.49.0:13492"
    }
  }
}
```

Note: after editing the `config.json` file manually with a text editor
while `wireleap` is already running, you will need to issue `wireleap
reload` for the changes to take effect.

The `wireleap config` command provides a convenient interface for both
_setting_ and _getting_ configuration variables.

```shell
# display help related to the config command
wireleap help config

# display current value of configuration variable
wireleap config forwarders.socks.address

# set a configuration variable
wireleap config broker.circuit.hops 3
```

After changing configuration options via `wireleap config`, the changes
will be applied immediately (except for `address` fields).

### Accesskeys

Accesskeys are a convenience abstraction to proof of funding's and
service keys, and include contract metadata for association when
originally imported.

An accesskey is required to use relays enrolled in a service contract.
Accesskeys are provided by contracts after obtaining access. They are
used to cryptographically and independently generate tokens by the
client for each relay in the routing path (circuit), which are included
in the appropriate encrypted onion layer of traffic being sent, allowing
the relay to authorize service. This increases the degrees of separation
between payment information and network usage.

```shell
# import accesskeys
wireleap accesskeys import file:///path/to/accesskeys.json
wireleap accesskeys import https://example.com/accesskeys/...
```

A proof of funding is used to activate servicekeys, which can be done
automatically (`broker.accesskey.use_on_demand`) when needed (e.g.,
previous one has expired), or can be manually generated and activated.

```shell
# automatically generate and activate as needed (default)
wireleap config broker.accesskey.use_on_demand true

# manually (only needed if broker.accesskey.use_on_demand is false)
wireleap accesskeys activate
```

### Circuit

The circuit defines which relays will be used to transmit traffic. Each
relay enrolled into a contract assigns itself a role related to its
position in the connection circuit. A `fronting` relay provides an
on-ramp to the routing layer, while a `backing` relay provides an exit
from the routing layer. `entropic` relays add additional entropy to the
circuit in the routing layer.

Depending on requirements, `broker.circuit.hops` may be any positive
integer, setting the amount of relays used in a circuit. The amount of
hops specified implicitly asserts the relay roles as well.

Hops | Fronting | Entropic | Backing
---- | -------- | -------- | -------
`1`  | 0        | 0        | 1
`2`  | 1        | 0        | 1
`3+` | 1        | N        | 1

```shell
# set the number of circuit hops (will auto-generate a new circuit)
wireleap config broker.circuit.hops 3
```

A circuit is generated by randomly selecting from the available relays
enrolled in a service contract. Additionally, a whitelist may be
specified allowing the creation of an exact circuit when coupled with a
specific amount of hops, or a more general *only use these relays*.

```shell
# set the number of circuit hops
wireleap config broker.circuit.hops 1

# set a whitelist of relays to use
wireleap config broker.circuit.whitelist "wireleap://relay1.example.com:13499"

# manually trigger new circuit generation
wireleap reload

# display currently active circuit
wireleap status
```

An initial circuit is generated upon launch and regenerated either
automatically if issues are encountered or when the wireleap daemon
receives the `SIGUSR1` signal (which also happens when settings are
modified via `wireleap config` or a reload is requested via `wireleap
reload`).

## Forwarders

### Specific traffic (SOCKSv5)

To forward specific traffic on a system through the `wireleap`
connection broker, it is possible to use `wireleap socks`.

The `wireleap socks` subcommand will use the bundled `wireleap_socks` binary
(unpacked on `wireleap init`). Once the `wireleap` SOCKS forwarder is
running, any application that supports the `SOCKSv5` protocol can be
configured to route its traffic via the connection broker.

```shell
# start the wireleap controller (if not already running)
wireleap start
wireleap status

# start the socks daemon
wireleap socks start
wireleap socks status

# any traffic sent to the socks forwarder should now be tunneled...

# show the log
wireleap socks log

# (at some later time) stop the wireleap socks daemon
wireleap socks stop
```

#### proxy settings

Unfortunately, there is no standard for configuration so a few examples
are provided.

```shell
# manually specifying the --proxy flag
curl --proxy socks5h://$(wireleap config forwarders.socks.address) URL

# manually exporting environment variable
export ALL_PROXY="socks5h://$(wireleap config forwarders.socks.address)"
curl URL

# manually specifying the --proxy-server flag
google-chrome \
    --proxy-server="socks5://$(wireleap config forwarders.socks.address)" \
    --host-resolver-rules="MAP * ~NOTFOUND, EXCLUDE 127.0.0.1" \
    --user-data-dir="$HOME/.config/google-chrome-wireleap" \
    --incognito

# manually configuring firefox
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

#### wireleap exec

As mentioned above, there is no standard for proxy configuration among
applications, so a few wrapper scripts are included in
`scripts/default/` which can be executed by invoking `wireleap exec`.

```shell
# list available exec commands
wireleap exec list

# example usage
wireleap exec curl URL
wireleap exec git clone URL
wireleap exec firefox [URL]
wireleap exec google-chrome [URL]
wireleap exec chromium-browser [URL]
wireleap exec brave-browser [URL]
```

It is possible to create custom user-defined scripts. They should
be placed in `scripts` which take preference over the default scripts,
and will not be overwritten on upgrade.

When a script is executed, the following environmental variables will be
available inside the script for convenience:

- `WIRELEAP_HOME`
- `WIRELEAP_SOCKS`
- `WIRELEAP_SOCKS_HOST`
- `WIRELEAP_SOCKS_PORT`

#### wireleap intercept

For applications that do not support proxying via the `SOCKSv5` protocol
natively (or even those that do), it may be possible to use `wireleap
intercept` (experimental: Linux only).

The `wireleap_intercept.so` library is used by `wireleap intercept` to
intercept network connections from arbitrary programs and tunnel them
through the configured circuit.

```shell
wireleap intercept curl URL
wireleap intercept ssh USER@HOST
```

### All traffic (TUN)

To forward all traffic on a system (both TCP and UDP) through the
`wireleap` connection broker, it is possible to use `wireleap tun`
(currently Linux and macOS only).

The `wireleap tun` subcommand will use the bundled `wireleap_tun` binary
(unpacked on `wireleap init`) to set up a
[tun device](https://en.wikipedia.org/wiki/TUN/TAP) and configure
default routes through it so that all traffic from the local system goes
through the tun device, effectively meaning that it is routed through
the currently used wireleap broker circuit.

Note: `wireleap_tun` needs sufficient privileges to create a tun device
and manage routes during the lifetime of the daemon, hence the `suid
bit`. Alternatively, `wireleap tun` commands can be run with `sudo` or
`su` (as root).

```shell
# set suid bit
sudo chown 0:0 $HOME/wireleap/wireleap_tun
sudo chmod u+s $HOME/wireleap/wireleap_tun
```

```shell
# start the wireleap controller (if not already running)
wireleap start
wireleap status

# setup tun device, configure routes, and verify its running
wireleap tun start
wireleap tun status

# all tcp/udp traffic on the system should now be tunneled...

# show the log
wireleap tun log

# (at some later time) stop the wireleap tun daemon
wireleap tun stop
```

#### Potential application firewall issues

Application firewalls could interfere with `wireleap tun`, making it
fail silently instead of tunneling through the configured circuit.

For example on macOS, if the built-in [application firewall](https://support.apple.com/en-us/HT201642)
is enabled this could happen. To remedy this, a firewall rule needs to
be added for `wireleap`.

Try [adding the wireleap binary manually](https://support.apple.com/guide/mac-help/block-connections-to-your-mac-with-a-firewall-mh34041/mac#mchlp218b2b0)
in the firewall settings with the firewall mode set to **Allow incoming
connections**.

If that does not work, disabling the firewall altogether would allow
`wireleap tun` to work. However, please note that disabling the firewall
may affect the security the system.

## Upgrade

The precompiled binary of `wireleap` includes manual upgrade
functionality. Due to protocol versioning, it is highly recommended to
keep the client up to date. A client which is out of date with regard to
the directory's required client version will refuse to run.

The client update channels supported by the directory and the respective
latest version is exposed via the directory's `/info` endpoint. The
upgrade process is interactive so you will have the possibility to
accept or decline based on the changelog for the new release version.

```shell
wireleap upgrade
```

If the upgrade was successful, the old binary is not deleted but kept as
`wireleap.prev` for rollback purposes, in case issues manifest
post-upgrade.

```shell
wireleap rollback
```

If the upgrade was not successful, it is possible to skip the faulty
version explicitly. For example, the following with skip upgrades to
version 1.2.3.

Note: since the `.skip-upgrade-version` file has to be valid JSON, the
version number to be skipped should be quoted.

Linux/macOS:

```shell
# skip upgrades to version 1.2.3
echo '"1.2.3"' > $HOME/wireleap/.skip-upgrade-version
```

Windows:

```powershell
echo '"1.2.3"' > ((Split-Path -Parent (Get-Command wireleap).Source) + "\.skip-upgrade-version")
```

## Files

The client stores its configuration and other essential files on the
filesystem in the same directory as the `wireleap` binary. It can be any
directory but `$HOME/wireleap` is a sensible value.

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
├── wireleap_socks
├── wireleap_socks.pid
├── wireleap_socks.log
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

Some of the files are described below:

**contract.json**

Contains a snapshot of the `/info` API endpoint contents of the
currently used service contract.

**servicekey.json**

If present, contains the currently active servicekey for the currently
active service contract. If `broker.accesskey.use_on_demand` is set to
`true`, it is generated automatically using the proofs of funding from
`pofs.json`. If `broker.accesskey.use_on_demand` is set to `false` and an
expired servicekey is read from this file, `wireleap` will return an
error. In that case, a new key can be generated via `wireleap
accesskeys activate`.

**pofs.json**

Contains the list of proof-of-funding tokens for the currently active
service contract obtained from importing `accesskeys.json` files. It is
managed automatically by `wireleap` if `broker.accesskey.use_on_demand`
is set to `true`.  Alternatively, it can be managed manually via the
`wireleap accesskeys activate` command.

**relays.json**

Contains the list of known relays of the currently active service
contract obtained from its relay directory. It is refreshed on startup,
reload or when `wireleap` receives the `SIGUSR1` signal.

**wireleap.pid**

Contains the PID (process ID) of the currently running `wireleap start`
daemon (if any). It can be used to send control signals to the daemon
such as `SIGUSR1` to reload config file and contract info, and `SIGINT`,
`SIGTERM` or `SIGQUIT` to terminate gracefully.

**wireleap_intercept.so**

This is the library which is used by `wireleap intercept` to intercept
connections from arbitrary programs and tunnel them through the
configured network circuit. The command `wireleap intercept`, this file
and their associated command line options and configuration variables
are only present on Linux.

**scripts/**

This directory contains scripts to be run via `wireleap exec`. On
execution, the `WIRELEAP_SOCKS` environmental variable will be available
inside the script containing the current `wireleap` SOCKSv5 listening
address.

This directory is for user-defined scripts which take preference over
the default scripts (described below).

Scripts on Windows have the `.bat` extension standard for batch files.

**scripts/default/**

This directory contains default `wireleap`-supplied scripts. If
modifications are required, just save your version of the script under
the same name in `scripts/` as the `scripts/default/` script you wish to
alter. This ensures that updates will not overwrite user changes to
scripts.

Scripts on Windows have the `.bat` extension standard for batch files.

## Versioning

Releases are based on [semantic versioning](https://semver.org),
and use the format `MAJOR.MINOR.PATCH`. While the MAJOR version is `0`,
MINOR version bumps are considered MAJOR bumps per the semver spec.

Git tags are used to specify the software version, which are manually
assigned by tagging the relevant changelog entry. Only tagged versions
are CI-built and released after all unit and integration tests have
passed successfully.

Note: Locally built binaries will include a suffix in addition to the
latest tagged version, consisting of the number of commits past the tag
and the abbreviated hash of the HEAD commit.

## Building

Note: If you would like to make changes to the source code, please
following the [contributing](#contributing) instructions instead.

**Clone the repository**

```shell
git clone https://github.com/wireleap/client.git
```

**Checkout the latest tagged version**

For locally built binaries to match the latest stable `wireleap`
version, you will need to check out the latest git tag prior to
building as opposed to building from master.

```shell
cd client
git pull --tags origin master
git checkout $(git describe `git rev-list --tags --max-count=1`)
```

**Build the binary**

It is recommended to build the binary using docker, as described below
which uses the official `golang` docker image.

```shell
# for your host operating system
./contrib/docker/build-bin.sh build/

# for a specific target os (linux / darwin)
TARGET_OS=linux ./contrib/docker/build-bin.sh build/

# specify a cache for faster subsequent builds
mkdir -p build/.deps
DEPS_CACHE=build/.deps ./contrib/docker/build-bin.sh build/
```

If you prefer to use your host system instead of docker, you can do so
with `contrib/build-bin.sh` provided you have the relevant dependencies
installed.

## Contributing

This flow is loosely based on the standard [GitHub flow][github_flow]
collaborative development model.

Collaboration between developers is facilitated via pull requests from
topic branches towards the `master` branch, and pull request reviews are
used to achieve consensus before merging the changes into the `master`
branch.

A note about the `master` branch:

- Anything in the master branch is deployable, builds successfully and
  is tested to work. The CI/CD system performs both integration and unit
  tests, but should be considered as only a filter to immediately
  highlight PRs which would break the master branch and therefore need
  to be either discarded or amended. Automated checks are no substitute
  for code review, so all PRs are manually reviewed prior to merge.

- Direct commits to the master branch are **prohibited**, with the
  only exception being a core-dev pushing a signed git-tag signifying a
  release.

[github_flow]: https://guides.github.com/introduction/flow/

**Fork, clone and setup upstream remote**

The following instructions outline the recommended procedure for
creating a fork of this repository in order to contribute changes.

Firstly, click the `fork` button at the top of the page. Once forked,
clone your fork and set an upstream remote to keep track of changes.

```shell
git clone git@github.com:USERNAME/client.git

cd client
git remote add upstream git@github.com:wireleap/client.git
git checkout master
git pull --tags upstream master
git config commit.gpgsign true
```

**Create a feature branch and make your changes**

Create a descriptively named topic branch based on the `master` branch.
Please take care to only address **one** issue/bug/feature per pull
request.

```shell
git checkout master
git pull --tags upstream master
git checkout -b DESCRIPTIVE_BRANCH_NAME
```

When making your changes, test and commit as you go. Try to make commits
that capture an atomic change to the codebase. Source code should be
documented where necessary and the rationale for changes included in
commits should be clear.

If a commit resolves a known issue or relates to other commits or PRs,
please refer to them.

**Unit testing**

The unit tests can either be run on your host or within docker using the
official golang docker image.

```shell
# run unit tests on host
./contrib/run-tests.sh

# run unit tests in docker
./contrib/docker/run-tests.sh

# run unit tests in docker (specify cache for faster subsequent tests)
mkdir -p build/.deps
DEPS_CACHE=build/.deps ./contrib/docker/run-tests.sh
```

**Rebase on master if needed**

It can happen that as you were working on a feature, the state of the
`upstream/master` branch has changed due to merging other pull requests.
In this case, rebase your topic branch on top of the `master` branch. If
needed, resolve merge conflicts.

```shell
git checkout master
git fetch upstream
git merge upstream/master
git rebase --interactive master DESCRIPTIVE_BRANCH_NAME
```

After every change to the git history of your topic branch, perform
testing to avoid regressions.

**Push changes and submit a pull request**

When you think the topic branch is ready for merging, passes all tests,
all changes are committed with appropriate commit messages, and your
topic branch is based on the current state of the `upstream/master`
branch, push them to the **topic branch** (not master) of your fork.

```shell
# push changes
git push origin DESCRIPTIVE_BRANCH_NAME

# if you have already pushed commits to a topic branch, and later
# performed a rebase on top of master, a force push will be required
git push --force origin DESCRIPTIVE_BRANCH_NAME
```

Once pushed, follow the link specified in the `git push` output. Give
your changes a last-minute correctness check, and supply the high-level
description of the changes.

Finally, click `create pull request` so the reviewers can review and
approve the changes, or request modifications prior to performing the
merge.

**Review process and merge**

The pull request may be approved or additional modifications might be
requested by one of the reviewers. If modifications are requested,
commit and push more changes to the **same** topic branch and they will
be included in the original pull request until it is ultimately closed.

Branch protection rules are in place. They include:

- Requiring all commits in PRs to be signed.
- Requiring all integration and unit tests to complete successfully.
- Requiring at least one approval from a core-dev.

If there is an issue with the proposed changes, modifications should be
requested. For discussions on the rationale of certain choices in the
code, GitHub comments in the respective files can be left for the author
of the pull request to address.

Please note that every merged pull request is considered final and it is
always better to hold off on merging a pull request than have to open
another one correcting the changes from the first one. Additionally, it
is also sometimes a good idea to create pull requests towards another
PRs topic branch instead of master. This allows unifying multiple sets
of changes from different developers within the scope of a single PR.

Merging changes that are not unanimously approved by all reviewers is
**not** allowed unless special arrangements are in place (e.g. a
reviewer is away and explicitly asked to not wait on them for merging
changes).

Once the above is satisfied and all the reviewers have approved the
changes, the last person who gives their approval and has merge
permissions will close the pull request by merging it into the `master`
branch. However, if the author of the pull request has merge
permissions, they may perform the merge subject to the above.

## License

The MIT License (MIT)

