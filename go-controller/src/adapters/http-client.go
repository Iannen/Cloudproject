package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/models"
	"net"
	"net/http"
	"time"
)

type HTTPClientAdapter struct {
	client        *http.Client
	assimilateURL string
	activateURL   string
}

func NewHTTPClientAdapter(timeout time.Duration, assimilateURLPattern, activateURLPattern string) *HTTPClientAdapter {
	return &HTTPClientAdapter{
		client: &http.Client{
			Timeout: timeout,
		},
		assimilateURL: assimilateURLPattern,
		activateURL:   activateURLPattern,
	}
}

func (c *HTTPClientAdapter) WaitEndpointReady(ctx context.Context, endpoint string, retries int, interval time.Duration) error {
	for i := 0; i < retries; i++ {
		conn, err := net.DialTimeout("tcp", endpoint, interval)
		if err == nil {
			_ = conn.Close()
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

func (c *HTTPClientAdapter) Assimilate(ctx context.Context, targetIP string, payload models.AssimilatePayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal assimilate payload: %w", err)
	}

	url := fmt.Sprintf(c.assimilateURL, targetIP)
	if !c.Post(ctx, url, b) {
		return fmt.Errorf("failed to post assimilate payload to %s", url)
	}

	return nil
}

func (c *HTTPClientAdapter) Activate(ctx context.Context, targetIP string) error {
	url := fmt.Sprintf(c.activateURL, targetIP)
	if !c.Post(ctx, url, nil) {
		return fmt.Errorf("failed to post activate request to %s", url)
	}

	return nil
}

func (c *HTTPClientAdapter) Post(ctx context.Context, url string, body []byte) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode == http.StatusOK
}
