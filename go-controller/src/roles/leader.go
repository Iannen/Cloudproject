package roles

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud-controller/src/config"
	"cloud-controller/src/models"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type LeaderStore interface {
	GetActiveNodeIDs(ctx context.Context) ([]string, error)
	GetAllAssignments(ctx context.Context) ([]models.Assignment, error)
	CreateAssignment(ctx context.Context, a models.Assignment) error
}

type LeaderRole struct{ store LeaderStore }

func (l *LeaderRole) Run(ctx context.Context, a *models.Assignment, s *concurrency.Session) error {
	tk := time.NewTicker(config.ReconcileInterval)
	defer tk.Stop()

	log.Printf("[Leader] Active on %s (%s)", a.NodeID, a.ID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tk.C:
			reconcile(ctx, l.store)
		}
	}
}

func reconcile(ctx context.Context, str LeaderStore) {
	nodes, err := str.GetActiveNodeIDs(ctx)
	if err != nil {
		log.Printf("[Leader] Failed to fetch active nodes from etcd: %v", err)
		return
	}
	if len(nodes) == 0 {
		return
	}

	active := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		active[n] = true
	}

	asgs, err := str.GetAllAssignments(ctx)
	if err != nil {
		log.Printf("[Leader] Failed to fetch existing assignments from etcd: %v", err)
		return
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
			node := pickNode(nodes, curr)
			if node == "" {
				log.Printf("[Leader] No suitable node available for role %s", spec.Name)
				break
			}
			id := fmt.Sprintf("%s-%s-%d", spec.Name, node, time.Now().UnixNano()%10000)
			if err := str.CreateAssignment(ctx, models.Assignment{ID: id, NodeID: node, Role: spec.Name}); err != nil {
				log.Printf("[Leader] Assign fail %s->%s: %v", spec.Name, node, err)
			} else {
				log.Printf("[Leader] Assigned %s to %s", id, node)
			}
		}
	}
}

func pickNode(nodes []string, existing []models.Assignment) string {
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
