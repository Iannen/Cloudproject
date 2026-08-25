package adapters

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"io/fs"
	"os"
	"path/filepath"
)

type OsConfig struct {
	BootstrapDir string
	EnvFileName  string
	FilePerms    fs.FileMode
	EnvTemplate  string
}

type OsAdapter struct {
	bootstrapDir string
	envFileName  string
	filePerms    fs.FileMode
	envTemplate  string
}

func NewOsAdapter(cfg OsConfig) *OsAdapter {
	return &OsAdapter{
		bootstrapDir: cfg.BootstrapDir,
		envFileName:  cfg.EnvFileName,
		filePerms:    cfg.FilePerms,
		envTemplate:  cfg.EnvTemplate,
	}
}

func (b *OsAdapter) GetNodeID() string {
	return os.Getenv("NODE_ID")
}

func (b *OsAdapter) WriteEnvConfig(ctx context.Context, id string, p models.AssimilatePayload) error {
	env := fmt.Sprintf(b.envTemplate, id, p.AssignedIP, id, p.EtcdInitialCluster)

	return os.WriteFile(filepath.Join(b.bootstrapDir, b.envFileName), []byte(env), b.filePerms)
}
