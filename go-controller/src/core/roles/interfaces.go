package roles

import (
	"context"
	"go-controller/src/core/models"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type RegistryInterface interface {
	Start(a *models.Assignment, s *concurrency.Session) error
	Stop(assignmentID string)
	StopAll()
	ActiveAssignments() map[string]bool
	InitializeStore() error
}

type TailscaleProvider interface {
	GetPeers(ctx context.Context) ([]*models.TSPeer, error)
	GetLocalIP(ctx context.Context) (string, error)
}
