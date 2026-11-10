package hosts

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkDataNeverReturnsConfiguration(t *testing.T) {
	raw := []byte(`{"agent_id":"agent","data":[{"instance_id":"a","result":{"config":"network_secret=secret-value","hostname":"yun","peer_routes":[{"route":{"proxy_cidrs":["192.168.10.0/24"],"network_secret":"secret-value"},"peer":{"conns":[{"stats":{"rx_bytes":9007199254740993},"noise_local_static_pubkey":"secret-value"}]}}]}}]}`)
	filtered, err := filterNetworkData(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(filtered), "secret-value") || strings.Contains(string(filtered), "config") {
		t.Fatal("网络配置或密钥泄漏")
	}
	for _, want := range []string{"192.168.10.0/24", "9007199254740993", "yun"} {
		if !strings.Contains(string(filtered), want) {
			t.Fatalf("丢失展示数据: %s", want)
		}
	}
}

func TestNetworkConnectionDetailsPreserveDisplayFieldsAndStripSecrets(t *testing.T) {
	raw := []byte(`{"data":{"peer_routes":[{"route":{"stun_info":{"udp_nat_type":"FullCone"},"feature_flag":{"is_public_server":true,"avoid_relay_data":true,"unknown":"secret-value"}},"peer":{"default_conn_id":{"part1":1,"part2":2,"part3":3,"part4":4},"conns":[{"conn_id":"00000001-0000-0002-0000-000300000004","loss_rate":0.125,"tunnel":{"tunnel_type":"udp","local_addr":{"url":"udp://[2001:db8::1]:11010"},"remote_addr":{"url":"wss://user:secret-value@[2001:db8::2]:443/secret-value?token=secret-value#secret-value","token":"secret-value"}},"noise_remote_static_pubkey":"secret-value","stats":{"rx_bytes":42,"latency_us":2500}}]}}]}}`)
	filtered, err := filterNetworkData(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(filtered), "secret-value") || strings.Contains(string(filtered), "user") {
		t.Fatal("连接地址或未授权字段泄漏凭据")
	}
	var got struct {
		Data struct {
			PeerRoutes []struct {
				Route struct {
					StunInfo    map[string]any `json:"stun_info"`
					FeatureFlag map[string]any `json:"feature_flag"`
				} `json:"route"`
				Peer struct {
					DefaultConnID map[string]int `json:"default_conn_id"`
					Conns         []struct {
						ConnID   string                     `json:"conn_id"`
						LossRate float64                    `json:"loss_rate"`
						Tunnel   map[string]json.RawMessage `json:"tunnel"`
					} `json:"conns"`
				} `json:"peer"`
			} `json:"peer_routes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(filtered, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.PeerRoutes) != 1 {
		t.Fatal("丢失节点数据")
	}
	pair := got.Data.PeerRoutes[0]
	if pair.Route.StunInfo["udp_nat_type"] != "FullCone" || pair.Route.FeatureFlag["is_public_server"] != true || pair.Route.FeatureFlag["avoid_relay_data"] != true {
		t.Fatal("丢失 NAT 类型或节点标签")
	}
	if pair.Peer.DefaultConnID["part4"] != 4 || len(pair.Peer.Conns) != 1 || pair.Peer.Conns[0].ConnID != "00000001-0000-0002-0000-000300000004" || pair.Peer.Conns[0].LossRate != 0.125 {
		t.Fatal("丢失默认连接、连接标识或丢包率")
	}
	for key, want := range map[string]string{"local_addr": "udp://[2001:db8::1]:11010", "remote_addr": "wss://[2001:db8::2]:443"} {
		var address map[string]string
		if err := json.Unmarshal(pair.Peer.Conns[0].Tunnel[key], &address); err != nil || address["url"] != want {
			t.Fatalf("连接端点未正确保留: %s", key)
		}
	}
}

func TestNetworkEndpointInvalidInput(t *testing.T) {
	for _, value := range []any{nil, "secret", map[string]any{}, map[string]any{"url": 123}, map[string]any{"url": "invalid"}, map[string]any{"url": "tcp://[invalid"}} {
		if got := networkEndpoint(value); got != nil {
			t.Fatal("无效连接地址不应向浏览器输出")
		}
	}
}

func TestNetworkDataUsesPairedAgentAndRejectsMismatch(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	agentID := "11111111-1111-4111-8111-111111111111"
	token := strings.Repeat("b", 64)
	mismatch := false
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/adapters/easytier/node" || r.URL.Query().Get("resource_id") != "compose:easytier" || r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-EHA-Controller-ID") != db.ControllerID() {
			t.Error("查询未遵循配对和固定路径契约")
		}
		w.Header().Set("X-EHA-Agent-ID", agentID)
		id := agentID
		if mismatch {
			id = "other"
		}
		fmt.Fprintf(w, `{"agent_id":%q,"data":{"hostname":"yun","config":"secret"}}`, id)
	}))
	defer server.Close()
	h, err := svc.Register(ctx, "yun", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.hosts.SavePairing(ctx, h, agentID, strings.Repeat("a", 64), token); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.NetworkData(ctx, h.ID, "node"); err == nil || calls != 0 {
		t.Fatal("非入口发出了网络查询")
	}
	if _, err = svc.SetNetworkEntry(ctx, h.ID, true); err != nil {
		t.Fatal(err)
	}
	raw, err := svc.NetworkData(ctx, h.ID, "node")
	if err != nil || !strings.Contains(string(raw), "yun") || strings.Contains(string(raw), "secret") {
		t.Fatalf("采集失败或泄密: %v", err)
	}
	if _, err = svc.NetworkData(ctx, h.ID, "config"); err == nil {
		t.Fatal("允许任意能力")
	}
	mismatch = true
	if _, err = svc.NetworkData(ctx, h.ID, "node"); err == nil {
		t.Fatal("未拒绝身份不匹配")
	}
}
