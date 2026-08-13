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

	"cloud-controller/src/adapters"
	"cloud-controller/src/config"
	"cloud-controller/src/models"
)

type TsManagerStore interface {
	GetActiveNodeIDs(ctx context.Context) ([]string, error)
	AddLearner(ctx context.Context, peerURL string) (*models.MemberInfo, []models.MemberInfo, error)
	PromoteMember(ctx context.Context, memberID uint64) error
	RemoveMember(ctx context.Context, memberID uint64) error
}

type TailscaleManagerRole struct {
	store      TsManagerStore
	httpClient *http.Client
}

type TailscaleStatus struct {
	Peer map[string]*TailscalePeer `json:"Peer"`
	Self *TailscalePeer            `json:"Self"`
}

type TailscalePeer struct {
	HostName     string   `json:"HostName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

func init() {
	RegisterRole("tailscale-manager", func(store any) RoleRunner {
		tsStore, ok := store.(TsManagerStore)
		if !ok {
			panic("store passed to tailscale-manager does not implement TsManagerStore")
		}
		return &TailscaleManagerRole{
			store:      tsStore,
			httpClient: &http.Client{Timeout: 5 * time.Second},
		}
	})
}

func (t *TailscaleManagerRole) Run(ctx context.Context, asg *models.Assignment, sess adapters.SessionWrapper) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Printf("[TailscaleManager] Started on node %s for assignment %s", asg.NodeID, asg.ID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[TailscaleManager] Stopping manager assignment %s", asg.ID)
			return nil
		case <-ticker.C:
			t.reconcileTailnet(ctx)
		}
	}
}

func (t *TailscaleManagerRole) reconcileTailnet(ctx context.Context) {
	activeMembers, err := t.store.GetActiveNodeIDs(ctx)
	if err != nil {
		log.Printf("[TailscaleManager] Failed to get active members: %v", err)
		return
	}
	memberSet := make(map[string]bool)
	for _, id := range activeMembers {
		memberSet[id] = true
	}

	peers, err := t.getTailscalePeers(ctx)
	if err != nil {
		log.Printf("[TailscaleManager] Failed to discover tailscale peers: %v", err)
		return
	}

	for _, peer := range peers {
		if !peer.Online || len(peer.TailscaleIPs) == 0 {
			continue
		}
		if !strings.HasPrefix(peer.HostName, "kaffcloud") {
			continue
		}

		if !memberSet[peer.HostName] {
			log.Printf("[TailscaleManager] Found non-member candidate: %s (IP: %s)", peer.HostName, peer.TailscaleIPs[0])
			t.assimilateNode(ctx, peer)
		}
	}
}

func (t *TailscaleManagerRole) getTailscalePeers(ctx context.Context) ([]*TailscalePeer, error) {
	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run tailscale status: %w", err)
	}

	var status TailscaleStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tailscale status: %w", err)
	}

	var peers []*TailscalePeer
	if status.Self != nil {
		peers = append(peers, status.Self)
	}
	for _, peer := range status.Peer {
		peers = append(peers, peer)
	}
	return peers, nil
}

func (t *TailscaleManagerRole) assimilateNode(ctx context.Context, peer *TailscalePeer) {
	targetIP := peer.TailscaleIPs[0]
	targetPeerURL := fmt.Sprintf("http://%s:2380", targetIP)

	localIP, err := t.getLocalTailscaleIP(ctx)
	if err != nil {
		log.Printf("[TailscaleManager] Failed to resolve local Tailscale IP: %v", err)
		return
	}

	newLearner, currentMembers, err := t.store.AddLearner(ctx, targetPeerURL)
	if err != nil {
		log.Printf("[TailscaleManager] Failed to register learner %s: %v", peer.HostName, err)
		return
	}

	learnerID := newLearner.ID
	defer func() {
		if learnerID != 0 {
			log.Printf("[TailscaleManager] Evicting unintegrated learner %x...", learnerID)
			_ = t.store.RemoveMember(context.Background(), learnerID)
		}
	}()

	var clusterTokens []string
	for _, m := range currentMembers {
		name := m.Name
		if name == "" {
			name = peer.HostName
		}
		if len(m.PeerURLs) > 0 {
			clusterTokens = append(clusterTokens, fmt.Sprintf("%s=%s", name, m.PeerURLs[0]))
		}
	}

	payload := models.AssimilatePayload{
		LeaderName:         config.NodeID(),
		LeaderPeerURL:      fmt.Sprintf("http://%s:2380", localIP),
		EtcdInitialCluster: strings.Join(clusterTokens, ","),
		AssignedIP:         targetIP,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[TailscaleManager] Failed to marshal payload for %s: %v", peer.HostName, err)
		return
	}

	log.Printf("[TailscaleManager] Step 1: Posting /assimilate payload to %s...", peer.HostName)
	assimilateURL := fmt.Sprintf("http://%s:8080/assimilate", targetIP)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, assimilateURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[TailscaleManager] Failed to build request for %s: %v", peer.HostName, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		log.Printf("[TailscaleManager] Assimilate call failed for %s (%s): %v", peer.HostName, targetIP, err)
		return
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[TailscaleManager] /assimilate on %s returned status: %d", peer.HostName, resp.StatusCode)
		return
	}

	time.Sleep(2 * time.Second)

	log.Printf("[TailscaleManager] Step 2: Promoting %s (%x) to full voter status...", peer.HostName, learnerID)
	if err := t.store.PromoteMember(ctx, learnerID); err != nil {
		log.Printf("[TailscaleManager] Failed to promote member %s: %v", peer.HostName, err)
		return
	}

	learnerID = 0

	log.Printf("[TailscaleManager] Step 3: Posting /activate payload to %s...", peer.HostName)
	activateURL := fmt.Sprintf("http://%s:8080/activate", targetIP)

	actReq, err := http.NewRequestWithContext(ctx, http.MethodPost, activateURL, nil)
	if err != nil {
		log.Printf("[TailscaleManager] Failed to build activate request for %s: %v", peer.HostName, err)
		return
	}

	actResp, err := t.httpClient.Do(actReq)
	if err != nil {
		log.Printf("[TailscaleManager] Activate call failed for %s (%s): %v", peer.HostName, targetIP, err)
		return
	}
	_ = actResp.Body.Close()

	if actResp.StatusCode != http.StatusOK {
		log.Printf("[TailscaleManager] /activate on %s returned status: %d", peer.HostName, actResp.StatusCode)
		return
	}

	log.Printf("[TailscaleManager] Successfully assimilated and activated %s into the cluster!", peer.HostName)
}

func (t *TailscaleManagerRole) getLocalTailscaleIP(ctx context.Context) (string, error) {
	if ip := os.Getenv("TAILSCALE_IP"); ip != "" {
		return ip, nil
	}

	cmd := exec.CommandContext(ctx, "tailscale", "ip", "-4")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute 'tailscale ip -4': %w", err)
	}

	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("tailscale returned an empty IP address")
	}

	return ip, nil
}
