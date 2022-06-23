package restapi

import (
	"time"

	"github.com/wireleap/common/api/pof"
	"github.com/wireleap/common/api/servicekey"
	"github.com/wireleap/common/api/texturl"
)

type AccesskeyImportRequest struct {
	URL *texturl.URL `json:"url"`
}

type AccesskeyReply struct {
	Contract   *texturl.URL `json:"contract"`
	Duration   int64        `json:"duration"`
	State      string       `json:"state"`
	Expiration int64        `json:"expiration"`
}

func (t *T) accesskeysFromSks(sks ...*servicekey.T) (rs []*AccesskeyReply) {
	ci := t.br.ContractInfo()
	for _, sk := range sks {
		if sk == nil {
			continue
		}
		state := "active"
		if sk.IsExpiredAt(time.Now().Unix()) {
			state = "expired"
		}
		rs = append(rs, &AccesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      state,
			Expiration: sk.Contract.SettlementOpen,
		})
	}
	return
}

func (t *T) accesskeysFromPofs(pofs ...*pof.T) (rs []*AccesskeyReply) {
	ci := t.br.ContractInfo()
	for _, p := range pofs {
		if p == nil {
			continue
		}
		rs = append(rs, &AccesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      "inactive",
			Expiration: p.Expiration,
		})
	}
	return
}

func (t *T) newAccesskeysReply() (rs []*AccesskeyReply) {
	rs = append(rs, t.accesskeysFromSks(t.br.CurrentSK())...)
	rs = append(rs, t.accesskeysFromPofs(t.br.CurrentPofs()...)...)
	return
}
