package config

import (
	"fmt"
	"os"
	"sync"
)

var (
	nodeID string
	once   sync.Once
)

const (
	ClusterLeaderKey = "cluster/leader"
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
	return fmt.Sprintf("heartbeats/nodes/%s", id)
}

func AssignmentHeartbeatPath(id string) string {
	return fmt.Sprintf("heartbeats/assignments/%s", id)
}

func NodeAssignmentsPath(id string) string {
	return fmt.Sprintf("assignments/nodes/%s", id)
}

func AssignmentDefinitionPath(id string) string {
	return fmt.Sprintf("assignments/definitions/%s", id)
}
