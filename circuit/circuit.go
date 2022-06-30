// Copyright (c) 2021 Wireleap

package circuit

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/wireleap/common/api/interfaces/clientrelay"
	"github.com/wireleap/common/api/relayentry"
)

func init() { rand.Seed(time.Now().Unix()) }

type T []*relayentry.T

// Partition partitions an arbitrary circuit onto 3 parts: fronting, entropic
// and backing relays. It also excludes all relays which have a version which
// is incompatible with this wireleap.
func (t T) Partition() (fronting T, entropic T, backing T) {
	for _, r := range t {
		// exclude older protocol & incompatible relays
		if r.Versions.ClientRelay == nil || r.Versions.ClientRelay.Minor != clientrelay.T.Version.Minor {
			continue
		}

		// separate
		switch r.Role {
		case "fronting":
			fronting = append(fronting, r)
		case "entropic":
			entropic = append(entropic, r)
		case "backing":
			backing = append(backing, r)
		}
	}

	return
}

// Join joins a partitioned circuit back.
func Join(fronting T, entropic T, backing T) (t T) {
	t = append(t, fronting...)
	t = append(t, entropic...)
	t = append(t, backing...)
	return
}

// Make attempts to create a viable circuit given a type, number of requested
// hops and a list of all relays to consider.
func Make(hops int, all T) (t T, err error) {
	have := len(all)

	switch {
	case hops < 1:
		err = fmt.Errorf(
			"invalid number of hops requested: %d",
			hops,
		)
	case hops > have:
		// not enough relays
		err = fmt.Errorf(
			"not enough relays to construct circuit: need %d hops, have %d suitable relays",
			hops,
			have,
		)
	case hops <= have:
		// general case
		f, e, b := all.Partition()

		// always need at least a backing relay
		if len(b) < 1 {
			err = fmt.Errorf("cannot construct circuit: no backing relays")
			return
		}

		switch hops {
		case 1:
			// one random backing relay
			t = T{b[rand.Intn(len(b))]}
		case 2:
			// one random fronting and one random backing relay
			if len(f) < 1 {
				err = fmt.Errorf("cannot construct circuit: no fronting relays")
				return
			}

			t = T{f[rand.Intn(len(f))], b[rand.Intn(len(b))]}
		default:
			// one random fronting and one random backing relay and however
			// many random entropic relays
			if len(f) < 1 {
				err = fmt.Errorf("cannot construct circuit: no fronting relays")
				return
			}

			// shuffle entropic relays to break directory order
			rand.Shuffle(len(e), func(i, j int) { e[i], e[j] = e[j], e[i] })

			// number of entropic relays needed
			need := hops - 2

			if len(e) < need {
				err = fmt.Errorf(
					"cannot construct circuit: not enough entropic relays; need %d for %d hops, have %d",
					need,
					hops,
					len(e),
				)
				return
			}

			t = append(t, f[rand.Intn(len(f))])
			t = append(t, e[:need]...)
			t = append(t, b[rand.Intn(len(b))])
			return
		}
	}

	return
}
