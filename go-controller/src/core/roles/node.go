package roles

import (
	"cloud-controller/src/core/config"
	"cloud-controller/src/core/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type ListenerCreature interface {
	RegisterHandler(pattern string, handler http.HandlerFunc)
	Start(addr string)
	Shutdown(ctx context.Context) error
}
type SpeakerCreature interface {
	WaitEndpointReady(ctx context.Context, endpoint string) error
}
type OsCreature interface {
	WriteEnvConfig(ctx context.Context, payload models.AssimilatePayload) error
}
type DockerCreature interface {
	IsEtcdRunning(ctx context.Context) (bool, error)
	StartEtcd(ctx context.Context) error
	ResetEtcd(ctx context.Context) error
}

type NodeRole struct {
	dcr DockerCreature
	osa OsCreature
	cms ListenerCreature
	spk SpeakerCreature
	reg *Registry
}

func NewNodeRole(reg *Registry, dcr DockerCreature, osa OsCreature, cms ListenerCreature, spk SpeakerCreature) *NodeRole {
	n := &NodeRole{reg: reg, dcr: dcr, osa: osa, cms: cms, spk: spk}
	n.cms.RegisterHandler("/initialize", n.handleInit)
	n.cms.RegisterHandler("/assimilate", n.handleAssimilate)
	n.cms.RegisterHandler("/activate", n.handleActivate)
	return n
}

func (n *NodeRole) Run(ctx context.Context, a *models.Assignment, s *concurrency.Session) error {
	log.Printf("[NodeRole] Starting HTTP listener on %s for node %s", config.HTTPPort, a.NodeID)
	n.cms.Start(config.HTTPPort)

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

func httpErr(w http.ResponseWriter, msg string, code int) {
	log.Printf("[HTTP] %d: %s", code, msg)
	http.Error(w, msg, code)
}

func (n *NodeRole) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := config.NodeID()
	if running, err := n.dcr.IsEtcdRunning(r.Context()); err == nil && running {
		fmt.Fprintf(w, "Node %s already initialized.\n", id)
		return
	}

	_ = n.dcr.ResetEtcd(r.Context())
	if err := n.dcr.StartEtcd(r.Context()); err != nil {
		httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := n.spk.WaitEndpointReady(r.Context(), config.EtcdEndpoint); err != nil {
		httpErr(w, "etcd ready check failed", http.StatusInternalServerError)
		return
	}

	if err := n.activateMember(); err != nil {
		httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Node %s initialized.\n", id)
}

func (n *NodeRole) handleAssimilate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p models.AssimilatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpErr(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}

	if err := n.osa.WriteEnvConfig(r.Context(), p); err != nil {
		httpErr(w, "config write failed", http.StatusInternalServerError)
		return
	}

	_ = n.dcr.ResetEtcd(r.Context())
	if err := n.dcr.StartEtcd(r.Context()); err != nil {
		httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := n.spk.WaitEndpointReady(r.Context(), config.EtcdEndpoint); err != nil {
		if r.Context().Err() != nil {
			httpErr(w, "timeout", http.StatusRequestTimeout)
			return
		}
		httpErr(w, "etcd socket timeout", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Learner ready.\n"))
}

func (n *NodeRole) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := n.activateMember(); err != nil {
		httpErr(w, err.Error(), http.StatusInternalServerError)
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
	return n.reg.Start(assignment, nil)
}

func (n *NodeRole) InitializeStore() error {
	return n.reg.InitializeStore()
}
