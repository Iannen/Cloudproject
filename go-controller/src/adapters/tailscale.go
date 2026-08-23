package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/models"
	"os"
	"os/exec"
	"strings"
)

type TailscaleAdapter struct {
	binaryPath string
	envKey     string
}

func NewTailscaleAdapter(binaryPath, envKey string) *TailscaleAdapter {
	if binaryPath == "" {
		binaryPath = "tailscale"
	}
	if envKey == "" {
		envKey = "TAILSCALE_IP"
	}
	return &TailscaleAdapter{
		binaryPath: binaryPath,
		envKey:     envKey,
	}
}

func (t *TailscaleAdapter) GetPeers(ctx context.Context) ([]*models.TSPeer, error) {
	out, err := exec.CommandContext(ctx, t.binaryPath, "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}

	var st models.TSStatus
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	res := make([]*models.TSPeer, 0, len(st.Peer)+1)
	if st.Self != nil {
		res = append(res, st.Self)
	}
	for _, p := range st.Peer {
		res = append(res, p)
	}
	return res, nil
}

func (t *TailscaleAdapter) GetLocalIP(ctx context.Context) (string, error) {
	if ip := os.Getenv(t.envKey); ip != "" {
		return ip, nil
	}
	out, err := exec.CommandContext(ctx, t.binaryPath, "ip", "-4").Output()
	if err != nil {
		return "", err
	}
	if ip := strings.TrimSpace(string(out)); ip != "" {
		return ip, nil
	}
	return "", fmt.Errorf("empty tailscale ip")
}
