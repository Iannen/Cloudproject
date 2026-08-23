package adapters

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"go-controller/src/core/roles"
)

type HTTPServerAdapter struct {
	mux    *http.ServeMux
	server *http.Server
}

func NewHTTPServerAdapter() *HTTPServerAdapter {
	return &HTTPServerAdapter{
		mux: http.NewServeMux(),
	}
}

func (s *HTTPServerAdapter) RegisterGetRoute(pattern string, handler roles.DomainHandler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDomainRequest(w, r, handler)
	})
}

func (s *HTTPServerAdapter) RegisterPostRoute(pattern string, handler roles.DomainHandler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDomainRequest(w, r, handler)
	})
}

func (s *HTTPServerAdapter) handleDomainRequest(w http.ResponseWriter, r *http.Request, handler roles.DomainHandler) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := handler(r.Context(), body)
	if err != nil {
		log.Printf("[HTTPServerAdapter] Handler error: %v", err)
		if r.Context().Err() != nil {
			http.Error(w, err.Error(), http.StatusRequestTimeout)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(resp))
}

func (s *HTTPServerAdapter) Start(addr string, clientTimeout time.Duration) {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  clientTimeout,
		WriteTimeout: clientTimeout,
	}

	go func() {
		log.Printf("[HTTPServerAdapter] Listening on %s", addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTPServerAdapter] HTTP server error: %v", err)
		}
	}()
}

func (s *HTTPServerAdapter) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	log.Println("[HTTPServerAdapter] Shutting down HTTP server...")
	return s.server.Shutdown(ctx)
}
