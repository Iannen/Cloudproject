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

func stopAllRuntimes(runningRuntimes map[string]*AssignmentRuntime) {
	for id, runtime := range runningRuntimes {
		log.Printf("[Member] Stopping active runtime %s during session transition/teardown", id)
		runtime.Stop()
		delete(runningRuntimes, id)
	}
}

func RunMemberRole(ctx context.Context, s MemberStore) {
	nodeID := config.NodeID()
	log.Printf("[Member] Permanent member role started for node %s", nodeID)

	nodeHeartbeatKey := "heartbeats/nodes/" + nodeID
	runningRuntimes := make(map[string]*AssignmentRuntime)

	for {
		select {
		case <-ctx.Done():
			stopAllRuntimes(runningRuntimes)
			return
		default:
		}

		sess, err := s.NewSession(ctx, 5)
		if err != nil {
			log.Printf("[Member] Failed to establish fresh session: %v. Retrying in 2 seconds...", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		sessCtx, cancelSess := context.WithCancel(ctx)
		eventChan := make(chan ReconcileEvent, 10)

		if err := s.PutWithSession(sessCtx, sess, nodeHeartbeatKey, "alive"); err != nil {
			log.Printf("[Member] Failed to register node heartbeat: %v. Resetting session...", err)
			cancelSess()
			_ = sess.Close()
			continue
		}

		initialIDs, lastRevision, err := s.GetNodeAssignmentsWithRev(sessCtx, nodeID)
		if err != nil {
			log.Printf("[Member] Error fetching initial bootstrap assignments: %v. Resetting session...", err)
			cancelSess()
			_ = sess.Close()
			continue
		}

		reconcile(sessCtx, s, initialIDs, runningRuntimes, sess)

		go func() {
			currentRev := lastRevision
			for {
				select {
				case <-sessCtx.Done():
					return
				default:
					watchChan := s.WatchNodeAssignmentsFromRev(sessCtx, nodeID, currentRev+1)
					log.Printf("[Member] Monitoring assignment watch stream from revision: %d", currentRev+1)

					for {
						select {
						case <-sessCtx.Done():
							return
						case resp, ok := <-watchChan:
							if !ok {
								log.Printf("[Member] Assignment watch channel closed. Reconnecting...")
								goto Reconnect
							}
							if resp.Canceled {
								log.Printf("[Member] Assignment watch canceled (Error: %v). Reconnecting...", resp.Err())
								if errors.Is(resp.Err(), context.Canceled) {
									return
								}
								goto Reconnect
							}

							if resp.Header.Revision > currentRev {
								currentRev = resp.Header.Revision
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
				Reconnect:
					select {
					case <-sessCtx.Done():
						return
					case <-time.After(1 * time.Second):
					}
				}
			}
		}()

		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-sessCtx.Done():
					return
				case <-ticker.C:
					evt := ReconcileEvent{Type: EventTick}
					select {
					case eventChan <- evt:
					default:
					}
				}
			}
		}()

		sessionAlive := true
		for sessionAlive {
			select {
			case <-ctx.Done():
				cancelSess()
				stopAllRuntimes(runningRuntimes)
				_ = sess.Close()
				return

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
					ids, _, err := s.GetNodeAssignmentsWithRev(sessCtx, nodeID)
					if err != nil {
						log.Printf("[Member] Error updating assignment state from ticker: %v", err)
						continue
					}
					targetIDs = ids
				}

				reconcile(sessCtx, s, targetIDs, runningRuntimes, sess)
			}
		}
		_ = sess.Close()
	}
}

func reconcile(
	ctx context.Context,
	s MemberStore,
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
			asg, err := s.GetAssignmentDefinition(ctx, id)
			if err != nil {
				log.Printf("[Reconciliation] Failed to fetch definition for %s: %v", id, err)
				continue
			}

			entry, found := Registry[asg.Role]
			if !found {
				log.Printf("[Reconciliation] Unknown role '%s' for assignment %s", asg.Role, id)
				continue
			}

			log.Printf("[Reconciliation] Starting assignment %s with role %s", id, asg.Role)
			runtime := NewAssignmentRuntime(asg)
			runtime.Start(ctx, entry, sess)
			runningRuntimes[id] = runtime
		}
	}
}
