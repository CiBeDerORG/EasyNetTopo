package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type AppError struct {
	Status  int
	Message string
}

func (e *AppError) Error() string { return e.Message }

// AppAction 仅允许经现有主机绑定和固定 TLS 身份发送这三个应用操作。
func (s *Service) AppAction(ctx context.Context, hostID, name string, body any) (json.RawMessage, error) {
	if name != "status" && name != "install" && name != "update" && name != "uninstall" && name != "adopt" && name != "force_update" {
		return nil, errors.New("未知应用动作")
	}
	h, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return nil, err
	}
	token, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return nil, err
	}
	return client.request(ctx, http.MethodPost, "/v1/apps/"+name, body, s.db.ControllerID(), token)
}

// RequireManifestVersion 在传包之前检查目标能力，旧 Agent 不会误执行新配置契约。
func (s *Service) RequireManifestVersion(ctx context.Context, hostID string, version int) error {
	if version <= 1 {
		return nil
	}
	h, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return err
	}
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return err
	}
	identity, err := client.Identity(ctx)
	if err != nil {
		return err
	}
	if identity.AgentID != h.AgentID || identity.SPKISHA256 != h.SPKISHA256 {
		return errors.New("Agent 身份不匹配")
	}
	for _, supported := range identity.AppManifestVersions {
		if supported == version {
			return nil
		}
	}
	return errors.New("目标 Agent 尚不支持此应用配置版本，请先更新 Agent")
}
