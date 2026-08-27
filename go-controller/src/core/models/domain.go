package models

type Assignment struct {
	NodeID string
	ID     string
	Role   string
	Config string
}

type MemberInfo struct {
	ID       uint64
	Name     string
	PeerURLs []string
}

type TSPeer struct {
	HostName     string
	TailscaleIPs []string
	Online       bool
}
