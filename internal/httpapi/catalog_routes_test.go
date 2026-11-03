package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestCatalogRoutesAuthorizationAndSourceValidation(t *testing.T) {
	t.Setenv("EASYNETTOPO_GIT_ALLOWED_HOSTS", "github.com")
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := catalog.NewService(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := db.NewSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(t.TempDir(), db, service)
	call := func(method, path, body string, authorized, mutation bool, origin string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authorized {
			req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
		}
		if mutation {
			req.Header.Set("X-ENT-Request", "1")
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if got := call("GET", "/api/v1/catalog/sources", "", false, false, ""); got.Code != 401 {
		t.Fatal("未登录可读应用源", got.Code)
	}
	body := `{"name":"Deploy","url":"git@github.com:CiBeDerORG/deploy.git","branch":"main"}`
	if got := call("POST", "/api/v1/catalog/sources", body, true, false, ""); got.Code != 403 {
		t.Fatal("无 CSRF 标记可以创建来源", got.Code)
	}
	if got := call("POST", "/api/v1/catalog/sources", body, true, true, "https://malicious.test"); got.Code != 403 {
		t.Fatal("外部 Origin 被允许", got.Code)
	}
	if got := call("POST", "/api/v1/catalog/sources", strings.ReplaceAll(body, "git@github.com:CiBeDerORG/deploy.git", "file:///etc/hosts"), true, true, ""); got.Code != 400 {
		t.Fatal("不安全 Git URL 未拒绝", got.Code)
	}
	created := call("POST", "/api/v1/catalog/sources", body, true, true, "")
	if created.Code != 201 {
		t.Fatalf("合法来源创建失败: %d %s", created.Code, created.Body.String())
	}
	var s storage.CatalogSource
	if err := json.Unmarshal(created.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.ID) != 36 {
		t.Fatal("无稳定 UUID")
	}
	if got := call("POST", "/api/v1/catalog/sources", body, true, true, ""); got.Code != 400 {
		t.Fatal("重复来源未拒绝", got.Code)
	}
	list := call("GET", "/api/v1/catalog/sources", "", true, false, "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), s.ID) {
		t.Fatal("来源读取失败")
	}
	bad := call("POST", "/api/v1/catalog/sources/00000000-0000-4000-8000-000000000000/refresh", "", true, true, "")
	if bad.Code != 404 {
		t.Fatal("未知来源没有返回 404", bad.Code)
	}
	apps := call("GET", "/api/v1/catalog/apps", "", true, false, "")
	if apps.Code != 200 || !strings.Contains(apps.Body.String(), `"install_enabled":false`) {
		t.Fatal("应用安装安全开关异常")
	}

	// 删除来源只影响目标 ID 的记录和目录，不卸载其他来源的应用。
	otherBody := `{"name":"Deploy Dev","url":"https://github.com/CiBeDerORG/deploy.git","branch":"feat/docker-app-store"}`
	otherCreated := call("POST", "/api/v1/catalog/sources", otherBody, true, true, "")
	if otherCreated.Code != 201 {
		t.Fatalf("创建第二个来源失败: %d", otherCreated.Code)
	}
	var other storage.CatalogSource
	if err := json.Unmarshal(otherCreated.Body.Bytes(), &other); err != nil {
		t.Fatal(err)
	}
	for _, source := range []storage.CatalogSource{s, other} {
		if err := db.SaveCatalogSnapshot(context.Background(), source.ID, strings.Repeat("a", 40), []storage.CatalogApp{
			{SourceID: source.ID, AppID: "sub2api", Name: "Sub2API", Revision: strings.Repeat("a", 40)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	cache := filepath.Join(db.DataDir(), "git-cache", s.ID+".git")
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	deletePath := "/api/v1/catalog/sources/" + s.ID
	if got := call("DELETE", deletePath, "", false, true, ""); got.Code != 401 {
		t.Fatal("未认证可删除 Git 来源", got.Code)
	}
	if got := call("DELETE", deletePath, "", true, false, ""); got.Code != 403 {
		t.Fatal("无防跨站标记可删除来源", got.Code)
	}
	if got := call("DELETE", deletePath, "", true, true, "https://malicious.test"); got.Code != 403 {
		t.Fatal("恶意 Origin 可删除来源", got.Code)
	}
	if got := call("DELETE", "/api/v1/catalog/sources/00000000-0000-4000-8000-000000000000", "", true, true, ""); got.Code != 404 {
		t.Fatal("不存在的来源未返回 404", got.Code)
	}
	if got := call("DELETE", deletePath, "", true, true, ""); got.Code != 204 {
		t.Fatal("删除来源失败", got.Code, got.Body.String())
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("删除后 Git 来源缓存仍然存在", err)
	}
	if got := call("DELETE", deletePath, "", true, true, ""); got.Code != 404 {
		t.Fatal("重复删除未返回 404", got.Code)
	}
	remaining := call("GET", "/api/v1/catalog/sources", "", true, false, "")
	if remaining.Code != 200 || strings.Contains(remaining.Body.String(), s.ID) || !strings.Contains(remaining.Body.String(), other.ID) {
		t.Fatal("删除后来源列表未正确更新", remaining.Body.String())
	}
	remainingApps := call("GET", "/api/v1/catalog/apps", "", true, false, "")
	if remainingApps.Code != 200 || strings.Contains(remainingApps.Body.String(), s.ID) || !strings.Contains(remainingApps.Body.String(), other.ID) {
		t.Fatal("删除后应用列表错误地保留或删除了其他来源", remainingApps.Body.String())
	}
	if got := call("POST", "/api/v1/catalog/sources", body, true, true, ""); got.Code != 201 {
		t.Fatal("删除来源后无法重新添加", got.Code)
	}
}
