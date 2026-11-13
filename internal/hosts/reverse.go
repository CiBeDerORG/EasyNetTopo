package hosts

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func (s *Service) OpenDiscovery(ctx context.Context) (string, storage.DiscoveryState, error) {
	return s.hosts.OpenDiscovery(ctx)
}

func (s *Service) CloseDiscovery(ctx context.Context) error {
	return s.hosts.CloseDiscovery(ctx)
}

func (s *Service) Discovery(ctx context.Context) (storage.DiscoveryState, error) {
	return s.hosts.Discovery(ctx)
}

func (s *Service) ListDiscoveries(ctx context.Context) ([]storage.DiscoveredAgent, error) {
	return s.hosts.ListDiscoveries(ctx)
}

func (s *Service) RegisterDiscovery(ctx context.Context, token string, item storage.DiscoveredAgent, certificateDER []byte) (string, string, string, error) {
	state, err := s.hosts.RegisterDiscovery(ctx, token, item, certificateDER)
	if err != nil {
		return "", "", "", err
	}
	if state != "accepted" {
		return state, "", "", nil
	}
	h, err := s.hosts.GetByAgentID(ctx, item.AgentID)
	if err != nil {
		return "", "", "", err
	}
	if !s.hosts.TemporaryActive(h) {
		return "", "", "", storage.ErrTemporaryHostExpired
	}
	agentToken, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return "", "", "", err
	}
	return state, s.db.ControllerID(), agentToken, nil
}

func (s *Service) AcceptDiscovery(ctx context.Context, agentID string) (storage.Host, error) {
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	h, _, err := s.hosts.AcceptDiscovery(ctx, agentID)
	return h, err
}

func (s *Service) ReverseHost(ctx context.Context, agentID string) (storage.Host, error) {
	return s.hosts.GetByAgentID(ctx, agentID)
}

func (s *Service) AuthenticateReverseAgent(ctx context.Context, agentID, token string) (storage.Host, error) {
	h, err := s.hosts.GetByAgentID(ctx, agentID)
	if err != nil {
		return storage.Host{}, err
	}
	if !s.hosts.TemporaryActive(h) {
		return storage.Host{}, storage.ErrTemporaryHostExpired
	}
	expected, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return storage.Host{}, err
	}
	if len(token) != len(expected) || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return storage.Host{}, errors.New("Agent 反向接入授权无效")
	}
	return h, nil
}

var ErrInvalidOverlayIP = errors.New("EasyTier 组网 IP 无效")

func (s *Service) RequestPromotion(ctx context.Context, hostID, overlayIP string) (storage.Host, error) {
	ip := net.ParseIP(strings.TrimSpace(overlayIP))
	if ip == nil || !ip.IsGlobalUnicast() {
		return storage.Host{}, ErrInvalidOverlayIP
	}
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	h, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return storage.Host{}, err
	}
	if h.ConnectionMode != "reverse" || h.RevokedAt != "" ||
		!(s.hosts.TemporaryActive(h) || h.HostType == "permanent") {
		return storage.Host{}, storage.ErrTemporaryHostExpired
	}
	if h.ListenPort < 1 || h.ListenPort > 65535 || (h.Transport != "http" && h.Transport != "https") {
		return storage.Host{}, errors.New("Agent 尚未上报有效的管理端口与传输协议，请先更新 Agent")
	}
	endpoint := fmt.Sprintf("%s://%s", h.Transport, net.JoinHostPort(ip.String(), strconv.Itoa(h.ListenPort)))
	return s.hosts.SetPromotionEndpoint(ctx, hostID, endpoint)
}

// Poll authentication survives the short handoff between becoming permanent
// and the first verified direct connection. The long-term grant is unchanged.
func (s *Service) AuthenticateReversePollAgent(ctx context.Context, agentID, token string) (storage.Host, bool, error) {
	h, err := s.hosts.GetByAgentID(ctx, agentID)
	if err != nil {
		return storage.Host{}, false, err
	}
	promoted := h.HostType == "permanent" && h.ConnectionMode == "direct" && h.Endpoint != ""
	if !promoted && !s.hosts.TemporaryActive(h) &&
		!(h.HostType == "permanent" && h.ConnectionMode == "reverse" && h.RevokedAt == "") {
		return storage.Host{}, false, storage.ErrTemporaryHostExpired
	}
	expected, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return storage.Host{}, false, err
	}
	if len(token) != len(expected) || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return storage.Host{}, false, errors.New("Agent 反向接入授权无效")
	}
	return h, promoted, nil
}

// CompletePromotion verifies the Agent via its pinned TLS identity and
// existing Bearer grant before replacing reverse polling with direct control.
func (s *Service) CompletePromotion(ctx context.Context, agentID, token string) (storage.Host, bool, error) {
	h, promoted, err := s.AuthenticateReversePollAgent(ctx, agentID, token)
	if err != nil || promoted {
		return h, promoted, err
	}
	if h.HostType != "permanent" || h.ConnectionMode != "reverse" || h.Endpoint == "" {
		return h, false, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return h, false, err
	}
	raw, err := client.Host(probeCtx, s.db.ControllerID(), token)
	if err != nil {
		return h, false, err
	}
	var response struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.AgentID != agentID {
		return h, false, errors.New("组网直连响应中的 Agent 身份不匹配")
	}
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	updated, err := s.hosts.CompleteOverlayHandoff(ctx, h.ID, h.Endpoint)
	return updated, err == nil, err
}

func (s *Service) UpdateReversePoll(ctx context.Context, agentID string) (storage.Host, error) {
	return s.hosts.UpdateAgentPoll(ctx, agentID, 10*time.Second)
}

func (s *Service) RevokeTemporary(ctx context.Context, id string) error {
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	return s.hosts.RevokeTemporary(ctx, id)
}

func (s *Service) Get(ctx context.Context, id string) (storage.Host, error) {
	return s.hosts.Get(ctx, id)
}

func (s *Service) TemporaryActive(h storage.Host) bool {
	return s.hosts.TemporaryActive(h)
}

func (s *Service) AuthenticateReverseTaskAgent(ctx context.Context, agentID, token string) (storage.Host, error) {
	h, err := s.hosts.GetByAgentID(ctx, agentID)
	if err != nil {
		return storage.Host{}, err
	}
	if h.HostType != "temporary" || h.ConnectionMode != "reverse" || h.RevokedAt != "" {
		return storage.Host{}, storage.ErrTemporaryHostExpired
	}
	expected, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return storage.Host{}, err
	}
	if len(token) != len(expected) || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return storage.Host{}, errors.New("Agent 反向接入授权无效")
	}
	return h, nil
}

func (s *Service) UpdateReverseReport(ctx context.Context, item storage.DiscoveredAgent, certificateDER []byte) (storage.Host, error) {
	return s.hosts.UpdateAgentReport(ctx, item, certificateDER)
}
