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

type accesskeyReply struct {
	Contract   *texturl.URL `json:"contract"`
	Duration   int64        `json:"duration"`
	State      string       `json:"state"`
	Expiration int64        `json:"expiration"`
}

func (t *T) accesskeysFromSks(sks ...*servicekey.T) (rs []*accesskeyReply) {
	ci := t.br.ContractInfo()
	for _, sk := range sks {
		state := "active"
		if sk.IsExpiredAt(time.Now().Unix()) {
			state = "expired"
		}
		rs = append(rs, &accesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      state,
			Expiration: sk.Contract.SettlementOpen,
		})
	}
	return
}

func (t *T) accesskeysFromPofs(pofs ...*pof.T) (rs []*accesskeyReply) {
	ci := t.br.ContractInfo()
	for _, p := range pofs {
		rs = append(rs, &accesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      "inactive",
			Expiration: p.Expiration,
		})
	}
	return
}

func (t *T) newAccesskeysReply() (rs []*accesskeyReply) {
	rs = append(rs, t.accesskeysFromSks(t.br.CurrentSK())...)
	rs = append(rs, t.accesskeysFromPofs(t.br.CurrentPofs()...)...)
	return
}
