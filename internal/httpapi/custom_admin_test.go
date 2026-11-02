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

func TestCustomAdministratorLoginReportsActualUsername(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateAdmin(context.Background(), "operator_01", "Strong-$Pass-For-User!"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(t.TempDir(), db))
	defer server.Close()

	requestLogin := func(username string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login",
			strings.NewReader(`{"username":"`+username+`","password":"Strong-$Pass-For-User!"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-ENT-Request", "1")
		reply, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return reply
	}
	reply := requestLogin("admin")
	if reply.StatusCode != http.StatusUnauthorized {
		t.Fatalf("旧 admin 不应接受自定义管理员口令: %d", reply.StatusCode)
	}
	reply.Body.Close()
	reply = requestLogin("operator_01")
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		t.Fatalf("新管理员登录失败: %d", reply.StatusCode)
	}
	var response struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(reply.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Username != "operator_01" {
		t.Fatalf("登录响应未返回真实用户名: %s", response.Username)
	}
}
