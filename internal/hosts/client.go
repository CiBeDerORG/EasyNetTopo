package hosts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

var uuidRE = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
var hashRE = regexp.MustCompile("^[0-9a-f]{64}$")
var codeRE = regexp.MustCompile("^[0-9a-f]{32}$")
var tokenRE = regexp.MustCompile("^[0-9a-f]{64}$")

// Agent 自更新错误使用固定的协议状态，不向 UI 透传远端任意错误正文。
var (
	ErrAgentUpdateUnsupported  = errors.New("Agent 版本尚不支持远程自更新")
	ErrAgentUpdateEnvironment  = errors.New("Agent 原生自更新环境或 systemd 调度不可用")
	ErrAgentUpdateUnauthorized = errors.New("Agent 已拒绝当前控制端授权")
	ErrAgentUpdateBusy         = errors.New("Agent 正在处理其他应用操作")
)

// parseEndpoint 兼容原 IP:port（默认 HTTPS）以及显式 HTTP/HTTPS 协议。
func validEndpointHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 || strings.Contains(host, "%") {
		return false
	}
	allNumeric := true
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
				(r < '0' || r > '9') && r != '-' {
				return false
			}
			if r < '0' || r > '9' {
				allNumeric = false
			}
		}
	}
	return !allNumeric
}

// parseEndpoint accepts IP:port or DNS hostname:port. Missing schemes retain
// the legacy HTTPS default; explicit HTTP remains available for trusted LANs.
func parseEndpoint(value string) (string, error) {
	if len(value) > 255 || strings.TrimSpace(value) != value ||
		strings.ContainsAny(value, "\r\n\t\x00") {
		return "", errors.New("主机地址必须为 IP:port、主机名:port 或 http(s)://主机:port")
	}
	scheme, address := "https", value
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", errors.New("主机地址 URL 格式不合法")
		}
		scheme = strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			return "", errors.New("只允许 HTTP 或 HTTPS")
		}
		if parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" ||
			(parsed.Path != "" && parsed.Path != "/") ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", errors.New("Agent 地址不能包含账号、路径、查询参数或片段")
		}
		address = parsed.Host
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", errors.New("Agent 地址必须包含主机名或 IP 及端口")
	}
	if strings.Contains(host, "%") {
		return "", errors.New("不支持带区域标识的主机地址")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "", errors.New("不允许未指定、多播或链路本地 IP 地址")
		}
	} else if !validEndpointHostname(host) {
		return "", errors.New("主机名格式不合法")
	}
	if port == "" || len(port) > 5 {
		return "", errors.New("端口不合法")
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return "", errors.New("端口必须为数字")
		}
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("端口必须在 1 至 65535 之间")
	}
	return scheme + "://" + net.JoinHostPort(host, port), nil
}
func ValidateEndpoint(value string) error {
	_, err := parseEndpoint(value)
	return err
}
func ValidateIdentity(agentID, pin string) error {
	if !uuidRE.MatchString(agentID) || !hashRE.MatchString(pin) {
		return errors.New("请填写本地 Agent 终端显示的 UUID 和 SHA-256 公钥指纹")
	}
	return nil
}

// pinnedClient 不采用系统 CA 或不受信任的 identity JSON 建立信任，
// 而是在 TLS 握手时匹配用户从可信 Agent 终端核对的 SubjectPublicKeyInfo。
func pinnedClient(agentID, pin string) (*http.Client, error) {
	if err := ValidateIdentity(agentID, pin); err != nil {
		return nil, err
	}
	want, err := hex.DecodeString(pin)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // 配合下面的强制 SPKI/Agent 身份核验，绝不无条件跳过校验。
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) != 1 {
				return errors.New("Agent TLS 证书链不符合契约")
			}
			leaf := cs.PeerCertificates[0]
			now := time.Now()
			if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
				return errors.New("Agent TLS 证书已失效")
			}
			if leaf.Subject.CommonName != "EasyHostAgent "+agentID {
				return errors.New("Agent TLS 身份不匹配")
			}
			key, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(key)
			if subtle.ConstantTimeCompare(want, digest[:]) != 1 {
				return errors.New("Agent TLS 公钥指纹不匹配")
			}
			return nil
		},
	}
	transport := &http.Transport{Proxy: nil, TLSClientConfig: tlsConfig, MaxIdleConns: 2, IdleConnTimeout: 10 * time.Second,
		DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

type Identity struct {
	AppManifestVersions []int  `json:"app_manifest_versions"`
	AgentID             string `json:"agent_id"`
	SPKISHA256          string `json:"tls_spki_sha256"`
	ProtocolVersion     int    `json:"protocol_version"`
	Hostname            string `json:"hostname"`
	Platform            string `json:"platform"`
	Arch                string `json:"arch"`
	Version             string `json:"version"`
}
type PairingResult struct {
	AgentID      string `json:"agent_id"`
	ControllerID string `json:"controller_id"`
	DeviceID     string `json:"device_id"`
	AccessToken  string `json:"access_token"`
}
type Client struct {
	http     *http.Client
	endpoint string
	agentID  string
}

func NewClient(endpoint, agentID, pin string) (*Client, error) {
	baseURL, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	if err := ValidateIdentity(agentID, pin); err != nil {
		return nil, err
	}
	var client *http.Client
	if strings.HasPrefix(baseURL, "http://") {
		// 仅显式选择 HTTP 的来源允许明文连接；禁用系统代理和跨主机重定向。
		client = &http.Client{
			Transport: &http.Transport{Proxy: nil, MaxIdleConns: 2, IdleConnTimeout: 10 * time.Second,
				DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext},
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	} else {
		client, err = pinnedClient(agentID, pin)
		if err != nil {
			return nil, err
		}
	}
	return &Client{http: client, endpoint: baseURL, agentID: agentID}, nil
}
func (c *Client) request(ctx context.Context, method, path string, body any, controllerID, token string) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-EHA-Controller-ID", controllerID)
		req.Header.Set("X-EHA-Agent-ID", c.agentID)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法安全连接 Agent: %w", err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("X-EHA-Agent-ID"); got != c.agentID {
		return nil, errors.New("响应 Agent ID 不匹配")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if strings.HasPrefix(path, "/v1/host/maintenance/") {
			// 旧版本 Agent 缺少路由时会返回 404。按固定 HTTP 状态分类，不转发响应正文。
			switch res.StatusCode {
			case http.StatusNotFound, http.StatusMethodNotAllowed:
				return nil, ErrAgentUpdateUnsupported
			case http.StatusServiceUnavailable:
				return nil, ErrAgentUpdateEnvironment
			case http.StatusUnauthorized, http.StatusForbidden:
				return nil, ErrAgentUpdateUnauthorized
			case http.StatusConflict:
				return nil, ErrAgentUpdateBusy
			}
		}
		if path == "/v1/pairings" {
			// Agent 错误码是固定协议字段，拒绝透传远程自由文本或包含凭据的响应。
			var failure struct {
				Code string `json:"code"`
			}
			_ = json.NewDecoder(io.LimitReader(res.Body, 2048)).Decode(&failure)
			switch failure.Code {
			case "invalid_pairing_code", "pairing_window_closed", "pairing_attempts_exhausted", "pairing_unavailable", "invalid_identity":
				// 仅允许 Agent 契约中已知的错误码进入诊断链路。
			default:
				failure.Code = "unknown"
			}
			return nil, &PairingHTTPError{Status: res.StatusCode, Code: failure.Code}
		}
		if strings.HasPrefix(path, "/v1/adapters/easytier/port-forwards") {
			var failure struct {
				Code string `json:"code"`
			}
			_ = json.NewDecoder(io.LimitReader(res.Body, 2048)).Decode(&failure)
			if failure.Code == "invalid_request" || failure.Code == "invalid_port_forward" ||
				failure.Code == "driver_disabled" || failure.Code == "port_forward_failed" {
				return nil, &AdapterHTTPError{Status: res.StatusCode, Code: failure.Code}
			}
		}
		if strings.HasPrefix(path, "/v1/apps/") {
			var failure struct{ Code, Message string }
			if json.NewDecoder(io.LimitReader(res.Body, 2048)).Decode(&failure) == nil && (failure.Code == "app_action" || failure.Code == "app_status") {
				return nil, &AppError{Status: res.StatusCode, Message: failure.Message}
			}
		}
		return nil, fmt.Errorf("Agent 返回 HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 2*1024*1024 {
		return nil, errors.New("Agent 响应超过上限")
	}
	if !json.Valid(raw) {
		return nil, errors.New("Agent JSON 响应不合法")
	}
	return raw, nil
}
func (c *Client) Identity(ctx context.Context) (Identity, error) {
	raw, err := c.request(ctx, http.MethodGet, "/v1/identity", nil, "", "")
	if err != nil {
		return Identity{}, err
	}
	var x Identity
	if err := json.Unmarshal(raw, &x); err != nil {
		return x, err
	}
	if x.AgentID != c.agentID || x.ProtocolVersion != 1 {
		return x, errors.New("Agent 身份或协议版本不一致")
	}
	return x, nil
}
func (c *Client) Pair(ctx context.Context, controllerID, hostID, code string) (PairingResult, error) {
	if !uuidRE.MatchString(controllerID) || !uuidRE.MatchString(hostID) || !codeRE.MatchString(code) {
		return PairingResult{}, errors.New("配对参数不合法")
	}
	raw, err := c.request(ctx, http.MethodPost, "/v1/pairings", map[string]string{
		"controller_id": controllerID, "device_id": hostID, "pairing_code": code,
	}, "", "")
	if err != nil {
		return PairingResult{}, err
	}
	var x PairingResult
	if err := json.Unmarshal(raw, &x); err != nil {
		return x, err
	}
	if x.AgentID != c.agentID || x.ControllerID != controllerID || x.DeviceID != hostID || !tokenRE.MatchString(x.AccessToken) {
		return PairingResult{}, &PairingStageError{Stage: "response", Cause: errors.New("Agent 配对响应身份不合法")}
	}
	return x, nil
}
func (c *Client) Host(ctx context.Context, controllerID, token string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, "/v1/host", nil, controllerID, token)
}
func (c *Client) Inventory(ctx context.Context, controllerID, token string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, "/v1/docker/inventory", nil, controllerID, token)
}

// IdentityAndPair 在握手验证通过后仍检查 Agent 自报的 SPKI，与用户输入保持一致。
func IdentityAndPair(ctx context.Context, endpoint, controllerID, hostID, agentID, pin, code string) (string, error) {
	client, err := NewClient(endpoint, agentID, pin)
	if err != nil {
		return "", &PairingStageError{Stage: "identity", Cause: err}
	}
	identity, err := client.Identity(ctx)
	if err != nil {
		return "", &PairingStageError{Stage: "identity", Cause: err}
	}
	if identity.SPKISHA256 != pin {
		return "", &PairingStageError{Stage: "identity", Cause: errors.New("TLS 公钥与 Agent 身份响应的指纹不一致")}
	}
	result, err := client.Pair(ctx, controllerID, hostID, code)
	if err != nil {
		var stage *PairingStageError
		if errors.As(err, &stage) {
			return "", err
		}
		return "", &PairingStageError{Stage: "exchange", Cause: err}
	}
	return result.AccessToken, nil
}

func FetchSnapshot(ctx context.Context, h storage.Host, controllerID, token, kind string) (json.RawMessage, error) {
	client, err := NewClient(h.Endpoint, h.AgentID, h.SPKISHA256)
	if err != nil {
		return nil, err
	}
	if kind == "host" {
		return client.Host(ctx, controllerID, token)
	}
	if kind == "docker" {
		return client.Inventory(ctx, controllerID, token)
	}
	if kind == "adapters" {
		return client.request(ctx, http.MethodGet, "/v1/adapters/types", nil, controllerID, token)
	}
	return nil, errors.New("未知 Agent 查询类型")
}

// AdapterHTTPError returns only allowlisted errors from the paired Agent.
// Error response bodies and arbitrary remote messages are never forwarded.
type AdapterHTTPError struct {
	Status int
	Code   string
}
func (e *AdapterHTTPError) Error() string { return "EasyTier Agent adapter error: " + e.Code }
