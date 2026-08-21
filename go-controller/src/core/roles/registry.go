package roles

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"cloud-controller/src/core/config"
	"cloud-controller/src/core/models"
	adapters "cloud-controller/src/infra"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type RoleRunner interface {
	Run(ctx context.Context, asg *models.Assignment, sess *concurrency.Session) error
}

type Registry struct {
	ctx context.Context
	//mu  sync.RWMutex
	str any
}

func NewRegistry(ctx context.Context) *Registry {
	return &Registry{
		ctx: ctx,
	}
}

func (r *Registry) InitializeStore() error {
	store, err := adapters.NewStore(r.ctx, config.EtcdEndpoint)
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}

	r.str = store
	return nil
}

func (r *Registry) Start(a *models.Assignment, s *concurrency.Session) error {
	if a == nil {
		return fmt.Errorf("assignment is nil")
	}

	//r.mu.RLock()
	//str := r.str
	//r.mu.RUnlock()

	if r.str == nil {
		return fmt.Errorf("store not initialized")
	}

	rn, err := r.runner(a.Role, r.str)
	if err != nil {
		return err
	}

	go func() {
		if err := rn.Run(r.ctx, a, s); err != nil && r.ctx.Err() == nil {
			log.Printf("[%s] Exited with error: %v", a.Role, err)
		}
	}()

	return nil
}

func (r *Registry) runner(role string, str any) (RoleRunner, error) {
	switch role {
	case "member":
		s, ok := str.(MemberStore)
		if !ok {
			return nil, fmt.Errorf("invalid MemberStore")
		}
		return &MemberRole{store: s, registry: r}, nil

	case "leader":
		s, ok := str.(LeaderStore)
		if !ok {
			return nil, fmt.Errorf("invalid LeaderStore")
		}
		return &LeaderRole{store: s}, nil

	case "tailscale-manager":
		s, ok := str.(TsManagerStore)
		if !ok {
			return nil, fmt.Errorf("invalid TsManagerStore")
		}
		return &TSMgr{
			str: s,
			cli: &http.Client{Timeout: config.Timeout},
		}, nil

	default:
		return nil, fmt.Errorf("unknown role: %s", role)
	}
}
