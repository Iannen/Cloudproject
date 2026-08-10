package roles

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

func init() {
	RegisterRole("leader", RunLeaderRole)
}

type LeaderConfig struct {
	// Add custom configuration fields here if needed in the future
}

// RunLeaderRole executes the leader scheduler logic when assigned to a node.
func RunLeaderRole(ctx context.Context, config json.RawMessage) error {
	var cfg LeaderConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return iwtError(err)
		}
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	log.Println("[Leader] Role started as a generic assignment")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Leader] Role assignment stopped")
			return nil
		case <-ticker.C:
			log.Println("[Leader] Scheduler running...")
			/*
			   TODO: Future leader scheduling logic implementation:
			   - determine desired cluster assignments
			   - create assignment IDs
			   - write assignment definitions
			   - update assignments/nodes/<nodeid>
			   - renew assignment leases
			   - delete obsolete assignments
			   - react to node heartbeats
			   - police assignment heartbeats
			*/
		}
	}
}

func iwtError(err error) error {
	return err
}
