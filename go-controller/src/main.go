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
	"time"
)

func main() {
	nodeID := config.NodeID()
	log.Printf("[Main] Starting node with ID: %s", nodeID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	docker := adapters.NewDockerAdapter(adapters.DockerConfig{
		BinaryPath:   "docker",
		BootstrapDir: config.BootstrapDir,
		ComposeCmd:   []string{"compose"},
		UpArgs:       []string{"up", "-d", "etcd"},
		DownArgs:     []string{"down", "etcd", "--volumes", "--remove-orphans"},
	})
	etcd := adapters.NewStore(adapters.StoreConfig{
		Endpoint:            config.EtcdEndpoint,
		Timeout:             config.Timeout,
		StartupInterval:     config.StartupInterval,
		StartupRetries:      config.StartupRetries,
		SessionTTL:          5,
		RetryInterval:       2 * time.Second,
		LeaderKey:           "cluster/leader",
		ReconcileInterval:   3 * time.Second,
		WatchReconnectDelay: 1 * time.Second,
		PrefixHeartbeats:    "heartbeats/nodes/",
		PrefixDefs:          "assignments/definitions/",
		NodeAssignmentsPath: config.NodeAssignmentsPath,
		AsgDefPath:          config.AsgDefPath,
	})
	httpSrv := adapters.NewHTTPServerAdapter(config.HTTPPort, config.Timeout)
	httpCli := adapters.NewHTTPClientAdapter(
		config.Timeout,
		config.AssimilateURLPattern,
		config.ActivateURLPattern,
		config.EtcdEndpoint,
		config.StartupRetries,
		config.StartupInterval,
	)
	osa := adapters.NewOsAdapter(config.BootstrapDir, config.EnvFileName, config.EnvFilePerm, config.EnvTemplate)
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
