package config

import (
	"os"
	"time"
)

const (
	ClusterLeaderKey = "cluster/leader"
	PrefixHeartbeats = "heartbeats/nodes/"
	PrefixDefs       = "assignments/definitions/"
)

const (
	HTTPPort       = ":8080"
	EtcdEndpoint   = "localhost:2379"
	EtcdPeerPort   = 2380
	NodeNamePrefix = "kaffcloud"
	BootstrapDir   = "/root/bootstrap"
)

const (
	ReconcileInterval   = 3 * time.Second
	Timeout             = 3 * time.Second
	SessionTTL          = 5
	StartupRetries      = 10
	StartupInterval     = 1 * time.Second
	RetryInterval       = 2 * time.Second
	WatchReconnectDelay = 1 * time.Second
)

type RoleSpec struct {
	Name     string
	Replicas int
}

var ClusterSpec = []RoleSpec{
	{Name: "tailscale-manager", Replicas: 1},
}

func NodeID() string { return os.Getenv("NODE_ID") }

func NodeHeartbeatPath(id string) string { return PrefixHeartbeats + id }

func NodeAssignmentsPath(id string) string { return "assignments/nodes/" + id }

func AsgDefPath(id string) string { return PrefixDefs + id }
