package main

import (
	"context"
	"go-controller/src/adapters"
	"go-controller/src/core/registry"
	"go-controller/src/core/roles"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var timeout = 3 * time.Second
	var bootstrapDir = "/root/bootstrap"
	var etcdEndpoint = "localhost:2379"
	var startupInterval = 1 * time.Second
	var startupRetries = 10

	osa := adapters.NewOsAdapter(adapters.OsConfig{
		BootstrapDir: bootstrapDir,
		EnvFileName:  ".env",
		FilePerms:    0644,
		EnvTemplate:  "HOSTNAME=%s\nTAILSCALE_IP=%s\nETCD_NAME=%s\nETCD_INITIAL_CLUSTER=%s\nETCD_INITIAL_CLUSTER_STATE=existing\n",
	})

	nodeID := osa.GetNodeID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	docker := adapters.NewDockerAdapter(adapters.DockerConfig{
		BinaryPath:      "docker",
		BootstrapDir:    bootstrapDir,
		ComposeCmd:      []string{"compose"},
		UpArgs:          []string{"up", "-d", "etcd"},
		DownArgs:        []string{"down", "etcd", "--volumes", "--remove-orphans"},
		StartupRetries:  startupRetries,
		StartupInterval: startupInterval,
		EtcdEndpoint:    etcdEndpoint,
	})
	etcd := adapters.NewStore(adapters.StoreConfig{
		NodeID:              nodeID,
		Endpoint:            etcdEndpoint,
		Timeout:             timeout,
		StartupInterval:     startupInterval,
		StartupRetries:      startupRetries,
		SessionTTL:          5,
		RetryInterval:       2 * time.Second,
		LeaderKey:           "cluster/leader",
		ReconcileInterval:   3 * time.Second,
		WatchReconnectDelay: 1 * time.Second,
		PrefixHeartbeats:    "heartbeats/nodes/",
		PrefixDefs:          "assignments/definitions/",
		PrefixNodeAsgs:      "assignments/nodes/",
	})
	httpSrv := adapters.NewHTTPServerAdapter(adapters.HTTPServerConfig{
		Addr:          ":8080",
		ClientTimeout: timeout,
	})
	httpCli := adapters.NewHTTPClientAdapter(adapters.HTTPClientConfig{
		Timeout:              3 * time.Second,
		AssimilateURLPattern: "http://%s:8080/assimilate",
		ActivateURLPattern:   "http://%s:8080/activate",
		EtcdEndpoint:         etcdEndpoint,
		StartupRetries:       startupRetries,
		StartupInterval:      startupInterval,
	})
	ts := adapters.NewTailscaleAdapter(adapters.TailscaleConfig{
		BinaryPath:     "tailscale",
		EnvKey:         "TAILSCALE_IP",
		NodeNamePrefix: "kaffcloud",
	})

	reg := registry.NewRegistry(
		ctx,
		etcd,
		httpCli,
		ts,
	)

	memberAsg := roles.NewMemberAssignment(nodeID)
	memberRole := roles.NewMemberRole(memberAsg, etcd, reg)

	rpcHandler := roles.NewRPCHandler(ctx, memberRole, docker, osa)
	adapters.RegisterGet(httpSrv, "/initialize", rpcHandler.HandleInit)
	adapters.RegisterGet(httpSrv, "/logs", rpcHandler.HandleGetLogs)
	adapters.RegisterGet(httpSrv, "/activate", rpcHandler.HandleActivate)
	adapters.RegisterPost(httpSrv, "/assimilate", rpcHandler.HandleAssimilate)

	errCh := httpSrv.Start()
	log.Printf("[Main] %s is initialized", nodeID)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		log.Printf("[Main] HTTP server error: %v", err)
	case <-sigChan:
		log.Println("[Main] Shutting down node...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Main] Error shutting down HTTP server: %v", err)
	}

	reg.StopAll()
	cancel()

	log.Println("[Main] Node stopped cleanly")
}
