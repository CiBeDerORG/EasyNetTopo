package httpapi

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

func TestLoginGuardsAndSession(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(NewHandler(t.TempDir(), db))
	defer server.Close()
	run := func(method, path, payload, cookie string, flag bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		if flag {
			req.Header.Set("X-ENT-Request", "1")
		}
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := run("GET", "/api/v1/system/info", "", "", false)
	if res.StatusCode != 401 {
		t.Fatal("未授权读取系统身份")
	}
	res.Body.Close()
	res = run("GET", "/api/v1/auth/status", "", "", false)
	var v map[string]bool
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if v["initialized"] {
		t.Fatal("未配置系统被标记已初始化")
	}
	if err := db.CreateAdmin(context.Background(), "admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	payload := `{"username":"admin","password":"correct-horse-battery"}`
	res = run("POST", "/api/v1/auth/login", payload, "", false)
	if res.StatusCode != 403 {
		t.Fatal("CSRF 请求头无效")
	}
	res.Body.Close()
	res = run("POST", "/api/v1/auth/login", `{"username":"admin","password":"wrong-password"}`, "", true)
	if res.StatusCode != 401 {
		t.Fatal("错误密码成功")
	}
	res.Body.Close()
	res = run("POST", "/api/v1/auth/login", payload, "", true)
	if res.StatusCode != 200 {
		t.Fatal("有效密码拒绝", res.StatusCode)
	}
	cookies := res.Cookies()
	res.Body.Close()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("没有 HttpOnly Cookie")
	}
	cookie := cookies[0].Name + "=" + cookies[0].Value
	res = run("GET", "/api/v1/system/info", "", cookie, false)
	if res.StatusCode != 200 {
		t.Fatal("会话未授权", res.StatusCode)
	}
	res.Body.Close()
	res = run("POST", "/api/v1/auth/logout", "", cookie, true)
	if res.StatusCode != 204 {
		t.Fatal("注销失败", res.StatusCode)
	}
	res.Body.Close()
	res = run("GET", "/api/v1/system/info", "", cookie, false)
	if res.StatusCode != 401 {
		t.Fatal("注销后仍允许访问", res.StatusCode)
	}
	res.Body.Close()
}

func TestLoginAcceptsLongAndUnicodeCredentials(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user := strings.Repeat("用户@ ", 50)
	password := strings.Repeat("很长的密码! ", 90)
	if err := db.CreateAdmin(context.Background(), user, password); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(t.TempDir(), db))
	defer server.Close()
	body, err := json.Marshal(map[string]string{"username": user, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-ENT-Request", "1")
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("长账号与长密码不能登录: HTTP %d", res.StatusCode)
	}
}
