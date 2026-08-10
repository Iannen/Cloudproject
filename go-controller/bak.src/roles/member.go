package roles

import (
	"context"
	"log"
	"time"

	"cloud-controller/src/adapters"
)

// RunMemberRole is the permanent process running on every node.
// Responsibilities: maintain an active lease-backed node heartbeat.
func RunMemberRole(ctx context.Context, s *adapters.Store, nodeID string) {
	log.Printf("[Member] Permanent member role started for node %s", nodeID)
	key := adapters.NodeHeartbeatPath(nodeID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Member] Permanent member role stopped for node %s", nodeID)
			return
		default:
			// Create a distinct lease sub-context that cancels if parent cancels or lease crashes
			leaseCtx, leaseCancel := context.WithCancel(ctx)

			log.Printf("[Member] Establishing new persistent heartbeat lease for node %s", nodeID)
			err := s.KeepAliveLease(leaseCtx, key, "alive", 5)
			if err != nil {
				log.Printf("[Member] Heartbeat lease session failed: %v. Retrying...", err)
				leaseCancel()
				time.Sleep(2 * time.Second)
				continue
			}

			// Wait until the lease manager context encounters an error or finishes
			<-leaseCtx.Done()
			leaseCancel()
		}
	}
}
