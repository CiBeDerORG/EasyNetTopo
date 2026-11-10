package hosts

import (
	"context"
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
