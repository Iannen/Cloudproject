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

type MemberEvent interface {
	isMemberEvent()
}

type TickEvent struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func (TickEvent) isRecruiterEvent() {}
func (TickEvent) isLeaderEvent()    {}
func (TickEvent) isMemberEvent()    {}

type MemberAssignmentChangeEvent struct{}

func (MemberAssignmentChangeEvent) isMemberEvent() {}

type MemberLeaderDeletedEvent struct{}

func (MemberLeaderDeletedEvent) isMemberEvent() {}

type MemberSessionExpiredEvent struct{}

func (MemberSessionExpiredEvent) isMemberEvent() {}
