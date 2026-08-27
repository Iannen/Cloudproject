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

type TSStatus struct {
	Peer map[string]models.TSPeer
	Self models.TSPeer
}

type TailscaleConfig struct {
	BinaryPath     string
	EnvKey         string
	NodeNamePrefix string
}

type TailscaleAdapter struct {
	binaryPath     string
	envKey         string
	nodeNamePrefix string
}

func NewTailscaleAdapter(cfg TailscaleConfig) *TailscaleAdapter {
	return &TailscaleAdapter{
		binaryPath:     cfg.BinaryPath,
		envKey:         cfg.EnvKey,
		nodeNamePrefix: cfg.NodeNamePrefix,
	}
}

func (t *TailscaleAdapter) GetPeers(ctx context.Context) ([]models.TSPeer, error) {
	out, err := exec.CommandContext(ctx, t.binaryPath, "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}

	var st TSStatus
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	allPeers := make([]models.TSPeer, 0, len(st.Peer)+1)
	if st.Self.HostName != "" || len(st.Self.TailscaleIPs) > 0 {
		allPeers = append(allPeers, st.Self)
	}
	for _, p := range st.Peer {
		allPeers = append(allPeers, p)
	}

	res := make([]models.TSPeer, 0, len(allPeers))
	for _, p := range allPeers {
		if !p.Online || len(p.TailscaleIPs) == 0 || (t.nodeNamePrefix != "" && !strings.HasPrefix(p.HostName, t.nodeNamePrefix)) {
			continue
		}
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
