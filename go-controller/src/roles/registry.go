package roles

import (
	"cloud-controller/src/adapters"
	"cloud-controller/src/models"
	"context"
)

type RoleFunc func(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error

type RegistryEntry struct {
	Name  string
	Logic RoleFunc
}

var Registry = map[string]RegistryEntry{}

func RegisterRole(name string, logic RoleFunc) {
	Registry[name] = RegistryEntry{
		Name:  name,
		Logic: logic,
	}
}
