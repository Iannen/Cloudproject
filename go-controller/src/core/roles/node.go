package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"sync"
)

type RPCHandler struct {
	appCtx     context.Context
	dcr        DockerMgr
	osa        FileMgr
	memberRole *MemberRole
	mu         sync.Mutex
	started    bool
}

func (h *RPCHandler) HandleInit(ctx context.Context) (string, error) {
	id := h.osa.GetNodeID()

	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return fmt.Sprintf("Node %s already initialized.\n", id), nil
	}
	h.mu.Unlock()

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
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}
	h.started = true
	h.mu.Unlock()

	go func() {
		h.memberRole.Run(h.appCtx)
	}()

	return nil
}

func NewRPCHandler(ctx context.Context, memberRole *MemberRole, dcr DockerMgr, osa FileMgr) *RPCHandler {
	return &RPCHandler{
		appCtx:     ctx,
		memberRole: memberRole,
		dcr:        dcr,
		osa:        osa,
	}
}
