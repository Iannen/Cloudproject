package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/models"
	"log"
	"net"
	"net/http"
	"time"
)

type ListenerAdapter struct {
	mux *http.ServeMux
	srv *http.Server
	cli *http.Client
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

func (l *ListenerAdapter) Start(addr string, clientTimeout time.Duration) {
	l.srv = &http.Server{
		Addr:    addr,
		Handler: l.mux,
	}
	l.cli = &http.Client{
		Timeout: clientTimeout,
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

func (l *ListenerAdapter) WaitEndpointReady(ctx context.Context, endpoint string, retries int, interval time.Duration) error {
	for i := 0; i < retries; i++ {
		c, err := net.DialTimeout("tcp", endpoint, interval)
		if err == nil {
			_ = c.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("endpoint %s not ready after %d retries", endpoint, retries)
}

func (t *ListenerAdapter) Post(ctx context.Context, url string, body []byte) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := t.cli.Do(req)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func (l *ListenerAdapter) Assimilate(ctx context.Context, targetIP string, payload models.AssimilatePayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal assimilate payload: %w", err)
	}

	url := fmt.Sprintf("http://%s:8080/assimilate", targetIP)
	if !l.Post(ctx, url, b) {
		return fmt.Errorf("failed to post assimilate payload to %s", url)
	}

	return nil
}

func (l *ListenerAdapter) Activate(ctx context.Context, targetIP string) error {
	url := fmt.Sprintf("http://%s:8080/activate", targetIP)
	if !l.Post(ctx, url, nil) {
		return fmt.Errorf("failed to post activate request to %s", url)
	}

	return nil
}
