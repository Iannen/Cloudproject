package roles

import (
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
