// Copyright (c) 2021 Wireleap

package clientlib

import (
	"fmt"

	"github.com/wireleap/client/filenames"
	"github.com/wireleap/common/api/client"
	"github.com/wireleap/common/api/consume"
	"github.com/wireleap/common/api/contractinfo"
	"github.com/wireleap/common/api/relaylist"
	"github.com/wireleap/common/api/texturl"
	"github.com/wireleap/common/cli/fsdir"
)

func GetContractInfo(cl *client.Client, sc *texturl.URL) (info *contractinfo.T, rl relaylist.T, err error) {
	if info, err = consume.ContractInfo(cl, sc); err != nil {
		err = fmt.Errorf(
			"could not get contract info for %s: %s",
			sc.String(), err,
		)
		return
	}
	if rl, err = consume.ContractRelays(cl, sc); err != nil {
		err = fmt.Errorf(
			"could not get contract relays for %s: %s",
			sc.String(), err,
		)
	}
	return
}

func SaveContractInfo(fm fsdir.T, ci *contractinfo.T, rl relaylist.T) (err error) {
	if err = fm.Set(ci, filenames.Contract); err != nil {
		return fmt.Errorf("could not save contract info: %s", err)
	}
	if err = fm.Set(rl, filenames.Relays); err != nil {
		return fmt.Errorf("could not save contract relays: %s", err)
	}
	return
}

func ContractInfo(fm fsdir.T) (ci *contractinfo.T, err error) {
	err = fm.Get(ci, filenames.Contract)
	return
}

func ContractURL(fm fsdir.T) *texturl.URL {
	ci, err := ContractInfo(fm)
	if err == nil {
		return ci.Endpoint
	} else {
		return nil
	}
}
