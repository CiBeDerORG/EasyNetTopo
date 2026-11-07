package hosts

import (
	"context"
	"encoding/json"
	"errors"
)

// withDockerAgent loads the saved credentials and verifies the Agent identity
// in every response before returning Docker resource or task data.
func (s *Service) withDockerAgent(
	ctx context.Context,
	hostID string,
	request func(*Client, string, string) (json.RawMessage, error),
) (json.RawMessage, error) {
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
	raw, err := request(client, s.db.ControllerID(), token)
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
		return nil, errors.New("Agent Docker 响应身份不匹配")
	}
	return raw, nil
}

func (s *Service) QueryDockerResource(ctx context.Context, hostID, resourceID, detail string, tail int) (json.RawMessage, error) {
	return s.withDockerAgent(ctx, hostID, func(c *Client, controllerID, token string) (json.RawMessage, error) {
		return c.DockerResource(ctx, controllerID, token, resourceID, detail, tail)
	})
}

func (s *Service) StartDockerTask(ctx context.Context, hostID string, task DockerTaskRequest) (json.RawMessage, error) {
	return s.withDockerAgent(ctx, hostID, func(c *Client, controllerID, token string) (json.RawMessage, error) {
		return c.StartDockerTask(ctx, controllerID, token, task)
	})
}

func (s *Service) GetDockerTask(ctx context.Context, hostID, taskID string) (json.RawMessage, error) {
	return s.withDockerAgent(ctx, hostID, func(c *Client, controllerID, token string) (json.RawMessage, error) {
		return c.GetDockerTask(ctx, controllerID, token, taskID)
	})
}

func (s *Service) CancelDockerTask(ctx context.Context, hostID, taskID string) (json.RawMessage, error) {
	return s.withDockerAgent(ctx, hostID, func(c *Client, controllerID, token string) (json.RawMessage, error) {
		return c.CancelDockerTask(ctx, controllerID, token, taskID)
	})
}
