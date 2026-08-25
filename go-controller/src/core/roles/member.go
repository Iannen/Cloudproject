package roles

import (
	"context"
	"errors"

	"go-controller/src/core/models"
	"log"
)

type MemberRole struct {
	asg      models.Assignment
	store    ParticipantStore
	registry RoleMgr
}

func NewMemberAssignment(nodeID string) models.Assignment {
	return models.Assignment{
		NodeID: nodeID,
		ID:     "member-" + nodeID,
		Role:   "member",
	}
}

func NewMemberRole(asg models.Assignment, store ParticipantStore, registry RoleMgr) *MemberRole {
	return &MemberRole{
		asg:      asg,
		store:    store,
		registry: registry,
	}
}

func (m *MemberRole) Run(ctx context.Context) {
	nodeID := m.asg.NodeID
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

		if err := m.runSession(ctx, sess); err != nil {
			log.Printf("[Member] Session terminated: %v", err)
		}

		_ = sess.Close()
		m.registry.StopManagedAssignments()
	}
}

func (m *MemberRole) runSession(sCtx context.Context, sess models.Session) error {
	m.tryClaimLeadership(sCtx, sess)
	m.reconcile(sCtx)

	ch, err := m.store.SubscribeEvents(sCtx, m.asg.NodeID)
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
				m.tryClaimLeadership(sCtx, sess)

			case models.EventAssignmentChange, models.EventReconcileTick:
				m.reconcile(sCtx)

			default:
				log.Printf("[Member] Unhandled event type: %s", ev.Type)
			}
		}
	}
}

func (m *MemberRole) tryClaimLeadership(ctx context.Context, sess models.Session) {
	isLeader, err := m.store.ClaimLeader(ctx, sess, m.asg.NodeID)
	if err != nil {
		log.Printf("[Member] Leadership claim attempt error: %v", err)
		return
	}
	if !isLeader {
		log.Println("[Member] Leadership race lost")
		return
	}
	log.Println("[Member] Won leadership! Launching Leader Role...")
	asg := NewLeaderAssignment(m.asg.NodeID)
	if err := m.store.CreateAssignment(ctx, asg); err != nil {
		log.Printf("[Member] Failed to write leader assignment definition: %v", err)
	}
}

func (m *MemberRole) reconcile(ctx context.Context) {
	ids, _, err := m.store.NodeAssignments(ctx, m.asg.NodeID)
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
			if asg != nil {
				if err := m.registry.Start(*asg); err != nil {
					log.Printf("[Member] start assignment failed id=%s role=%s: %v", id, asg.Role, err)
				}
			}
		}
	}
}
