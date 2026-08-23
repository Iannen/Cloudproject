package roles

import (
	"context"
	"errors"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type MemberRole struct {
	store    ParticipantStore
	registry RoleMgr
}

func (m *MemberRole) Run(ctx context.Context, asg *models.Assignment) {
	nodeID := config.NodeID()
	log.Printf("[Member] Member role starting setup for node %s", nodeID)

	sess, err := m.store.NewSession(ctx, config.SessionTTL, config.StartupRetries, config.RetryInterval)
	if err != nil {
		log.Printf("[Member] Failed to establish initial session: %v", err)
		return
	}

	defer func() {
		_ = sess.Close()
		m.stopManagedAssignments()
	}()
	hbKey := config.NodeHeartbeatPath(nodeID)
	if err := m.store.PutWithSession(ctx, sess, hbKey, "alive"); err != nil {
		log.Printf("[Member] Failed to register heartbeat presence: %v", err)
		return
	}

	if err := m.runSession(ctx, sess, nodeID); err != nil {
		log.Printf("[Member] Session terminated: %v", err)
	}
}

func (m *MemberRole) runSession(sCtx context.Context, sess *concurrency.Session, nodeID string) error {
	if isLeader, err := m.store.ClaimLeader(sCtx, sess, config.ClusterLeaderKey, nodeID); err == nil && isLeader {
		log.Println("[Member] Won leadership! Launching Leader Role...")
		_ = m.registry.Start(&models.Assignment{
			NodeID: nodeID,
			ID:     "leader-" + nodeID,
			Role:   "leader",
		})
	}

	_, rev, err := m.store.NodeAssignments(sCtx, config.NodeAssignmentsPath(nodeID))
	if err != nil {
		return err
	}

	m.reconcile(sCtx, nodeID)
	ch := m.createEventChannel(sCtx, nodeID, rev)

	for {
		select {
		case <-sCtx.Done():
			return sCtx.Err()

		case <-sess.Done():
			return errors.New("session lease expired")

		case <-ch:
			m.reconcile(sCtx, nodeID)
		}
	}
}

func (m *MemberRole) stopManagedAssignments() {
	active := m.registry.ActiveAssignments()
	for id := range active {
		if strings.HasPrefix(id, "node-") || strings.HasPrefix(id, "member-") {
			continue
		}
		m.registry.Stop(id)
	}
}

func (m *MemberRole) reconcile(ctx context.Context, nodeID string) {
	ids, _, err := m.store.NodeAssignments(ctx, config.NodeAssignmentsPath(nodeID))
	if err != nil {
		log.Printf("[Member] Error updating assignment state: %v", err)
		return
	}

	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}

	active := m.registry.ActiveAssignments()
	for id := range active {
		if strings.HasPrefix(id, "leader-") || strings.HasPrefix(id, "member-") || strings.HasPrefix(id, "node-") {
			continue
		}
		if !want[id] {
			m.registry.Stop(id)
		}
	}

	for _, id := range ids {
		if !active[id] {
			asg, err := m.store.AssignmentDef(ctx, config.AsgDefPath(id))
			if err != nil {
				log.Printf("[Member] fetch assignment def failed id=%s: %v", id, err)
				continue
			}
			if err := m.registry.Start(asg); err != nil {
				log.Printf("[Member] start assignment failed id=%s role=%s: %v", id, asg.Role, err)
			}
		}
	}
}

type ParticipantStore interface {
	NodeAssignments(ctx context.Context, nodeAsgPath string) ([]string, int64, error)
	WatchAssignments(ctx context.Context, nodeAsgPath string, rev int64) clientv3.WatchChan
	AssignmentDef(ctx context.Context, asgDefPath string) (*models.Assignment, error)
	NewSession(ctx context.Context, ttl int64, retries int, interval time.Duration) (*concurrency.Session, error)
	PutWithSession(ctx context.Context, sess *concurrency.Session, key string, value string) error
	ClaimLeader(ctx context.Context, sess *concurrency.Session, leaderKey, nodeID string) (bool, error)
	WatchLeaderKey(ctx context.Context, leaderKey string, notifyChan chan<- struct{})
}

func NewMemberAssignment(nodeID string) *models.Assignment {
	return &models.Assignment{
		NodeID: nodeID,
		ID:     "member-" + nodeID,
		Role:   "member",
	}
}

func NewMemberRole(store ParticipantStore, registry RoleMgr) *MemberRole {
	return &MemberRole{
		store:    store,
		registry: registry,
	}
}

func (m *MemberRole) createEventChannel(ctx context.Context, nodeID string, rev int64) <-chan struct{} {
	ch := make(chan struct{}, 10)
	m.startLeaderWatcher(ctx, ch)
	m.startWatch(ctx, nodeID, rev, ch)
	m.startTicker(ctx, ch)
	return ch
}

func (m *MemberRole) notifySignal(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (m *MemberRole) startTicker(ctx context.Context, ch chan<- struct{}) {
	go func() {
		tk := time.NewTicker(config.ReconcileInterval)
		defer tk.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				m.notifySignal(ch)
			}
		}
	}()
}

func (m *MemberRole) startLeaderWatcher(ctx context.Context, ch chan<- struct{}) {
	delCh := make(chan struct{}, 1)
	go m.store.WatchLeaderKey(ctx, config.ClusterLeaderKey, delCh)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-delCh:
			m.notifySignal(ch)
		}
	}()
}

func (m *MemberRole) startWatch(ctx context.Context, nodeID string, rev int64, ch chan<- struct{}) {
	go func() {
		for {
			if !m.watchLoop(ctx, nodeID, &rev, ch) {
				return
			}
			t := time.NewTimer(config.WatchReconnectDelay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
	}()
}

func (m *MemberRole) watchLoop(ctx context.Context, nodeID string, rev *int64, ch chan<- struct{}) bool {
	wCh := m.store.WatchAssignments(ctx, config.NodeAssignmentsPath(nodeID), *rev+1)

	for {
		select {
		case <-ctx.Done():
			return false
		case resp, ok := <-wCh:
			if !ok {
				return true
			}
			if resp.Canceled {
				return !errors.Is(resp.Err(), context.Canceled)
			}
			if resp.Header.Revision > *rev {
				*rev = resp.Header.Revision
			}

			if len(resp.Events) > 0 {
				m.notifySignal(ch)
			}
		}
	}
}
