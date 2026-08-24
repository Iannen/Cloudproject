package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"time"
)

type NodeRole struct {
	dcr DockerMgr
	osa FileMgr
	cms HTTPServer
	spk HealthChecker
	reg RoleMgr
}

func (n *NodeRole) Run(ctx context.Context, a *models.Assignment) {
	log.Printf("[NodeRole] Starting server on port %s for node %s", config.HTTPPort, a.NodeID)
	errCh := n.cms.Start(config.HTTPPort, config.Timeout)

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
	id := config.NodeID()
	if n.reg.IsActive("member-" + id) {
		return fmt.Sprintf("Node %s already initialized.\n", id), nil
	}

	_ = n.dcr.ResetEtcd(ctx, config.BootstrapDir)
	if err := n.dcr.StartEtcd(ctx, config.BootstrapDir); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := n.spk.WaitEndpointReady(ctx, config.EtcdEndpoint, config.StartupRetries, config.StartupInterval); err != nil {
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

	if err := n.osa.WriteEnvConfig(ctx, config.NodeID(), config.BootstrapDir, p); err != nil {
		return "", fmt.Errorf("config write failed: %w", err)
	}

	_ = n.dcr.ResetEtcd(ctx, config.BootstrapDir)
	if err := n.dcr.StartEtcd(ctx, config.BootstrapDir); err != nil {
		return "", fmt.Errorf("etcd start failed: %w", err)
	}

	if err := n.spk.WaitEndpointReady(ctx, config.EtcdEndpoint, config.StartupRetries, config.StartupInterval); err != nil {
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
	return fmt.Sprintf("Node %s activated.\n", config.NodeID()), nil
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
	return n.reg.Start(NewMemberAssignment(config.NodeID()))
}

type HTTPServer interface {
	RegisterGetRoute(pattern string, handler models.DomainHandler)
	RegisterPostRoute(pattern string, handler models.DomainHandler)
	Start(addr string, clientTimeout time.Duration) <-chan error
	Shutdown(ctx context.Context) error
}
type HealthChecker interface {
	WaitEndpointReady(ctx context.Context, endpoint string, retries int, interval time.Duration) error
}
type FileMgr interface {
	WriteEnvConfig(ctx context.Context, nodeID string, bootstrapDir string, payload models.AssimilatePayload) error
}

type DockerMgr interface {
	StartEtcd(ctx context.Context, bootstrapDir string) error
	ResetEtcd(ctx context.Context, bootstrapDir string) error
	GetLogs(ctx context.Context, containerID string) (string, error)
}

func NewNodeRole(reg RoleMgr, dcr DockerMgr, osa FileMgr, cms HTTPServer, spk HealthChecker) *NodeRole {
	n := &NodeRole{reg: reg, dcr: dcr, osa: osa, cms: cms, spk: spk}
	n.cms.RegisterGetRoute("/initialize", n.handleInit)
	n.cms.RegisterGetRoute("/logs", n.handleGetLogs)
	n.cms.RegisterPostRoute("/assimilate", n.handleAssimilate)
	n.cms.RegisterPostRoute("/activate", n.handleActivate)
	return n
}
