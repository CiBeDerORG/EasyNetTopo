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

func TestNetworkNodeAttributesAPI(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hostSvc, err := hosts.NewService(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithHosts(t.TempDir(), db, nil, hostSvc)
	token, _, err := db.NewSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, body string, auth, sameOrigin bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/network-nodes/attributes", strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth {
			req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
		}
		if sameOrigin {
			req.Header.Set("X-ENT-Request", "1")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	payload := `{"network_key":"home","peer_id":"123","alias":"别名","has_public_ip":true,"public_ip":"8.8.8.8","location":{"country":"中国","province":"浙江","city":"杭州","county":"西湖","latitude":30.2,"longitude":120.1}}`
	if got := request("GET", "", false, false); got.Code != 401 {
		t.Fatal("未登录仍可读取", got.Code)
	}
	if got := request("PUT", payload, true, false); got.Code != 403 {
		t.Fatal("缺少 CSRF 校验", got.Code)
	}
	if got := request("PUT", payload, true, true); got.Code != 200 {
		t.Fatal("保存失败", got.Code, got.Body.String())
	}
	if got := request("GET", "", true, false); got.Code != 200 || !strings.Contains(got.Body.String(), "别名") {
		t.Fatal("读取失败", got.Code, got.Body.String())
	}
	if got := request("PUT", strings.Replace(payload, "8.8.8.8", "127.0.0.1", 1), true, true); got.Code != 400 {
		t.Fatal("非法公网 IP 未拦截", got.Code)
	}
	if got := request("DELETE", "", false, true); got.Code != 401 {
		t.Fatal("未登录仍可清除节点数据", got.Code)
	}
	if got := request("DELETE", "", true, false); got.Code != 403 {
		t.Fatal("清除节点数据接口缺少同源校验", got.Code)
	}
	if got := request("DELETE", "", true, true); got.Code != 204 || got.Body.Len() != 0 {
		t.Fatal("清除节点数据失败", got.Code, got.Body.String())
	}
	if got := request("GET", "", true, false); got.Code != 200 || !strings.Contains(got.Body.String(), `"attributes":[]`) {
		t.Fatal("节点数据未清空", got.Code, got.Body.String())
	}
}
