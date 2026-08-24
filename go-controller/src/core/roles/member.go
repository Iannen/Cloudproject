package roles

import (
	"context"
	"errors"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"strings"
	"time"
)

type MemberRole struct {
	store    ParticipantStore
	registry RoleMgr
}

type ParticipantStore interface {
	NodeAssignments(ctx context.Context, nodeAsgPath string) ([]string, int64, error)
	AssignmentDef(ctx context.Context, asgDefPath string) (*models.Assignment, error)
	CreateAssignment(ctx context.Context, asgDefPath, nodeAsgPath string, a models.Assignment) error
	NewSession(ctx context.Context, ttl int64, retries int, interval time.Duration) (models.Session, error)
	PutWithSession(ctx context.Context, sess models.Session, key string, value string) error
	ClaimLeader(ctx context.Context, sess models.Session, leaderKey, nodeID string) (bool, error)
	SubscribeEvents(ctx context.Context, nodeID string) (<-chan models.MemberEvent, error)
}

func NewMemberAssignment(nodeID string) *models.Assignment {
	return &models.Assignment{
		NodeID: nodeID,
		ID:     "member-" + nodeID,
		Role:   "member",
	}
}

func NewMemberRole(store ParticipantStore, registry RoleMgr) *MemberRole {
	return &MemberRole{
		store:    store,
		registry: registry,
	}
}

func (m *MemberRole) Run(ctx context.Context, asg *models.Assignment) {
	nodeID := config.NodeID()
	log.Printf("[Member] Member role starting setup for node %s", nodeID)

	sess, err := m.store.NewSession(ctx, config.SessionTTL, config.StartupRetries, config.RetryInterval)
	if err != nil {
		log.Printf("[Member] Failed to establish initial session: %v", err)
		return
	}

	defer func() {
		_ = sess.Close()
		m.stopManagedAssignments()
	}()

	hbKey := config.NodeHeartbeatPath(nodeID)
	if err := m.store.PutWithSession(ctx, sess, hbKey, "alive"); err != nil {
		log.Printf("[Member] Failed to register heartbeat presence: %v", err)
		return
	}

	if err := m.runSession(ctx, sess, nodeID); err != nil {
		log.Printf("[Member] Session terminated: %v", err)
	}
}

func (m *MemberRole) runSession(sCtx context.Context, sess models.Session, nodeID string) error {
	m.tryClaimLeadership(sCtx, sess, nodeID)
	m.reconcile(sCtx, nodeID)

	ch, err := m.store.SubscribeEvents(sCtx, nodeID)
	if err != nil {
		return err
	}

	for {
		select {
		case <-sCtx.Done():
			return sCtx.Err()

		case <-sess.Done():
			return errors.New("session lease expired")

		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if ev.Err != nil {
				log.Printf("[Member] Event stream error: %v", ev.Err)
				continue
			}

			switch ev.Type {
			case models.EventLeaderDeleted:
				m.tryClaimLeadership(sCtx, sess, nodeID)

			case models.EventAssignmentChange, models.EventReconcileTick:
				m.reconcile(sCtx, nodeID)

			default:
				log.Printf("[Member] Unhandled event type: %s", ev.Type)
			}
		}
	}
}

func (m *MemberRole) tryClaimLeadership(ctx context.Context, sess models.Session, nodeID string) {
	isLeader, err := m.store.ClaimLeader(ctx, sess, config.ClusterLeaderKey, nodeID)
	if err != nil {
		log.Printf("[Member] Leadership claim attempt error: %v", err)
		return
	}
	if !isLeader {
		log.Println("[Member] Leadership race lost")
		return
	}
	log.Println("[Member] Won leadership! Launching Leader Role...")
	asg := NewLeaderAssignment(nodeID)
	if err := m.store.CreateAssignment(ctx, config.AsgDefPath(asg.ID), config.NodeAssignmentsPath(nodeID), asg); err != nil {
		log.Printf("[Member] Failed to write leader assignment definition: %v", err)
	}
}

func (m *MemberRole) stopManagedAssignments() {
	active := m.registry.ActiveAssignments()
	for id := range active {
		if strings.HasPrefix(id, "node-") || strings.HasPrefix(id, "member-") {
			continue
		}
		m.registry.Stop(id)
	}
}

func (m *MemberRole) reconcile(ctx context.Context, nodeID string) {
	ids, _, err := m.store.NodeAssignments(ctx, config.NodeAssignmentsPath(nodeID))
	if err != nil {
		log.Printf("[Member] Error updating assignment state: %v", err)
		return
	}

	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}

	active := m.registry.ActiveAssignments()
	for id := range active {
		if strings.HasPrefix(id, "member-") || strings.HasPrefix(id, "node-") {
			continue
		}
		if !want[id] {
			m.registry.Stop(id)
		}
	}

	for _, id := range ids {
		if !active[id] {
			asg, err := m.store.AssignmentDef(ctx, config.AsgDefPath(id))
			if err != nil {
				log.Printf("[Member] fetch assignment def failed id=%s: %v", id, err)
				continue
			}
			if err := m.registry.Start(asg); err != nil {
				log.Printf("[Member] start assignment failed id=%s role=%s: %v", id, asg.Role, err)
			}
		}
	}
}
