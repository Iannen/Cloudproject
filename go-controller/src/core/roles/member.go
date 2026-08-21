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
	store    MemberStore
	registry RegistryInterface
}

func (m *MemberRole) Run(ctx context.Context, asg *models.Assignment) error {
	nodeID := config.NodeID()
	log.Printf("[Member] Permanent member role started for node %s", nodeID)

	hbKey := config.NodeHeartbeatPath(nodeID)

	for {
		select {
		case <-ctx.Done():
			m.registry.StopAll()
			return nil
		default:
		}

		sess, err := m.store.NewSession(ctx, config.SessionTTL)
		if err != nil {
			log.Printf("[Member] Failed to establish fresh session: %v. Retrying in 2 seconds...", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(config.RetryInterval):
			}
			continue
		}

		sCtx, cancel := context.WithCancel(ctx)
		ch := make(chan event, 10)

		if err := m.store.PutWithSession(sCtx, sess, hbKey, "alive"); err != nil {
			log.Printf("[Member] Failed to register node heartbeat: %v. Resetting session...", err)
			cancel()
			_ = sess.Close()
			continue
		}

		isLeader, err := m.store.ClaimLeader(sCtx, sess, nodeID)
		if err != nil {
			log.Printf("[Member] Leader check failed: %v", err)
		}

		if isLeader {
			log.Println("[Member] This node won leadership! Launching Leader Role...")
			leaderAsg := &models.Assignment{
				NodeID: nodeID,
				ID:     "leader-" + nodeID,
				Role:   "leader",
			}
			if err := m.registry.Start(leaderAsg, sess); err != nil {
				log.Printf("[Member] Failed to start leader role: %v", err)
			}
		} else {
			m.startLeaderWatcher(sCtx, ch)
		}

		initIDs, rev, err := m.store.NodeAssignments(sCtx, nodeID)
		if err != nil {
			log.Printf("[Member] Error fetching initial bootstrap assignments: %v. Resetting session...", err)
			cancel()
			_ = sess.Close()
			continue
		}

		m.reconcile(sCtx, initIDs, sess)

		m.startWatch(sCtx, nodeID, rev, ch)
		m.startTicker(sCtx, ch)

		alive := true
		for alive {
			select {
			case <-ctx.Done():
				cancel()
				m.registry.StopAll()
				_ = sess.Close()
				return nil

			case <-sess.Done():
				log.Println("[Member] CRITICAL: Session lease lost! Evicting child roles...")
				cancel()
				m.registry.StopAll()
				alive = false

			case e := <-ch:
				var targets []string
				if e.kind == evtWatch && e.hasIDs {
					targets = e.ids
				} else {
					ids, _, err := m.store.NodeAssignments(sCtx, nodeID)
					if err != nil {
						log.Printf("[Member] Error updating assignment state from ticker: %v", err)
						continue
					}
					targets = ids
				}

				m.reconcile(sCtx, targets, sess)
			}
		}

		cancel()
		_ = sess.Close()
	}
}

func (m *MemberRole) reconcile(
	ctx context.Context,
	ids []string,
	sess *concurrency.Session,
) {
	log.Printf("[Member] enter reconcile")
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
			asg, err := m.store.AssignmentDef(ctx, id)
			if err != nil {
				log.Printf("[Member] fetch assignment def failed id=%s: %v", id, err)
				continue
			}

			if err := m.registry.Start(asg, sess); err != nil {
				log.Printf("[Member] start assignment failed id=%s role=%s: %v", id, asg.Role, err)
				continue
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
	go m.store.WatchLeaderKey(ctx, delCh)

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
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !m.watchLoop(ctx, nodeID, &rev, ch) {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(config.WatchReconnectDelay):
			}
		}
	}()
}

func (m *MemberRole) watchLoop(ctx context.Context, nodeID string, rev *int64, ch chan<- event) bool {
	wCh := m.store.WatchAssignments(ctx, nodeID, *rev+1)

	for {
		select {
		case <-ctx.Done():
			return false
		case resp, ok := <-wCh:
			if !ok {
				return true
			}

			if resp.Canceled {
				if !errors.Is(resp.Err(), context.Canceled) {
					log.Printf("[Member] watch canceled revision=%d: %v", *rev, resp.Err())
				}
				return !errors.Is(resp.Err(), context.Canceled)
			}

			if resp.Header.Revision > *rev {
				*rev = resp.Header.Revision
			}

			for _, ev := range resp.Events {
				var ids []string
				if ev.Type != clientv3.EventTypeDelete {
					if err := json.Unmarshal(ev.Kv.Value, &ids); err != nil {
						log.Printf("[Member] Error unmarshaling watched assignments: %v", err)
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

type MemberStore interface {
	NodeAssignments(ctx context.Context, nodeID string) ([]string, int64, error)
	WatchAssignments(ctx context.Context, nodeID string, rev int64) clientv3.WatchChan
	AssignmentDef(ctx context.Context, assignmentID string) (*models.Assignment, error)
	NewSession(ctx context.Context, ttl int64) (*concurrency.Session, error)
	PutWithSession(ctx context.Context, sess *concurrency.Session, key string, value string) error
	ClaimLeader(ctx context.Context, sess *concurrency.Session, nodeID string) (bool, error)
	WatchLeaderKey(ctx context.Context, notifyChan chan<- struct{})
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

func NewMemberRole(store MemberStore, registry RegistryInterface) *MemberRole {
	return &MemberRole{
		store:    store,
		registry: registry,
	}
}
