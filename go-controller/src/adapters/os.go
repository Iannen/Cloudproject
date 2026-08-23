package adapters

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"io/fs"
	"os"
	"path/filepath"
)

type OsAdapter struct {
	envFileName string
	filePerms   fs.FileMode
	envTemplate string
}

func NewOsAdapter(envFileName string, filePerms fs.FileMode, envTemplate string) *OsAdapter {
	return &OsAdapter{
		envFileName: envFileName,
		filePerms:   filePerms,
		envTemplate: envTemplate,
	}
}

func (b *OsAdapter) WriteEnvConfig(ctx context.Context, id string, bootstrapDir string, p models.AssimilatePayload) error {
	env := fmt.Sprintf(b.envTemplate, id, p.AssignedIP, id, p.EtcdInitialCluster)

	return os.WriteFile(filepath.Join(bootstrapDir, b.envFileName), []byte(env), b.filePerms)
}
