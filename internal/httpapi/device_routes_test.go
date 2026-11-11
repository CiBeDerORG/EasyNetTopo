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

func TestDeviceManagementAPIAndOpenWrtDeviceGuard(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deviceStore, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithDevices(t.TempDir(), db, nil, nil, nil, deviceStore)
	token, _, err := db.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string, auth, csrf bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth {
			req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
		}
		if csrf {
			req.Header.Set("X-ENT-Request", "1")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if got := request("GET", "/api/v1/devices", "", false, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录设备接口仍可访问: %d", got.Code)
	}
	if got := request("POST", "/api/v1/device-groups", `{"name":"家庭"}`, true, false); got.Code != http.StatusForbidden {
		t.Fatalf("缺少 CSRF 标记仍允许创建分组: %d", got.Code)
	}
	group := request("POST", "/api/v1/device-groups", `{"name":"家庭"}`, true, true)
	if group.Code != http.StatusCreated {
		t.Fatalf("创建分组失败: %d %s", group.Code, group.Body.String())
	}
	var groupData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(group.Body.Bytes(), &groupData); err != nil {
		t.Fatal(err)
	}

	deviceBody := `{"name":"OpenWrt 路由器","group_id":"` + groupData.ID + `","device_type":"router","ip_address":"192.168.1.1","is_openwrt":true,"network_key":"network:home","peer_id":"peer-openwrt"}`
	created := request("POST", "/api/v1/devices", deviceBody, true, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("创建设备失败: %d %s", created.Code, created.Body.String())
	}
	var device struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &device); err != nil {
		t.Fatal(err)
	}

	config := request("PUT", "/api/v1/devices/"+device.ID+"/openwrt",
		`{"rpcd_port":80,"https":false,"username":"easy-device","password":"secret-value"}`, true, true)
	if config.Code != http.StatusOK {
		t.Fatalf("保存 OpenWrt 配置失败: %d %s", config.Code, config.Body.String())
	}
	if strings.Contains(config.Body.String(), "secret-value") {
		t.Fatal("OpenWrt 密钥泄漏到 API 响应")
	}
	read := request("GET", "/api/v1/devices/"+device.ID+"/openwrt", "", true, false)
	if read.Code != http.StatusOK || strings.Contains(read.Body.String(), "secret-value") {
		t.Fatalf("读取 OpenWrt 配置失败或泄漏密钥: %d %s", read.Code, read.Body.String())
	}

	normal := request("POST", "/api/v1/devices", `{"name":"普通路由器","device_type":"router","ip_address":"192.168.1.2"}`, true, true)
	if normal.Code != http.StatusCreated {
		t.Fatalf("创建普通路由器失败: %d %s", normal.Code, normal.Body.String())
	}
	var normalDevice struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(normal.Body.Bytes(), &normalDevice); err != nil {
		t.Fatal(err)
	}
	rejected := request("PUT", "/api/v1/devices/"+normalDevice.ID+"/openwrt",
		`{"rpcd_port":80,"username":"user","password":"secret"}`, true, true)
	if rejected.Code != http.StatusConflict {
		t.Fatalf("普通设备竟可配置 OpenWrt 服务: %d %s", rejected.Code, rejected.Body.String())
	}

	list := request("GET", "/api/v1/devices", "", true, false)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "peer-openwrt") {
		t.Fatalf("设备列表缺少关联数据: %d %s", list.Code, list.Body.String())
	}
}

func TestNetworkNodeSyncAPI(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deviceStore, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithDevices(t.TempDir(), db, nil, nil, nil, deviceStore)
	token, _, err := db.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string, auth, csrf bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth {
			req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
		}
		if csrf {
			req.Header.Set("X-ENT-Request", "1")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	status := request("GET", "/api/v1/devices/network-nodes/sync", "", true, false)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("读取自动补充状态失败: %d %s", status.Code, status.Body.String())
	}
	body := `{"nodes":[{"name":"入口节点","network_key":"network:home","peer_id":"peer-1"}]}`
	synced := request("POST", "/api/v1/devices/network-nodes/sync", body, true, true)
	if synced.Code != http.StatusOK || !strings.Contains(synced.Body.String(), `"created":1`) || !strings.Contains(synced.Body.String(), `"enabled":true`) {
		t.Fatalf("同步网络节点失败: %d %s", synced.Code, synced.Body.String())
	}
	status = request("GET", "/api/v1/devices/network-nodes/sync", "", true, false)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":true`) {
		t.Fatalf("自动补充状态未更新: %d %s", status.Code, status.Body.String())
	}
	list := request("GET", "/api/v1/devices", "", true, false)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"is_node_device":true`) {
		t.Fatalf("节点设备标识未通过 API 返回: %d %s", list.Code, list.Body.String())
	}
}
