package config

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	nodeID string
	once   sync.Once
)

const (
	ClusterLeaderKey            = "cluster/leader"
	PrefixHeartbeatsNodes       = "heartbeats/nodes/"
	PrefixHeartbeatsAssignments = "heartbeats/assignments/"
	PrefixAssignmentsNodes      = "assignments/nodes/"
	PrefixAssignmentsDefs       = "assignments/definitions/"
)

const (
	DefaultHTTPPort  = ":8080"
	EtcdGRPCEndpoint = "localhost:2379"
	EtcdDialSocket   = "127.0.0.1:2379"
	EtcdPeerPort     = 2380
	NodeNamePrefix   = "kaffcloud"
)

const (
	BootstrapDir = "/root/bootstrap"
)

const (
	LeaderReconcileInterval = 3 * time.Second
	MemberReconcileInterval = 3 * time.Second
	TSManagerPollInterval   = 10 * time.Second
	EtcdDialTimeout         = 2 * time.Second
	EtcdStatusTimeout       = 2 * time.Second
	EtcdSessionTTLSeconds   = 5
	HTTPTimeout             = 5 * time.Second

	EtcdConnectRetryAttempts   = 5
	EtcdConnectRetryInterval   = 1 * time.Second
	EtcdSocketPollAttempts     = 15
	EtcdSocketPollInterval     = 1 * time.Second
	MemberConnectRetryInterval = 2 * time.Second
	MemberWatchReconnectDelay  = 1 * time.Second
)

type RoleSpec struct {
	Name     string
	Replicas int
}

var ClusterSpec = []RoleSpec{
	{Name: "tailscale-manager", Replicas: 1},
}

func InitNodeID() {
	once.Do(func() {
		nodeID = os.Getenv("NODE_ID")
	})
}

func NodeID() string {
	return nodeID
}

func NodeHeartbeatPath(id string) string {
	return fmt.Sprintf("%s%s", PrefixHeartbeatsNodes, id)
}

func AssignmentHeartbeatPath(id string) string {
	return fmt.Sprintf("%s%s", PrefixHeartbeatsAssignments, id)
}

func NodeAssignmentsPath(id string) string {
	return fmt.Sprintf("%s%s", PrefixAssignmentsNodes, id)
}

func AssignmentDefinitionPath(id string) string {
	return fmt.Sprintf("%s%s", PrefixAssignmentsDefs, id)
}
