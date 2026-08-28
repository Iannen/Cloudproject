package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-controller/src/core/models"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func NewEvent(typ models.EventType) models.Event {
	return models.Event{
		Type: typ,
	}
}

type StoreConfig struct {
	NodeID              string
	Endpoint            string
	Timeout             time.Duration
	StartupInterval     time.Duration
	StartupRetries      int
	SessionTTL          int64
	RetryInterval       time.Duration
	LeaderKey           string
	ReconcileInterval   time.Duration
	WatchReconnectDelay time.Duration
	PrefixHeartbeats    string
	PrefixDefs          string
	PrefixNodeAsgs      string
	TickTimeout         time.Duration
}

type Store struct {
	cli  *clientv3.Client
	cfg  StoreConfig
	sess *concurrency.Session
}

func NewStore(cfg StoreConfig) *Store {
	return &Store{
		cfg: cfg,
	}
}

func (s *Store) Connect(ctx context.Context) error {
	var lastErr error
	for i := 0; i < s.cfg.StartupRetries; i++ {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{s.cfg.Endpoint},
			DialTimeout: s.cfg.Timeout,
		})
		if err == nil {
			sCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
			_, err = cli.Status(sCtx, s.cfg.Endpoint)
			cancel()
			if err == nil {
				s.cli = cli
				return nil
			}
			lastErr = fmt.Errorf("etcd status check failed: %w", err)
			cli.Close()
		} else {
			lastErr = fmt.Errorf("etcd client creation failed: %w", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.cfg.StartupInterval):
		}
	}
	return fmt.Errorf("etcd connection failed after %d retries: %w", s.cfg.StartupRetries, lastErr)
}

func (s *Store) nodeAssignmentsPath(id string) string {
	return s.cfg.PrefixNodeAsgs + id
}

func (s *Store) asgDefPath(id string) string {
	return s.cfg.PrefixDefs + id
}

func (s *Store) CreateAssignment(ctx context.Context, a models.Assignment) error {
	if a.ID == "" {
		a.ID = fmt.Sprintf("%s-%s-%d", a.Role, a.NodeID, time.Now().UnixNano()%10000)
	}

	asgDefPath := s.asgDefPath(a.ID)
	nodeAsgPath := s.nodeAssignmentsPath(a.NodeID)

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
		clientv3.OpPut(asgDefPath, string(b)),
		clientv3.OpPut(nodeAsgPath, string(idsB)),
	).Commit()
	if err != nil {
		return fmt.Errorf("etcd txn: %w", err)
	}
	return nil
}

func (s *Store) EnsureSession(ctx context.Context) error {
	if s.sess != nil {
		select {
		case <-s.sess.Done():
			s.sess = nil
		default:
			return nil
		}
	}

	var lastErr error
	for i := 0; i < s.cfg.StartupRetries; i++ {
		sess, err := concurrency.NewSession(s.cli, concurrency.WithTTL(int(s.cfg.SessionTTL)), concurrency.WithContext(ctx))
		if err == nil {
			s.sess = sess
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.cfg.RetryInterval):
		}
	}
	return lastErr
}

func (s *Store) CloseSession() error {
	if s.sess != nil {
		err := s.sess.Close()
		s.sess = nil
		return err
	}
	return nil
}

func (s *Store) PutHeartbeat(ctx context.Context, nodeID, val string) error {
	if s.sess == nil {
		return fmt.Errorf("active session required")
	}
	key := s.cfg.PrefixHeartbeats + nodeID
	_, err := s.cli.Put(ctx, key, val, clientv3.WithLease(s.sess.Lease()))
	return err
}

func (s *Store) AssignmentDef(ctx context.Context, assignmentID string) (*models.Assignment, error) {
	asgDefPath := s.asgDefPath(assignmentID)
	resp, err := s.cli.Get(ctx, asgDefPath)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("asg not found at path: %s", asgDefPath)
	}
	var a models.Assignment
	if err := json.Unmarshal(resp.Kvs[0].Value, &a); err != nil {
		return nil, fmt.Errorf("unmarshal asg: %w", err)
	}
	return &a, nil
}

func (s *Store) NodeAssignments(ctx context.Context, nodeID string) ([]string, int64, error) {
	nodeAsgPath := s.nodeAssignmentsPath(nodeID)
	resp, err := s.cli.Get(ctx, nodeAsgPath)
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

func (s *Store) ClaimLeader(ctx context.Context, nodeID string) (bool, error) {
	if s.sess == nil {
		return false, fmt.Errorf("active session required")
	}
	resp, err := s.cli.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(s.cfg.LeaderKey), "=", 0)).
		Then(clientv3.OpPut(s.cfg.LeaderKey, nodeID, clientv3.WithLease(s.sess.Lease()))).
		Else(clientv3.OpGet(s.cfg.LeaderKey)).
		Commit()
	if err != nil {
		return false, fmt.Errorf("leader election txn: %w", err)
	}
	return resp.Succeeded, nil
}

func (s *Store) SubscribeEvents(ctx context.Context, nodeID string) (<-chan models.Event, error) {
	_, rev, err := s.NodeAssignments(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	nodeAsgPath := s.nodeAssignmentsPath(nodeID)
	ch := make(chan models.Event, 10)

	go s.runSessionWatcher(ctx, ch)
	go s.runLeaderWatcher(ctx, ch)
	go s.runTicker(ctx, ch)
	go s.runAssignmentWatch(ctx, nodeAsgPath, rev, ch)

	return ch, nil
}

func (s *Store) runSessionWatcher(ctx context.Context, ch chan<- models.Event) {
	if s.sess == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-s.sess.Done():
		s.notifyEvent(ch, models.Event{Type: models.EventSessionExpired})
	}
}

func (s *Store) SubscribeLeaderEvents(ctx context.Context) (<-chan models.LeaderEvent, error) {
	ch := make(chan models.LeaderEvent, 10)
	go func() {
		tk := time.NewTicker(s.cfg.ReconcileInterval)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				tickCtx, tickCancel := context.WithTimeout(ctx, s.cfg.TickTimeout)
				ev := models.TickEvent{
					Ctx:    tickCtx,
					Cancel: tickCancel,
				}
				select {
				case ch <- ev:
				default:
					if ev.Cancel != nil {
						ev.Cancel()
					}
				}
			}
		}
	}()
	return ch, nil
}

func (s *Store) SubscribeRecruiterEvents(ctx context.Context) (<-chan models.RecruiterEvent, error) {
	ch := make(chan models.RecruiterEvent, 10)
	go func() {
		tk := time.NewTicker(s.cfg.ReconcileInterval)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				tickCtx, tickCancel := context.WithTimeout(ctx, s.cfg.TickTimeout)
				ev := models.TickEvent{
					Ctx:    tickCtx,
					Cancel: tickCancel,
				}
				select {
				case ch <- ev:
				default:
					if ev.Cancel != nil {
						ev.Cancel()
					}
				}
			}
		}
	}()
	return ch, nil
}

func (s *Store) notifyEvent(ch chan<- models.Event, ev models.Event) {
	select {
	case ch <- ev:
	default:
	}
}

func (s *Store) runLeaderWatcher(ctx context.Context, ch chan<- models.Event) {
	wChan := s.cli.Watch(ctx, s.cfg.LeaderKey)
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-wChan:
			if !ok {
				return
			}
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypeDelete {
					s.notifyEvent(ch, models.Event{Type: models.EventLeaderDeleted})
				}
			}
		}
	}
}

func (s *Store) runTicker(ctx context.Context, ch chan<- models.Event) {
	tk := time.NewTicker(s.cfg.ReconcileInterval)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			s.notifyEvent(ch, models.Event{Type: models.EventReconcileTick})
		}
	}
}

func (s *Store) runAssignmentWatch(ctx context.Context, nodeAsgPath string, rev int64, ch chan<- models.Event) {
	for {
		if !s.watchAssignmentLoop(ctx, nodeAsgPath, &rev, ch) {
			return
		}
		t := time.NewTimer(s.cfg.WatchReconnectDelay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func (s *Store) watchAssignmentLoop(ctx context.Context, nodeAsgPath string, rev *int64, ch chan<- models.Event) bool {
	wCh := s.cli.Watch(ctx, nodeAsgPath, clientv3.WithRev(*rev+1))

	for {
		select {
		case <-ctx.Done():
			return false
		case resp, ok := <-wCh:
			if !ok {
				return true
			}
			if resp.Canceled {
				if errors.Is(resp.Err(), context.Canceled) {
					return false
				}
				return true
			}
			if resp.Header.Revision > *rev {
				*rev = resp.Header.Revision
			}

			if len(resp.Events) > 0 {
				s.notifyEvent(ch, models.Event{Type: models.EventAssignmentChange})
			}
		}
	}
}

func (s *Store) GetActiveNodeIDs(ctx context.Context) ([]string, error) {
	resp, err := s.cli.Get(ctx, s.cfg.PrefixHeartbeats, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, kv := range resp.Kvs {
		if id := string(kv.Key[len(s.cfg.PrefixHeartbeats):]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *Store) GetAllAssignments(ctx context.Context) ([]models.Assignment, error) {
	resp, err := s.cli.Get(ctx, s.cfg.PrefixDefs, clientv3.WithPrefix())
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.cfg.RetryInterval):
	}
	_, err := s.cli.MemberPromote(ctx, id)
	return err
}

func (s *Store) RemoveMember(id uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout)
	defer cancel()
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

func (s *Store) GetClusterPeerURLs(ctx context.Context) (map[string]bool, error) {
	members, err := s.GetClusterMembers(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		for _, u := range m.PeerURLs {
			seen[u] = true
		}
	}
	return seen, nil
}
