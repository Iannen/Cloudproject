package registry

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"go-controller/src/core/roles"
	"sync"
)

type RoleRunner interface {
	Run(ctx context.Context)
}

type Registry struct {
	appCtx    context.Context
	mu        sync.Mutex
	dcr       roles.DockerMgr
	etcd      roles.StoreAdapter
	rpcClient roles.RpcClient
	osa       roles.FileMgr
	ts        roles.TSClient
	runtimes  map[string]*AssignmentRuntime
}

func NewRegistry(
	appCtx context.Context,
	dcr roles.DockerMgr,
	etcd roles.StoreAdapter,
	rpcClient roles.RpcClient,
	osa roles.FileMgr,
	ts roles.TSClient,
) *Registry {
	return &Registry{
		appCtx:    appCtx,
		dcr:       dcr,
		etcd:      etcd,
		rpcClient: rpcClient,
		osa:       osa,
		ts:        ts,
		runtimes:  make(map[string]*AssignmentRuntime),
	}
}

func (r *Registry) Start(a models.Assignment) error {
	r.mu.Lock()
	if r.etcd == nil {
		r.mu.Unlock()
		return fmt.Errorf("store not initialized")
	}

	if _, exists := r.runtimes[a.ID]; exists {
		r.mu.Unlock()
		return nil
	}

	rn, err := r.runner(a)
	if err != nil {
		r.mu.Unlock()
		return err
	}

	rt := NewAssignmentRuntime(a)
	r.runtimes[a.ID] = rt
	r.mu.Unlock()

	rt.Start(r.appCtx, rn)
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

func (r *Registry) StopManagedAssignments() {
	active := r.ActiveAssignments()
	for id := range active {
		r.Stop(id)
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

func (r *Registry) IsActive(assignmentID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.runtimes[assignmentID]
	return exists
}

func (r *Registry) runner(a models.Assignment) (RoleRunner, error) {
	switch a.Role {

	case "leader":
		return roles.NewLeaderRole(a, r.etcd), nil

	case "tailscale-manager":
		return roles.NewRecruiter(
			a,
			r.etcd,
			r.ts,
			r.rpcClient,
		), nil

	default:
		return nil, fmt.Errorf("unknown role: %s", a.Role)
	}
}

type AssignmentRuntime struct {
	AssignmentID string
	Definition   models.Assignment
	CancelFunc   context.CancelFunc
	wg           sync.WaitGroup
}

func NewAssignmentRuntime(asg models.Assignment) *AssignmentRuntime {
	return &AssignmentRuntime{
		AssignmentID: asg.ID,
		Definition:   asg,
	}
}

func (r *AssignmentRuntime) Start(
	parentCtx context.Context,
	runner RoleRunner,
) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.CancelFunc = cancel

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer cancel()

		runner.Run(ctx)
	}()
}

func (r *AssignmentRuntime) Stop() {
	if r.CancelFunc != nil {
		r.CancelFunc()
	}
	r.wg.Wait()
}
