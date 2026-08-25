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

	var timeout = 3 * time.Second
	docker := adapters.NewDockerAdapter(adapters.DockerConfig{
		BinaryPath:   "docker",
		BootstrapDir: config.BootstrapDir,
		ComposeCmd:   []string{"compose"},
		UpArgs:       []string{"up", "-d", "etcd"},
		DownArgs:     []string{"down", "etcd", "--volumes", "--remove-orphans"},
	})
	etcd := adapters.NewStore(adapters.StoreConfig{
		Endpoint:            config.EtcdEndpoint,
		Timeout:             timeout,
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
	httpSrv := adapters.NewHTTPServerAdapter(adapters.HTTPServerConfig{
		Addr:          ":8080",
		ClientTimeout: timeout,
	})
	httpCli := adapters.NewHTTPClientAdapter(adapters.HTTPClientConfig{
		Timeout:              3 * time.Second,
		AssimilateURLPattern: "http://%s:8080/assimilate",
		ActivateURLPattern:   "http://%s:8080/activate",
		EtcdEndpoint:         "localhost:2379",
		StartupRetries:       10,
		StartupInterval:      1 * time.Second,
	})
	osa := adapters.NewOsAdapter(adapters.OsConfig{
		BootstrapDir: config.BootstrapDir,
		EnvFileName:  ".env",
		FilePerms:    0644,
		EnvTemplate:  "HOSTNAME=%s\nTAILSCALE_IP=%s\nETCD_NAME=%s\nETCD_INITIAL_CLUSTER=%s\nETCD_INITIAL_CLUSTER_STATE=existing\n",
	})
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
