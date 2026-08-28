package roles

import (
	"context"
	"fmt"
	"go-controller/src/core/models"
	"log"
	"strings"
	"sync/atomic"
)

type Recruiter struct {
	asg      models.Assignment
	str      ClusterMgr
	ts       TSClient
	http     RpcClient
	inFlight atomic.Bool
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
		case ev, ok := <-ch:
			if !ok {
				return
			}
			t.handleEvent(ev)
		}
	}
}

func (t *Recruiter) handleEvent(ev models.RecruiterEvent) {
	switch e := ev.(type) {
	case models.RecruiterTickEvent:
		if !t.inFlight.CompareAndSwap(false, true) {
			if e.Cancel != nil {
				e.Cancel()
			}
			return
		}

		go func(te models.RecruiterTickEvent) {
			if te.Cancel != nil {
				defer te.Cancel()
			}
			defer t.inFlight.Store(false)

			if err := t.reconcile(te.Ctx); err != nil {
				log.Println(err)
			}
		}(e)
	}
}

func (t *Recruiter) peerURL(ip string) string {
	return fmt.Sprintf("http://%s:2380", ip)
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
		peerURL := t.peerURL(p.TailscaleIPs[0])
		if seenIPs[peerURL] {
			continue
		}

		if err := t.recruit(ctx, t.asg.NodeID, p); err != nil {
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
			_ = t.str.RemoveMember(lid)
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
