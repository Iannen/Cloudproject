package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
)

type LeaderRole struct {
	asg   models.Assignment
	store AssignmentStore
}

func (l *LeaderRole) Run(ctx context.Context) {
	log.Printf("[Leader] Started: %s on %s", l.asg.ID, l.asg.NodeID)
	ch, err := l.store.SubscribeLeaderEvents(ctx)
	if err != nil {
		log.Printf("[Leader] Failed to subscribe to leader events: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.Err != nil {
				log.Printf("[Leader] Event stream error: %v", ev.Err)
				continue
			}
			if err := l.reconcile(ctx); err != nil {
				log.Println(err)
			}
		}
	}
}

func (l *LeaderRole) reconcile(ctx context.Context) error {
	nodes, err := l.store.GetActiveNodeIDs(ctx)
	if err != nil {
		return fmt.Errorf("[Leader] active node lookup failed: %w", err)
	}
	if len(nodes) == 0 {
		return nil
	}

	active := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		active[n] = true
	}

	asgs, err := l.store.GetAllAssignments(ctx)
	if err != nil {
		return fmt.Errorf("[Leader] assignment lookup failed: %w", err)
	}

	byRole := make(map[string][]models.Assignment)
	for _, a := range asgs {
		if active[a.NodeID] {
			byRole[a.Role] = append(byRole[a.Role], a)
		}
	}

	for _, spec := range config.ClusterSpec {
		curr := byRole[spec.Name]
		for i := 0; i < spec.Replicas-len(curr); i++ {
			node := l.pickNode(nodes, curr)
			if node == "" {
				log.Printf("[Leader] No suitable node available for role %s", spec.Name)
				break
			}

			asg := models.Assignment{NodeID: node, Role: spec.Name}
			if err := l.store.CreateAssignment(ctx, asg); err != nil {
				log.Printf("[Leader] assign failed role=%s node=%s: %v", spec.Name, node, err)
			} else {
				log.Printf("[Leader] assigned role=%s node=%s", spec.Name, node)
			}
		}
	}

	return nil
}

func (l *LeaderRole) pickNode(nodes []string, existing []models.Assignment) string {
	if len(nodes) == 0 {
		return ""
	}
	counts := make(map[string]int, len(nodes))
	for _, a := range existing {
		counts[a.NodeID]++
	}

	best, min := nodes[0], counts[nodes[0]]
	for _, n := range nodes[1:] {
		if c := counts[n]; c < min {
			best, min = n, c
		}
	}
	return best
}

func NewLeaderAssignment(nodeID string) models.Assignment {
	return models.Assignment{
		NodeID: nodeID,
		ID:     "leader-" + nodeID,
		Role:   "leader",
	}
}

func NewLeaderRole(asg models.Assignment, store AssignmentStore) *LeaderRole {
	return &LeaderRole{
		asg:   asg,
		store: store,
	}
}
