package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestHealthAndMethodRestriction(t *testing.T) {
	handler := testHandler(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("健康接口状态码：%d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["service"] != "easynettopo" {
		t.Fatalf("健康接口返回异常：%v", body)
	}
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("未授权方法应返回 405，得到 %d", post.Code)
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/v1/install", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("未知业务接口不能落入前端页面：%d", unknown.Code)
	}
}

func TestBuiltFrontendAndFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>EasyNetTopo</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := testHandler(t, dir)
	for _, path := range []string{"/", "/hosts"} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK || res.Body.String() != "<h1>EasyNetTopo</h1>" {
			t.Fatalf("SPA 回退失败：%s status=%d", path, res.Code)
		}
	}
}

func TestFrontendMissingDoesNotDisableHealth(t *testing.T) {
	handler := testHandler(t, t.TempDir())
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequest(http.MethodGet, "/", nil))
	if web.Code != http.StatusServiceUnavailable {
		t.Fatalf("未构建前端时应返回 503，得到 %d", web.Code)
	}
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if api.Code != http.StatusOK {
		t.Fatalf("缺少前端构建不应影响 API：%d", api.Code)
	}
}

func testHandler(t *testing.T, dir string) http.Handler {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewHandler(dir, db)
}
