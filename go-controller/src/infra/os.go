package adapters

import (
	"cloud-controller/src/core/config"
	"cloud-controller/src/core/models"
	"context"
	"fmt"
	"os"
)

type OsAdapter struct{}

func NewOsAdapter() *OsAdapter {
	return &OsAdapter{}
}

func (b *OsAdapter) WriteEnvConfig(ctx context.Context, p models.AssimilatePayload) error {
	id := config.NodeID()
	env := fmt.Sprintf("HOSTNAME=%s\nTAILSCALE_IP=%s\nETCD_NAME=%s\nETCD_INITIAL_CLUSTER=%s\nETCD_INITIAL_CLUSTER_STATE=existing\n",
		id, p.AssignedIP, id, p.EtcdInitialCluster)

	return os.WriteFile(config.BootstrapDir+"/.env", []byte(env), 0644)
}
