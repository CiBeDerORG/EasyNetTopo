package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestOpenWrtDiscoveryRPCUsesFixedObjectsAndStoredCredential(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ubus" || r.Method != http.MethodPost {
			t.Errorf("非法 RPCD 路径或方法：%s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var request struct {
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Params) != 4 {
			t.Error("RPCD 参数格式错误")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var session, object, method string
		_ = json.Unmarshal(request.Params[0], &session)
		_ = json.Unmarshal(request.Params[1], &object)
		_ = json.Unmarshal(request.Params[2], &method)
		mu.Lock()
		calls = append(calls, object+"."+method)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if object == "session" && method == "login" {
			var login struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			_ = json.Unmarshal(request.Params[3], &login)
			if session != strings.Repeat("0", 32) || login.Username != "easyadb" || login.Password != "private-secret" {
				t.Error("RPCD 登录未正确使用保存的专用账号")
			}
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":[0,{"ubus_rpc_session":"0123456789abcdef0123456789abcdef"}]}`)
			return
		}
		if session != "0123456789abcdef0123456789abcdef" {
			t.Errorf("未使用登录返回的 RPCD 会话：%q", session)
		}
		fmt.Fprint(w, `{"jsonrpc":"2.0","result":[0,{"status":"ok","has_snapshot":true,"count":1,"devices":[{"name":"测试设备"}]}]}`)
	}))
	defer server.Close()

	secret := storage.DeviceOpenWrtSecret{URL: server.URL + "/ubus", Username: "easyadb", Password: "private-secret"}
	for _, item := range []struct{ kind, action, object string }{
		{"devices", "list", "easy-discovery.devices"},
		{"devices", "scan", "easy-discovery.devices"},
		{"adb", "list", "easy-discovery.adb"},
		{"adb", "scan", "easy-discovery.adb"},
	} {
		data, err := callOpenWrtDiscoveryRPC(context.Background(), server.Client(), secret, item.kind, item.action)
		if err != nil || !strings.Contains(string(data), `"测试设备"`) {
			t.Fatalf("%s.%s 调用失败: %s %v", item.kind, item.action, data, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 8 {
		t.Fatalf("预期 4 次登录和 4 次发现，实际：%v", calls)
	}
	for i, item := range []struct{ kind, action, object string }{
		{"devices", "list", "easy-discovery.devices"},
		{"devices", "scan", "easy-discovery.devices"},
		{"adb", "list", "easy-discovery.adb"},
		{"adb", "scan", "easy-discovery.adb"},
	} {
		if calls[i*2] != "session.login" || calls[i*2+1] != item.object+"."+item.action {
			t.Fatalf("固定对象/方法不正确: %v", calls)
		}
	}
}

func TestOpenWrtDiscoveryRPCRejectsRemoteErrors(t *testing.T) {
	for _, tc := range []struct {
		name, reply, want string
	}{
		{"权限不足", `{"result":[6,{}]}`, "openwrt_rpcd_permission_denied"},
		{"缺少插件", `{"result":[4,{}]}`, "openwrt_discovery_unavailable"},
		{"JSON-RPC 错误", `{"error":{"code":-32603,"message":"bad"}}`, "openwrt_rpcd_invalid_response"},
		{"不支持结果", `{"result":[0,{"unexpected":true}]}`, "openwrt_discovery_invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct { Params []json.RawMessage `json:"params"` }
				_ = json.NewDecoder(r.Body).Decode(&request)
				var object string
				_ = json.Unmarshal(request.Params[1], &object)
				if object == "session" {
					fmt.Fprint(w, `{"result":[0,{"ubus_rpc_session":"0123456789abcdef0123456789abcdef"}]}`)
				} else {
					fmt.Fprint(w, tc.reply)
				}
			}))
			defer server.Close()
			_, err := callOpenWrtDiscoveryRPC(context.Background(), server.Client(),
				storage.DeviceOpenWrtSecret{URL: server.URL+"/ubus", Username:"easyadb", Password:"safe"},
				"devices", "scan")
			if err == nil || err.code != tc.want {
				t.Fatalf("应返回 %s，实际：%v", tc.want, err)
			}
		})
	}
}

func TestOpenWrtDiscoveryRouteGuards(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	ds, err := storage.NewDeviceStore(ctx, db)
	if err != nil { t.Fatal(err) }
	handler := NewHandlerWithDevices(t.TempDir(), db, nil, nil, nil, ds)
	session, _, err := db.NewSession(ctx)
	if err != nil { t.Fatal(err) }
	request := func(method, path string, authenticated, csrf bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if authenticated { req.AddCookie(&http.Cookie{Name: authCookie, Value: session}) }
		if csrf { req.Header.Set("X-ENT-Request", "1") }
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	normal, err := ds.CreateDevice(ctx, storage.DeviceInput{Name:"普通设备", SemanticKinds:[]string{"router"}, IPAddress:"192.168.1.30"})
	if err != nil { t.Fatal(err) }
	notGateway, err := ds.CreateDevice(ctx, storage.DeviceInput{Name:"非网关 OpenWrt", SemanticKinds:[]string{"openwrt"}, IPAddress:"192.168.1.31"})
	if err != nil { t.Fatal(err) }
	gateway, err := ds.CreateDevice(ctx, storage.DeviceInput{Name:"网关 OpenWrt", SemanticKinds:[]string{"openwrt"}, IPAddress:"192.168.1.32",
		IsGateway:true, NetworkKey:"network:home", PeerID:"peer-a"})
	if err != nil { t.Fatal(err) }
	base := "/api/v1/devices/"+gateway.ID+"/openwrt/discovery/devices"
	if got := request("GET", base, false, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("未经授权读取：%d", got.Code)
	}
	if got := request("POST", base, true, false); got.Code != http.StatusForbidden {
		t.Fatalf("未做同源验证仍允许扫描：%d", got.Code)
	}
	if got := request("GET", base, true, false); got.Code != http.StatusConflict || !strings.Contains(got.Body.String(), "openwrt_discovery_not_configured") {
		t.Fatalf("没有 RPCD 配置仍允许读取：%d %s", got.Code, got.Body.String())
	}
	if got := request("GET", "/api/v1/devices/"+notGateway.ID+"/openwrt/discovery/adb", true, false); got.Code != http.StatusConflict ||
		!strings.Contains(got.Body.String(), "openwrt_gateway_required") {
		t.Fatalf("非网关设备仍可调用：%d %s", got.Code, got.Body.String())
	}
	if got := request("GET", "/api/v1/devices/"+normal.ID+"/openwrt/discovery/devices", true, false); got.Code != http.StatusConflict ||
		!strings.Contains(got.Body.String(), "device_not_openwrt") {
		t.Fatalf("非 OpenWrt 设备仍可调用：%d %s", got.Code, got.Body.String())
	}
	if got := request("POST", "/api/v1/devices/"+normal.ID+"/openwrt/discovery/adb", true, true); got.Code != http.StatusConflict {
		t.Fatalf("非 OpenWrt 设备仍可扫描：%d", got.Code)
	}
	if got := request("GET", "/api/v1/devices/"+gateway.ID+"/openwrt/discovery/unknown", true, false); got.Code != http.StatusNotFound {
		t.Fatalf("非法发现类型未返回 404：%d", got.Code)
	}
	if got := request("DELETE", base, true, true); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("非法方法未被拒绝：%d", got.Code)
	}
}
