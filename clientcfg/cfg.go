// Copyright (c) 2022 Wireleap

// Package clientcfg describes the configuration file format and data types
// used by wireleap.
package clientcfg

import (
	"time"

	"github.com/wireleap/common/api/duration"
)

// C is the type of the config struct describing the config file format.
type C struct {
	// Address describes the listening address of the api/controller.
	Address *string `json:"address,omitempty"`
	// Broker holds the settings specific to the wireleap broker.
	Broker Broker `json:"broker,omitempty"`
	// Forwarders holds the settings specific to the wireleap broker.
	Forwarders Forwarders `json:"forwarders,omitempty"`
}

type Broker struct {
	// Address describes the h2c listening address of the broker.
	Address *string `json:"address,omitempty"`
	// Timeout is the dial timeout for relay connections.
	Timeout duration.T `json:"timeout,omitempty"`
	// Accesskey is the section dealing with accesskey configuration.
	Accesskey Accesskey `json:"accesskey,omitempty"`
	// Circuit describes the configuration of the Wireleap connection circuit.
	Circuit Circuit `json:"circuit,omitempty"`
}

// Accesskey is the section dealing with accesskey configuration.
type Accesskey struct {
	// UseOnDemand sets whether pofs are used to generate new servicekeys
	// automatically.
	UseOnDemand bool `json:"use_on_demand,omitempty"`
}

// Circuit describes the configuration of the Wireleap connection circuit.
type Circuit struct {
	// Whitelist is the optional user-defined list of relays to use exclusively.
	Whitelist *[]string `json:"whitelist,omitempty"`
	// Hops is the desired number of hops to use for the circuit.
	Hops int `json:"hops,omitempty"`
}

// Forwarders describes the settings of the available forwarders.
type Forwarders struct {
	// Socks is the SOCKSv5 TCP and UDP listening address.
	Socks *string `json:"socks,omitempty"`
	// Tun is the listening address configuration for wireleap_tun.
	Tun *string `json:"tun,omitempty"`
}

// Defaults provides a config with sane defaults whenever possible.
func Defaults() C {
	var (
		sksaddr = "127.0.0.1:13491"
		h2caddr = "127.0.0.1:13492"
		tunaddr = "10.13.49.0:13493"
	)
	return C{
		Address: &h2caddr,
		Broker: Broker{
			Address: &h2caddr,
			Timeout: duration.T(time.Second * 5),
			Circuit: Circuit{Hops: 1},
		},
		Forwarders: Forwarders{
			Socks: &sksaddr,
			Tun:   &tunaddr,
		},
	}
}

type Meta struct {
	// Option name
	Name string
	// Name of the "type'
	Type string
	// Description
	Desc string
	// Pointer to value to feed to Unmarshal()
	Val interface{}
	// Whether the input needs to be quoted before calling Unmarshal()
	Quote bool
}

func (c *C) Metadata() []*Meta {
	return []*Meta{
		{"broker.address", "str", "H2C proxy address of wireleap daemon", &c.Broker.Address, true},
		{"broker.timeout", "str", "Dial timeout duration", &c.Broker.Timeout, true},
		{"broker.circuit.hops", "int", "Number of relay hops to use in a circuit", &c.Broker.Circuit.Hops, false},
		{"broker.circuit.whitelist", "list", "Whitelist of relays to use", &c.Broker.Circuit.Whitelist, false},
		{"broker.accesskey.use_on_demand", "bool", "Activate accesskeys as needed", &c.Broker.Accesskey.UseOnDemand, false},
		{"forwarders.socks", "str", "SOCKS5 proxy address of wireleap daemon", &c.Forwarders.Socks, true},
		{"forwarders.tun", "str", "TUN device address (not loopback)", &c.Forwarders.Tun, true},
	}
}
