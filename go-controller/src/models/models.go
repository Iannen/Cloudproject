package models

import "encoding/json"

type Assignment struct {
	NodeID string          `json:"node_id"`
	ID     string          `json:"id"`
	Role   string          `json:"role"`
	Config json.RawMessage `json:"config"`
}

type MemberInfo struct {
	ID       uint64
	Name     string
	PeerURLs []string
}

type AssimilatePayload struct {
	LeaderName         string `json:"leader_name"`
	LeaderPeerURL      string `json:"leader_peer_url"`
	EtcdInitialCluster string `json:"etcd_initial_cluster"`
	AssignedIP         string `json:"assigned_ip"`
}
