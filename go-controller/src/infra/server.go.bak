package infra

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"cloud-controller/src/config"
	"cloud-controller/src/models"
)

type AssignmentCreator interface {
	CreateAssignment(ctx context.Context, assignment models.Assignment) error
}

type HttpServer struct {
	server *http.Server
}

func NewHttpServer(creator AssignmentCreator, addr string) *HttpServer {
	mux := http.NewServeMux()

	mux.HandleFunc("/make-leader", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		asg := models.Assignment{
			NodeID: config.NodeID(),
			ID:     "initial",
			Role:   "leader",
			Config: json.RawMessage("{}"),
		}

		if err := creator.CreateAssignment(r.Context(), asg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("[HTTP] Leader assignment request submitted for node %s", config.NodeID())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("leadership campaign triggered\n"))
	})

	return &HttpServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
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
