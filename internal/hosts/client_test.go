package hosts

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

const mockAgentID = "11111111-1111-4111-8111-111111111111"

func mockAgent(t *testing.T) (endpoint, pin string, closeFn func()) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "EasyHostAgent " + mockAgentID},
		NotBefore:    time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:        true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(parsed.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(pub)
	pin = hex.EncodeToString(digest[:])
	token := strings.Repeat("a", 64)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-EHA-Agent-ID", mockAgentID)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/identity":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"agent_id": mockAgentID, "tls_spki_sha256": pin, "protocol_version": 1, "hostname": "Test Agent",
			})
		case "/v1/pairings":
			if r.Method != "POST" {
				http.Error(w, "method", 405)
				return
			}
			var v struct {
				ControllerID string `json:"controller_id"`
				DeviceID     string `json:"device_id"`
				PairingCode  string `json:"pairing_code"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				http.Error(w, "invalid", 400)
				return
			}
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"agent_id": mockAgentID, "controller_id": v.ControllerID,
				"device_id": v.DeviceID, "access_token": token,
			})
		case "/v1/docker/resources/compose:test/logs":
			if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-EHA-Agent-ID") != mockAgentID {
				http.Error(w, "unauthorized", 401)
				return
			}
			if r.URL.Query().Get("tail") != "20" {
				http.Error(w, "bad tail", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": mockAgentID, "resource_id": "compose:test", "text": "sample log"})
		case "/v1/docker/resources/compose:test/stats":
			if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-EHA-Agent-ID") != mockAgentID {
				http.Error(w, "unauthorized", 401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": mockAgentID, "resource_id": "compose:test", "complete": true, "containers": []any{}})
		case "/v1/host", "/v1/docker/inventory":
			if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-EHA-Agent-ID") != mockAgentID || r.Header.Get("X-EHA-Controller-ID") == "" {
				http.Error(w, "unauthorized", 401)
				return
			}
			if r.URL.Path == "/v1/host" {
				_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": mockAgentID, "hostname": "test-host", "software_root": "/software"})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": mockAgentID, "complete": false, "status": "error", "resources": []any{}})
			}
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	return strings.TrimPrefix(server.URL, "https://"), pin, server.Close
}

func TestEndToEndPinnedPairAndPrivateToken(t *testing.T) {
	endpoint, pin, done := mockAgent(t)
	defer done()
	s, err := storage.Open(filepath.Join(t.TempDir(), "data", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	svc, err := NewService(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	host, err := svc.Register(context.Background(), "FnOS", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	code := strings.Repeat("b", 32)
	if _, err := svc.Pair(context.Background(), host.ID, mockAgentID, strings.Repeat("f", 64), code); err == nil {
		t.Fatal("伪造 TLS 指纹也配对成功")
	}
	valid, err := svc.Pair(context.Background(), host.ID, mockAgentID, pin, code)
	if err != nil {
		t.Fatal(err)
	}
	if valid.AgentID != mockAgentID || valid.SPKISHA256 != pin {
		t.Fatal("持久化 Agent 身份错误")
	}
	encoded, _ := json.Marshal(valid)
	if strings.Contains(string(encoded), strings.Repeat("a", 64)) {
		t.Fatal("主机 JSON 泄露 Agent Token")
	}
	raw, err := svc.Query(context.Background(), host.ID, "host")
	if err != nil || !strings.Contains(string(raw), "test-host") {
		t.Fatalf("主机查询失败: %s %v", raw, err)
	}
	inventory, err := svc.Query(context.Background(), host.ID, "docker")
	if err != nil || !strings.Contains(string(inventory), `"complete":false`) {
		t.Fatalf("不完整 Docker 查询未保留原始状态: %v", err)
	}
	logs, err := svc.QueryDockerResource(context.Background(), host.ID, "compose:test", "logs", 20)
	if err != nil || !strings.Contains(string(logs), "sample log") {
		t.Fatalf("Docker 日志代理没有保留配对验证：%s %v", logs, err)
	}
	stats, err := svc.QueryDockerResource(context.Background(), host.ID, "compose:test", "stats", 100)
	if err != nil || !strings.Contains(string(stats), `"complete":true`) {
		t.Fatalf("Docker 资源统计代理失败：%s %v", stats, err)
	}
	for _, invalid := range []string{"../etc/passwd", "compose:../escape", "compose:test/escape", "compose:test?anything=1"} {
		if _, err := svc.QueryDockerResource(context.Background(), host.ID, invalid, "stats", 100); err == nil {
			t.Fatalf("允许路径注入资源标识：%q", invalid)
		}
	}
	if _, err := svc.QueryDockerResource(context.Background(), host.ID, "compose:test", "logs", 501); err == nil {
		t.Fatal("日志查询允许超过 500 行")
	}
	if _, err := svc.Pair(context.Background(), host.ID, "22222222-2222-4222-8222-222222222222", pin, code); err == nil {
		t.Fatal("允许已绑定主机更改 Agent ID")
	}
	if _, err := svc.Register(context.Background(), "other", "file:///etc/passwd"); err == nil {
		t.Fatal("非法主机地址被登记")
	}
}

func TestEndpointAndFingerprintValidation(t *testing.T) {
	for _, addr := range []string{"192.168.1.3:7014", "127.0.0.1:7014", "[::1]:7014"} {
		if err := ValidateEndpoint(addr); err != nil {
			t.Fatal("合法地址错误", addr, err)
		}
	}
	for _, addr := range []string{"169.254.169.254:80", "0.0.0.0:7014", "localhost:7014", "https://127.0.0.1:7014", "127.0.0.1:0", "[ff02::1]:7014"} {
		if ValidateEndpoint(addr) == nil {
			t.Error("不合法地址被接受", addr)
		}
	}
	if err := ValidateIdentity(mockAgentID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if ValidateIdentity("untrusted-agent", strings.Repeat("a", 64)) == nil {
		t.Fatal("非法 Agent ID 被允许")
	}
}
