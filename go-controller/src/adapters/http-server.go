package adapters

import (
	"context"
	"go-controller/src/core/models"
	"io"
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

func (s *HTTPServerAdapter) RegisterGetRoute(pattern string, handler models.DomainHandler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDomainRequest(w, r, handler)
	})
}

func (s *HTTPServerAdapter) RegisterPostRoute(pattern string, handler models.DomainHandler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDomainRequest(w, r, handler)
	})
}

func (s *HTTPServerAdapter) handleDomainRequest(w http.ResponseWriter, r *http.Request, handler models.DomainHandler) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := handler(r.Context(), body)
	if err != nil {
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

func (s *HTTPServerAdapter) Start(addr string, clientTimeout time.Duration) <-chan error {
	errCh := make(chan error, 1)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  clientTimeout,
		WriteTimeout: clientTimeout,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return errCh
}

func (s *HTTPServerAdapter) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
