package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestHostAPIAuthorizationAndRegistration(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "data", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc, err := hosts.NewService(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithHosts(t.TempDir(), db, nil, svc)
	token, _, err := db.NewSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := func(method, path, body string, authorized, csrf bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if authorized {
			r.AddCookie(&http.Cookie{Name: authCookie, Value: token})
		}
		if csrf {
			r.Header.Set("X-ENT-Request", "1")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if got := req("GET", "/api/v1/hosts", "", false, false); got.Code != 401 {
		t.Fatal("未登录读取主机", got.Code)
	}
	body := `{"name":"FnOS","endpoint":"127.0.0.1:7014"}`
	if got := req("POST", "/api/v1/hosts", body, true, false); got.Code != 403 {
		t.Fatal("未限制 CSRF", got.Code)
	}
	if got := req("POST", "/api/v1/hosts", body, true, true); got.Code != 201 {
		t.Fatal("合法主机登记失败", got.Code, got.Body.String())
	}
	list := req("GET", "/api/v1/hosts", "", true, false)
	if list.Code != 200 || !strings.Contains(list.Body.String(), "FnOS") {
		t.Fatal("主机列表异常")
	}
	if got := req("POST", "/api/v1/hosts", `{"name":"invalid","endpoint":"169.254.169.254:80"}`, true, true); got.Code != 400 {
		t.Fatal("允许元数据地址", got.Code)
	}
	var id string
	hs, err := svc.List(context.Background())
	if err != nil || len(hs) != 1 {
		t.Fatal("主机列表错误", err)
	}
	id = hs[0].ID
	if got := req("GET", "/api/v1/hosts/"+id+"/docker/resources/compose:test/logs", "", false, false); got.Code != 401 {
		t.Fatal("未经登录访问 Docker 容器日志", got.Code)
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/docker/resources/compose:test/stats", "", true, false); got.Code != 502 {
		t.Fatal("未配对主机不得查询 Docker 资源统计", got.Code)
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/docker/resources/compose:test/logs?tail=1000", "", true, false); got.Code != 400 {
		t.Fatal("允许超过日志行数限制", got.Code)
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/snapshot", "", false, false); got.Code != 401 {
		t.Fatal("快照未认证", got.Code)
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/snapshot", "", true, false); got.Code != 502 {
		t.Fatal("未配对主机不应查询成功", got.Code)
	}
	if got := req("POST", "/api/v1/hosts/"+id+"/pair", `{"agent_id":"bad","spki_sha256":"wrong","pairing_code":"bad"}`, true, true); got.Code != 502 {
		t.Fatal("无效指纹参数未拒绝", got.Code)
	}
}
