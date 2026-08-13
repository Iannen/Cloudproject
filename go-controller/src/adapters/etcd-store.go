package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud-controller/src/config"
	"cloud-controller/src/models"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
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

type SessionWrapper interface {
	Done() <-chan struct{}
	Close() error
	LeaseID() int64
}

type EtcdSession struct {
	sess *concurrency.Session
}

func (es *EtcdSession) Done() <-chan struct{} { return es.sess.Done() }
func (es *EtcdSession) Close() error          { return es.sess.Close() }
func (es *EtcdSession) LeaseID() int64        { return int64(es.sess.Lease()) }

type Store struct {
	cli *clientv3.Client
}

func NewStore(cli *clientv3.Client) *Store {
	return &Store{cli: cli}
}

func (s *Store) CreateAssignment(ctx context.Context, assignment models.Assignment) error {
	asgJSON, err := json.Marshal(assignment)
	if err != nil {
		return fmt.Errorf("failed to marshal assignment definition: %w", err)
	}

	defKey := AssignmentDefinitionPath(assignment.ID)
	nodeKey := NodeAssignmentsPath(assignment.NodeID)

	// Fetch existing assignments for the node
	existingIDs, _, err := s.GetNodeAssignmentsWithRev(ctx, assignment.NodeID)
	if err != nil {
		return fmt.Errorf("failed to get existing node assignments: %w", err)
	}

	// Append assignment ID if not already present
	alreadyExists := false
	for _, id := range existingIDs {
		if id == assignment.ID {
			alreadyExists = true
			break
		}
	}

	if !alreadyExists {
		existingIDs = append(existingIDs, assignment.ID)
	}

	idsJSON, err := json.Marshal(existingIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal node assignments list: %w", err)
	}

	_, err = s.cli.Txn(ctx).Then(
		clientv3.OpPut(defKey, string(asgJSON)),
		clientv3.OpPut(nodeKey, string(idsJSON)),
	).Commit()
	if err != nil {
		return fmt.Errorf("etcd transaction failed: %w", err)
	}

	return nil
}

func (s *Store) NewSession(ctx context.Context, ttl int64) (SessionWrapper, error) {
	sess, err := concurrency.NewSession(s.cli, concurrency.WithTTL(int(ttl)))
	if err != nil {
		return nil, err
	}
	return &EtcdSession{sess: sess}, nil
}

func (s *Store) PutWithSession(ctx context.Context, sess SessionWrapper, key string, value string) error {
	_, err := s.cli.Put(ctx, key, value, clientv3.WithLease(clientv3.LeaseID(sess.LeaseID())))
	return err
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

func (s *Store) TryClaimLeadership(ctx context.Context, sess SessionWrapper, nodeID string) (bool, error) {
	leaderKey := config.ClusterLeaderKey

	// Condition: Key does NOT exist in etcd yet
	cond := clientv3.Compare(clientv3.CreateRevision(leaderKey), "=", 0)

	// Put our nodeID on the leader key with our heartbeat lease
	thenOp := clientv3.OpPut(leaderKey, nodeID, clientv3.WithLease(clientv3.LeaseID(sess.LeaseID())))

	// If key exists, read current leader info
	elseOp := clientv3.OpGet(leaderKey)

	txnResp, err := s.cli.Txn(ctx).If(cond).Then(thenOp).Else(elseOp).Commit()
	if err != nil {
		return false, fmt.Errorf("leader election transaction failed: %w", err)
	}

	return txnResp.Succeeded, nil
}

func (s *Store) WatchLeaderKey(ctx context.Context, notifyChan chan<- struct{}) {
	leaderKey := config.ClusterLeaderKey
	watchChan := s.cli.Watch(ctx, leaderKey)

	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				return
			}
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypeDelete {
					select {
					case notifyChan <- struct{}{}:
					default:
					}
					return
				}
			}
		}
	}
}

func (s *Store) GetActiveNodeIDs(ctx context.Context) ([]string, error) {
	resp, err := s.cli.Get(ctx, "heartbeats/nodes/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var nodeIDs []string
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		nodeID := strings.TrimPrefix(key, "heartbeats/nodes/")
		if nodeID != "" {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	return nodeIDs, nil
}

func (s *Store) GetAllAssignments(ctx context.Context) ([]models.Assignment, error) {
	resp, err := s.cli.Get(ctx, "assignments/definitions/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var assignments []models.Assignment
	for _, kv := range resp.Kvs {
		var asg models.Assignment
		if err := json.Unmarshal(kv.Value, &asg); err == nil {
			assignments = append(assignments, asg)
		}
	}
	return assignments, nil
}

func (s *Store) AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error) {
	resp, err := s.cli.MemberAddAsLearner(ctx, []string{peerURL})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add learner %s: %w", peerURL, err)
	}

	newMember := &models.MemberInfo{
		ID:       resp.Member.ID,
		Name:     resp.Member.Name,
		PeerURLs: resp.Member.PeerURLs,
	}

	var allMembers []models.MemberInfo
	for _, m := range resp.Members {
		allMembers = append(allMembers, models.MemberInfo{
			ID:       m.ID,
			Name:     m.Name,
			PeerURLs: m.PeerURLs,
		})
	}

	return newMember, allMembers, nil
}

func (s *Store) PromoteMember(ctx context.Context, memberID uint64) error {
	_, err := s.cli.MemberPromote(ctx, memberID)
	if err != nil {
		return fmt.Errorf("failed to promote member %x: %w", memberID, err)
	}
	return nil
}

func (s *Store) RemoveMember(ctx context.Context, memberID uint64) error {
	_, err := s.cli.MemberRemove(ctx, memberID)
	if err != nil {
		return fmt.Errorf("failed to remove member %x: %w", memberID, err)
	}
	return nil
}
