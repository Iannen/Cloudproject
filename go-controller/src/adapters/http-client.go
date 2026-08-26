package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/models"
	"net/http"
	"time"
)

type HTTPClientConfig struct {
	Timeout              time.Duration
	AssimilateURLPattern string
	ActivateURLPattern   string
	EtcdEndpoint         string
	StartupRetries       int
	StartupInterval      time.Duration
}

type HTTPClientAdapter struct {
	client          *http.Client
	assimilateURL   string
	activateURL     string
	etcdEndpoint    string
	startupRetries  int
	startupInterval time.Duration
}

func NewHTTPClientAdapter(cfg HTTPClientConfig) *HTTPClientAdapter {
	return &HTTPClientAdapter{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		assimilateURL:   cfg.AssimilateURLPattern,
		activateURL:     cfg.ActivateURLPattern,
		etcdEndpoint:    cfg.EtcdEndpoint,
		startupRetries:  cfg.StartupRetries,
		startupInterval: cfg.StartupInterval,
	}
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

func (h *HTTPClientAdapter) Activate(ctx context.Context, targetIP string) error {
	url := fmt.Sprintf(h.activateURL, targetIP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create activate request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send activate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("activate returned status: %s", resp.Status)
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
