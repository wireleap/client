# Wireleap client API

## Table of Contents

- [Introduction](#introduction)
- [Versioning](#versioning)
- [Errors](#errors)
- [Core](#core)
    - [Controller](#controller)
        - [The controller object](#the-controller-object)
        - [Get controller status](#get-controller-status)
        - [Reload controller](#reload-controller)
        - [Get controller log](#get-controller-log)
    - [Configuration](#configuration)
        - [The config object](#the-config-object)
        - [Get configuration](#get-configuration)
        - [Set configuration](#set-configuration)
    - [Runtime](#runtime)
        - [The runtime object](#the-runtime-object)
        - [Get runtime object](#get-runtime-object)
- [Resources](#resources)
    - [Accesskey](#accesskey)
        - [The accesskey object](#the-accesskey-object)
        - [List all accesskeys](#list-all-accesskeys)
        - [Import accesskeys](#import-accesskeys)
        - [Activate new accesskey](#activate-new-accesskey)
    - [Relay](#relay)
        - [The relay object](#the-relay-object)
        - [List all relays](#list-all-relays)
    - [Contract](#contract)
        - [The contract object](#the-contract-object)
        - [Get active contract](#get-active-contract)
- [Forwarders](#forwarders)
    - [SOCKSv5](#socksv5)
        - [The SOCKS object](#the-socks-object)
        - [Get SOCKS information](#get-socks-information)
        - [Start SOCKS daemon](#start-socks-daemon)
        - [Stop SOCKS daemon](#stop-socks-daemon)
        - [Get SOCKS log](#get-socks-log)
    - [TUN](#tun)
        - [The TUN object](#the-tun-object)
        - [Get TUN information](#get-tun-information)
        - [Start TUN daemon](#start-tun-daemon)
        - [Stop TUN daemon](#stop-tun-daemon)
        - [Get TUN log](#get-tun-log)

## Introduction

> BASE URL

```shell
BASE_URL="$(wireleap config address)/api"
```

The Wireleap client controller provides a REST API, accepts JSON encoded
requests and responds with JSON-encoded responses, and uses standard
HTTP response codes.

## Versioning

> Get API version

```shell
$ curl $BASE_URL/version
```

> Response

```json
{
  "version": "0.1.0"
}
```

Releases are based on [semantic versioning](https://semver.org),
and use the format `MAJOR.MINOR.PATCH`. While the MAJOR version is `0`,
MINOR version bumps are considered MAJOR bumps per the semver spec.

The API exposes the endpoint `GET /version`, which should never change.
It should be used by API consumers to verify compatibilty.

For more versioning information, refer to the [runtime object](#the-runtime-object).

#### Attributes

Key     | Type     | Comment
---     | ----     | -------
version | `string` | Version of controller API interface

## Errors

> HTTP status code summary

```
200 - OK                 Everything worked as expected
400 - Bad Request        The request was unacceptable
401 - Unauthorized       No valid authentication
402 - Request Failed     Parameters valid but the request failed
403 - Forbidden          No permission to perform request
404 - Not Found          The requested resource doesn't exist
405 - Method Not Allowed The requested resource exists but method not supported
409 - Conflict           The request conflicts with another request
500-504 - Error          Server Errors
```

The REST API uses conventional HTTP response codes to indicate the
success or failure of an API request. In general: Codes in the `2xx` range
indicate success. Codes in the `4xx` range indicate an error that failed
given the information provided (e.g., a required parameter was omitted).
Codes in the `5xx` range indicate an error with the REST API server.

Most errors may include the `code`, `desc` and `cause` of the error in
the body of the response.

# Core

## Controller

> Endpoints

```
GET  /status
POST /reload
GET  /log
```

The Wireleap client controller is at the core of the client, providing
the `API` and `connection broker` functionality, as well as other
various functionality, such as controlling forwarding daemons.

### The controller object

> The controller object

```json
{
  "home": "/home/user/wireleap",
  "pid": 12345,
  "state": "active",
  "broker": {
    "active_circuit": [
      "wireleap://relay1.example.com:443/wireleap",
      "wireleap://relay3.example.com:13495"
    ]
  },
  "upgrade": {
    "required": false
  }
}
```

#### Attributes

Key                   | Type     | Comment
---                   | ----     | -------
home                  | `string` | Wireleap home directory path
pid                   | `int`    | PID of controller daemon
state                 | `string` | One of `active` `inactive` `activating` `deactivating` `failed` `unknown`
broker.active_circuit | `list`   | List of relays in active circuit
upgrade.required      | `bool`   | Whether upgrade is required per directory

### Get controller status

> Get controller status

```shell
$ curl $BASE_URL/status
```

Retrieves the current status of the controller.

#### Parameters

None

#### Returns

The `controller` object.

### Reload controller

> Reload controller

```shell
$ curl -X POST $BASE_URL/reload
```

Triggers the controller to reload the [client configuration](#configuration),
refreshes the relay list and regenerates the circuit.

#### Parameters

None.

#### Returns

The `controller` object.

### Get controller log

> Get controller log

```shell
$ curl $BASE_URL/log
```

> Response

```
2021/06/17 10:24:16 [controller] initializing...
2021/06/17 10:24:19 [api] listening on: [127.0.0.1:13490 ...
2021/06/17 10:24:19 [broker] listening on: [127.0.0.1:13490 ...
2021/06/17 10:24:55 [broker] found existing servicekey gLiurkcRy8KmVyJ...
2021/06/17 10:24:55 [broker] Connecting to circuit link: fronting wire...
2021/06/17 10:24:55 [broker] Connecting to circuit link: backing wirel...
```

Retrieves the controller logs.

#### Parameters

None

#### Returns

Returns contents of `wireleap.log`.


## Configuration

> Endpoints

```
GET  /config
POST /config
```

The client configuration is stored in `config.json`. This file is
automatically created upon `wireleap init`. These endpoints provide an
interface for querying and manipulating the configuration.

### The config object

> The config object

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

#### Attributes

Key                            | Type     | Comment
---                            | ----     | -------
address                        | `string` | Provides `/`, `/broker`, `/api`
broker.address                 | `string` | Override default broker address
broker.accesskey.use_on_demand | `bool`   | Activate accesskeys as needed
broker.circuit.timeout         | `string` | Dial timeout duration
broker.circuit.hops            | `int`    | Number of relays to use in a circuit
broker.circuit.whitelist       | `list`   | Whitelist of relay addresses to use in circuit
forwarders.socks.address       | `string` | SOCKSv5 proxy address
forwarders.tun.address         | `string` | TUN device address (not loopback)

#### Circuit notes

The circuit defines which relays will be used to transmit traffic. Each
relay enrolled into a contract assigns itself a role related to its
position in the connection circuit. A **fronting** relay provides an on-ramp
to the routing layer, while a **backing** relay provides an exit from the
routing layer. **entropic** relays add additional entropy to the circuit in
the routing layer.

Depending on requirements, **broker.circuit.hops** may be any positive integer,
setting the amount of relays used in a circuit. The amount of hops
specified implicitly asserts the relay roles as well.

Hops | Fronting | Entropic | Backing
---- | -------- | -------- | -------
`1`  | 0        | 0        | 1
`2`  | 1        | 0        | 1
`3+` | 1        | N        | 1

A circuit is generated by randomly selecting from the available relays
enrolled in a service contract. Additionally, the
**broker.circuit.whitelist** may be specified allowing the creation of
an exact circuit when coupled with a specific amount of hops, or a more
general only use these relays.

### Get configuration

> Get config

```shell
$ curl $BASE_URL/config
```

Retrieves the current configuration.

#### Parameters

None

#### Returns

The `config` object.

### Set configuration

> Set config

```shell
$ curl -X POST $BASE_URL/config \
  -H 'Content-Type: application/json' \
  -d '{"broker": {"accesskey": {"use_on_demand": true}}}'

$ curl -X POST $BASE_URL/config \
  -H 'Content-Type: application/json' \
  -d '{"broker": {"circuit": {"hops": 1, "whitelist": ["..."]}}}'
```

Set configuration values (note: not all settings can be changed via the
API).

A [controller reload](#reload-controller) will be triggered upon success.

#### Parameters

Key                            | Type     | Comment
---                            | ----     | -------
broker.accesskey.use_on_demand | `bool`   | Activate accesskeys as needed
broker.circuit.timeout         | `string` | Dial timeout duration
broker.circuit.hops            | `int`    | Number of relays to use in a circuit
broker.circuit.whitelist       | `list`   | Whitelist of relay addresses to use in a circuit

#### Returns

The `config` object.


## Runtime

> Endpoints

```
GET  /runtime
```

Versions and build information of the Wireleap client, its capabilities
and the platform host system the client is running on.

### The runtime object

> The runtime object

```json
{
  "versions": {
    "software": "0.6.0",
    "api": "0.1.0",
    "client-contract": "0.1.0",
    "client-dir": "0.2.0",
    "client-relay": "0.2.0"
  },
  "upgrade": {
    "supported": true,
  },
  "build": {
    "gitcommit": "a2e84ec0a30ba19961f4fcdc21e9b7ea4a47cf57",
    "goversion": "go1.16",
    "time": 1591959182,
    "flags": []
  },
  "platform": {
    "os": "linux",
    "arch": "amd64"
  }
}
```

#### Attributes

Key                      | Type     | Comment
---                      | ----     | -------
versions.software        | `string` | Version of client software
versions.api             | `string` | Version of controller API interface
versions.client-contract | `string` | Client/Contract interface version
versions.client-dir      | `string` | Client/Directory interface version
versions.client-relay    | `string` | Client/Relay interface version
upgrade.supported        | `bool`   | Whether inline upgrades are supported in the build
build.gitcommit          | `string` | Git commit the build was built from
build.goversion          | `string` | GoLang version used to build
build.time               | `int64`  | Unix epoch time of build
build.flags              | `list`   | List of flags specified at build time
platform.os              | `string` | Name of operating system
platform.arch            | `string` | Name of architecture

### Get runtime object

> Get runtime object

```shell
curl $BASE_URL/runtime
```

Retrieves runtime information.

#### Parameters

None

#### Returns

The `runtime` object.

# Resources

## Accesskey

> Endpoints

```
GET  /accesskeys
POST /accesskeys/import
POST /accesskeys/activate
```

Accesskeys are a convenience abstraction to _proof of funding_'s and _service
keys_, and include contract metadata for association when originally
[imported](#import-accesskeys).

An accesskey is required to use relays enrolled in a service contract.
Accesskeys are provided by contracts after obtaining access. They are
used to cryptographically and independently generate tokens by the
client for each relay in the routing path (circuit), which are included
in the appropriate encrypted onion layer of traffic being sent, allowing
the relay to authorize service. This increases the degrees of separation
between payment information and network usage.

A _proof of funding_ is used to activate servicekeys, which can be done
automatically ([broker.accesskey.use_on_demand](#the-config-object))
when needed (e.g., previous one has expired), or can be manually
[generated and activated](#activate-new-accesskey).

### The accesskey object

> The accesskey object

```json
{
  "contract": "https://contract1.example.com",
  "duration": 86400,
  "status": "active",
  "expiration": 1651570846
}
```

#### Attributes

Key                              | Type     | Comment
---                              | ----     | -------
contract                         | `string` | Associated contract endpoint
duration                         | `int`    | Duration in seconds of accesskey from activating until expiry
state                            | `string` | One of `active` ( _sk_ ) `inactive` ( _pof_ ) `expired` ( _sk_ or _pof_ )
expiration _(if state `inactive`)_ | `int64`  | Unix time when must be activated ( _pof.expiration_ )
expiration _(if state `active`)_   | `int64`  | Unix time when duration runs out ( _sk.contract.settlement_close_ )

### List all accesskeys

> List all accesskeys

```shell
$ curl $BASE_URL/accesskeys
```

Retrieves accesskeys.

#### Parameters

None.

#### Returns

List of `accesskey` objects.


### Import accesskeys

> Import accesskeys

```shell
$ curl -X POST $BASE_URL/accesskeys/import \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://example.com/accesskeys/..."}'

$ curl -X POST $BASE_URL/accesskeys/import \
  -H 'Content-Type: application/json' \
  -d '{"url": "file:///path/to/accesskeys.json"}'
```

Provides an interface for importing accesskeys.

#### Parameters

Key | Type     | Comment
--- | ----     | -------
url | `string` | URL of accesskeys to import (supported schemes: `https` `file`)

#### Returns

List of `accesskey` objects imported.


### Activate new accesskey

> Activate new accesskey

```shell
$ curl -X POST $BASE_URL/accesskeys/activate
```

Activate a new access key.

This is only needed if [broker.accesskey.use_on_demand](#the-config-object) is set to `false`.

#### Parameters

None

#### Returns

The `accesskey` object.


## Relay

> Endpoints

```
GET  /relays
```

Wireleap relays are used to relay traffic from clients and other
relays, depending on their designated role.

Each relay enrolled into a contract assigns itself a role related to its
position in the connection circuit. A `fronting` relay provides an
on-ramp to the routing layer, while a `backing` relay provides an exit
from the routing layer. `entropic` relays add additional entropy to the
circuit in the routing layer.

Circuit settings can be configured [here](#the-config-object).

### The relay object

> The relay object

```json
{
  "role": "backing",
  "address": "wireleap://relay3.example.com:13495",
  "pubkey": "bZ3ppgVRz3wPSsJy2o_1KRBrySCzOz9OHdxSwP0riCk",
  "versions": {
    "software": "0.5.1",
    "client-relay": "0.2.0",
  }
}
```

#### Attributes

Key      | Type     | Comment
---      | ----     | -------
role     | `string` | Type of relay (`fronting`, `backing`, `entropic`)
address  | `string` | Address of relay
pubkey   | `string` | Public key of relay
versions | `dict`   | Relay and interface versions

### List all relays

> List all relays

```shell
$ curl $BASE_URL/relays
```

Retrieves the list of all available relays enrolled in the configured
[contract](#contract) relay directory.

#### Parameters

None

#### Returns

List of `relay` objects.


## Contract

> Endpoints

```
GET  /contract
```

A service contract acts as an intermediary between customers (users) and
service providers (relays). A service contract defines the service
parameters, and facilitates disbursing funds provided by a customer to
service providers in proportion to service provided based on proof of
service.

### The contract object

> The contract object

```json
{
  "pubkey": "JcqJBnw7OOYSDDDTQg3N7gtP1BFK-VkRZk-HGQRyOBY",
  "version": "0.3.1",
  "endpoint": "https://contract1.example.com",
  "proof_of_funding": [
    {
      "type": "basic",
      "endpoint": "https://example.net/accesskeys",
      "pubkey": "hd8O-YaYb8tCDNxdxKSszQkWB3qer-N1oZJktcsJ6Es"
    }
  ],
  "servicekey": {
    "duration": "24h0m0s",
    "currency": "usd",
    "value": "100"
  },
  "directory": {
    "endpoint": "https://directory.example.com/",
    "public_key": "P8DPGkuxhPqZsf7C0Qem8MD7DcQ0qGjh-zayTOrXxlI"
  },
  "metadata": {
    "operator": "Example Inc.",
    "operator_url": "https://example.com",
    "name": "Example Contract 01",
    "terms_of_service": "https://example.com/legal/tos/",
    "privacy_policy": "https://example.com/legal/privacy/"
  }
}
```

#### Attributes

Key                       | Type     | Comment
---                       | ----     | -------
pubkey                    | `string` | Contract public key
version                   | `string` | Contract version
endpoint                  | `string` | Contract endpoint URL
proof_of_funding.type     | `string` | Proof of funding type
proof_of_funding.pubkey   | `string` | Proof of funding signer public key
proof_of_funding.endpoint | `string` | URL for obtaining proof of funding
servicekey.duration       | `string` | Duration of an activated servicekey until it expires
servicekey.currency       | `string` | Backed value currency
servicekey.value          | `int`    | Backed value of servicekey (smallest currency denomination)
directory.endpoint        | `string` | URL of public relays supporting service contract
metadata                  | `dict`   | Contract associated metadata

### Get active contract

> Get active contract

```shell
curl $BASE_URL/contract
```

Retrieves the current snapshot of the active contract's `/info`
endpoint.

#### Parameters

None

#### Returns

The `contract` object.


# Forwarders

## SOCKSv5

> Endpoints

```
GET  /socks
POST /socks/start
POST /socks/stop
GET  /socks/log
```

Provides an interface to manage the `wireleap_socks` daemon.

Any application that supports the SOCKSv5 protocol can be configured to
route its traffic to the wireleap socks forwarder, which in turn
forwards the traffic via the controller connection broker.

### The SOCKS object

> The SOCKS object

```json
{
  "pid": 12346,
  "state": "active",
  "address": "127.0.0.1:13491",
  "binary": {
    "ok": true,
    "status": {
      "exists": true,
      "chmod_x": true,
    }
  }
}
```

#### Attributes

Key           | Type     | Comment
---           | ----     | -------
pid           | `int`    | PID of SOCKSv5 daemon
state         | `string` | One of `active` `inactive` `activating` `deactivating` `failed` `unknown`
address       | `string` | SOCKSv5 address
binary.ok     | `bool`   | Whether SOCKSv5 binary passed all required verification checks
binary.status | `dict`   | SOCKSv5 binary status verification checks results

### Get SOCKS information

> Get SOCKS information

```shell
$ curl $BASE_URL/socks
```

Retrieves the current status of the SOCKS daemon.

#### Parameters

None

#### Returns

The `socks` object.

### Start SOCKS daemon

> Start SOCKS daemon

```shell
$ curl -X POST $BASE_URL/socks/start
```

Starts the SOCKS daemon.

#### Parameters

None

#### Returns

The `socks` object.

### Stop SOCKS daemon

> Stop SOCKS daemon

```shell
$ curl -X POST $BASE_URL/socks/stop
```

Stops the SOCKS daemon.

#### Parameters

None

#### Returns

The `socks` object.

### Get SOCKS log

> Get SOCKS log

```shell
$ curl $BASE_URL/socks/log
```

> Response

```
2021/06/17 10:24:19 listening on: [socksv5://127.0.0.1:13491 ...
2021/06/17 10:24:55 SOCKSv5 tcp socket accepted: 127.0.0.1:45...
2021/06/17 10:24:55 SOCKSv5 tcp socket accepted: 127.0.0.1:45...
```

Retrieves the SOCKS daemon logs.

#### Parameters

None

#### Returns

Returns contents of `wireleap_socks.log`.


## TUN

> Endpoints

```
GET  /tun
POST /tun/start
POST /tun/stop
GET  /tun/log
```

Provides an interface to manage the `wireleap_tun` daemon.

All traffic on the system (both TCP and UDP) can be funneled through the
controller connection broker by starting wireleap_tun.

### The TUN object

> The TUN object

```json
{
  "pid": 12346,
  "state": "active",
  "address": "10.13.49.0:13492",
  "binary": {
    "ok": true,
    "status": {
      "exists": true,
      "chown_0": true,
      "chmod_x": true,
      "chmod_us": true
    }
  }
}
```

Note: `wireleap_tun` needs sufficient privileges to create a TUN device
and manage routes during the lifetime of the daemon, hence the suid bit
and verification checks.

#### Attributes

Key           | Type     | Comment
---           | ----     | -------
pid           | `int`    | PID of TUN daemon
state         | `string` | One of `active` `inactive` `activating` `deactivating` `failed` `unknown`
address       | `string` | TUN device address
binary.ok     | `bool`   | Whether TUN binary passed all required verification checks
binary.status | `dict`   | TUN binary status verification checks results

### Get TUN information

> Get TUN information

```shell
$ curl $BASE_URL/tun
```

Retrieves the current status of the TUN daemon.

#### Parameters

None

#### Returns

The `tun` object.

### Start TUN daemon

> Start TUN daemon

```shell
$ curl -X POST $BASE_URL/tun/start
```

Starting the TUN daemon will set up a TUN device and configure default
routes so that all traffic (both TCP and UDP) from the local system pass
through it and forwarded via the controller connection broker.

#### Parameters

None

#### Returns

The `tun` object.

### Stop TUN daemon

> Stop TUN daemon

```shell
$ curl -X POST $BASE_URL/tun/stop
```

Stops the TUN daemon.

#### Parameters

None

#### Returns

The `tun` object.

### Get TUN log

> Get TUN log

```shell
$ curl $BASE_URL/tun/log
```

> Response

```
2021/06/16 13:24:17 listening on tcp4 socket 10.13.49.0:13492...
2021/06/16 13:24:17 listening on tcp6 socket [fe80::7d3d:9577...
2021/06/16 13:24:17 adding route: {Ifindex: 820 Dst: 0.0.0.0/...
2021/06/16 13:24:17 adding route: {Ifindex: 820 Dst: 128.0.0....
2021/06/16 13:24:17 adding route: {Ifindex: 820 Dst: 2000::/3...
2021/06/16 13:24:17 adding route: {Ifindex: 3 Dst: 167.71.0.2...
2021/06/16 13:24:17 adding route: {Ifindex: 3 Dst: 167.71.0.2...
2021/06/16 13:24:17 adding route: {Ifindex: 3 Dst: 162.216.18...
2021/06/16 13:24:17 capturing packets from tun0 and proxying ...
```

Retrieves the TUN daemon logs.

#### Parameters

None

#### Returns

Returns contents of `wireleap_tun.log`.


