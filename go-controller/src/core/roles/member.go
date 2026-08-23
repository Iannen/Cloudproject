package roles

import (
	"context"
	"encoding/json"
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
	log.Printf("[Member] Permanent member role started for node %s", nodeID)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sess, err := m.store.NewSession(ctx, config.SessionTTL, config.StartupRetries, config.RetryInterval)
		if err != nil {
			log.Printf("[Member] Failed to establish fresh session: %v", err)
			return
		}

		sCtx, cancel := context.WithCancel(ctx)
		if err := m.runSession(sCtx, sess, nodeID); err != nil {
			log.Printf("[Member] Session ended: %v", err)
		}

		cancel()
		_ = sess.Close()
		m.stopManagedAssignments()
	}
}

func (m *MemberRole) runSession(sCtx context.Context, sess *concurrency.Session, nodeID string) error {
	hbKey := config.NodeHeartbeatPath(nodeID)
	if err := m.store.PutWithSession(sCtx, sess, hbKey, "alive"); err != nil {
		return err
	}

	if isLeader, err := m.store.ClaimLeader(sCtx, sess, config.ClusterLeaderKey, nodeID); err == nil && isLeader {
		log.Println("[Member] Won leadership! Launching Leader Role...")
		_ = m.registry.Start(&models.Assignment{
			NodeID: nodeID,
			ID:     "leader-" + nodeID,
			Role:   "leader",
		})
	}

	initIDs, rev, err := m.store.NodeAssignments(sCtx, config.NodeAssignmentsPath(nodeID))
	if err != nil {
		return err
	}

	m.reconcile(sCtx, initIDs)

	ch := make(chan event, 10)
	m.startLeaderWatcher(sCtx, ch)
	m.startWatch(sCtx, nodeID, rev, ch)
	m.startTicker(sCtx, ch)

	for {
		select {
		case <-sCtx.Done():
			return sCtx.Err()

		case <-sess.Done():
			return errors.New("session lease expired")

		case e := <-ch:
			var targets []string
			if e.kind == evtWatch && e.hasIDs {
				targets = e.ids
			} else {
				ids, _, err := m.store.NodeAssignments(sCtx, config.NodeAssignmentsPath(nodeID))
				if err != nil {
					log.Printf("[Member] Error updating assignment state: %v", err)
					continue
				}
				targets = ids
			}
			m.reconcile(sCtx, targets)
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

func (m *MemberRole) reconcile(ctx context.Context, ids []string) {
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

func (m *MemberRole) startTicker(ctx context.Context, ch chan<- event) {
	go func() {
		tk := time.NewTicker(config.ReconcileInterval)
		defer tk.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				select {
				case ch <- event{kind: evtTick}:
				default:
				}
			}
		}
	}()
}

func (m *MemberRole) startLeaderWatcher(ctx context.Context, ch chan<- event) {
	delCh := make(chan struct{}, 1)
	go m.store.WatchLeaderKey(ctx, config.ClusterLeaderKey, delCh)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-delCh:
			select {
			case ch <- event{kind: evtTick}:
			default:
			}
		}
	}()
}

func (m *MemberRole) startWatch(ctx context.Context, nodeID string, rev int64, ch chan<- event) {
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

func (m *MemberRole) watchLoop(ctx context.Context, nodeID string, rev *int64, ch chan<- event) bool {
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

			for _, ev := range resp.Events {
				var ids []string
				if ev.Type != clientv3.EventTypeDelete {
					if err := json.Unmarshal(ev.Kv.Value, &ids); err != nil {
						continue
					}
				}
				select {
				case ch <- event{kind: evtWatch, ids: ids, hasIDs: true}:
				default:
				}
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

type eventType int

const (
	evtTick eventType = iota
	evtWatch
)

type event struct {
	kind   eventType
	ids    []string
	hasIDs bool
}

func NewMemberRole(store ParticipantStore, registry RoleMgr) *MemberRole {
	return &MemberRole{
		store:    store,
		registry: registry,
	}
}
