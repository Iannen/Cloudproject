package roles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud-controller/src/config"
	"cloud-controller/src/models"

	"go.etcd.io/etcd/client/v3/concurrency"
)

type TsManagerStore interface {
	GetClusterMembers(ctx context.Context) ([]models.MemberInfo, error)
	AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error)
	PromoteMember(ctx context.Context, memberID uint64) error
	RemoveMember(ctx context.Context, memberID uint64) error
}

type TSMgr struct {
	str TsManagerStore
	cli *http.Client
}

type TSPeer struct {
	HostName     string   `json:"HostName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

type TSStatus struct {
	Peer map[string]*TSPeer `json:"Peer"`
	Self *TSPeer            `json:"Self"`
}

func (t *TSMgr) Run(ctx context.Context, a *models.Assignment, s *concurrency.Session) error {
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

	peers, err := t.peers(ctx)
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

func (t *TSMgr) peers(ctx context.Context) ([]*TSPeer, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}

	var st TSStatus
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	res := make([]*TSPeer, 0, len(st.Peer)+1)
	if st.Self != nil {
		res = append(res, st.Self)
	}
	for _, p := range st.Peer {
		res = append(res, p)
	}
	return res, nil
}

func (t *TSMgr) assimilate(ctx context.Context, p *TSPeer) {
	ip := p.TailscaleIPs[0]
	locIP, err := t.localIP(ctx)
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

	b, _ := json.Marshal(models.AssimilatePayload{
		LeaderName:         config.NodeID(),
		LeaderPeerURL:      fmt.Sprintf("http://%s:2380", locIP),
		EtcdInitialCluster: strings.Join(toks, ","),
		AssignedIP:         ip,
	})

	if !t.post(ctx, fmt.Sprintf("http://%s:8080/assimilate", ip), b) {
		log.Printf("[TSMgr] assimilation failed host=%s step=assimilate_rpc", p.HostName)
		return
	}

	time.Sleep(2 * time.Second)

	if err := t.str.PromoteMember(ctx, lid); err != nil {
		log.Printf("[TSMgr] assimilation failed host=%s step=promote: %v", p.HostName, err)
		return
	}
	lid = 0

	if !t.post(ctx, fmt.Sprintf("http://%s:8080/activate", ip), nil) {
		log.Printf("[TSMgr] assimilation failed host=%s step=activate_rpc", p.HostName)
		return
	}

	log.Printf("[TSMgr] assimilated and activated host=%s ip=%s", p.HostName, ip)
}

func (t *TSMgr) post(ctx context.Context, url string, body []byte) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := t.cli.Do(req)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func (t *TSMgr) localIP(ctx context.Context) (string, error) {
	if ip := os.Getenv("TAILSCALE_IP"); ip != "" {
		return ip, nil
	}
	out, err := exec.CommandContext(ctx, "tailscale", "ip", "-4").Output()
	if err != nil {
		return "", err
	}
	if ip := strings.TrimSpace(string(out)); ip != "" {
		return ip, nil
	}
	return "", fmt.Errorf("empty tailscale ip")
}
