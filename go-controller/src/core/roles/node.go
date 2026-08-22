package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"net/http"
	"time"
)

type NodeRole struct {
	dcr DockerMgr
	osa FileMgr
	cms HTTPServer
	spk HealthChecker
	reg RoleMgr
}

func (n *NodeRole) Run(ctx context.Context, a *models.Assignment) error {
	log.Printf("[NodeRole] Starting HTTP listener on %s for node %s", config.HTTPPort, a.NodeID)
	n.cms.Start(config.HTTPPort, config.Timeout)

	<-ctx.Done()

	log.Printf("[NodeRole] Shutting down HTTP listener for node %s", a.NodeID)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	if err := n.cms.Shutdown(shutdownCtx); err != nil {
		log.Printf("[NodeRole] Error during HTTP shutdown: %v", err)
		return err
	}
	return nil
}

func (n *NodeRole) httpErr(w http.ResponseWriter, msg string, code int) {
	log.Printf("[HTTP] %d: %s", code, msg)
	http.Error(w, msg, code)
}

func (n *NodeRole) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		n.httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := config.NodeID()
	if n.reg.IsActive("member-" + id) {
		fmt.Fprintf(w, "Node %s already initialized.\n", id)
		return
	}

	_ = n.dcr.ResetEtcd(r.Context(), config.BootstrapDir)
	if err := n.dcr.StartEtcd(r.Context(), config.BootstrapDir); err != nil {
		n.httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := n.spk.WaitEndpointReady(r.Context(), config.EtcdEndpoint, config.StartupRetries, config.StartupInterval); err != nil {
		n.httpErr(w, "etcd ready check failed", http.StatusInternalServerError)
		return
	}

	if err := n.activateMember(); err != nil {
		n.httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Node %s initialized.\n", id)
}

func (n *NodeRole) handleAssimilate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		n.httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p models.AssimilatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		n.httpErr(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}

	if err := n.osa.WriteEnvConfig(r.Context(), config.NodeID(), config.BootstrapDir, p); err != nil {
		n.httpErr(w, "config write failed", http.StatusInternalServerError)
		return
	}

	_ = n.dcr.ResetEtcd(r.Context(), config.BootstrapDir)
	if err := n.dcr.StartEtcd(r.Context(), config.BootstrapDir); err != nil {
		n.httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := n.spk.WaitEndpointReady(r.Context(), config.EtcdEndpoint, config.StartupRetries, config.StartupInterval); err != nil {
		if r.Context().Err() != nil {
			n.httpErr(w, "timeout", http.StatusRequestTimeout)
			return
		}
		n.httpErr(w, "etcd socket timeout", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Learner ready.\n"))
}

func (n *NodeRole) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		n.httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := n.activateMember(); err != nil {
		n.httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Node %s activated.\n", config.NodeID())
}

func (n *NodeRole) activateMember() error {
	if err := n.InitializeStore(); err != nil {
		return fmt.Errorf("etcd connect failed: %w", err)
	}

	id := config.NodeID()
	assignment := &models.Assignment{
		NodeID: id,
		ID:     "member-" + id,
		Role:   "member",
	}
	return n.reg.Start(assignment)
}

func (n *NodeRole) InitializeStore() error {
	return n.reg.InitializeStore()
}

type HTTPServer interface {
	RegisterHandler(pattern string, handler http.HandlerFunc)
	Start(addr string, clientTimeout time.Duration)
	Shutdown(ctx context.Context) error
}
type HealthChecker interface {
	WaitEndpointReady(ctx context.Context, endpoint string, retries int, interval time.Duration) error
}
type FileMgr interface {
	WriteEnvConfig(ctx context.Context, nodeID string, bootstrapDir string, payload models.AssimilatePayload) error
}

type DockerMgr interface {
	StartEtcd(ctx context.Context, bootstrapDir string) error
	ResetEtcd(ctx context.Context, bootstrapDir string) error
}

func NewNodeRole(reg RoleMgr, dcr DockerMgr, osa FileMgr, cms HTTPServer, spk HealthChecker) *NodeRole {
	n := &NodeRole{reg: reg, dcr: dcr, osa: osa, cms: cms, spk: spk}
	n.cms.RegisterHandler("/initialize", n.handleInit)
	n.cms.RegisterHandler("/assimilate", n.handleAssimilate)
	n.cms.RegisterHandler("/activate", n.handleActivate)
	return n
}
