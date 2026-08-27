package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/config"
	"go-controller/src/core/models"
	"log"
	"strings"
)

type Recruiter struct {
	asg  models.Assignment
	str  ClusterMgr
	ts   TSClient
	http RpcClient
}

func (t *Recruiter) Run(ctx context.Context) {
	log.Printf("[TSMgr] Started: %s on %s", t.asg.ID, t.asg.NodeID)
	ch, err := t.str.SubscribeRecruiterEvents(ctx)
	if err != nil {
		log.Printf("[TSMgr] Failed to subscribe to recruiter events: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			if err := t.reconcile(ctx, t.asg.NodeID); err != nil {
				log.Println(err)
			}
		}
	}
}

func (t *Recruiter) peerURL(ip string) string {
	return fmt.Sprintf("http://%s:2380", ip)
}

func (t *Recruiter) reconcile(ctx context.Context, nodeID string) error {
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

		peerURL := t.peerURL(p.TailscaleIPs[0])
		if seenIPs[peerURL] {
			continue
		}

		if err := t.recruit(ctx, nodeID, p); err != nil {
			return err
		}
	}

	return nil
}

func (t *Recruiter) recruit(ctx context.Context, nodeID string, p models.TSPeer) error {
	ip := p.TailscaleIPs[0]

	locIP, err := t.ts.GetLocalIP(ctx)
	if err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=get_local_ip: %w", p.HostName, err)
	}

	l, mems, err := t.str.AddLearner(ctx, t.peerURL(ip))
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
		LeaderName:         nodeID,
		LeaderPeerURL:      t.peerURL(locIP),
		EtcdInitialCluster: strings.Join(toks, ","),
		AssignedIP:         ip,
	}

	if err := t.http.Assimilate(ctx, ip, payload); err != nil {
		return fmt.Errorf("[TSMgr] host=%s step=assimilate_rpc: %w", p.HostName, err)
	}

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

func NewRecruiter(asg models.Assignment, str ClusterMgr, ts TSClient, http RpcClient) *Recruiter {
	return &Recruiter{
		asg:  asg,
		str:  str,
		ts:   ts,
		http: http,
	}
}
