package infra

import (
	"cloud-controller/src/core/config"
	"cloud-controller/src/core/models"
	"cloud-controller/src/core/roles"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

type HttpServer struct {
	dcr DockerCreature
	osa OsCreature
	cms ListenerCreature
	spk SpeakerCreature
	reg *roles.Registry
}

func NewHttpServer(ctx context.Context, addr string, reg *roles.Registry, dcr DockerCreature, osa OsCreature, cms ListenerCreature, spk SpeakerCreature) *HttpServer {
	s := &HttpServer{reg: reg, dcr: dcr, osa: osa, cms: cms, spk: spk}
	s.cms.RegisterHandler("/initialize", s.handleInit)
	s.cms.RegisterHandler("/assimilate", s.handleAssimilate)
	s.cms.RegisterHandler("/activate", s.handleActivate)
	return s
}

func (s *HttpServer) Start(addr string) {
	s.cms.Start(addr)
}

func (s *HttpServer) Shutdown(ctx context.Context) error {
	return s.cms.Shutdown(ctx)
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
	if running, err := s.dcr.IsEtcdRunning(r.Context()); err == nil && running {
		fmt.Fprintf(w, "Node %s already initialized.\n", id)
		return
	}

	_ = s.dcr.ResetEtcd(r.Context())
	if err := s.dcr.StartEtcd(r.Context()); err != nil {
		httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := s.spk.WaitEndpointReady(r.Context(), config.EtcdEndpoint); err != nil {
		httpErr(w, "etcd ready check failed", http.StatusInternalServerError)
		return
	}

	if err := s.activateMember(); err != nil {
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

	if err := s.osa.WriteEnvConfig(r.Context(), p); err != nil {
		httpErr(w, "config write failed", http.StatusInternalServerError)
		return
	}

	_ = s.dcr.ResetEtcd(r.Context())
	if err := s.dcr.StartEtcd(r.Context()); err != nil {
		httpErr(w, fmt.Sprintf("etcd start failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := s.spk.WaitEndpointReady(r.Context(), config.EtcdEndpoint); err != nil {
		if r.Context().Err() != nil {
			httpErr(w, "timeout", http.StatusRequestTimeout)
			return
		}
		httpErr(w, "etcd socket timeout", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Learner ready.\n"))
}

func (s *HttpServer) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.activateMember(); err != nil {
		httpErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Node %s activated.\n", config.NodeID())
}

func (s *HttpServer) activateMember() error {
	if err := s.reg.InitializeStore(); err != nil {
		return fmt.Errorf("etcd connect failed: %w", err)
	}

	id := config.NodeID()
	assignment := &models.Assignment{
		NodeID: id,
		ID:     "member-" + id,
		Role:   "member",
	}
	return s.reg.Start(assignment, nil)
}
