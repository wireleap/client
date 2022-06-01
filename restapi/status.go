package restapi

type statusReply struct {
	Home    string        `json:"home"`
	Pid     int           `json:"pid"`
	State   string        `json:"state"`
	Broker  statusBroker  `json:"broker"`
	Upgrade statusUpgrade `json:"upgrade"`
}

type statusBroker struct {
	ActiveCircuit []string `json:"active_circuit"`
}

type statusUpgrade struct {
	Required bool `json:"required"`
}
