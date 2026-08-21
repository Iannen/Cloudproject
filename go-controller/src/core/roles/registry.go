package roles

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"cloud-controller/src/core/config"
	"cloud-controller/src/core/models"
	adapters "cloud-controller/src/infra"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type RoleRunner interface {
	Run(ctx context.Context, asg *models.Assignment, sess *concurrency.Session) error
}

/*
	type Registry struct {
		ctx context.Context
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
*/
type Registry struct {
	ctx      context.Context
	mu       sync.Mutex
	str      any
	runtimes map[string]*AssignmentRuntime
}

func NewRegistry(ctx context.Context) *Registry {
	return &Registry{
		ctx:      ctx,
		runtimes: make(map[string]*AssignmentRuntime),
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

	r.mu.Lock()
	if r.str == nil {
		r.mu.Unlock()
		return fmt.Errorf("store not initialized")
	}

	if _, exists := r.runtimes[a.ID]; exists {
		r.mu.Unlock()
		return nil
	}

	rn, err := r.runner(a.Role, r.str)
	if err != nil {
		r.mu.Unlock()
		return err
	}

	rt := NewAssignmentRuntime(a)
	r.runtimes[a.ID] = rt
	r.mu.Unlock()

	rt.Start(r.ctx, rn, s)
	return nil
}

func (r *Registry) Stop(assignmentID string) {
	r.mu.Lock()
	rt, exists := r.runtimes[assignmentID]
	if exists {
		delete(r.runtimes, assignmentID)
	}
	r.mu.Unlock()

	if exists {
		rt.Stop()
	}
}

func (r *Registry) StopAll() {
	r.mu.Lock()
	toStop := make([]*AssignmentRuntime, 0, len(r.runtimes))
	for id, rt := range r.runtimes {
		toStop = append(toStop, rt)
		delete(r.runtimes, id)
	}
	r.mu.Unlock()

	for _, rt := range toStop {
		rt.Stop()
	}
}

func (r *Registry) ActiveAssignments() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	active := make(map[string]bool, len(r.runtimes))
	for id := range r.runtimes {
		active[id] = true
	}
	return active
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

type AssignmentRuntime struct {
	AssignmentID string
	Definition   *models.Assignment
	CancelFunc   context.CancelFunc
	DoneChan     chan struct{}
}

func NewAssignmentRuntime(asg *models.Assignment) *AssignmentRuntime {
	return &AssignmentRuntime{
		AssignmentID: asg.ID,
		Definition:   asg,
		DoneChan:     make(chan struct{}),
	}
}

func (r *AssignmentRuntime) Start(
	parentCtx context.Context,
	runner RoleRunner,
	sess *concurrency.Session,
) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.CancelFunc = cancel

	go func() {
		defer close(r.DoneChan)
		defer cancel()

		err := runner.Run(ctx, r.Definition, sess)
		if err != nil && ctx.Err() == nil {
			log.Printf("[Runtime] execution failed id=%s role=%s: %v", r.AssignmentID, r.Definition.Role, err)
		}
	}()
}

func (r *AssignmentRuntime) Stop() {
	if r.CancelFunc != nil {
		r.CancelFunc()
	}
	<-r.DoneChan
}
