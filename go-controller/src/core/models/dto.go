package models

type AssimilatePayload struct {
	LeaderName         string `json:"leader_name"`
	LeaderPeerURL      string `json:"leader_peer_url"`
	EtcdInitialCluster string `json:"etcd_initial_cluster"`
	AssignedIP         string `json:"assigned_ip"`
}

type TSStatus struct {
	Peer map[string]TSPeer `json:"Peer"`
	Self TSPeer            `json:"Self"`
}
