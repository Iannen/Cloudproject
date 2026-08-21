package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type Store struct {
	cli *clientv3.Client
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Connect(ctx context.Context, endpoint string) error {
	for i := 0; i < config.StartupRetries; i++ {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{endpoint},
			DialTimeout: config.Timeout,
		})
		if err == nil {
			sCtx, cancel := context.WithTimeout(ctx, config.Timeout)
			_, err = cli.Status(sCtx, endpoint)
			cancel()
			if err == nil {
				s.cli = cli
				return nil
			}
			cli.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(config.StartupInterval):
		}
	}
	return fmt.Errorf("etcd connection failed after retries")
}

func (s *Store) CreateAssignment(ctx context.Context, a models.Assignment) error {
	b, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal asg: %w", err)
	}

	ids, _, err := s.NodeAssignments(ctx, a.NodeID)
	if err != nil {
		return fmt.Errorf("get node asgs: %w", err)
	}

	exists := false
	for _, id := range ids {
		if id == a.ID {
			exists = true
			break
		}
	}
	if !exists {
		ids = append(ids, a.ID)
	}

	idsB, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("marshal ids: %w", err)
	}

	_, err = s.cli.Txn(ctx).Then(
		clientv3.OpPut(config.AsgDefPath(a.ID), string(b)),
		clientv3.OpPut(config.NodeAssignmentsPath(a.NodeID), string(idsB)),
	).Commit()
	if err != nil {
		return fmt.Errorf("etcd txn: %w", err)
	}
	return nil
}

func (s *Store) NewSession(ctx context.Context, ttl int64) (*concurrency.Session, error) {
	return concurrency.NewSession(s.cli, concurrency.WithTTL(int(ttl)))
}

func (s *Store) PutWithSession(ctx context.Context, sess *concurrency.Session, key, val string) error {
	_, err := s.cli.Put(ctx, key, val, clientv3.WithLease(sess.Lease()))
	return err
}

func (s *Store) AssignmentDef(ctx context.Context, id string) (*models.Assignment, error) {
	resp, err := s.cli.Get(ctx, config.AsgDefPath(id))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("asg not found: %s", id)
	}
	var a models.Assignment
	if err := json.Unmarshal(resp.Kvs[0].Value, &a); err != nil {
		return nil, fmt.Errorf("unmarshal asg: %w", err)
	}
	return &a, nil
}

func (s *Store) WatchAssignments(ctx context.Context, nodeID string, rev int64) clientv3.WatchChan {
	return s.cli.Watch(ctx, config.NodeAssignmentsPath(nodeID), clientv3.WithRev(rev))
}

func (s *Store) NodeAssignments(ctx context.Context, nodeID string) ([]string, int64, error) {
	resp, err := s.cli.Get(ctx, config.NodeAssignmentsPath(nodeID))
	if err != nil {
		return nil, 0, err
	}
	rev := resp.Header.Revision
	if len(resp.Kvs) == 0 {
		return nil, rev, nil
	}
	var ids []string
	if err := json.Unmarshal(resp.Kvs[0].Value, &ids); err != nil {
		return nil, rev, fmt.Errorf("unmarshal node asgs: %w", err)
	}
	return ids, rev, nil
}

func (s *Store) ClaimLeader(ctx context.Context, sess *concurrency.Session, nodeID string) (bool, error) {
	key := config.ClusterLeaderKey
	resp, err := s.cli.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, nodeID, clientv3.WithLease(sess.Lease()))).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return false, fmt.Errorf("leader election txn: %w", err)
	}
	return resp.Succeeded, nil
}

func (s *Store) WatchLeaderKey(ctx context.Context, ch chan<- struct{}) {
	chWatch := s.cli.Watch(ctx, config.ClusterLeaderKey)
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-chWatch:
			if !ok {
				return
			}
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypeDelete {
					select {
					case ch <- struct{}{}:
					default:
					}
					return
				}
			}
		}
	}
}

func (s *Store) GetActiveNodeIDs(ctx context.Context) ([]string, error) {
	resp, err := s.cli.Get(ctx, config.PrefixHeartbeats, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, kv := range resp.Kvs {
		if id := string(kv.Key[len(config.PrefixHeartbeats):]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *Store) GetAllAssignments(ctx context.Context) ([]models.Assignment, error) {
	resp, err := s.cli.Get(ctx, config.PrefixDefs, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var asgs []models.Assignment
	for _, kv := range resp.Kvs {
		var a models.Assignment
		if err := json.Unmarshal(kv.Value, &a); err == nil {
			asgs = append(asgs, a)
		}
	}
	return asgs, nil
}

func (s *Store) AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error) {
	resp, err := s.cli.MemberAddAsLearner(ctx, []string{peerURL})
	if err != nil {
		return nil, nil, fmt.Errorf("add learner %s: %w", peerURL, err)
	}
	all := make([]models.MemberInfo, 0, len(resp.Members))
	for _, m := range resp.Members {
		all = append(all, models.MemberInfo{ID: m.ID, Name: m.Name, PeerURLs: m.PeerURLs})
	}
	return &models.MemberInfo{ID: resp.Member.ID, Name: resp.Member.Name, PeerURLs: resp.Member.PeerURLs}, all, nil
}

func (s *Store) PromoteMember(ctx context.Context, id uint64) error {
	_, err := s.cli.MemberPromote(ctx, id)
	return err
}

func (s *Store) RemoveMember(ctx context.Context, id uint64) error {
	_, err := s.cli.MemberRemove(ctx, id)
	return err
}

func (s *Store) GetClusterMembers(ctx context.Context) ([]models.MemberInfo, error) {
	resp, err := s.cli.MemberList(ctx)
	if err != nil {
		return nil, fmt.Errorf("member list: %w", err)
	}
	res := make([]models.MemberInfo, 0, len(resp.Members))
	for _, m := range resp.Members {
		res = append(res, models.MemberInfo{ID: m.ID, Name: m.Name, PeerURLs: m.PeerURLs})
	}
	return res, nil
}

/*

 */
