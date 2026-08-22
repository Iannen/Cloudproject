package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"time"
)

type LeaderRole struct {
	store AssignmentStore
}

func (l *LeaderRole) Run(ctx context.Context, a *models.Assignment) {
	tk := time.NewTicker(config.ReconcileInterval)
	defer tk.Stop()

	log.Printf("[Leader] Started: %s on %s", a.ID, a.NodeID)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			if err := l.reconcile(ctx); err != nil {
				log.Println(err)
			}
		}
	}
}

func (l *LeaderRole) reconcile(ctx context.Context) error {
	nodes, err := l.store.GetActiveNodeIDs(ctx, config.PrefixHeartbeats)
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

	asgs, err := l.store.GetAllAssignments(ctx, config.PrefixDefs)
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
			id := fmt.Sprintf("%s-%s-%d", spec.Name, node, time.Now().UnixNano()%10000)

			if err := l.store.CreateAssignment(ctx, config.AsgDefPath(id), config.NodeAssignmentsPath(node), models.Assignment{ID: id, NodeID: node, Role: spec.Name}); err != nil {
				log.Printf("[Leader] assign failed role=%s node=%s: %v", spec.Name, node, err)
			} else {
				log.Printf("[Leader] assigned id=%s node=%s", id, node)
			}
		}
	}

	return nil
}

type AssignmentStore interface {
	GetActiveNodeIDs(ctx context.Context, prefix string) ([]string, error)
	GetAllAssignments(ctx context.Context, prefix string) ([]models.Assignment, error)
	CreateAssignment(ctx context.Context, asgDefPath, nodeAsgPath string, a models.Assignment) error
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

func NewLeaderRole(store AssignmentStore) *LeaderRole {
	return &LeaderRole{
		store: store,
	}
}
