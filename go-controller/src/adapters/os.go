package adapters

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"os"
	"path/filepath"
)

type OsAdapter struct{}

func NewOsAdapter() *OsAdapter {
	return &OsAdapter{}
}

func (b *OsAdapter) WriteEnvConfig(ctx context.Context, id string, bootstrapDir string, p models.AssimilatePayload) error {
	env := fmt.Sprintf("HOSTNAME=%s\nTAILSCALE_IP=%s\nETCD_NAME=%s\nETCD_INITIAL_CLUSTER=%s\nETCD_INITIAL_CLUSTER_STATE=existing\n",
		id, p.AssignedIP, id, p.EtcdInitialCluster)

	return os.WriteFile(filepath.Join(bootstrapDir, ".env"), []byte(env), 0644)
}
