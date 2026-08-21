package main

import (
	"context"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"go-controller/src/core/roles"
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

	listener := adapters.NewListenerAdapter()
	deps := roles.Dependencies{
		Docker:   adapters.NewDockerAdapter(),
		Os:       adapters.NewOsAdapter(),
		Listener: listener,
		Speaker:  listener,
	}

	reg := roles.NewRegistry(ctx, deps)

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
