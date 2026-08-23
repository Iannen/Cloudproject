package registry

import (
	"context"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"go-controller/src/core/roles"
	"sync"
)

type RoleRunner interface {
	Run(ctx context.Context, asg *models.Assignment)
}

type Registry struct {
	ctx           context.Context
	mu            sync.Mutex
	dcr           roles.DockerMgr
	etcd          roles.StoreAdapter
	httpSrv       roles.HTTPServer
	healthChecker roles.HealthChecker
	rpcClient     roles.RpcClient
	osa           roles.FileMgr
	ts            roles.TSClient
	runtimes      map[string]*AssignmentRuntime
}

func NewRegistry(
	ctx context.Context,
	docker roles.DockerMgr,
	etcd roles.StoreAdapter,
	httpSrv roles.HTTPServer,
	healthChecker roles.HealthChecker,
	rpcClient roles.RpcClient,
	osa roles.FileMgr,
	ts roles.TSClient,
) *Registry {
	return &Registry{
		ctx:           ctx,
		dcr:           docker,
		etcd:          etcd,
		httpSrv:       httpSrv,
		healthChecker: healthChecker,
		rpcClient:     rpcClient,
		osa:           osa,
		ts:            ts,
		runtimes:      make(map[string]*AssignmentRuntime),
	}
}

func (r *Registry) InitializeStore() error {
	return r.etcd.Connect(
		r.ctx,
		config.EtcdEndpoint,
		config.Timeout,
		config.StartupInterval,
		config.StartupRetries,
	)
}

func (r *Registry) Start(a *models.Assignment) error {
	if a == nil {
		return fmt.Errorf("assignment is nil")
	}

	r.mu.Lock()
	if a.Role != "node" && r.etcd == nil {
		r.mu.Unlock()
		return fmt.Errorf("store not initialized")
	}

	if _, exists := r.runtimes[a.ID]; exists {
		r.mu.Unlock()
		return nil
	}

	rn, err := r.runner(a.Role)
	if err != nil {
		r.mu.Unlock()
		return err
	}

	rt := NewAssignmentRuntime(a)
	r.runtimes[a.ID] = rt
	r.mu.Unlock()

	rt.Start(r.ctx, rn)
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

func (r *Registry) IsActive(assignmentID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.runtimes[assignmentID]
	return exists
}

func (r *Registry) runner(role string) (RoleRunner, error) {
	switch role {
	case "node":
		return roles.NewNodeRole(r, r.dcr, r.osa, r.httpSrv, r.healthChecker), nil

	case "member":
		return roles.NewMemberRole(r.etcd, r), nil

	case "leader":
		return roles.NewLeaderRole(r.etcd), nil

	case "tailscale-manager":
		return roles.NewRecruiter(
			r.etcd,
			r.ts,
			r.rpcClient,
		), nil

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
) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.CancelFunc = cancel

	go func() {
		defer close(r.DoneChan)
		defer cancel()

		runner.Run(ctx, r.Definition)
	}()
}

func (r *AssignmentRuntime) Stop() {
	if r.CancelFunc != nil {
		r.CancelFunc()
	}
	<-r.DoneChan
}
