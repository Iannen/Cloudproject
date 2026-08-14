package roles

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"cloud-controller/src/adapters"
	"cloud-controller/src/config"
	"cloud-controller/src/models"
)

type RoleRunner interface {
	Run(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error
}

type Registry struct {
	mu    sync.RWMutex
	store any
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) SetStore(store any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = store
}

func (r *Registry) Start(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error {
	if asg == nil {
		return fmt.Errorf("registry error: assignment cannot be nil")
	}

	r.mu.RLock()
	store := r.store
	r.mu.RUnlock()

	if store == nil {
		return fmt.Errorf("registry error: store not initialized")
	}

	runner, err := r.createRunner(asg.Role, store)
	if err != nil {
		return fmt.Errorf("registry error: %w", err)
	}

	go func() {
		if err := runner.Run(ctx, asg, sess); err != nil && ctx.Err() == nil {
			log.Printf("[%s] Role execution exited with error: %v", asg.Role, err)
		}
	}()

	return nil
}

func (r *Registry) createRunner(role string, store any) (RoleRunner, error) {
	switch role {
	case "member":
		memberStore, ok := store.(MemberStore)
		if !ok {
			return nil, fmt.Errorf("store does not implement MemberStore")
		}
		return &MemberRole{
			store:    memberStore,
			registry: r,
		}, nil

	case "leader":
		leaderStore, ok := store.(LeaderStore)
		if !ok {
			return nil, fmt.Errorf("store does not implement LeaderStore")
		}
		return &LeaderRole{
			store: leaderStore,
		}, nil

	case "tailscale-manager":
		tsStore, ok := store.(TsManagerStore)
		if !ok {
			return nil, fmt.Errorf("store does not implement TsManagerStore")
		}
		return &TailscaleManagerRole{
			store:      tsStore,
			httpClient: &http.Client{Timeout: config.HTTPTimeout},
		}, nil

	default:
		return nil, fmt.Errorf("unknown role '%s'", role)
	}
}
