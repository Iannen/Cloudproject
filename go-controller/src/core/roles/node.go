package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
)

type RPCHandler struct {
	dcr DockerMgr
	osa FileMgr
	reg RoleMgr
}

func (h *RPCHandler) HandleInit(ctx context.Context) (string, error) {
	id := h.osa.GetNodeID()
	if h.reg.IsActive("member-" + id) {
		return fmt.Sprintf("Node %s already initialized.\n", id), nil
	}

	_ = h.dcr.ResetEtcd(ctx)
	if err := h.dcr.StartEtcd(ctx); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := h.dcr.WaitEtcdReady(ctx); err != nil {
		return "", fmt.Errorf("etcd ready check failed: %w", err)
	}

	if err := h.activateMember(ctx); err != nil {
		return "", err
	}

	return fmt.Sprintf("Node %s initialized.\n", id), nil
}

func (h *RPCHandler) HandleAssimilate(ctx context.Context, p models.AssimilatePayload) (string, error) {
	if err := h.osa.WriteEnvConfig(ctx, p); err != nil {
		return "", fmt.Errorf("config write failed: %w", err)
	}

	_ = h.dcr.ResetEtcd(ctx)
	if err := h.dcr.StartEtcd(ctx); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := h.dcr.WaitEtcdReady(ctx); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("timeout: %w", ctx.Err())
		}
		return "", fmt.Errorf("etcd socket timeout: %w", err)
	}
	return "Learner ready.\n", nil
}

func (h *RPCHandler) HandleActivate(ctx context.Context) (string, error) {
	if err := h.activateMember(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Node %s activated.\n", h.osa.GetNodeID()), nil
}

func (h *RPCHandler) HandleGetLogs(ctx context.Context) (string, error) {
	logs, err := h.dcr.GetLogs(ctx, "controller")
	if err != nil {
		return "", fmt.Errorf("failed to fetch container logs: %w", err)
	}
	return logs, nil
}

func (h *RPCHandler) activateMember(_ context.Context) error {
	if err := h.reg.InitializeStore(); err != nil {
		return fmt.Errorf("etcd connect failed: %w", err)
	}
	asg := NewMemberAssignment(h.osa.GetNodeID())
	return h.reg.Start(asg)
}

func NewRPCHandler(reg RoleMgr, dcr DockerMgr, osa FileMgr) *RPCHandler {
	return &RPCHandler{reg: reg, dcr: dcr, osa: osa}
}
