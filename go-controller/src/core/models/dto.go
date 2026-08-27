package models

type AssimilatePayload struct {
	LeaderName         string
	LeaderPeerURL      string
	EtcdInitialCluster string
	AssignedIP         string
}
