package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

var ErrInvalidHostMetadata = errors.New("主机名称或连接地址不能为空或过长")

type Service struct {
	db        *storage.Store
	hosts     *storage.HostStore
	pairingMu sync.Mutex
}

func NewService(ctx context.Context, db *storage.Store) (*Service, error) {
	store, err := storage.NewHostStore(ctx, db)
	if err != nil {
		return nil, err
	}
	return &Service{db: db, hosts: store}, nil
}
func validateHostMetadata(name, endpoint string) (string, error) {
	name = strings.TrimSpace(name)
	endpoint = strings.TrimSpace(endpoint)
	if len(name) == 0 || len(name) > 100 || strings.ContainsAny(name, "\r\n\x00") {
		return "", errors.New("主机名称不合法")
	}
	if len(endpoint) == 0 || len(endpoint) > 255 || strings.ContainsAny(endpoint, "\r\n\x00") {
		return "", errors.New("连接地址不能为空、过长或包含控制字符")
	}
	return name, nil
}
func (s *Service) Register(ctx context.Context, name, endpoint string) (storage.Host, error) {
	endpoint = strings.TrimSpace(endpoint)
	name, err := validateHostMetadata(name, endpoint)
	if err != nil {
		return storage.Host{}, err
	}
	if err := ValidateEndpoint(endpoint); err != nil {
		return storage.Host{}, err
	}
	return s.hosts.Create(ctx, name, endpoint)
}

// Update only edits local display name and endpoint; credentials remain byte-for-byte unchanged.
// A change of Agent identity must use an explicit pairing operation.
func (s *Service) Update(ctx context.Context, id, name, endpoint string) (storage.Host, error) {
	endpoint = strings.TrimSpace(endpoint)
	name, err := validateHostMetadata(name, endpoint)
	if err != nil {
		return storage.Host{}, fmt.Errorf("%w: %v", ErrInvalidHostMetadata, err)
	}
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	return s.hosts.UpdateMetadata(ctx, id, name, endpoint)
}

// Delete only removes a local registration; it does not remove Docker resources
// or revoke Agent grants remotely. Never turn this into a cascade deletion.
func (s *Service) Delete(ctx context.Context, id string) error {
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	return s.hosts.Delete(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]storage.Host, error) { return s.hosts.List(ctx) }
func (s *Service) Pair(ctx context.Context, id, agentID, pin, code string) (storage.Host, error) {
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	h, err := s.hosts.Get(ctx, id)
	if err != nil {
		return storage.Host{}, err
	}
	if err := ValidateIdentity(agentID, pin); err != nil {
		return storage.Host{}, &PairingStageError{Stage: "input", Cause: err}
	}
	if !codeRE.MatchString(code) {
		return storage.Host{}, &PairingStageError{Stage: "input", Cause: errors.New("配对码格式必须为 32 位十六进制")}
	}
	if h.AgentID != "" && (h.AgentID != agentID || h.SPKISHA256 != pin) {
		return storage.Host{}, storage.ErrHostBinding
	}
	if err := s.hosts.AssertAgentNotBoundElsewhere(ctx, id, agentID); err != nil {
		return storage.Host{}, err
	}
	token, err := IdentityAndPair(ctx, h.Endpoint, s.db.ControllerID(), id, agentID, pin, code)
	if err != nil {
		return storage.Host{}, err
	}
	if err := s.hosts.SavePairing(ctx, h, agentID, pin, token); err != nil {
		return storage.Host{}, &PairingStageError{Stage: "save", Cause: err}
	}
	paired, err := s.hosts.Get(ctx, id)
	if err != nil {
		return storage.Host{}, &PairingStageError{Stage: "readback", Cause: err}
	}
	return paired, nil
}
func (s *Service) Query(ctx context.Context, id, kind string) (json.RawMessage, error) {
	h, err := s.hosts.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if h.AgentID == "" {
		return nil, errors.New("主机尚未配对")
	}
	token, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return nil, err
	}
	raw, err := FetchSnapshot(ctx, h, s.db.ControllerID(), token, kind)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.AgentID != h.AgentID {
		return nil, errors.New("Agent 返回了不匹配的资源身份")
	}
	return raw, nil
}
