package roles

import (
	"context"
	"log"
	"time"

	"cloud-controller/src/adapters"
	"cloud-controller/src/models"
)

// AssignmentRuntime handles the life cycle of a single running assignment.
type AssignmentRuntime struct {
	AssignmentID string
	Definition   *models.Assignment
	Store        *adapters.Store
	CancelFunc   context.CancelFunc
	DoneChan     chan struct{}
}

func NewAssignmentRuntime(asg *models.Assignment, s *adapters.Store) *AssignmentRuntime {
	return &AssignmentRuntime{
		AssignmentID: asg.ID,
		Definition:   asg,
		Store:        s,
		DoneChan:     make(chan struct{}),
	}
}

func (r *AssignmentRuntime) Start(parentCtx context.Context, entry RegistryEntry) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.CancelFunc = cancel

	go func() {
		defer close(r.DoneChan)
		defer cancel()

		// Context managing the lifespan of the dynamic heartbeat lease stream
		heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
		defer heartbeatCancel()

		go func() {
			key := adapters.AssignmentHeartbeatPath(r.AssignmentID)
			for {
				select {
				case <-heartbeatCtx.Done():
					return
				default:
					leaseCtx, leaseCancel := context.WithCancel(heartbeatCtx)
					err := r.Store.KeepAliveLease(leaseCtx, key, "running", 5)
					if err != nil {
						if heartbeatCtx.Err() == nil {
							log.Printf("[Runtime] KeepAlive lease failed for assignment %s: %v. Re-establishing...", r.AssignmentID, err)
						}
						leaseCancel()
						time.Sleep(2 * time.Second)
						continue
					}

					<-leaseCtx.Done()
					leaseCancel()
				}
			}
		}()

		log.Printf("[Runtime] Executing role '%s' for assignment %s", r.Definition.Role, r.AssignmentID)
		err := entry.Logic(ctx, r.Definition)
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
