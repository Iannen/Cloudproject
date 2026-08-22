package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"strings"
	"time"
)

type TSClient interface {
	GetPeers(ctx context.Context) ([]*models.TSPeer, error)
	GetLocalIP(ctx context.Context) (string, error)
}

type Recruiter struct {
	str  ClusterMgr
	ts   TSClient
	http RpcClient
}

func (t *Recruiter) Run(ctx context.Context, a *models.Assignment) {
	tk := time.NewTicker(config.ReconcileInterval)
	defer tk.Stop()

	log.Printf("[TSMgr] Started: %s on %s", a.ID, a.NodeID)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			if err := t.reconcile(ctx); err != nil {
				log.Println(err)
			}
		}
	}
}

func (t *Recruiter) reconcile(ctx context.Context) error {
	seenIPs, err := t.str.GetClusterPeerURLs(ctx)
	if err != nil {
		return fmt.Errorf("[TSMgr] cluster member lookup failed: %w", err)
	}

	peers, err := t.ts.GetPeers(ctx)
	if err != nil {
		return fmt.Errorf("[TSMgr] peer discovery failed: %w", err)
	}

	for _, p := range peers {
		if !p.Online || len(p.TailscaleIPs) == 0 || !strings.HasPrefix(p.HostName, config.NodeNamePrefix) {
			continue
		}

		peerURL := fmt.Sprintf("http://%s:%d", p.TailscaleIPs[0], config.EtcdPeerPort)
		if seenIPs[peerURL] {
			continue
		}

		if err := t.recruit(ctx, p); err != nil {
			return err
		}
	}

	return nil
}

func (t *Recruiter) recruit(ctx context.Context, p *models.TSPeer) error {
	ip := p.TailscaleIPs[0]

	locIP, err := t.ts.GetLocalIP(ctx)
	if err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=get_local_ip: %w", p.HostName, err)
	}

	l, mems, err := t.str.AddLearner(ctx, fmt.Sprintf("http://%s:%d", ip, config.EtcdPeerPort))
	if err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=add_learner: %w", p.HostName, err)
	}

	lid := l.ID
	defer func() {
		if lid != 0 {
			_ = t.str.RemoveMember(context.Background(), lid)
		}
	}()

	toks := make([]string, 0, len(mems))
	for _, m := range mems {
		n := m.Name
		if n == "" {
			n = p.HostName
		}
		if len(m.PeerURLs) > 0 {
			toks = append(toks, fmt.Sprintf("%s=%s", n, m.PeerURLs[0]))
		}
	}

	payload := models.AssimilatePayload{
		LeaderName:         config.NodeID(),
		LeaderPeerURL:      fmt.Sprintf("http://%s:%d", locIP, config.EtcdPeerPort),
		EtcdInitialCluster: strings.Join(toks, ","),
		AssignedIP:         ip,
	}

	if err := t.http.Assimilate(ctx, ip, payload); err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=assimilate_rpc: %w", p.HostName, err)
	}

	time.Sleep(2 * time.Second)

	if err := t.str.PromoteMember(ctx, lid); err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=promote: %w", p.HostName, err)
	}
	lid = 0

	if err := t.http.Activate(ctx, ip); err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=activate_rpc: %w", p.HostName, err)
	}

	log.Printf("[TSMgr] assimilated and activated host=%s ip=%s", p.HostName, ip)
	return nil
}

func NewRecruiter(str ClusterMgr, ts TSClient, http RpcClient) *Recruiter {
	return &Recruiter{
		str:  str,
		ts:   ts,
		http: http,
	}
}

type RpcClient interface {
	Assimilate(ctx context.Context, targetIP string, payload models.AssimilatePayload) error
	Activate(ctx context.Context, targetIP string) error
}

type ClusterMgr interface {
	GetClusterPeerURLs(ctx context.Context) (map[string]bool, error)
	AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error)
	PromoteMember(ctx context.Context, memberID uint64) error
	RemoveMember(ctx context.Context, memberID uint64) error
}
