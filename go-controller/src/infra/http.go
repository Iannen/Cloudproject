package adapters

import (
	"context"
	"fmt"
	"go-controller/src/core/config"
	"log"
	"net"
	"net/http"
	"time"
)

type ListenerAdapter struct {
	mux *http.ServeMux
	srv *http.Server
}

func NewListenerAdapter() *ListenerAdapter {
	mux := http.NewServeMux()
	return &ListenerAdapter{
		mux: mux,
	}
}

func (l *ListenerAdapter) RegisterHandler(pattern string, handler http.HandlerFunc) {
	l.mux.HandleFunc(pattern, handler)
}

func (l *ListenerAdapter) Start(addr string) {
	l.srv = &http.Server{
		Addr:    addr,
		Handler: l.mux,
	}

	log.Printf("[HTTP Listener] Running on %s", addr)
	go func() {
		if err := l.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP Listener] Error: %v", err)
		}
	}()
}

func (l *ListenerAdapter) Shutdown(ctx context.Context) error {
	if l.srv == nil {
		return nil
	}
	return l.srv.Shutdown(ctx)
}

func (l *ListenerAdapter) WaitEndpointReady(ctx context.Context, endpoint string) error {
	for i := 0; i < config.StartupRetries; i++ {
		c, err := net.DialTimeout("tcp", endpoint, config.StartupInterval)
		if err == nil {
			_ = c.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(config.StartupInterval):
		}
	}
	return fmt.Errorf("endpoint %s not ready after retries", endpoint)
}
