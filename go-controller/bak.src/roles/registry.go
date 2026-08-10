package roles

import (
	"context"
	"encoding/json"
)

type RoleFunc func(ctx context.Context, config json.RawMessage) error

type RegistryEntry struct {
	Name  string
	Logic RoleFunc
}

// Registry maps role names to their executable implementations.
var Registry = map[string]RegistryEntry{}

func RegisterRole(name string, logic RoleFunc) {
	Registry[name] = RegistryEntry{
		Name:  name,
		Logic: logic,
	}
}
