package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// NativeAgentUpdate 只允许已配对主机上两种固定自更新动作；不传远端 Shell 参数。
func (s *Service) NativeAgentUpdate(ctx context.Context, hostID, action string) (json.RawMessage, error) {
	if action != "update" && action != "force-update" {
		return nil, errors.New("不支持的 Agent 自更新动作")
	}
	h, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if h.AgentID == "" {
		return nil, errors.New("主机尚未配对")
	}
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return nil, err
	}
	token, err := s.hosts.Credentials(ctx, h)
	if err != nil {
		return nil, err
	}
	return client.request(ctx, http.MethodPost, "/v1/host/maintenance/"+action, nil, s.db.ControllerID(), token)
}
