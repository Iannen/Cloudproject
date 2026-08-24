package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type DockerAdapter struct {
	binaryPath string
	composeCmd []string
	upArgs     []string
	downArgs   []string
}

func NewDockerAdapter(binaryPath string, composeCmd []string, upArgs []string, downArgs []string) *DockerAdapter {
	return &DockerAdapter{
		binaryPath: binaryPath,
		composeCmd: composeCmd,
		upArgs:     upArgs,
		downArgs:   downArgs,
	}
}

func (d *DockerAdapter) StartEtcd(ctx context.Context, bootstrapDir string) error {
	_, err := d.runCompose(ctx, bootstrapDir, d.upArgs...)
	return err
}

func (d *DockerAdapter) ResetEtcd(ctx context.Context, bootstrapDir string) error {
	_, err := d.runCompose(ctx, bootstrapDir, d.downArgs...)
	return err
}

func (d *DockerAdapter) runCompose(ctx context.Context, bootstrapDir string, args ...string) ([]byte, error) {
	cmdArgs := append(append([]string{}, d.composeCmd...), args...)
	cmd := exec.CommandContext(ctx, d.binaryPath, cmdArgs...)
	cmd.Dir = bootstrapDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("docker compose %s: %w (%s)", strings.Join(args, " "), err, out)
	}
	return out, nil
}
func (d *DockerAdapter) GetLogs(ctx context.Context, containerID string) (string, error) {
	cmd := exec.CommandContext(ctx, d.binaryPath, "logs", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs %s failed: %w (%s)", containerID, err, out)
	}
	return string(out), nil
}
