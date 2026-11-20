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

	nodeSync := request("POST", "/api/v1/devices/network-nodes/sync",
		`{"nodes":[{"name":"入口节点","network_key":"network:home","peer_id":"peer-openwrt","hostname":"owrt-home"}]}`, true, true)
	if nodeSync.Code != http.StatusOK {
		t.Fatalf("初始化网络节点失败: %d %s", nodeSync.Code, nodeSync.Body.String())
	}
	deviceBody := `{"name":"OpenWrt 路由器","group_id":"` + groupData.ID + `","create_gateway_group":true,"gateway_group_name":"家庭网络","semantic_kinds":["openwrt"],"ip_address":"192.168.1.1","is_gateway":true,"node_hostname":"owrt-home"}`
	created := request("POST", "/api/v1/devices", deviceBody, true, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("创建设备失败: %d %s", created.Code, created.Body.String())
	}
	var device struct {
		ID           string `json:"id"`
		OwnedGroupID string `json:"owned_gateway_group_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &device); err != nil {
		t.Fatal(err)
	}
	if device.OwnedGroupID == "" || strings.Contains(created.Body.String(), `"peer_id"`) ||
		strings.Contains(created.Body.String(), `"network_key"`) || strings.Contains(created.Body.String(), `"is_openwrt"`) ||
		!strings.Contains(created.Body.String(), `"semantic_kinds":["openwrt"]`) {
		t.Fatalf("主机名创建应生成网关组，且仅返回主机名与 OpenWrt 语义标签: %s", created.Body.String())
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

	normal := request("POST", "/api/v1/devices", `{"name":"普通路由器","semantic_kinds":["router"],"ip_address":"192.168.1.2"}`, true, true)
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

	child := request("POST", "/api/v1/devices",
		`{"name":"客厅电视","semantic_kinds":["tv"],"gateway_group_id":"`+device.OwnedGroupID+`","node_hostname":"OWRT-HOME"}`, true, true)
	if child.Code != http.StatusCreated {
		t.Fatalf("同一节点网关组下添加第二台设备失败: %d %s", child.Code, child.Body.String())
	}
	filtered := request("GET", "/api/v1/devices?node_hostname=OWRT-HOME", "", true, false)
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), "OpenWrt 路由器") ||
		!strings.Contains(filtered.Body.String(), "客厅电视") || !strings.Contains(filtered.Body.String(), `"is_node_device":true`) {
		t.Fatalf("按网络节点筛选设备失败: %d %s", filtered.Code, filtered.Body.String())
	}
	groups := request("GET", "/api/v1/gateway-groups?node_hostname=owrt-home", "", true, false)
	if groups.Code != http.StatusOK || !strings.Contains(groups.Body.String(), "家庭网络") ||
		!strings.Contains(groups.Body.String(), "OpenWrt 路由器") {
		t.Fatalf("读取节点网关组失败: %d %s", groups.Code, groups.Body.String())
	}
	nodeDevices := request("GET", "/api/v1/devices/network-nodes", "", true, false)
	if nodeDevices.Code != http.StatusOK || !strings.Contains(nodeDevices.Body.String(), "peer-openwrt") {
		t.Fatalf("读取网络节点基础设备失败: %d %s", nodeDevices.Code, nodeDevices.Body.String())
	}
	list := request("GET", "/api/v1/devices", "", true, false)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "peer-openwrt") {
		t.Fatalf("设备列表缺少关联数据: %d %s", list.Code, list.Body.String())
	}

	servicePath := "/api/v1/devices/" + device.ID + "/services"
	serviceBody := `{"name":"管理后台","protocols":["udp","tcp"],"port":443,"access_url":"https://router.example.com/","icon":"web","note":"局域网管理"}`
	if got := request(http.MethodGet, servicePath, "", false, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("未认证仍可读取设备服务: %d", got.Code)
	}
	if got := request(http.MethodPost, servicePath, serviceBody, true, false); got.Code != http.StatusForbidden {
		t.Fatalf("缺少同源标记仍可写入设备服务: %d", got.Code)
	}
	if got := request(http.MethodPost, servicePath, strings.Replace(serviceBody, "https://router.example.com/", "javascript:alert(1)", 1), true, true); got.Code != http.StatusBadRequest {
		t.Fatalf("危险访问地址仍可登记: %d", got.Code)
	}
	if got := request(http.MethodPost, "/api/v1/devices/missing/services", serviceBody, true, true); got.Code != http.StatusNotFound {
		t.Fatalf("可为不存在的设备登记服务: %d", got.Code)
	}
	added := request(http.MethodPost, servicePath, serviceBody, true, true)
	var service storage.DeviceService
	if added.Code != http.StatusCreated || json.Unmarshal(added.Body.Bytes(), &service) != nil ||
		service.DeviceID != device.ID || service.ID == "" || service.Port != 443 || len(service.Protocols) != 2 {
		t.Fatalf("登记服务失败: %d %s", added.Code, added.Body.String())
	}
	if got := request(http.MethodGet, servicePath, "", true, false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), service.ID) {
		t.Fatalf("设备服务列表缺少已登记服务: %d %s", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, "/api/v1/devices/"+normalDevice.ID+"/services", "", true, false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"services":[]`) {
		t.Fatalf("其他设备不应读取到该服务: %d %s", got.Code, got.Body.String())
	}
	const navigationList = "/api/v1/device-services/navigation"
	navigationPath := servicePath + "/" + service.ID + "/navigation"
	otherDevicePath := "/api/v1/devices/" + normalDevice.ID + "/services/" + service.ID + "/navigation"
	for _, check := range []struct {
		method, path, body string
		auth, csrf         bool
		status             int
		contains           string
	}{
		{http.MethodGet, navigationList, "", false, false, http.StatusUnauthorized, ""},
		{http.MethodGet, navigationList, "", true, false, http.StatusOK, `"services":[]`},
		{http.MethodPut, navigationPath, `{"enabled":true}`, true, false, http.StatusForbidden, ""},
		{http.MethodPut, navigationPath, `{}`, true, true, http.StatusBadRequest, ""},
		{http.MethodPut, otherDevicePath, `{"enabled":true}`, true, true, http.StatusNotFound, ""},
		{http.MethodPut, navigationPath, `{"enabled":true}`, true, true, http.StatusOK, `"navigation_enabled":true`},
		{http.MethodPut, navigationPath, `{"enabled":true}`, true, true, http.StatusOK, `"navigation_enabled":true`},
		{http.MethodGet, navigationList, "", true, false, http.StatusOK, service.ID},
		{http.MethodGet, servicePath, "", true, false, http.StatusOK, `"navigation_enabled":true`},
		{http.MethodPut, navigationPath, `{"enabled":false}`, true, true, http.StatusOK, `"navigation_enabled":false`},
		{http.MethodGet, navigationList, "", true, false, http.StatusOK, `"services":[]`},
		{http.MethodPut, navigationPath, `{"enabled":true}`, true, true, http.StatusOK, `"navigation_enabled":true`},
	} {
		got := request(check.method, check.path, check.body, check.auth, check.csrf)
		if got.Code != check.status || !strings.Contains(got.Body.String(), check.contains) {
			t.Fatalf("导航请求 %s %s 返回异常: %d %s", check.method, check.path, got.Code, got.Body.String())
		}
	}
	if got := request(http.MethodDelete, "/api/v1/devices/"+device.ID, "", true, true); got.Code != http.StatusNoContent {
		t.Fatalf("关联服务不应阻止设备删除: %d %s", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, servicePath, "", true, false); got.Code != http.StatusNotFound {
		t.Fatalf("已删除的设备服务路径应返回不存在: %d", got.Code)
	}
	if got := request(http.MethodGet, navigationList, "", true, false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"services":[]`) {
		t.Fatalf("已删除设备的服务仍留在导航中: %d %s", got.Code, got.Body.String())
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
	nodeDevices := request("GET", "/api/v1/devices/network-nodes", "", true, false)
	var nodeData struct {
		NodeDevices []storage.Device `json:"node_devices"`
	}
	if nodeDevices.Code != http.StatusOK || json.Unmarshal(nodeDevices.Body.Bytes(), &nodeData) != nil || len(nodeData.NodeDevices) != 1 {
		t.Fatalf("节点基础设备列表 API 错误: %d %s", nodeDevices.Code, nodeDevices.Body.String())
	}
	deleted := request("DELETE", "/api/v1/devices/"+nodeData.NodeDevices[0].ID, "", true, true)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("应允许删除节点基础设备: %d %s", deleted.Code, deleted.Body.String())
	}
	afterDeletion := request("GET", "/api/v1/devices/network-nodes", "", true, false)
	if afterDeletion.Code != http.StatusOK || !strings.Contains(afterDeletion.Body.String(), `"node_devices":[]`) {
		t.Fatalf("被删除的节点基础设备不应留在列表中: %d %s", afterDeletion.Code, afterDeletion.Body.String())
	}
	resynced := request("POST", "/api/v1/devices/network-nodes/sync", body, true, true)
	if resynced.Code != http.StatusOK || !strings.Contains(resynced.Body.String(), `"skipped":1`) {
		t.Fatalf("已删除的节点基础设备不应被同步重新建立: %d %s", resynced.Code, resynced.Body.String())
	}
}

func TestClearDeviceDataAPIAndKeepsNetworkNodeAttributes(t *testing.T) {
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
	if _, err := db.SaveNodeAttributes(ctx, storage.NodeAttributes{
		NetworkKey: "network:home", PeerID: "peer-1", Alias: "保留节点属性",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := deviceStore.SyncNetworkNodeDevices(ctx, []storage.NetworkNodeDeviceInput{
		{Name: "节点设备", NetworkKey: "network:home", PeerID: "peer-1"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := deviceStore.CreateDevice(ctx, storage.DeviceInput{Name: "历史设备"}); err != nil {
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
	if got := request(http.MethodDelete, "/api/v1/devices", "", false, true); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录仍可清除设备数据: %d", got.Code)
	}
	if got := request(http.MethodDelete, "/api/v1/devices", "", true, false); got.Code != http.StatusForbidden {
		t.Fatalf("清除设备数据接口缺少同源校验: %d", got.Code)
	}
	cleared := request(http.MethodDelete, "/api/v1/devices", "", true, true)
	if cleared.Code != http.StatusNoContent || cleared.Body.Len() != 0 {
		t.Fatalf("清除设备数据失败: %d %s", cleared.Code, cleared.Body.String())
	}
	list := request(http.MethodGet, "/api/v1/devices", "", true, false)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"devices":[]`) {
		t.Fatalf("清空后仍存在设备: %d %s", list.Code, list.Body.String())
	}
	status := request(http.MethodGet, "/api/v1/devices/network-nodes/sync", "", true, false)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("清空后自动补充未关闭: %d %s", status.Code, status.Body.String())
	}
	backgroundSync := request(http.MethodPost, "/api/v1/devices/network-nodes/sync",
		`{"nodes":[{"name":"节点设备","network_key":"network:home","peer_id":"peer-1"}],"enable_auto_sync":false}`, true, true)
	if backgroundSync.Code != http.StatusConflict {
		t.Fatalf("清空后后台同步应被拒绝: %d %s", backgroundSync.Code, backgroundSync.Body.String())
	}
	attributes, err := db.ListNodeAttributes(ctx)
	if err != nil || len(attributes) != 1 || attributes[0].Alias != "保留节点属性" {
		t.Fatalf("清除设备数据误删网络节点属性: %+v %v", attributes, err)
	}
	manualSync := request(http.MethodPost, "/api/v1/devices/network-nodes/sync",
		`{"nodes":[{"name":"节点设备","network_key":"network:home","peer_id":"peer-1"}]}`, true, true)
	if manualSync.Code != http.StatusOK {
		t.Fatalf("清空后手动重新初始化失败: %d %s", manualSync.Code, manualSync.Body.String())
	}
}
