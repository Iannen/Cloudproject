package roles

import (
	"context"
	"errors"

	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
)

type MemberRole struct {
	store    ParticipantStore
	registry RoleMgr
}

type ParticipantStore interface {
	NodeAssignments(ctx context.Context, nodeID string) ([]string, int64, error)
	AssignmentDef(ctx context.Context, assignmentID string) (*models.Assignment, error)
	CreateAssignment(ctx context.Context, a models.Assignment) error
	NewSession(ctx context.Context) (models.Session, error)
	PutWithSession(ctx context.Context, sess models.Session, nodeID string, value string) error
	ClaimLeader(ctx context.Context, sess models.Session, nodeID string) (bool, error)
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
	log.Println("[Member] Member role starting ")

	for {
		if ctx.Err() != nil {
			log.Printf("[Member] Stopping member role loop: %v", ctx.Err())
			return
		}

		sess, err := m.store.NewSession(ctx)
		if err != nil {
			log.Printf("[Member] Failed to establish session: %v", err)
			m.registry.StopManagedAssignments()
			if ctx.Err() != nil {
				return
			}
			continue
		}

		if err := m.store.PutWithSession(ctx, sess, nodeID, "alive"); err != nil {
			log.Printf("[Member] Failed to register heartbeat presence: %v", err)
			_ = sess.Close()
			m.registry.StopManagedAssignments()
			if ctx.Err() != nil {
				return
			}
			continue
		}

		if err := m.runSession(ctx, sess, nodeID); err != nil {
			log.Printf("[Member] Session terminated: %v", err)
		}

		_ = sess.Close()
		m.registry.StopManagedAssignments()
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
	isLeader, err := m.store.ClaimLeader(ctx, sess, nodeID)
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
	if err := m.store.CreateAssignment(ctx, asg); err != nil {
		log.Printf("[Member] Failed to write leader assignment definition: %v", err)
	}
}

func (m *MemberRole) reconcile(ctx context.Context, nodeID string) {
	ids, _, err := m.store.NodeAssignments(ctx, nodeID)
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
		if !want[id] {
			m.registry.Stop(id)
		}
	}

	for _, id := range ids {
		if !active[id] {
			asg, err := m.store.AssignmentDef(ctx, id)
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
