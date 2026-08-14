package roles

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"cloud-controller/src/adapters"
	"cloud-controller/src/config"
	"cloud-controller/src/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type MemberStore interface {
	GetNodeAssignmentsWithRev(ctx context.Context, nodeID string) ([]string, int64, error)
	WatchNodeAssignmentsFromRev(ctx context.Context, nodeID string, rev int64) clientv3.WatchChan
	GetAssignmentDefinition(ctx context.Context, assignmentID string) (*models.Assignment, error)
	NewSession(ctx context.Context, ttl int64) (adapters.SessionWrapper, error)
	PutWithSession(ctx context.Context, sess adapters.SessionWrapper, key string, value string) error
	TryClaimLeadership(ctx context.Context, sess adapters.SessionWrapper, nodeID string) (bool, error)
	WatchLeaderKey(ctx context.Context, notifyChan chan<- struct{})
}

func NewMemberAssignment(nodeID string) *models.Assignment {
	return &models.Assignment{
		NodeID: nodeID,
		ID:     "member-" + nodeID,
		Role:   "member",
	}
}

type EventType int

const (
	EventTick EventType = iota
	EventWatch
)

type ReconcileEvent struct {
	Type        EventType
	WatchIDs    []string
	HasWatchIDs bool
}

type MemberRole struct {
	store    MemberStore
	registry *Registry
}

func stopAllRuntimes(runningRuntimes map[string]*AssignmentRuntime) {
	for id, runtime := range runningRuntimes {
		log.Printf("[Member] Stopping active runtime %s during session transition/teardown", id)
		runtime.Stop()
		delete(runningRuntimes, id)
	}
}

func (m *MemberRole) Run(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error {
	nodeID := config.NodeID()
	log.Printf("[Member] Permanent member role started for node %s", nodeID)

	nodeHeartbeatKey := config.NodeHeartbeatPath(nodeID)
	runningRuntimes := make(map[string]*AssignmentRuntime)

	for {
		select {
		case <-ctx.Done():
			stopAllRuntimes(runningRuntimes)
			return nil
		default:
		}

		sess, err := m.store.NewSession(ctx, config.EtcdSessionTTLSeconds)
		if err != nil {
			log.Printf("[Member] Failed to establish fresh session: %v. Retrying in 2 seconds...", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(config.MemberConnectRetryInterval):
			}
			continue
		}

		sessCtx, cancelSess := context.WithCancel(ctx)
		eventChan := make(chan ReconcileEvent, 10)

		if err := m.store.PutWithSession(sessCtx, sess, nodeHeartbeatKey, "alive"); err != nil {
			log.Printf("[Member] Failed to register node heartbeat: %v. Resetting session...", err)
			cancelSess()
			_ = sess.Close()
			continue
		}

		isLeader, err := m.store.TryClaimLeadership(sessCtx, sess, nodeID)
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

			if err := m.registry.Start(sessCtx, leaderAsg, sess); err != nil {
				log.Printf("[Member] Failed to start leader role: %v", err)
			}
		} else {
			m.spawnLeaderWatcher(sessCtx, eventChan)
		}

		initialIDs, lastRevision, err := m.store.GetNodeAssignmentsWithRev(sessCtx, nodeID)
		if err != nil {
			log.Printf("[Member] Error fetching initial bootstrap assignments: %v. Resetting session...", err)
			cancelSess()
			_ = sess.Close()
			continue
		}

		m.reconcile(sessCtx, initialIDs, runningRuntimes, sess)

		m.spawnWatchListener(sessCtx, nodeID, lastRevision, eventChan)
		m.spawnTickerNotifier(sessCtx, eventChan)

		sessionAlive := true
		for sessionAlive {
			select {
			case <-ctx.Done():
				cancelSess()
				stopAllRuntimes(runningRuntimes)
				_ = sess.Close()
				return nil

			case <-sess.Done():
				log.Println("[Member] CRITICAL: Session lease lost! Evicting child roles...")
				cancelSess()
				stopAllRuntimes(runningRuntimes)
				sessionAlive = false

			case evt := <-eventChan:
				var targetIDs []string
				if evt.Type == EventWatch && evt.HasWatchIDs {
					targetIDs = evt.WatchIDs
				} else {
					ids, _, err := m.store.GetNodeAssignmentsWithRev(sessCtx, nodeID)
					if err != nil {
						log.Printf("[Member] Error updating assignment state from ticker: %v", err)
						continue
					}
					targetIDs = ids
				}

				m.reconcile(sessCtx, targetIDs, runningRuntimes, sess)
			}
		}

		cancelSess()
		_ = sess.Close()
	}
}

func (m *MemberRole) spawnTickerNotifier(ctx context.Context, eventChan chan<- ReconcileEvent) {
	go func() {
		ticker := time.NewTicker(config.MemberReconcileInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case eventChan <- ReconcileEvent{Type: EventTick}:
				default:
				}
			}
		}
	}()
}

func (m *MemberRole) spawnLeaderWatcher(ctx context.Context, eventChan chan<- ReconcileEvent) {
	leaderDeletedChan := make(chan struct{}, 1)
	go m.store.WatchLeaderKey(ctx, leaderDeletedChan)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-leaderDeletedChan:
			select {
			case eventChan <- ReconcileEvent{Type: EventTick}:
			default:
			}
		}
	}()
}

func (m *MemberRole) spawnWatchListener(ctx context.Context, nodeID string, startRev int64, eventChan chan<- ReconcileEvent) {
	go func() {
		currentRev := startRev
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !m.watchStreamLoop(ctx, nodeID, &currentRev, eventChan) {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(config.MemberWatchReconnectDelay):
			}
		}
	}()
}

func (m *MemberRole) watchStreamLoop(ctx context.Context, nodeID string, currentRev *int64, eventChan chan<- ReconcileEvent) bool {
	watchChan := m.store.WatchNodeAssignmentsFromRev(ctx, nodeID, *currentRev+1)
	log.Printf("[Member] Monitoring assignment watch stream from revision: %d", *currentRev+1)

	for {
		select {
		case <-ctx.Done():
			return false
		case resp, ok := <-watchChan:
			if !ok {
				log.Printf("[Member] Assignment watch channel closed. Reconnecting...")
				return true
			}

			if resp.Canceled {
				log.Printf("[Member] Assignment watch canceled (Error: %v). Reconnecting...", resp.Err())
				if errors.Is(resp.Err(), context.Canceled) {
					return false
				}
				return true
			}

			if resp.Header.Revision > *currentRev {
				*currentRev = resp.Header.Revision
			}

			for _, ev := range resp.Events {
				var currentIDs []string
				if ev.Type != clientv3.EventTypeDelete {
					if err := json.Unmarshal(ev.Kv.Value, &currentIDs); err != nil {
						log.Printf("[Member] Error unmarshaling watched assignments: %v", err)
						continue
					}
				}

				evt := ReconcileEvent{
					Type:        EventWatch,
					WatchIDs:    currentIDs,
					HasWatchIDs: true,
				}

				select {
				case eventChan <- evt:
				default:
				}
			}
		}
	}
}

func (m *MemberRole) reconcile(
	ctx context.Context,
	desiredIDs []string,
	runningRuntimes map[string]*AssignmentRuntime,
	sess adapters.SessionWrapper,
) {
	desiredMap := make(map[string]bool)
	for _, id := range desiredIDs {
		desiredMap[id] = true
	}

	for id, runtime := range runningRuntimes {
		if !desiredMap[id] {
			log.Printf("[Reconciliation] Stopping assignment: %s", id)
			runtime.Stop()
			delete(runningRuntimes, id)
		}
	}

	for _, id := range desiredIDs {
		if _, running := runningRuntimes[id]; !running {
			log.Printf("[Reconciliation] Discovered new assignment: %s. Fetching definition...", id)
			asg, err := m.store.GetAssignmentDefinition(ctx, id)
			if err != nil {
				log.Printf("[Reconciliation] Failed to fetch definition for %s: %v", id, err)
				continue
			}

			log.Printf("[Reconciliation] Starting assignment %s with role %s", id, asg.Role)

			if err := m.registry.Start(ctx, asg, sess); err != nil {
				log.Printf("[Reconciliation] Failed to start assignment %s: %v", id, err)
				continue
			}

			runtime := NewAssignmentRuntime(asg)
			runningRuntimes[id] = runtime
		}
	}
}
