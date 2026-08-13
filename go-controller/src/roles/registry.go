package roles

import (
	"context"

	"cloud-controller/src/adapters"
	"cloud-controller/src/models"
)

type RoleRunner interface {
	Run(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error
}

type RoleFactory func(store any) RoleRunner

var Registry = map[string]RoleFactory{}

func RegisterRole(name string, factory RoleFactory) {
	Registry[name] = factory
}
