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
	bootstrapDir string
	envFileName  string
	filePerms    fs.FileMode
	envTemplate  string
}

func NewOsAdapter(bootstrapDir string, envFileName string, filePerms fs.FileMode, envTemplate string) *OsAdapter {
	return &OsAdapter{
		bootstrapDir: bootstrapDir,
		envFileName:  envFileName,
		filePerms:    filePerms,
		envTemplate:  envTemplate,
	}
}

func (b *OsAdapter) WriteEnvConfig(ctx context.Context, id string, p models.AssimilatePayload) error {
	env := fmt.Sprintf(b.envTemplate, id, p.AssignedIP, id, p.EtcdInitialCluster)

	return os.WriteFile(filepath.Join(b.bootstrapDir, b.envFileName), []byte(env), b.filePerms)
}
