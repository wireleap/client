// Copyright (c) 2021 Wireleap

package clientlib

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/wireleap/common/api/interfaces/clientrelay"
	"github.com/wireleap/common/api/relayentry"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/api/sharetoken"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/api/texturl"
	"github.com/wireleap/common/wlnet"
)

func CircuitDialer(
	skf func() (*servicekey.T, error),
	circuitf func() ([]*relayentry.T, error),
	dialf func(net.Conn, string, *url.URL, *wlnet.Init) (net.Conn, error),
) func(string, string) (net.Conn, error) {
	return func(protocol, target string) (c net.Conn, err error) {
		sk, err := skf()
		if err != nil {
			err = fmt.Errorf("could not obtain fresh servicekey: %w", err)
			return
		}
		circuit, err := circuitf()
		if err != nil {
			err = fmt.Errorf("could not obtain circuit: %w", err)
			return
		}
		var st *sharetoken.T
		for i, link := range circuit {
			log.Println(
				"Connecting to circuit link:",
				link.Role,
				link.Addr.String(),
				link.Pubkey.String(),
			)
			switch i {
			case 0:
				continue
			default:
				st, err = sharetoken.New(sk, circuit[i-1].Pubkey.T())
				if err != nil {
					return
				}
				c, err = dialf(c, "tcp", &link.Addr.URL, &wlnet.Init{
					Command:  "CONNECT",
					Protocol: "tcp",
					Remote:   link.Addr,
					Token:    st,
					Version:  &clientrelay.T.Version,
				})
				if err != nil {
					// return circuit-specific error
					err = &status.T{
						Code:   http.StatusBadGateway,
						Desc:   err.Error(),
						Origin: link.Pubkey.String(),
					}
					return
				}
			}
		}
		log.Printf("Now connecting to target: %s", target)
		st, err = sharetoken.New(sk, circuit[len(circuit)-1].Pubkey.T())
		if err != nil {
			return
		}
		u, err := url.Parse("target://" + target)
		if err != nil {
			return
		}
		c, err = dialf(c, "tcp", &circuit[len(circuit)-1].Addr.URL, &wlnet.Init{
			Command:  "CONNECT",
			Protocol: protocol,
			Remote:   &texturl.URL{*u},
			Token:    st,
			Version:  &clientrelay.T.Version,
		})
		if err != nil {
			return
		}
		return
	}
}
