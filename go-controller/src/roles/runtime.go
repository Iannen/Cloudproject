package roles

import (
	"context"
	"log"

	"cloud-controller/src/adapters"
	"cloud-controller/src/models"
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

func (r *AssignmentRuntime) Start(parentCtx context.Context, entry RegistryEntry, sess adapters.SessionWrapper) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.CancelFunc = cancel

	go func() {
		defer close(r.DoneChan)
		defer cancel()

		log.Printf("[Runtime] Executing role '%s' for assignment %s", r.Definition.Role, r.AssignmentID)
		err := entry.Logic(ctx, r.Definition, sess)
		if err != nil && ctx.Err() == nil {
			log.Printf("[Runtime] Assignment %s returned error: %v", r.AssignmentID, err)
		}

		log.Printf("[Runtime] Finished role execution for assignment %s", r.AssignmentID)
	}()
}

func (r *AssignmentRuntime) Stop() {
	if r.CancelFunc != nil {
		r.CancelFunc()
	}
	<-r.DoneChan
}
