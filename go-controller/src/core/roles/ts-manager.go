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

type TSMgr struct {
	str  TsManagerStore
	ts   TailscaleProvider
	http NodeClient
}

func (t *TSMgr) Run(ctx context.Context, a *models.Assignment) error {
	tk := time.NewTicker(config.ReconcileInterval)
	defer tk.Stop()

	log.Printf("[TSMgr] Started: %s on %s", a.ID, a.NodeID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tk.C:
			t.reconcile(ctx)
		}
	}
}

func (t *TSMgr) reconcile(ctx context.Context) {
	members, err := t.str.GetClusterMembers(ctx)
	if err != nil {
		log.Printf("[TSMgr] Cluster member lookup failed: %v", err)
		return
	}

	seenIPs := make(map[string]bool)
	for _, m := range members {
		for _, u := range m.PeerURLs {
			seenIPs[u] = true
		}
	}

	peers, err := t.ts.GetPeers(ctx)
	if err != nil {
		log.Printf("[TSMgr] Peer discovery failed: %v", err)
		return
	}

	for _, p := range peers {
		if !p.Online || len(p.TailscaleIPs) == 0 || !strings.HasPrefix(p.HostName, config.NodeNamePrefix) {
			continue
		}

		peerURL := fmt.Sprintf("http://%s:%d", p.TailscaleIPs[0], config.EtcdPeerPort)
		if seenIPs[peerURL] {
			continue
		}

		t.assimilate(ctx, p)
	}
}

func (t *TSMgr) assimilate(ctx context.Context, p *models.TSPeer) {
	ip := p.TailscaleIPs[0]
	locIP, err := t.ts.GetLocalIP(ctx)
	if err != nil {
		log.Printf("[TSMgr] Local IP lookup failed: %v", err)
		return
	}

	l, mems, err := t.str.AddLearner(ctx, fmt.Sprintf("http://%s:%d", ip, config.EtcdPeerPort))
	if err != nil {
		log.Printf("[TSMgr] assimilation failed host=%s step=add_learner: %v", p.HostName, err)
		return
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
		log.Printf("[TSMgr] assimilation failed host=%s step=assimilate_rpc: %v", p.HostName, err)
		return
	}

	time.Sleep(2 * time.Second)

	if err := t.str.PromoteMember(ctx, lid); err != nil {
		log.Printf("[TSMgr] assimilation failed host=%s step=promote: %v", p.HostName, err)
		return
	}
	lid = 0

	if err := t.http.Activate(ctx, ip); err != nil {
		log.Printf("[TSMgr] assimilation failed host=%s step=activate_rpc: %v", p.HostName, err)
		return
	}

	log.Printf("[TSMgr] assimilated and activated host=%s ip=%s", p.HostName, ip)
}

func NewTSMgr(str TsManagerStore, ts TailscaleProvider, http NodeClient) *TSMgr {
	return &TSMgr{
		str:  str,
		ts:   ts,
		http: http,
	}
}

type NodeClient interface {
	Assimilate(ctx context.Context, targetIP string, payload models.AssimilatePayload) error
	Activate(ctx context.Context, targetIP string) error
}

type TsManagerStore interface {
	GetClusterMembers(ctx context.Context) ([]models.MemberInfo, error)
	AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error)
	PromoteMember(ctx context.Context, memberID uint64) error
	RemoveMember(ctx context.Context, memberID uint64) error
}
