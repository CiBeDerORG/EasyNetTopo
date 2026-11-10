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
	if got := req("GET", "/api/v1/hosts/"+id+"/snapshot", "", false, false); got.Code != 401 {
		t.Fatal("快照未认证", got.Code)
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/snapshot", "", true, false); got.Code != 502 {
		t.Fatal("未配对主机不应查询成功", got.Code)
	}
	if got := req("POST", "/api/v1/hosts/"+id+"/pair", `{"agent_id":"bad","spki_sha256":"wrong","pairing_code":"bad"}`, true, true); got.Code != 422 {
		t.Fatal("无效指纹参数未拒绝", got.Code)
	} else if !strings.Contains(got.Body.String(), "invalid_pairing_input") {
		t.Fatal("输入校验异常没有明确错误码")
	}
	entryPath := "/api/v1/hosts/" + id + "/network-entry"
	for _, tc := range []struct {
		body       string
		auth, csrf bool
		status     int
	}{
		{`{"enabled":true}`, false, true, 401},
		{`{"enabled":true}`, true, false, 403},
		{`{}`, true, true, 400},
		{`{"enabled":true}`, true, true, 409},
		{`{"enabled":false}`, true, true, 200},
	} {
		if got := req("PUT", entryPath, tc.body, tc.auth, tc.csrf); got.Code != tc.status {
			t.Fatalf("入口权限或参数错误：%d，期望 %d", got.Code, tc.status)
		}
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/network/node", "", false, false); got.Code != 401 {
		t.Fatal("网络读取缺少鉴权")
	}
	if got := req("GET", "/api/v1/hosts/"+id+"/network/config", "", true, false); got.Code != 404 {
		t.Fatal("暴露了配置读取")
	}
	// 编辑和删除是需要管理员登录及同源请求标记的本地操作。
	editPath := "/api/v1/hosts/" + id
	payload := `{"name":"新 fnOS","endpoint":"http://host.docker.internal:7018"}`
	if got := req("PATCH", editPath, payload, false, true); got.Code != 401 {
		t.Fatal("未登录不应编辑主机", got.Code)
	}
	if got := req("PATCH", editPath, payload, true, false); got.Code != 403 {
		t.Fatal("编辑未限制同源请求", got.Code)
	}
	if got := req("PATCH", editPath, `{"name":"bad","endpoint":"http://169.254.169.254:80"}`, true, true); got.Code != 200 {
		t.Fatal("编辑主机资料不应强制校验连接地址格式", got.Code, got.Body.String())
	}
	if got := req("PATCH", editPath, payload, true, true); got.Code != 200 || !strings.Contains(got.Body.String(), "新 fnOS") {
		t.Fatal("编辑主机失败", got.Code, got.Body.String())
	}
	// Metadata API must reject identity or pairing data. Only /pair may explicitly change credentials.
	if got := req("PATCH", editPath,
		`{"name":"覆盖尝试","endpoint":"http://127.0.0.1:7018","agent_id":"11111111-1111-4111-8111-111111111111"}`,
		true, true); got.Code != 400 {
		t.Fatal("主机资料接口允许修改 Agent 身份", got.Code)
	}
	if got := req("PATCH", editPath,
		`{"name":"覆盖尝试","endpoint":"http://127.0.0.1:7018","access_token":"not-allowed"}`,
		true, true); got.Code != 400 {
		t.Fatal("主机资料接口允许写入配对 Token", got.Code)
	}
	if got := req("DELETE", editPath, "", false, true); got.Code != 401 {
		t.Fatal("未登录不应删除主机", got.Code)
	}
	if got := req("DELETE", editPath, "", true, false); got.Code != 403 {
		t.Fatal("删除未限制同源请求", got.Code)
	}
	if got := req("GET", editPath, "", true, false); got.Code != 405 {
		t.Fatal("主机详情不应隐式返回或删除", got.Code)
	}
	if got := req("DELETE", editPath, "", true, true); got.Code != 204 || got.Body.Len() != 0 {
		t.Fatal("删除主机失败或 204 带有响应体", got.Code, got.Body.String())
	}
	if got := req("GET", "/api/v1/hosts", "", true, false); got.Code != 200 || strings.Contains(got.Body.String(), "新 fnOS") {
		t.Fatal("删除后主机列表未更新", got.Code, got.Body.String())
	}
	if got := req("DELETE", editPath, "", true, true); got.Code != 404 {
		t.Fatal("重复删除应返回 404", got.Code)
	}
}
