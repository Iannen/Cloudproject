package models

type EventType string

const (
	EventAssignmentChange EventType = "ASSIGNMENT_CHANGE"
	EventLeaderDeleted    EventType = "LEADER_DELETED"
	EventReconcileTick    EventType = "RECONCILE_TICK"
)

type Event struct {
	Type EventType
}
