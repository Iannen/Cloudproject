package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud-controller/src/adapters"
	"cloud-controller/src/infra"
	"cloud-controller/src/roles"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func reconcile(ctx context.Context, s *adapters.Store, desiredIDs []string, runningRuntimes map[string]*roles.AssignmentRuntime) {
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

			entry, found := roles.Registry[asg.Role]
			if !found {
				log.Printf("[Reconciliation] Unknown role '%s' for assignment %s", asg.Role, id)
				continue
			}

			log.Printf("[Reconciliation] Starting assignment %s with role %s", id, asg.Role)
			runtime := roles.NewAssignmentRuntime(asg, s)
			runtime.Start(ctx, entry)
			runningRuntimes[id] = runtime
		}
	}
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		log.Fatalf("[Main] Critical Error: NODE_ID environment variable is required but not set.")
	}
	log.Printf("[Main] Starting node with ID: %s", nodeID)

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer cli.Close()

	s := adapters.NewStore(cli, nodeID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runningRuntimes := make(map[string]*roles.AssignmentRuntime)

	go roles.RunMemberRole(ctx, s, nodeID)

	go func() {
		log.Printf("[Main] Starting assignment watch loop for node: %s", nodeID)

		var lastRevision int64

		initialIDs, rev, err := s.GetNodeAssignmentsWithRev(ctx, nodeID)
		if err != nil {
			log.Printf("[Main] Error fetching initial assignments: %v", err)
		} else {
			lastRevision = rev
			reconcile(ctx, s, initialIDs, runningRuntimes)
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				watchChan := s.WatchNodeAssignmentsFromRev(ctx, nodeID, lastRevision+1)
				log.Printf("[Main] Monitoring assignment watch stream from revision: %d", lastRevision+1)

				if !handleWatchStream(ctx, watchChan, s, runningRuntimes, &lastRevision) {
					return
				}

				time.Sleep(1 * time.Second)
			}
		}
	}()

	server := infra.NewHttpServer(s, ":8080")
	server.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Shutting down node...")
	cancel()

	for id, runtime := range runningRuntimes {
		log.Printf("[Main] Stopping runtime for assignment %s during shutdown", id)
		runtime.Stop()
	}

	_ = server.Shutdown(context.Background())
	log.Println("[Main] Node stopped cleanly")
}

func handleWatchStream(
	ctx context.Context,
	watchChan clientv3.WatchChan,
	s *adapters.Store,
	runningRuntimes map[string]*roles.AssignmentRuntime,
	lastRevision *int64,
) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case resp, ok := <-watchChan:
			if !ok {
				log.Printf("[Main] Assignment watch channel closed. Reconnecting...")
				return true
			}
			if resp.Canceled {
				log.Printf("[Main] Assignment watch canceled (Error: %v). Reconnecting...", resp.Err())
				return true
			}

			if resp.Header.Revision > *lastRevision {
				*lastRevision = resp.Header.Revision
			}

			for _, ev := range resp.Events {
				var currentIDs []string
				if ev.Type == clientv3.EventTypeDelete {
					currentIDs = []string{}
				} else {
					if err := json.Unmarshal(ev.Kv.Value, &currentIDs); err != nil {
						log.Printf("[Main] Error unmarshaling watched assignments: %v", err)
						continue
					}
				}
				reconcile(ctx, s, currentIDs, runningRuntimes)
			}
		}
	}
}
