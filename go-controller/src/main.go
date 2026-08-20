package main

import (
	"cloud-controller/src/core/config"
	"cloud-controller/src/core/infra"
	"cloud-controller/src/core/roles"
	adapters "cloud-controller/src/infra"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Printf("[Main] Starting controller node with ID: %s", config.NodeID())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := roles.NewRegistry()

	dcr := adapters.NewDockerAdapter()
	osa := adapters.NewOsAdapter()
	cms := adapters.NewListenerAdapter()

	server := infra.NewHttpServer(ctx, ":8080", reg, dcr, osa, cms, cms)
	server.Start(":8080")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Shutting down node...")
	cancel()

	_ = server.Shutdown(context.Background())
	log.Println("[Main] Node stopped cleanly")
}
