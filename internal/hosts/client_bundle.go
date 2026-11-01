package hosts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const MaxBundleBytes = 64 << 20

type BundleReceipt struct {
	AgentID       string `json:"agent_id"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	AlreadyExists bool   `json:"already_exists"`
}

func validBundleAuth(controllerID, token string) bool {
	return uuidRE.MatchString(controllerID) && tokenRE.MatchString(token)
}
func (c *Client) bundleRequest(ctx context.Context, method, digest, controllerID, token string, body []byte) (*http.Response, error) {
	if !hashRE.MatchString(digest) || !validBundleAuth(controllerID, token) {
		return nil, errors.New("Bundle 摘要或 Agent 授权凭据格式不合法")
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+"/v1/bundles/"+digest, reader)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-EHA-Controller-ID", controllerID)
	req.Header.Set("X-EHA-Agent-ID", c.agentID)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法通过 TLS 安全连接 Agent Bundle 服务: %w", err)
	}
	if res.Header.Get("X-EHA-Agent-ID") != c.agentID {
		res.Body.Close()
		return nil, errors.New("Bundle 响应 Agent 身份不匹配")
	}
	return res, nil
}

// HasBundle requests an authenticated, pinned-TLS HEAD and never treats a
// remote error or missing authorization as "no bundle".
func (c *Client) HasBundle(ctx context.Context, controllerID, token, digest string) (bool, error) {
	res, err := c.bundleRequest(ctx, http.MethodHead, digest, controllerID, token, nil)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("Agent Bundle 状态查询失败：HTTP %d", res.StatusCode)
	}
}

// UploadBundle only transports an immutable ZIP's raw bytes. It does not
// approve, install, expand, execute or dispatch application tasks.
func (c *Client) UploadBundle(ctx context.Context, controllerID, token string, archive []byte) (BundleReceipt, error) {
	if len(archive) == 0 || len(archive) > MaxBundleBytes {
		return BundleReceipt{}, errors.New("Bundle 长度必须为 1 至 64 MiB")
	}
	hash := sha256.Sum256(archive)
	digest := hex.EncodeToString(hash[:])
	res, err := c.bundleRequest(ctx, http.MethodPut, digest, controllerID, token, archive)
	if err != nil {
		return BundleReceipt{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return BundleReceipt{}, fmt.Errorf("Agent 拒绝接收部署包：HTTP %d", res.StatusCode)
	}
	var receipt BundleReceipt
	decoder := json.NewDecoder(io.LimitReader(res.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return BundleReceipt{}, errors.New("Agent Bundle 回执无效")
	}
	if receipt.AgentID != c.agentID || receipt.SHA256 != digest || receipt.SizeBytes != int64(len(archive)) ||
		receipt.AlreadyExists != (res.StatusCode == http.StatusOK) {
		return BundleReceipt{}, errors.New("Agent Bundle 回执与原始字节或身份不一致")
	}
	return receipt, nil
}
