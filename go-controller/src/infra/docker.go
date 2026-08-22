package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type DockerAdapter struct{}

func NewDockerAdapter() *DockerAdapter {
	return &DockerAdapter{}
}

func (d *DockerAdapter) IsEtcdRunning(ctx context.Context, bootstrapDir string) (bool, error) {
	out, err := runCompose(ctx, bootstrapDir, "ps", "-q", "etcd")
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func (d *DockerAdapter) StartEtcd(ctx context.Context, bootstrapDir string) error {
	_, err := runCompose(ctx, bootstrapDir, "up", "-d", "etcd")
	return err
}

func (d *DockerAdapter) ResetEtcd(ctx context.Context, bootstrapDir string) error {
	_, err := runCompose(ctx, bootstrapDir, "down", "etcd", "--volumes", "--remove-orphans")
	return err
}

func runCompose(ctx context.Context, bootstrapDir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = bootstrapDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("docker compose %s: %w (%s)", strings.Join(args, " "), err, out)
	}
	return out, nil
}
