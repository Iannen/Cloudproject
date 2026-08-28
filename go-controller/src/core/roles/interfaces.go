package roles

import (
	"context"

	"go-controller/src/core/models"
)

type RoleMgr interface {
	Start(a models.Assignment) error
	Stop(assignmentID string)
	StopAll()
	StopManagedAssignments()
	ActiveAssignments() map[string]bool
	IsActive(assignmentID string) bool
}

type StoreAdapter interface {
	AssignmentStore
	ParticipantStore
	ClusterMgr
}

type AssignmentStore interface {
	GetActiveNodeIDs(ctx context.Context) ([]string, error)
	GetAllAssignments(ctx context.Context) ([]models.Assignment, error)
	CreateAssignment(ctx context.Context, a models.Assignment) error
	SubscribeLeaderEvents(ctx context.Context) (<-chan models.LeaderEvent, error)
}

type ParticipantStore interface {
	NodeAssignments(ctx context.Context, nodeID string) ([]string, int64, error)
	AssignmentDef(ctx context.Context, assignmentID string) (*models.Assignment, error)
	CreateAssignment(ctx context.Context, a models.Assignment) error
	NewSession(ctx context.Context) (models.Session, error)
	PutWithSession(ctx context.Context, sess models.Session, nodeID string, value string) error
	ClaimLeader(ctx context.Context, sess models.Session, nodeID string) (bool, error)
	SubscribeEvents(ctx context.Context, nodeID string) (<-chan models.Event, error)
	Connect(ctx context.Context) error
}

type FileMgr interface {
	GetNodeID() string
	WriteEnvConfig(ctx context.Context, payload models.AssimilatePayload) error
}

type DockerMgr interface {
	StartEtcd(ctx context.Context) error
	ResetEtcd(ctx context.Context) error
	WaitEtcdReady(ctx context.Context) error
	GetLogs(ctx context.Context, containerID string) (string, error)
}

type RpcClient interface {
	Assimilate(ctx context.Context, targetIP string, payload models.AssimilatePayload) error
	Activate(ctx context.Context, targetIP string) error
}

type ClusterMgr interface {
	GetClusterPeerURLs(ctx context.Context) (map[string]bool, error)
	AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error)
	PromoteMember(ctx context.Context, memberID uint64) error
	RemoveMember(memberID uint64) error
	SubscribeRecruiterEvents(ctx context.Context) (<-chan models.RecruiterEvent, error)
}

type TSClient interface {
	GetPeers(ctx context.Context) ([]models.TSPeer, error)
	GetLocalIP(ctx context.Context) (string, error)
}
