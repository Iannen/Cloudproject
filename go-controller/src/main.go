package main

import (
	"context"
	"go-controller/src/adapters"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"go-controller/src/core/registry"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	nodeID := config.NodeID()
	log.Printf("[Main] Starting node with ID: %s", nodeID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	docker := adapters.NewDockerAdapter(
		config.DockerBinary,
		config.DockerComposeCmd,
		config.DockerUpArgs,
		config.DockerDownArgs,
	)
	etcd := adapters.NewStore(
		config.ClusterLeaderKey,
		config.ReconcileInterval,
		config.WatchReconnectDelay,
		config.NodeAssignmentsPath,
		config.AsgDefPath,
	)
	httpSrv := adapters.NewHTTPServerAdapter()
	httpCli := adapters.NewHTTPClientAdapter(
		config.Timeout,
		config.AssimilateURLPattern,
		config.ActivateURLPattern,
	)
	osa := adapters.NewOsAdapter(config.EnvFileName, config.EnvFilePerm, config.EnvTemplate)
	ts := adapters.NewTailscaleAdapter(config.TailscaleBinary, config.TailscaleIPEnv)

	reg := registry.NewRegistry(
		ctx,
		docker,
		etcd,
		httpSrv,
		httpCli,
		httpCli,
		osa,
		ts,
	)

	nodeAsg := &models.Assignment{
		NodeID: nodeID,
		ID:     "node-" + nodeID,
		Role:   "node",
	}

	if err := reg.Start(nodeAsg); err != nil {
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
