package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"log"
)

type NodeRole struct {
	asg models.Assignment
	dcr DockerMgr
	osa FileMgr
	reg RoleMgr
}

func (n *NodeRole) Run(ctx context.Context) {
	log.Printf("[NodeRole] Node role runner active for node %s", n.asg.NodeID)
	<-ctx.Done()
}

func (n *NodeRole) HandleInit(ctx context.Context) (string, error) {
	id := n.osa.GetNodeID()
	if n.reg.IsActive("member-" + id) {
		return fmt.Sprintf("Node %s already initialized.\n", id), nil
	}

	_ = n.dcr.ResetEtcd(ctx)
	if err := n.dcr.StartEtcd(ctx); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := n.dcr.WaitEtcdReady(ctx); err != nil {
		return "", fmt.Errorf("etcd ready check failed: %w", err)
	}

	if err := n.activateMember(ctx); err != nil {
		return "", err
	}

	return fmt.Sprintf("Node %s initialized.\n", id), nil
}

func (n *NodeRole) HandleAssimilate(ctx context.Context, p models.AssimilatePayload) (string, error) {
	if err := n.osa.WriteEnvConfig(ctx, p); err != nil {
		return "", fmt.Errorf("config write failed: %w", err)
	}

	_ = n.dcr.ResetEtcd(ctx)
	if err := n.dcr.StartEtcd(ctx); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := n.dcr.WaitEtcdReady(ctx); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("timeout: %w", ctx.Err())
		}
		return "", fmt.Errorf("etcd socket timeout: %w", err)
	}
	return "Learner ready.\n", nil
}

func (n *NodeRole) HandleActivate(ctx context.Context) (string, error) {
	if err := n.activateMember(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Node %s activated.\n", n.osa.GetNodeID()), nil
}

func (n *NodeRole) HandleGetLogs(ctx context.Context) (string, error) {
	logs, err := n.dcr.GetLogs(ctx, "controller")
	if err != nil {
		return "", fmt.Errorf("failed to fetch container logs: %w", err)
	}
	return logs, nil
}

func (n *NodeRole) activateMember(_ context.Context) error {
	if err := n.reg.InitializeStore(); err != nil {
		return fmt.Errorf("etcd connect failed: %w", err)
	}
	asg := NewMemberAssignment(n.osa.GetNodeID())
	return n.reg.Start(asg)
}

func NewNodeRole(asg models.Assignment, reg RoleMgr, dcr DockerMgr, osa FileMgr) *NodeRole {
	return &NodeRole{asg: asg, reg: reg, dcr: dcr, osa: osa}
}
