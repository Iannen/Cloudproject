package roles

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud-controller/src/adapters"
	"cloud-controller/src/config"
	"cloud-controller/src/models"
)

type LeaderStore interface {
	GetActiveNodeIDs(ctx context.Context) ([]string, error)
	GetAllAssignments(ctx context.Context) ([]models.Assignment, error)
	CreateAssignment(ctx context.Context, assignment models.Assignment) error
}

type LeaderRole struct {
	store LeaderStore
}

func init() {
	RegisterRole("leader", func(store any) RoleRunner {
		// Go type-asserts store to LeaderStore interface here at instantiation time!
		ls, ok := store.(LeaderStore)
		if !ok {
			panic("store passed to leader factory does not implement LeaderStore")
		}
		return &LeaderRole{store: ls}
	})
}

func (l *LeaderRole) Run(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	log.Printf("[Leader] Node %s acting as leader for %s. Lease ID: %d", asg.NodeID, asg.ID, sess.LeaseID())

	for {
		select {
		case <-ctx.Done():
			log.Println("[Leader] Role assignment stopped")
			return nil
		case <-ticker.C:
			reconcileCluster(ctx, l.store)
		}
	}
}

func reconcileCluster(ctx context.Context, store LeaderStore) {
	activeNodes, err := store.GetActiveNodeIDs(ctx)
	if err != nil || len(activeNodes) == 0 {
		log.Printf("[Leader] Cannot reconcile: no active nodes found or etcd error: %v", err)
		return
	}

	existingAssignments, err := store.GetAllAssignments(ctx)
	if err != nil {
		log.Printf("[Leader] Failed to fetch existing assignments: %v", err)
		return
	}

	activeAssignmentsByRole := make(map[string][]models.Assignment)
	for _, asg := range existingAssignments {
		// Only count assignments belonging to currently healthy nodes
		if isNodeActive(asg.NodeID, activeNodes) {
			activeAssignmentsByRole[asg.Role] = append(activeAssignmentsByRole[asg.Role], asg)
		}
	}

	for _, roleSpec := range config.ClusterSpec {
		currentSupply := len(activeAssignmentsByRole[roleSpec.Name])
		demand := roleSpec.Replicas

		log.Printf("[Leader] Role '%s': Supply=%d, Demand=%d", roleSpec.Name, currentSupply, demand)

		if currentSupply < demand {
			missing := demand - currentSupply
			log.Printf("[Leader] Need to schedule %d missing replica(s) for role '%s'", missing, roleSpec.Name)

			for i := 0; i < missing; i++ {
				targetNode := pickTargetNode(activeNodes, activeAssignmentsByRole[roleSpec.Name])
				if targetNode == "" {
					log.Printf("[Leader] No suitable node found for role %s", roleSpec.Name)
					break
				}

				newAsg := models.Assignment{
					ID:     fmt.Sprintf("%s-%s-%d", roleSpec.Name, targetNode, time.Now().UnixNano()%10000),
					NodeID: targetNode,
					Role:   roleSpec.Name,
				}

				log.Printf("[Leader] Creating assignment %s for node %s", newAsg.ID, targetNode)
				if err := store.CreateAssignment(ctx, newAsg); err != nil {
					log.Printf("[Leader] Failed to write assignment to etcd: %v", err)
				}
			}
		}
	}
}

func isNodeActive(nodeID string, activeNodes []string) bool {
	for _, id := range activeNodes {
		if id == nodeID {
			return true
		}
	}
	return false
}

func pickTargetNode(activeNodes []string, existing []models.Assignment) string {
	if len(activeNodes) == 0 {
		return ""
	}

	counts := make(map[string]int)
	for _, asg := range existing {
		counts[asg.NodeID]++
	}

	bestNode := activeNodes[0]
	minCount := counts[bestNode]

	for _, node := range activeNodes {
		if counts[node] < minCount {
			minCount = counts[node]
			bestNode = node
		}
	}

	return bestNode
}
