package roles

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"cloud-controller/src/config"
	"cloud-controller/src/models"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// MemberStore defines the required storage capabilities required by the member role.
type MemberStore interface {
	GetNodeAssignmentsWithRev(ctx context.Context, nodeID string) ([]string, int64, error)
	WatchNodeAssignmentsFromRev(ctx context.Context, nodeID string, rev int64) clientv3.WatchChan
	GetAssignmentDefinition(ctx context.Context, assignmentID string) (*models.Assignment, error)
	KeepAliveLease(ctx context.Context, key string, value string, ttl int64) error
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

// RunMemberRole is the permanent assignment fulfillment and control loop processing node.
func RunMemberRole(ctx context.Context, s MemberStore) {
	nodeID := config.NodeID()
	log.Printf("[Member] Permanent member role started for node %s", nodeID)

	runningRuntimes := make(map[string]*AssignmentRuntime)
	eventChan := make(chan ReconcileEvent, 1)

	var lastRevision int64

	// A. BOOTSTRAP PHASE
	initialIDs, rev, err := s.GetNodeAssignmentsWithRev(ctx, nodeID)
	if err != nil {
		log.Printf("[Member] Error fetching initial bootstrap assignments: %v", err)
	} else {
		lastRevision = rev
		reconcile(ctx, s, initialIDs, runningRuntimes)
		// Seed eager cluster heartbeats for bootstrap assignments
		for _, id := range initialIDs {
			_ = s.KeepAliveLease(ctx, "heartbeats/assignments/"+id, "running", 5)
		}
	}

	// B. EVENT SOURCES (TWO PRODUCERS)

	// 1. etcd watch goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				watchChan := s.WatchNodeAssignmentsFromRev(ctx, nodeID, lastRevision+1)
				log.Printf("[Member] Monitoring assignment watch stream from revision: %d", lastRevision+1)

				for {
					select {
					case <-ctx.Done():
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

						if resp.Header.Revision > lastRevision {
							lastRevision = resp.Header.Revision
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
								// Buffer size 1 capacity guarantees shedding duplicate bursts
							}
						}
					}
				}
			Reconnect:
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// 2. ticker goroutine
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
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

	// C. SINGLE CONSUMER LOOP (RECONCILIATION GATE)
	go func() {
		nodeHeartbeatKey := "heartbeats/nodes/" + nodeID

		for {
			select {
			case <-ctx.Done():
				log.Printf("[Member] Permanent member role stopped for node %s", nodeID)
				for id, runtime := range runningRuntimes {
					log.Printf("[Member] Stopping active runtime %s during member teardown", id)
					runtime.Stop()
				}
				return
			case evt := <-eventChan:
				var targetIDs []string
				var newlySpawned []string

				if evt.Type == EventWatch && evt.HasWatchIDs {
					// Fast-path watch: utilize pre-extracted IDs directly
					targetIDs = evt.WatchIDs
				} else {
					// Ticker-driven or fallback: explicitly fetch the latest assignment state from store
					ids, _, err := s.GetNodeAssignmentsWithRev(ctx, nodeID)
					if err != nil {
						log.Printf("[Member] Error updating assignment state from ticker: %v", err)
						continue
					}
					targetIDs = ids
				}

				// Reconcile and track newly added runtime implementations
				newlySpawned = reconcile(ctx, s, targetIDs, runningRuntimes)

				// DUAL-TRACK HEARTBEAT EXECUTION
				if evt.Type == EventTick {
					// Track 1: Ticker-Driven Complete Batch Heartbeat Renewal
					log.Printf("[Member] Performing batch heartbeat renewal for node and active assignments.")
					_ = s.KeepAliveLease(ctx, nodeHeartbeatKey, "alive", 5)
					for id := range runningRuntimes {
						asgKey := "heartbeats/assignments/" + id
						_ = s.KeepAliveLease(ctx, asgKey, "running", 5)
					}
				} else {
					// Track 2: Event-Driven Fast-Path Eager Initialization
					if len(newlySpawned) > 0 {
						log.Printf("[Member] Executing immediate eager heartbeat initialization for newly spawned assignments: %v", newlySpawned)
						for _, id := range newlySpawned {
							asgKey := "heartbeats/assignments/" + id
							_ = s.KeepAliveLease(ctx, asgKey, "running", 5)
						}
					}
				}
			}
		}
	}()
}

// reconcile performs sequential single-threaded updates to existing application state runtimes.
// Returns a slice containing IDs of newly created runtimes to facilitate prompt event announcing.
func reconcile(
	ctx context.Context,
	s MemberStore,
	desiredIDs []string,
	runningRuntimes map[string]*AssignmentRuntime,
) []string {
	desiredMap := make(map[string]bool)
	for _, id := range desiredIDs {
		desiredMap[id] = true
	}

	// Stop any runtimes no longer registered
	for id, runtime := range runningRuntimes {
		if !desiredMap[id] {
			log.Printf("[Reconciliation] Stopping assignment: %s", id)
			runtime.Stop()
			delete(runningRuntimes, id)
		}
	}

	var newlySpawned []string

	// Initialize new runtimes cleanly matching criteria rules
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
			runtime.Start(ctx, entry)
			runningRuntimes[id] = runtime
			newlySpawned = append(newlySpawned, id)
		}
	}

	return newlySpawned
}
