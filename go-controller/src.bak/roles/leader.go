package roles

import (
	"context"
	"log"
	"time"

	"cloud-controller/src/models"
)

func init() {
	RegisterRole("leader", RunLeaderRole)
}

// RunLeaderRole executes the leader scheduler logic when assigned to a node.
func RunLeaderRole(ctx context.Context, asg *models.Assignment) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	log.Printf("[Leader] Node %s is now acting as cluster leader for assignment %s.", asg.NodeID, asg.ID)

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
