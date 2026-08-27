package models

type MemberEventType string

const (
	EventAssignmentChange MemberEventType = "ASSIGNMENT_CHANGE"
	EventLeaderDeleted    MemberEventType = "LEADER_DELETED"
	EventReconcileTick    MemberEventType = "RECONCILE_TICK"
)

type MemberEvent struct {
	Type MemberEventType
}

type LeaderEventType string

const (
	EventLeaderReconcileTick LeaderEventType = "RECONCILE_TICK"
)

type LeaderEvent struct {
	Type LeaderEventType
}

type RecruiterEventType string

const (
	EventRecruiterReconcileTick RecruiterEventType = "RECONCILE_TICK"
)

type RecruiterEvent struct {
	Type RecruiterEventType
}
