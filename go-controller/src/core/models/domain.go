package models

import (
	"encoding/json"
)

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
type RoleSpec struct {
	Name     string
	Replicas int
}
type TSPeer struct {
	HostName     string   `json:"HostName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}
type Session interface {
	Done() <-chan struct{}
	Close() error
}
