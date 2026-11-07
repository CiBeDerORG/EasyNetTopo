package hosts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestManifestVersionNegotiation(t *testing.T) {
	for _, supported := range [][]int{nil, {1}, {1, 2}} {
		t.Run([]string{"未声明能力", "仅支持v1", "支持v2"}[len(supported)], func(t *testing.T) {
			const id = "11111111-1111-4111-8111-111111111111"
			pin := strings.Repeat("a", 64)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-EHA-Agent-ID", id)
				if r.URL.Path != "/v1/identity" {
					t.Error("能力检查不应提交安装包")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": id, "tls_spki_sha256": pin, "protocol_version": 1, "app_manifest_versions": supported})
			}))
			defer server.Close()
			ctx := context.Background()
			db, err := storage.Open(filepath.Join(t.TempDir(), "state", "app.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			service, err := NewService(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			host, err := service.Register(ctx, "能力测试", server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.hosts.SavePairing(ctx, host, id, pin, strings.Repeat("b", 64)); err != nil {
				t.Fatal(err)
			}
			if err := service.RequireManifestVersion(ctx, host.ID, 1); err != nil {
				t.Fatal(err)
			}
			err = service.RequireManifestVersion(ctx, host.ID, 2)
			if (err == nil) != (len(supported) == 2) {
				t.Fatalf("v2 能力判断错误：%v", err)
			}
		})
	}
}
