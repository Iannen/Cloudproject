package adapters

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

type DockerConfig struct {
	BinaryPath      string
	BootstrapDir    string
	ComposeCmd      []string
	UpArgs          []string
	DownArgs        []string
	StartupRetries  int
	StartupInterval time.Duration
	EtcdEndpoint    string
}

type DockerAdapter struct {
	cfg DockerConfig
}

func NewDockerAdapter(cfg DockerConfig) *DockerAdapter {
	return &DockerAdapter{
		cfg: cfg,
	}
}

func (d *DockerAdapter) StartEtcd(ctx context.Context) error {
	_, err := d.runCompose(ctx, d.cfg.UpArgs...)
	return err
}

func (d *DockerAdapter) ResetEtcd(ctx context.Context) error {
	_, err := d.runCompose(ctx, d.cfg.DownArgs...)
	return err
}

func (d *DockerAdapter) runCompose(ctx context.Context, args ...string) ([]byte, error) {
	cmdArgs := append(append([]string{}, d.cfg.ComposeCmd...), args...)
	cmd := exec.CommandContext(ctx, d.cfg.BinaryPath, cmdArgs...)
	cmd.Dir = d.cfg.BootstrapDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("docker compose %s: %w (%s)", strings.Join(args, " "), err, out)
	}
	return out, nil
}

func (d *DockerAdapter) GetLogs(ctx context.Context, containerID string) (string, error) {
	cmd := exec.CommandContext(ctx, d.cfg.BinaryPath, "logs", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs %s failed: %w (%s)", containerID, err, out)
	}
	return string(out), nil
}

func (c *DockerAdapter) WaitEtcdReady(ctx context.Context) error {
	for i := 0; i < c.cfg.StartupRetries; i++ {
		conn, err := net.DialTimeout("tcp", c.cfg.EtcdEndpoint, c.cfg.StartupInterval)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.cfg.StartupInterval):
		}
	}
	return fmt.Errorf("etcd endpoint %s not ready after %d retries", c.cfg.EtcdEndpoint, c.cfg.StartupRetries)
}
