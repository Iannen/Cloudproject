package adapters

import (
	"context"
	"log"
	"net/http"
	"time"
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

func (s *HTTPServerAdapter) RegisterHandler(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
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
