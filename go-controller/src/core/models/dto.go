package models

type AssimilatePayload struct {
	LeaderName         string
	LeaderPeerURL      string
	EtcdInitialCluster string
	AssignedIP         string
}

type TSStatus struct {
	Peer map[string]TSPeer
	Self TSPeer
}
