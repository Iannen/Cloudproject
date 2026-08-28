package models

import "context"

type EventType string

const (
	EventAssignmentChange EventType = "ASSIGNMENT_CHANGE"
	EventLeaderDeleted    EventType = "LEADER_DELETED"
	EventReconcileTick    EventType = "RECONCILE_TICK"
	EventSessionExpired   EventType = "SESSION_EXPIRED"
)

type Event struct {
	Type   EventType
	Ctx    context.Context
	Cancel context.CancelFunc
}

type RecruiterEvent interface {
	isRecruiterEvent()
}

type LeaderEvent interface {
	isLeaderEvent()
}

type TickEvent struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func (TickEvent) isRecruiterEvent() {}
func (TickEvent) isLeaderEvent()    {}
