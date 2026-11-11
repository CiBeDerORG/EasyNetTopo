package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

const openWrtDiscoveryMaxResponse = 2 << 20

type openWrtDiscoveryError struct {
	status  int
	code    string
	message string
}

func (e *openWrtDiscoveryError) Error() string { return e.message }

func discoveryError(status int, code, message string) *openWrtDiscoveryError {
	return &openWrtDiscoveryError{status: status, code: code, message: message}
}

func hasOpenWrtSemantic(kinds []string) bool {
	for _, kind := range kinds {
		if kind == "openwrt" {
			return true
		}
	}
	return false
}

func privateOpenWrtIPv4(value string) bool {
	ip, err := netip.ParseAddr(value)
	if err != nil || !ip.Is4() {
		return false
	}
	return netip.MustParsePrefix("10.0.0.0/8").Contains(ip) ||
		netip.MustParsePrefix("172.16.0.0/12").Contains(ip) ||
		netip.MustParsePrefix("192.168.0.0/16").Contains(ip)
}

// 仅调用用户已配置的设备地址：不接受客户端提供的 URL、路径、对象或 RPC 方法。
func runOpenWrtDiscovery(ctx context.Context, device storage.Device, credential storage.DeviceOpenWrtSecret, kind, action string) (json.RawMessage, *openWrtDiscoveryError) {
	if !privateOpenWrtIPv4(device.IPAddress) {
		return nil, discoveryError(http.StatusConflict, "openwrt_invalid_management_ip", "设备管理 IP 必须为有效的内网 IPv4 地址")
	}
	target, err := url.Parse(credential.URL)
	if err != nil || target == nil || (target.Scheme != "https" && target.Scheme != "http") ||
		target.Hostname() != device.IPAddress || target.Port() == "" || target.Path != "/ubus" ||
		target.User != nil || target.RawQuery != "" || target.Fragment != "" || !strings.HasPrefix(target.Host, device.IPAddress+":") {
		return nil, discoveryError(http.StatusConflict, "openwrt_invalid_endpoint", "设备 RPCD 地址无效")
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 25 * time.Second,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return callOpenWrtDiscoveryRPC(ctx, client, credential, kind, action)
}

// callOpenWrtDiscoveryRPC 可使用隔离的 HTTP 测试客户端，生产入口必须先经过设备身份和地址校验。
func callOpenWrtDiscoveryRPC(ctx context.Context, client *http.Client, credential storage.DeviceOpenWrtSecret, kind, action string) (json.RawMessage, *openWrtDiscoveryError) {
	object := ""
	switch kind {
	case "devices":
		object = "easy-discovery.devices"
	case "adb":
		object = "easy-discovery.adb"
	default:
		return nil, discoveryError(http.StatusNotFound, "discovery_not_found", "不支持的发现类型")
	}
	if action != "list" && action != "scan" {
		return nil, discoveryError(http.StatusMethodNotAllowed, "discovery_method_not_allowed", "不支持的发现操作")
	}
	if credential.Username == "" || credential.Password == "" {
		return nil, discoveryError(http.StatusConflict, "openwrt_discovery_not_configured", "OpenWrt RPCD 账号尚未配置")
	}
	login, err := sendOpenWrtRPC(ctx, client, credential.URL, "00000000000000000000000000000000",
		"session", "login", map[string]any{"username": credential.Username, "password": credential.Password, "timeout": 300})
	if err != nil {
		if err.code == "openwrt_rpcd_permission_denied" {
			err.code = "openwrt_rpcd_auth_failed"
			err.message = "RPCD 账号登录失败，请检查用户名和密钥"
		}
		return nil, err
	}
	var session struct {
		ID string `json:"ubus_rpc_session"`
	}
	if json.Unmarshal(login, &session) != nil || !validRPCDSession(session.ID) {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_auth_failed", "RPCD 登录失败，请检查设备专用账号及密钥")
	}
	return sendOpenWrtRPC(ctx, client, credential.URL, session.ID, object, action, map[string]any{})
}

func validRPCDSession(value string) bool {
	if len(value) != 32 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func sendOpenWrtRPC(ctx context.Context, client *http.Client, endpoint, session, object, method string, args map[string]any) (json.RawMessage, *openWrtDiscoveryError) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "call",
		"params":  []any{session, object, method, args},
	})
	if err != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_request_failed", "构造 RPCD 请求失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_request_failed", "无法创建 RPCD 请求")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return nil, discoveryError(http.StatusGatewayTimeout, "openwrt_rpcd_timeout", "RPCD 请求超时或已取消，请检查网络连通性")
		}
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_unreachable", "无法连接设备 RPCD，请检查 IP、端口、协议以及 HTTPS 证书")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_http_error", "设备 RPCD 未正常响应，请检查协议、端口与服务状态")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, openWrtDiscoveryMaxResponse+1))
	if err != nil || len(body) > openWrtDiscoveryMaxResponse {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_response_too_large", "RPCD 响应读取失败或超过大小限制")
	}
	var response struct {
		Error  json.RawMessage   `json:"error"`
		Result []json.RawMessage `json:"result"`
	}
	if json.Unmarshal(body, &response) != nil || (len(response.Error) > 0 && string(response.Error) != "null") || len(response.Result) < 2 {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_invalid_response", "RPCD 返回的 JSON-RPC 结构无效")
	}
	var ubusCode int
	if json.Unmarshal(response.Result[0], &ubusCode) != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_invalid_response", "RPCD 返回的状态码无效")
	}
	if ubusCode != 0 {
		if object == "session" && method == "login" {
			return nil, discoveryError(http.StatusBadGateway, "openwrt_rpcd_auth_failed", "RPCD 账号登录失败，请检查用户名和密钥")
		}
		switch ubusCode {
		case 6:
			return nil, discoveryError(http.StatusForbidden, "openwrt_rpcd_permission_denied", "RPCD 账号没有对应发现权限，请在 OpenWrt 接口授权中启用")
		case 3, 4, 8:
			return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_unavailable", "OpenWrt 未提供该发现方法，请确认 Easy Discovery 已安装并更新")
		default:
			return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_rpc_error", "OpenWrt 发现接口调用失败")
		}
	}
	if object != "session" {
		var meta struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(response.Result[1], &meta) != nil ||
			(meta.Status != "ok" && meta.Status != "partial" && meta.Status != "error") {
			return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "发现接口未返回有效的业务结果")
		}
	}
	return response.Result[1], nil
}
