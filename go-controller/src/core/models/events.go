package models

import "context"

type EventType string

const (
	EventAssignmentChange EventType = "ASSIGNMENT_CHANGE"
	EventLeaderDeleted    EventType = "LEADER_DELETED"
	EventReconcileTick    EventType = "RECONCILE_TICK"
)

type Event struct {
	Type       EventType
	Ctx        context.Context
	cancelFunc context.CancelFunc
}

func (e Event) Cancel() {
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
}

func NewTickEvent(ctx context.Context, cancel context.CancelFunc) Event {
	return Event{
		Type:       EventReconcileTick,
		Ctx:        ctx,
		cancelFunc: cancel,
	}
}

func NewEvent(typ EventType) Event {
	return Event{
		Type: typ,
	}
}
