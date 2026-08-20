package roles

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"cloud-controller/src/core/config"
	"cloud-controller/src/core/models"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type RoleRunner interface {
	Run(ctx context.Context, asg *models.Assignment, sess *concurrency.Session) error
}

type Registry struct {
	mu  sync.RWMutex
	str any
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) SetStore(str any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.str = str
}

func (r *Registry) Start(ctx context.Context, a *models.Assignment, s *concurrency.Session) error {
	if a == nil {
		return fmt.Errorf("assignment is nil")
	}

	r.mu.RLock()
	str := r.str
	r.mu.RUnlock()

	if str == nil {
		return fmt.Errorf("store not initialized")
	}

	rn, err := r.runner(a.Role, str)
	if err != nil {
		return err
	}

	go func() {
		if err := rn.Run(ctx, a, s); err != nil && ctx.Err() == nil {
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
