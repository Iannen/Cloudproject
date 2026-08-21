package roles

import (
	"context"
	"go-controller/src/core/models"
)

type RegistryInterface interface {
	Start(a *models.Assignment) error
	Stop(assignmentID string)
	StopAll()
	ActiveAssignments() map[string]bool
	InitializeStore() error
}

type TailscaleProvider interface {
	GetPeers(ctx context.Context) ([]*models.TSPeer, error)
	GetLocalIP(ctx context.Context) (string, error)
}
