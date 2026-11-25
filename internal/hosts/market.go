package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
)

type AppError struct {
	Status  int
	Code    string
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

// 校验配对时记录的 Agent 身份，能力声明仅用于选择协议。
func (s *Service) marketIdentity(ctx context.Context, hostID string) (Identity, error) {
	h, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return Identity{}, err
	}
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return Identity{}, err
	}
	identity, err := client.Identity(ctx)
	if err != nil {
		return Identity{}, err
	}
	if identity.AgentID != h.AgentID || identity.SPKISHA256 != h.SPKISHA256 {
		return Identity{}, errors.New("Agent 身份不匹配")
	}
	return identity, nil
}

func (s *Service) RequireNativeInstaller(ctx context.Context, hostID, appID string) error {
	identity, err := s.marketIdentity(ctx, hostID)
	if err != nil {
		return errors.New("无法核验已配对主机的原生安装能力")
	}
	if identity.Platform != "macos" {
		return errors.New("此原生安装器仅适用于 Mac 主机")
	}
	if !slices.Contains(identity.NativeInstallers, appID) {
		return errors.New("请先更新此 Mac 上的 EasyHostAgent，再安装应用")
	}
	return nil
}

// RequireManifestVersion 在传包之前检查目标能力，旧 Agent 不会误执行新配置契约。
func (s *Service) RequireManifestVersion(ctx context.Context, hostID string, version int) error {
	if version <= 1 {
		return nil
	}
	identity, err := s.marketIdentity(ctx, hostID)
	if err != nil {
		return err
	}
	for _, supported := range identity.AppManifestVersions {
		if supported == version {
			return nil
		}
	}
	return errors.New("目标 Agent 尚不支持此应用配置版本，请先更新 Agent")
}
