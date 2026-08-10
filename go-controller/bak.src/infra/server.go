package infra

import (
	"context"
	"log"
	"net/http"
)

type NodeController interface {
	CampaignForLeadership(ctx context.Context) error
}

type HttpServer struct {
	server *http.Server
}

func NewHttpServer(controller NodeController, addr string) *HttpServer {
	mux := http.NewServeMux()

	mux.HandleFunc("/make-leader", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := controller.CampaignForLeadership(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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
