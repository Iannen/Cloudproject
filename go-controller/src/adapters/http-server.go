package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type PayloadHandler[T any] func(ctx context.Context, payload T) (string, error)
type ActionHandler func(ctx context.Context) (string, error)

type HTTPServerConfig struct {
	Addr          string
	ClientTimeout time.Duration
}

type HTTPServerAdapter struct {
	mux           *http.ServeMux
	server        *http.Server
	addr          string
	clientTimeout time.Duration
}

func NewHTTPServerAdapter(cfg HTTPServerConfig) *HTTPServerAdapter {
	return &HTTPServerAdapter{
		mux:           http.NewServeMux(),
		addr:          cfg.Addr,
		clientTimeout: cfg.ClientTimeout,
	}
}

func RegisterPost[T any](s *HTTPServerAdapter, pattern string, handler PayloadHandler[T]) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload T
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		resp, err := handler(r.Context(), payload)
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
	})
}

func RegisterGet(s *HTTPServerAdapter, pattern string, handler ActionHandler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp, err := handler(r.Context())
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
	})
}

func (s *HTTPServerAdapter) Start() <-chan error {
	errCh := make(chan error, 1)
	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      s.mux,
		ReadTimeout:  s.clientTimeout,
		WriteTimeout: s.clientTimeout,
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
