package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestConditionalCatalogConfigFieldInputPolicy(t *testing.T) {
	fields := map[string]map[string]any{
		"ROLE": {
			"key": "ROLE", "type": "string", "default": "cloud", "required": true,
			"options": []any{
				map[string]any{"label": "Cloud", "value": "cloud"},
				map[string]any{"label": "Intranet", "value": "intranet"},
			},
		},
		"CLOUD_URL": {
			"key": "CLOUD_URL", "type": "string", "required": true,
			"visible_when": map[string]any{"field": "ROLE", "equals": "cloud"},
		},
		"INTRANET_URL": {
			"key": "INTRANET_URL", "type": "string", "required": true,
			"visible_when": map[string]any{"field": "ROLE", "equals": "intranet"},
		},
		"INTERNAL": {"key": "INTERNAL", "custom_input": false},
	}
	cloudConfig := map[string]string{"ROLE": "cloud", "CLOUD_URL": "https://example.test"}
	if !catalogConfigFieldInputAllowed("CLOUD_URL", fields["CLOUD_URL"], fields, cloudConfig) {
		t.Fatal("cloud role should accept its visible field")
	}
	if catalogConfigFieldInputAllowed("INTRANET_URL", fields["INTRANET_URL"], fields, cloudConfig) {
		t.Fatal("cloud role must reject the hidden intranet field")
	}
	if catalogConfigFieldInputAllowed("INTERNAL", fields["INTERNAL"], fields, cloudConfig) {
		t.Fatal("custom_input=false field must remain protected")
	}
	if missing := catalogRequiredConfigFieldMissing(fields, map[string]string{"ROLE": "cloud"}); missing != "CLOUD_URL" {
		t.Fatalf("cloud role must require its visible URL, got %q", missing)
	}
	if missing := catalogRequiredConfigFieldMissing(fields, cloudConfig); missing != "" {
		t.Fatalf("complete cloud configuration should pass, got %q", missing)
	}

	intranetConfig := map[string]string{"ROLE": "intranet", "INTRANET_URL": "udp://example.test"}
	if !catalogConfigFieldInputAllowed("INTRANET_URL", fields["INTRANET_URL"], fields, intranetConfig) {
		t.Fatal("intranet role should accept its visible field")
	}
	if catalogConfigFieldInputAllowed("CLOUD_URL", fields["CLOUD_URL"], fields, intranetConfig) {
		t.Fatal("intranet role must reject the hidden cloud field")
	}
	if missing := catalogRequiredConfigFieldMissing(fields, map[string]string{"ROLE": "intranet"}); missing != "INTRANET_URL" {
		t.Fatalf("intranet role must require its visible URL, got %q", missing)
	}
	if missing := catalogRequiredConfigFieldMissing(fields, intranetConfig); missing != "" {
		t.Fatalf("complete intranet configuration should pass, got %q", missing)
	}

	defaultConfig := map[string]string{"CLOUD_URL": "https://example.test"}
	if !catalogConfigFieldInputAllowed("CLOUD_URL", fields["CLOUD_URL"], fields, defaultConfig) {
		t.Fatal("a missing selector should resolve through its declared default")
	}
	invalidOption := map[string]string{"ROLE": "unknown"}
	if catalogConfigFieldInputAllowed("ROLE", fields["ROLE"], fields, invalidOption) {
		t.Fatal("selector values outside options must be rejected")
	}
}

func TestEasyNetTopoMarketUpdateOnly(t *testing.T) {
	for _, action := range []string{"install", "uninstall", "force_update"} {
		if marketActionAllowed("easynettopo", action) {
			t.Errorf("EasyNetTopo must reject %s", action)
		}
	}
	for _, action := range []string{"update", "adopt", "status"} {
		if !marketActionAllowed("easynettopo", action) {
			t.Errorf("EasyNetTopo must allow %s (adopt supports automatic legacy linking)", action)
		}
	}
	if !marketActionAllowed("portainer", "install") || !marketActionAllowed("portainer", "uninstall") {
		t.Fatal("Other applications must retain their existing action policy")
	}
}

func TestMacOSNativeInstallUsesPairedAgent(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	catalogSvc, err := catalog.NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	source, err := db.CreateCatalogSource(ctx, "Deploy", "https://github.com/example/deploy.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("a", 40)
	app := storage.CatalogApp{SourceID: source.ID, AppID: "maestro", Revision: revision, NativeInstall: "macos_homebrew"}
	if err := db.SaveCatalogSnapshot(ctx, source.ID, revision, []storage.CatalogApp{app}); err != nil {
		t.Fatal(err)
	}

	const id = "11111111-1111-4111-8111-111111111111"
	pin, token := strings.Repeat("b", 64), strings.Repeat("c", 64)
	platform := "macos"
	var capabilities []string
	installs := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-EHA-Agent-ID", id)
		switch r.URL.Path {
		case "/v1/identity":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"agent_id": id, "tls_spki_sha256": pin, "protocol_version": 1,
				"platform": platform, "native_installers": capabilities,
			})
		case "/v1/apps/install":
			installs++
			var payload map[string]string
			if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-EHA-Controller-ID") != db.ControllerID() ||
				json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload) != 3 ||
				payload["native_installer"] != "maestro" || payload["source_id"] != source.ID || payload["app_id"] != "maestro" {
				t.Error("向 Agent 转发的固定原生动作或配对身份不匹配")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"result": "running"})
		default:
			t.Errorf("原生安装意外调用 %s", r.URL.Path)
		}
	}))
	defer server.Close()
	hostSvc, err := hosts.NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	host, err := hostSvc.Register(ctx, "Mac", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	hostStore, err := storage.NewHostStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if err := hostStore.SavePairing(ctx, host, id, pin, token); err != nil {
		t.Fatal(err)
	}
	call := func(rev string, config map[string]string) int {
		raw, err := json.Marshal(map[string]any{"source_id": source.ID, "app_id": app.AppID, "revision": rev, "config": config})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		appAction(response, req, host.ID, "install", hostSvc, catalogSvc)
		return response.Code
	}
	if call(revision, nil) != 422 {
		t.Fatal("旧 Agent 不应接收原生安装请求")
	}
	capabilities = []string{"maestro"}
	platform = "linux"
	if call(revision, nil) != 422 {
		t.Fatal("Linux 不应接收 macOS 安装请求")
	}
	platform = "macos"
	if call("旧版本", nil) != 409 || call(revision, map[string]string{"formula": "另一个配方"}) != 422 {
		t.Fatal("未拒绝过期快照或自定义参数")
	}
	if call(revision, nil) != 200 || installs != 1 {
		t.Fatal("安装请求应恰好向目标 Mac 发送一次")
	}
	if err := db.SaveCatalogSnapshot(ctx, source.ID, revision, nil); err != nil {
		t.Fatal(err)
	}
	if call(revision, nil) != 422 || installs != 1 {
		t.Fatal("已下架的 Maestro 被重复安装")
	}
}
