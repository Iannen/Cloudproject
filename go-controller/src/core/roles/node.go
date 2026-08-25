package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
)

type NodeRole struct {
	dcr DockerMgr
	osa FileMgr
	cms HTTPServer
	spk HealthChecker
	reg RoleMgr
}

func (n *NodeRole) Run(ctx context.Context, a *models.Assignment) {
	log.Printf("[NodeRole] Starting server for node %s", a.NodeID)
	errCh := n.cms.Start()

	for {
		select {
		case err, ok := <-errCh:
			if ok && err != nil {
				log.Printf("[NodeRole] HTTP server error: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[NodeRole] Shutting down HTTP listener for node %s", a.NodeID)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), config.Timeout)
			defer cancel()

			if err := n.cms.Shutdown(shutdownCtx); err != nil {
				log.Printf("[NodeRole] Error during HTTP shutdown: %v", err)
			}
			return
		}
	}
}

func (n *NodeRole) handleInit(ctx context.Context, body []byte) (string, error) {
	id := n.osa.GetNodeID()
	if n.reg.IsActive("member-" + id) {
		return fmt.Sprintf("Node %s already initialized.\n", id), nil
	}

	_ = n.dcr.ResetEtcd(ctx)
	if err := n.dcr.StartEtcd(ctx); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := n.spk.WaitEtcdReady(ctx); err != nil {
		return "", fmt.Errorf("etcd ready check failed: %w", err)
	}

	if err := n.activateMember(ctx); err != nil {
		return "", err
	}

	return fmt.Sprintf("Node %s initialized.\n", id), nil
}

func (n *NodeRole) handleAssimilate(ctx context.Context, body []byte) (string, error) {
	var p models.AssimilatePayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	if err := n.osa.WriteEnvConfig(ctx, p); err != nil {
		return "", fmt.Errorf("config write failed: %w", err)
	}

	_ = n.dcr.ResetEtcd(ctx)
	if err := n.dcr.StartEtcd(ctx); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := n.spk.WaitEtcdReady(ctx); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("timeout: %w", ctx.Err())
		}
		return "", fmt.Errorf("etcd socket timeout: %w", err)
	}
	return "Learner ready.\n", nil
}

func (n *NodeRole) handleActivate(ctx context.Context, body []byte) (string, error) {
	if err := n.activateMember(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Node %s activated.\n", n.osa.GetNodeID()), nil
}

func (n *NodeRole) handleGetLogs(ctx context.Context, body []byte) (string, error) {
	logs, err := n.dcr.GetLogs(ctx, "controller")
	if err != nil {
		return "", fmt.Errorf("failed to fetch container logs: %w", err)
	}
	return logs, nil
}

func (n *NodeRole) InitializeStore() error {
	return n.reg.InitializeStore()
}

func (n *NodeRole) activateMember(_ context.Context) error {
	if err := n.InitializeStore(); err != nil {
		return fmt.Errorf("etcd connect failed: %w", err)
	}
	asg := NewMemberAssignment(n.osa.GetNodeID())
	return n.reg.Start(&asg)
}

func NewNodeRole(reg RoleMgr, dcr DockerMgr, osa FileMgr, cms HTTPServer, spk HealthChecker) *NodeRole {
	n := &NodeRole{reg: reg, dcr: dcr, osa: osa, cms: cms, spk: spk}
	n.cms.RegisterGetRoute("/initialize", n.handleInit)
	n.cms.RegisterGetRoute("/logs", n.handleGetLogs)
	n.cms.RegisterPostRoute("/assimilate", n.handleAssimilate)
	n.cms.RegisterPostRoute("/activate", n.handleActivate)
	return n
}
