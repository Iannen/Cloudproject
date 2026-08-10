package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud-controller/src/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func NodeHeartbeatPath(nodeID string) string {
	return fmt.Sprintf("heartbeats/nodes/%s", nodeID)
}

func AssignmentHeartbeatPath(assignmentID string) string {
	return fmt.Sprintf("heartbeats/assignments/%s", assignmentID)
}

func NodeAssignmentsPath(nodeID string) string {
	return fmt.Sprintf("assignments/nodes/%s", nodeID)
}

func AssignmentDefinitionPath(assignmentID string) string {
	return fmt.Sprintf("assignments/definitions/%s", assignmentID)
}

type Store struct {
	cli *clientv3.Client
}

func NewStore(cli *clientv3.Client) *Store {
	return &Store{cli: cli}
}

// CreateAssignment satisfies the infra.AssignmentCreator interface.
func (s *Store) CreateAssignment(ctx context.Context, assignment models.Assignment) error {
	asgJSON, err := json.Marshal(assignment)
	if err != nil {
		return fmt.Errorf("failed to marshal assignment definition: %w", err)
	}

	asgIDs := []string{assignment.ID}
	idsJSON, err := json.Marshal(asgIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal node assignments list: %w", err)
	}

	defKey := AssignmentDefinitionPath(assignment.ID)
	nodeKey := NodeAssignmentsPath(assignment.NodeID)

	_, err = s.cli.Txn(ctx).Then(
		clientv3.OpPut(defKey, string(asgJSON)),
		clientv3.OpPut(nodeKey, string(idsJSON)),
	).Commit()
	if err != nil {
		return fmt.Errorf("etcd transaction failed: %w", err)
	}

	return nil
}

// KeepAliveLease establishes a long-running lease and runs an active KeepAlive background stream.
// It populates the initial key status and maintains the channel until the context is canceled.
func (s *Store) KeepAliveLease(ctx context.Context, key string, value string, ttl int64) error {
	lease, err := s.cli.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to grant lease: %w", err)
	}

	_, err = s.cli.Put(ctx, key, value, clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("failed to put key with lease: %w", err)
	}

	keepAliveChan, err := s.cli.KeepAlive(ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("failed to start keep-alive stream: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ka, ok := <-keepAliveChan:
				if !ok {
					log.Printf("[Store] KeepAlive channel closed for key: %s. Exiting loop.", key)
					return
				}
				if ka == nil {
					log.Printf("[Store] KeepAlive stream lost connection for key: %s", key)
					return
				}
			}
		}
	}()

	return nil
}

func (s *Store) GetAssignmentDefinition(ctx context.Context, assignmentID string) (*models.Assignment, error) {
	key := AssignmentDefinitionPath(assignmentID)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("assignment definition not found: %s", assignmentID)
	}
	var asg models.Assignment
	if err := json.Unmarshal(resp.Kvs[0].Value, &asg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal assignment definition: %w", err)
	}
	return &asg, nil
}

func (s *Store) WatchNodeAssignmentsFromRev(ctx context.Context, nodeID string, rev int64) clientv3.WatchChan {
	key := NodeAssignmentsPath(nodeID)
	return s.cli.Watch(ctx, key, clientv3.WithRev(rev))
}

func (s *Store) GetNodeAssignmentsWithRev(ctx context.Context, nodeID string) ([]string, int64, error) {
	key := NodeAssignmentsPath(nodeID)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	revision := resp.Header.Revision
	if len(resp.Kvs) == 0 {
		return nil, revision, nil
	}
	var ids []string
	if err := json.Unmarshal(resp.Kvs[0].Value, &ids); err != nil {
		return nil, revision, fmt.Errorf("failed to unmarshal node assignments: %w", err)
	}
	return ids, revision, nil
}
