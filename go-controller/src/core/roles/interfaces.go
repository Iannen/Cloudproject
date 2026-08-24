package roles

import (
	"context"

	"go-controller/src/core/models"
)

type RoleMgr interface {
	Start(a *models.Assignment) error
	Stop(assignmentID string)
	StopAll()
	StopManagedAssignments()
	ActiveAssignments() map[string]bool
	IsActive(assignmentID string) bool
	InitializeStore() error
}

type StoreAdapter interface {
	AssignmentStore
	ParticipantStore
	ClusterMgr
	Connect(ctx context.Context) error
}
