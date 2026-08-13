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
	server *http.Server
	appCtx context.Context
	store  *adapters.Store
}

func NewHttpServer(appCtx context.Context, addr string) *HttpServer {
	s := &HttpServer{appCtx: appCtx}
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

func (s *HttpServer) handleInitialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nodeID := config.NodeID()
	log.Printf("[HTTP] Received /initialize request for node %s", nodeID)

	checkCmd := exec.CommandContext(r.Context(), "docker", "compose", "ps", "-q", "etcd")
	checkCmd.Dir = "/root/bootstrap"
	out, err := checkCmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		log.Printf("[HTTP] etcd container already present for node %s. Skipping boot.", nodeID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf("Node %s is already initialized (etcd container detected).\n", nodeID)))
		return
	}

	log.Printf("[HTTP] Starting etcd container in 'new' mode...")
	cmd := exec.CommandContext(r.Context(), "docker", "compose", "up", "-d", "etcd")
	cmd.Dir = "/root/bootstrap"

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[HTTP] Docker Compose error: %v, Output: %s", err, string(out))
		http.Error(w, fmt.Sprintf("failed to start etcd container: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("[HTTP] Connecting gRPC client to localhost:2379...")

	var cli *clientv3.Client
	for i := 0; i < 5; i++ {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 2 * time.Second,
		})
		if err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			_, err = cli.Status(ctx, "localhost:2379")
			cancel()
			if err == nil {
				break
			}
			cli.Close()
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		log.Printf("[HTTP] Failed to connect to etcd gRPC: %v", err)
		http.Error(w, "etcd started but gRPC connection failed", http.StatusInternalServerError)
		return
	}

	s.store = adapters.NewStore(cli)
	log.Println("[HTTP] etcd store successfully bound to controller!")

	log.Println("[HTTP] Transitioning node into active Member role...")
	if factory, found := roles.Registry["member"]; found {
		memberRunner := factory(s.store)
		memberAsg := &models.Assignment{
			NodeID: nodeID,
			ID:     "member-" + nodeID,
			Role:   "member",
		}
		go func() {
			if err := memberRunner.Run(s.appCtx, memberAsg, nil); err != nil {
				log.Printf("[HTTP] Member role execution exited with error: %v", err)
			}
		}()
	} else {
		log.Println("[HTTP] CRITICAL ERROR: 'member' role not registered in roles.Registry")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf("Cluster initialized on node %s. etcd is online.\n", nodeID)))
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

	bootstrapDir := "/root/bootstrap"
	envFile := fmt.Sprintf("%s/.env", bootstrapDir)

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

	downCmd := exec.CommandContext(r.Context(), "docker", "compose", "down", "etcd", "--volumes", "--remove-orphans")
	downCmd.Dir = bootstrapDir
	_ = downCmd.Run()

	upCmd := exec.CommandContext(r.Context(), "docker", "compose", "up", "-d", "etcd")
	upCmd.Dir = bootstrapDir
	if err := upCmd.Run(); err != nil {
		http.Error(w, "failed to start etcd container", http.StatusInternalServerError)
		return
	}

	var ready bool
	for i := 0; i < 15; i++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:2379", 1*time.Second)
		if err == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
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

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Printf("[HTTP] Failed to connect to gRPC post-promotion: %v", err)
		http.Error(w, "gRPC connection failed", http.StatusInternalServerError)
		return
	}

	s.store = adapters.NewStore(cli)

	if factory, found := roles.Registry["member"]; found {
		memberRunner := factory(s.store)
		memberAsg := &models.Assignment{
			NodeID: nodeID,
			ID:     "member-" + nodeID,
			Role:   "member",
		}
		go func() {
			if err := memberRunner.Run(s.appCtx, memberAsg, nil); err != nil {
				log.Printf("[HTTP] Member role execution exited: %v", err)
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf("Node %s activated as active Voter member.\n", nodeID)))
}
