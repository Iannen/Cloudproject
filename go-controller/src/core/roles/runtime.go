package roles

import (
	"context"
	"log"

	"cloud-controller/src/core/models"

	"go.etcd.io/etcd/client/v3/concurrency"
)

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
