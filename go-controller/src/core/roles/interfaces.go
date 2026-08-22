package roles

import (
	"go-controller/src/core/models"
)

type RoleMgr interface {
	Start(a *models.Assignment) error
	Stop(assignmentID string)
	StopAll()
	ActiveAssignments() map[string]bool
	InitializeStore() error
}
