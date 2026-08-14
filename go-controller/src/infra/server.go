package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud-controller/src/adapters"
	"cloud-controller/src/config"
	"cloud-controller/src/models"
	"cloud-controller/src/roles"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type HttpServer struct {
	server   *http.Server
	appCtx   context.Context
	store    *adapters.Store
	registry *roles.Registry
}

func NewHttpServer(appCtx context.Context, addr string, registry *roles.Registry) *HttpServer {
	s := &HttpServer{
		appCtx:   appCtx,
		registry: registry,
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/initialize", s.handleInitialize)
	mux.HandleFunc("/assimilate", s.handleAssimilate)
	mux.HandleFunc("/activate", s.handleActivate)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

func (s *HttpServer) Start() {
	log.Printf("[HTTP] Server running on %s", s.server.Addr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()
}

func (s *HttpServer) Shutdown(ctx context.Context) error {
	log.Println("[HTTP] Shutting down...")
	return s.server.Shutdown(ctx)
}

func (s *HttpServer) handleInitialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeID := config.NodeID()
	log.Printf("[HTTP] Received /initialize request for node %s", nodeID)

	out, err := runDockerCompose(r.Context(), "ps", "-q", "etcd")
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		log.Printf("[HTTP] etcd container already present for node %s. Skipping boot.", nodeID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf("Node %s is already initialized (etcd container detected).\n", nodeID)))
		return
	}

	log.Printf("[HTTP] Starting etcd container in 'new' mode...")
	if _, err := runDockerCompose(r.Context(), "up", "-d", "etcd"); err != nil {
		log.Printf("[HTTP] Docker Compose error: %v", err)
		http.Error(w, fmt.Sprintf("failed to start etcd container: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[HTTP] Connecting gRPC client to %s...", config.EtcdGRPCEndpoint)
	cli, err := s.connectEtcdClient(r.Context())
	if err != nil {
		log.Printf("[HTTP] Failed to connect to etcd gRPC: %v", err)
		http.Error(w, "etcd started but gRPC connection failed", http.StatusInternalServerError)
		return
	}

	if err := s.startMemberRole(cli); err != nil {
		log.Printf("[HTTP] %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf("Cluster initialized on node %s. etcd is online.\n", nodeID)))
}

func (s *HttpServer) handleAssimilate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload models.AssimilatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}

	nodeID := config.NodeID()
	log.Printf("[HTTP] Received /assimilate for node %s", nodeID)

	envFile := fmt.Sprintf("%s/.env", config.BootstrapDir)
	envContent := fmt.Sprintf(
		"HOSTNAME=%s\nTAILSCALE_IP=%s\nETCD_NAME=%s\nETCD_INITIAL_CLUSTER=%s\nETCD_INITIAL_CLUSTER_STATE=existing\n",
		nodeID,
		payload.AssignedIP,
		nodeID,
		payload.EtcdInitialCluster,
	)

	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		http.Error(w, "failed to update configuration", http.StatusInternalServerError)
		return
	}

	_, _ = runDockerCompose(r.Context(), "down", "etcd", "--volumes", "--remove-orphans")

	if _, err := runDockerCompose(r.Context(), "up", "-d", "etcd"); err != nil {
		http.Error(w, fmt.Sprintf("failed to start etcd container: %v", err), http.StatusInternalServerError)
		return
	}

	var ready bool
	for i := 0; i < config.EtcdSocketPollAttempts; i++ {
		conn, err := net.DialTimeout("tcp", config.EtcdDialSocket, config.EtcdSocketPollInterval)
		if err == nil {
			_ = conn.Close()
			ready = true
			break
		}

		select {
		case <-r.Context().Done():
			http.Error(w, "request cancelled", http.StatusRequestTimeout)
			return
		case <-time.After(config.EtcdSocketPollInterval):
		}
	}

	if !ready {
		http.Error(w, "etcd socket timeout", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Learner container ready.\n"))
}

func (s *HttpServer) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeID := config.NodeID()
	log.Printf("[HTTP] Received /activate request for node %s. Connecting gRPC client...", nodeID)

	cli, err := s.connectEtcdClient(r.Context())
	if err != nil {
		log.Printf("[HTTP] Failed to connect to gRPC post-promotion: %v", err)
		http.Error(w, "gRPC connection failed", http.StatusInternalServerError)
		return
	}

	if err := s.startMemberRole(cli); err != nil {
		log.Printf("[HTTP] %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf("Node %s activated as active Voter member.\n", nodeID)))
}

func runDockerCompose(ctx context.Context, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"compose"}, args...)
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Dir = config.BootstrapDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("docker compose %s failed: %w (output: %s)", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

func (s *HttpServer) connectEtcdClient(ctx context.Context) (*clientv3.Client, error) {
	var cli *clientv3.Client
	var err error

	for i := 0; i < config.EtcdConnectRetryAttempts; i++ {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{config.EtcdGRPCEndpoint},
			DialTimeout: config.EtcdDialTimeout,
		})
		if err == nil {
			statusCtx, cancel := context.WithTimeout(ctx, config.EtcdStatusTimeout)
			_, err = cli.Status(statusCtx, config.EtcdGRPCEndpoint)
			cancel()
			if err == nil {
				return cli, nil
			}
			cli.Close()
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(config.EtcdConnectRetryInterval):
		}
	}

	return nil, fmt.Errorf("failed to establish healthy etcd gRPC connection after retries: %w", err)
}

func (s *HttpServer) startMemberRole(cli *clientv3.Client) error {
	s.store = adapters.NewStore(cli)
	log.Println("[HTTP] etcd store successfully bound to controller!")

	s.registry.SetStore(s.store)

	nodeID := config.NodeID()
	memberAsg := roles.NewMemberAssignment(nodeID)

	log.Println("[HTTP] Transitioning node into active Member role...")
	if err := s.registry.Start(s.appCtx, memberAsg, nil); err != nil {
		return fmt.Errorf("failed to start member role: %w", err)
	}

	return nil
}
