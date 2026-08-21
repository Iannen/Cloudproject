package main

import (
	"context"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	adapters "go-controller/src/infra"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	nodeID := config.NodeID()
	log.Printf("[Main] Starting controller node with ID: %s", nodeID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	docker := adapters.NewDockerAdapter()
	etcd := adapters.NewStore()
	http := adapters.NewListenerAdapter()
	osa := adapters.NewOsAdapter()
	ts := adapters.NewTailscaleAdapter()

	reg := NewRegistry(
		ctx,
		docker,
		etcd,
		http,
		osa,
		ts,
	)

	nodeAsg := &models.Assignment{
		NodeID: nodeID,
		ID:     "node-" + nodeID,
		Role:   "node",
	}

	if err := reg.Start(nodeAsg, nil); err != nil {
		log.Fatalf("[Main] Failed to start base node role: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Shutting down node...")
	reg.StopAll()
	cancel()

	log.Println("[Main] Node stopped cleanly")
}
