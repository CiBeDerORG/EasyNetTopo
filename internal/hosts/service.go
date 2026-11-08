package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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

// SetAdapter 仅转发管理端已配对主机的开关设置；不在控制端重复保存 Driver 状态。
func (s *Service) SetAdapter(ctx context.Context, id, name string, enabled bool) (json.RawMessage, error) {
	if name != "easytier" {
		return nil, errors.New("当前不支持的 Driver")
	}
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
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return nil, err
	}
	raw, err := client.request(ctx, "PUT", "/v1/adapters/types/"+name,
		map[string]bool{"enabled": enabled}, s.db.ControllerID(), token)
	if err != nil {
		return nil, err
	}
	var reply struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil || reply.AgentID != h.AgentID {
		return nil, errors.New("Agent Driver 响应身份不匹配")
	}
	return raw, nil
}

type PortForwardChange struct {
	ResourceID      string `json:"resource_id"`
	InstanceID      string `json:"instance_id,omitempty"`
	InstanceName    string `json:"instance_name,omitempty"`
	Protocol        string `json:"protocol"`
	Listen          string `json:"listen"`
	Destination     string `json:"destination,omitempty"`
	AllowPublicBind bool   `json:"allow_public_bind,omitempty"`
}
// EasyTierPortForwards proxies fixed read/write operations through the existing
// pinned identity, encrypted pairing credential and controller authorization.
func (s *Service) EasyTierPortForwards(ctx context.Context, hostID, method, instanceID, instanceName string, change *PortForwardChange) (json.RawMessage, error) {
	h, err := s.hosts.Get(ctx, hostID)
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
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return nil, err
	}
	path := "/v1/adapters/easytier/port-forwards"
	var body any
	switch method {
	case "GET":
		query := url.Values{"resource_id": {"compose:easytier"}}
		if instanceID != "" {
			query.Set("instance_id", instanceID)
		}
		if instanceName != "" {
			query.Set("instance_name", instanceName)
		}
		path += "?" + query.Encode()
	case "POST", "DELETE":
		if change == nil {
			return nil, errors.New("缺少端口转发请求")
		}
		body = change
	default:
		return nil, errors.New("不允许的端口转发操作")
	}
	raw, err := client.request(ctx, method, path, body, s.db.ControllerID(), token)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.AgentID != h.AgentID {
		return nil, errors.New("Agent 端口转发结果身份不匹配")
	}
	return raw, nil
}