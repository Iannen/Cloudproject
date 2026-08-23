package models

import (
	"context"
	"encoding/json"
)

// --- Domain Entities ---

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

// --- Cluster Config ---

type RoleSpec struct {
	Name     string
	Replicas int
}

// --- DTOs & RPC ---

type AssimilatePayload struct {
	LeaderName         string `json:"leader_name"`
	LeaderPeerURL      string `json:"leader_peer_url"`
	EtcdInitialCluster string `json:"etcd_initial_cluster"`
	AssignedIP         string `json:"assigned_ip"`
}

// --- External Integrations ---

type TSPeer struct {
	HostName     string   `json:"HostName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

type TSStatus struct {
	Peer map[string]*TSPeer `json:"Peer"`
	Self *TSPeer            `json:"Self"`
}

// --- Domain Infrastructure Handles & Events ---

// Session represents an abstract cluster session with a lifecycle signal.
type Session interface {
	Done() <-chan struct{}
	Close() error
}

// MemberEvent represents triggers that require the member role to reconcile assignment state.
type MemberEventType string

const (
	EventAssignmentChange MemberEventType = "ASSIGNMENT_CHANGE"
	EventLeaderDeleted    MemberEventType = "LEADER_DELETED"
	EventReconcileTick    MemberEventType = "RECONCILE_TICK"
)

type MemberEvent struct {
	Type MemberEventType
	Err  error
}

type DomainHandler func(ctx context.Context, body []byte) (string, error)
