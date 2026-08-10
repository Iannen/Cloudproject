package config

import (
	"os"
	"sync"
)

var (
	nodeID string
	once   sync.Once
)

func InitNodeID() {
	once.Do(func() {
		nodeID = os.Getenv("NODE_ID")
	})
}

func NodeID() string {
	return nodeID
}
