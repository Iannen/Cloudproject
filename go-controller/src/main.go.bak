package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud-controller/src/adapters"
	"cloud-controller/src/config"
	"cloud-controller/src/infra"
	"cloud-controller/src/roles"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	if os.Getenv("NODE_ID") == "" {
		log.Fatalf("[Main] Critical Error: NODE_ID environment variable is required but not set.")
	}
	config.InitNodeID()
	log.Printf("[Main] Starting node with ID: %s", config.NodeID())

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer cli.Close()

	s := adapters.NewStore(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go roles.RunMemberRole(ctx, s)

	server := infra.NewHttpServer(s, ":8080")
	server.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Shutting down node...")
	cancel()

	time.Sleep(500 * time.Millisecond)

	_ = server.Shutdown(context.Background())
	log.Println("[Main] Node stopped cleanly")
}
