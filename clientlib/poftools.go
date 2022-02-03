// Copyright (c) 2021 Wireleap

package clientlib

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/wireleap/client/clientcfg"
	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/pof"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/api/status"
	"github.com/wireleap/common/cli/fsdir"
)

type SKSourceFunc func(bool) (*servicekey.T, error)

// function to get a fresh sk if at all possible
func SKSource(fm fsdir.T, c *clientcfg.C, cl *client.Client) SKSourceFunc {
	var mu sync.Mutex
	var sk *servicekey.T
	return func(fetch bool) (r *servicekey.T, err error) {
		mu.Lock()
		defer mu.Unlock()

		if sk == nil {
			fm.Get(&sk, "servicekey.json")
		}
		if sk != nil && sk.Contract != nil && !sk.IsExpiredAt(time.Now().Unix()) {
			log.Printf(
				"found existing servicekey %s",
				sk.PublicKey,
			)
			return sk, nil
		}
		if !c.Broker.Accesskey.UseOnDemand {
			return nil, fmt.Errorf("no fresh servicekey available and accesskey.use_on_demand is false")
		}
		if !fetch {
			return nil, fmt.Errorf("no activated servicekey available")
		}
		// discard old servicekey & get a new one
		sk, err = RefreshSK(fm, func(p *pof.T) (*servicekey.T, error) {
			if c.Contract == nil {
				return nil, fmt.Errorf("no contract defined")
			}
			return NewSKFromPof(
				cl,
				c.Contract.String()+"/servicekey/activate",
				p,
			)
		})
		return sk, err
	}
}

type AlwaysFetchFunc func() (*servicekey.T, error)

func AlwaysFetch(f SKSourceFunc) AlwaysFetchFunc {
	return func() (*servicekey.T, error) { return f(true) }
}

func NewSKFromPof(cl *client.Client, skurl string, p *pof.T) (*servicekey.T, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	sk := servicekey.New(priv)
	req := &pof.SKActivationRequest{Pubkey: sk.PublicKey, Pof: p}
	if err = cl.Perform(http.MethodPost, skurl, req, sk.Contract); err != nil {
		return nil, fmt.Errorf("error while performing SK activation request: %w", err)
	}
	return sk, nil
}

func PickPofs(pofs ...*pof.T) (r []*pof.T) {
	for _, p := range pofs {
		if !p.IsExpiredAt(time.Now().Unix()) {
			// this one has not expired yet
			r = append(r, p)
		}
	}
	return r
}

func PickSK(sks ...*servicekey.T) (sk *servicekey.T) {
	for _, k := range sks {
		if !k.IsExpiredAt(time.Now().Unix()) {
			// this one has not expired yet
			sk = k
			break
		}
	}
	return
}

type Activator func(*pof.T) (*servicekey.T, error)

func RefreshSK(fm fsdir.T, actf Activator) (sk *servicekey.T, err error) {
	ps := []*pof.T{}
	if err = fm.Get(&ps, filenames.Pofs); err != nil {
		return nil, fmt.Errorf(
			"could not open %s: %s; did you run `wireleap import`?",
			filenames.Pofs,
			err,
		)
	}
	ps = PickPofs(ps...)
	if len(ps) == 0 {
		return nil, fmt.Errorf("no fresh pofs available")
	}
	newps := []*pof.T{}
	// filter pofs & get sk
	for _, p := range ps {
		if sk == nil {
			log.Printf(
				"generating new servicekey from pof %s...",
				p.Digest(),
			)
			sk, err = actf(p)
			if err != nil {
				log.Printf(
					"failed generating new servicekey from pof %s: %s",
					p.Digest(),
					err,
				)
				if errors.Is(err, status.ErrSneakyPof) {
					// skip already used pof
					continue
				}
				// keep if other error
				newps = append(newps, p)
				continue
			}
			// skip successfully-used pof
			continue
		}
		// keep the rest untouched
		newps = append(newps, p)
	}
	// write new pofs
	if err = fm.Set(&newps, filenames.Pofs); err != nil {
		return nil, fmt.Errorf(
			"could not write new %s: %s",
			filenames.Pofs,
			err,
		)
	}
	if sk == nil {
		return nil, fmt.Errorf("no servicekey available")
	}
	// write new servicekey
	if err = fm.Set(&sk, filenames.Servicekey); err != nil {
		return nil, fmt.Errorf(
			"could not write new %s: %s",
			filenames.Servicekey,
			err,
		)
	}
	return sk, nil
}
