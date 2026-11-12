package hosts

import (
	"context"
	"crypto/subtle"
	"errors"
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

func (s *Service) UpdateReversePoll(ctx context.Context, agentID string) (storage.Host, error) {
	return s.hosts.UpdateAgentPoll(ctx, agentID, 10*time.Second)
}

func (s *Service) RevokeTemporary(ctx context.Context, id string) error {
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	return s.hosts.RevokeTemporary(ctx, id)
}
