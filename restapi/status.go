package restapi

type StatusReply struct {
	Home    string         `json:"home"`
	Pid     int            `json:"pid"`
	State   string         `json:"state"`
	Broker  StatusBroker   `json:"broker"`
	Upgrade *StatusUpgrade `json:"upgrade,omitempty"`
}

type StatusBroker struct {
	ActiveCircuit []string `json:"active_circuit"`
}

type StatusUpgrade struct {
	Required bool `json:"required"`
}
