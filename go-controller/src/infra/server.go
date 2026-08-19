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
	srv *http.Server
	ctx context.Context
	str *adapters.Store
	reg *roles.Registry
}

func NewHttpServer(ctx context.Context, addr string, reg *roles.Registry) *HttpServer {
	mux := http.NewServeMux()
	s := &HttpServer{ctx: ctx, reg: reg}

	mux.HandleFunc("/initialize", s.handleInit)
	mux.HandleFunc("/assimilate", s.handleAssimilate)
	mux.HandleFunc("/activate", s.handleActivate)

	s.srv = &http.Server{Addr: addr, Handler: mux}
	return s
}

func (s *HttpServer) Start() {
	log.Printf("[HTTP] Running on %s", s.srv.Addr)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Error: %v", err)
		}
	}()
}

func (s *HttpServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func httpErr(w http.ResponseWriter, msg string, code int) {
	log.Printf("[HTTP] %d: %s", code, msg)
	http.Error(w, msg, code)
}

func (s *HttpServer) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := config.NodeID()
	if out, err := runCompose(r.Context(), "ps", "-q", "etcd"); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		fmt.Fprintf(w, "Node %s already initialized.\n", id)
		return
	}

	if _, err := runCompose(r.Context(), "up", "-d", "etcd"); err != nil {
		httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	cli, err := s.connectEtcd(r.Context())
	if err != nil {
		httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.startMember(cli); err != nil {
		httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Node %s initialized.\n", id)
}

func (s *HttpServer) handleAssimilate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p models.AssimilatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpErr(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}

	id := config.NodeID()
	env := fmt.Sprintf("HOSTNAME=%s\nTAILSCALE_IP=%s\nETCD_NAME=%s\nETCD_INITIAL_CLUSTER=%s\nETCD_INITIAL_CLUSTER_STATE=existing\n",
		id, p.AssignedIP, id, p.EtcdInitialCluster)

	if err := os.WriteFile(config.BootstrapDir+"/.env", []byte(env), 0644); err != nil {
		httpErr(w, "config write failed", http.StatusInternalServerError)
		return
	}

	_, _ = runCompose(r.Context(), "down", "etcd", "--volumes", "--remove-orphans")
	if _, err := runCompose(r.Context(), "up", "-d", "etcd"); err != nil {
		httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	for i := 0; i < config.StartupRetries; i++ {
		if c, err := net.DialTimeout("tcp", config.EtcdEndpoint, config.StartupInterval); err == nil {
			_ = c.Close()
			w.Write([]byte("Learner ready.\n"))
			return
		}
		select {
		case <-r.Context().Done():
			httpErr(w, "timeout", http.StatusRequestTimeout)
			return
		case <-time.After(config.StartupInterval):
		}
	}
	httpErr(w, "etcd socket timeout", http.StatusInternalServerError)
}

func (s *HttpServer) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cli, err := s.connectEtcd(r.Context())
	if err != nil {
		httpErr(w, "etcd connect failed", http.StatusInternalServerError)
		return
	}

	if err := s.startMember(cli); err != nil {
		httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Node %s activated.\n", config.NodeID())
}

func runCompose(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = config.BootstrapDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("docker compose %s: %w (%s)", strings.Join(args, " "), err, out)
	}
	return out, nil
}

func (s *HttpServer) connectEtcd(ctx context.Context) (*clientv3.Client, error) {
	for i := 0; i < config.StartupRetries; i++ {
		cli, err := clientv3.New(clientv3.Config{Endpoints: []string{config.EtcdEndpoint}, DialTimeout: config.Timeout})
		if err == nil {
			sCtx, cancel := context.WithTimeout(ctx, config.Timeout)
			_, err = cli.Status(sCtx, config.EtcdEndpoint)
			cancel()
			if err == nil {
				return cli, nil
			}
			cli.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(config.StartupInterval):
		}
	}
	return nil, fmt.Errorf("etcd connection failed after retries")
}

func (s *HttpServer) startMember(cli *clientv3.Client) error {
	s.str = adapters.NewStore(cli)
	s.reg.SetStore(s.str)
	id := config.NodeID()
	return s.reg.Start(s.ctx, &models.Assignment{NodeID: id, ID: "member-" + id, Role: "member"}, nil)
}
