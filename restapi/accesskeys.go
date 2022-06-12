package restapi

import (
	"time"

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

func (t *T) newAccesskeysReply() (rs []*accesskeyReply) {
	ci, err := t.br.ContractInfo()
	if err != nil {
		return
	}
	// get contract info
	if sk := t.br.CurrentSK(); sk != nil {
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
	for _, p := range t.br.CurrentPofs() {
		rs = append(rs, &accesskeyReply{
			Contract:   ci.Endpoint,
			Duration:   int64(ci.Servicekey.Duration),
			State:      "inactive",
			Expiration: p.Expiration,
		})
	}
	return
}
