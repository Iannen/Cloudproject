package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"cloud-controller/src/config"
	"cloud-controller/src/infra"
)

func main() {
	if os.Getenv("NODE_ID") == "" {
		log.Fatalf("[Main] Critical Error: NODE_ID environment variable is required but not set.")
	}
	config.InitNodeID()
	log.Printf("[Main] Starting controller node with ID: %s", config.NodeID())
	/*
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
	*/
	server := infra.NewHttpServer(":8080")

	go server.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Shutting down controller node...")
	/*
		cancel()
	*/
	_ = server.Shutdown(context.Background())
	log.Println("[Main] Node stopped cleanly")
}
