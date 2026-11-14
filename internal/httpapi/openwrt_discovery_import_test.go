package httpapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestOpenWrtDiscoveryScanAutomaticallyImportsAndNeverImportsOldSnapshot(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, storage.DeviceInput{
		Name:          "OWRT",
		SemanticKinds: []string{"openwrt"},
		IsGateway:     true,
		IPAddress:     "192.168.10.1",
		NetworkKey:    "network:home",
		PeerID:        "peer-owrt",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"status":"ok","has_snapshot":true,"count":2,"devices":[
		{"name":"电脑","ipv4":["192.168.10.20"],"mac":"02:11:22:33:44:55"},
		{"name":"无信息设备"}
	]}`)
	result, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", raw)
	if callErr != nil {
		t.Fatal(callErr)
	}
	if !strings.Contains(string(result), `"created":1`) ||
		!strings.Contains(string(result), `"skipped":1`) {
		t.Fatalf("未返回预期自动导入统计: %s", result)
	}
	items, err := store.ListDevicesForNode(ctx, "network:home", "peer-owrt")
	if err != nil || len(items) != 2 || items[1].GatewayGroupID != gateway.OwnedGatewayGroupID {
		t.Fatalf("发现设备应挂到网关下: %+v %v", items, err)
	}
	again, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", raw)
	if callErr != nil || !strings.Contains(string(again), `"created":0`) ||
		!strings.Contains(string(again), `"existing":1`) {
		t.Fatalf("重复扫描应幂等: %s %v", again, callErr)
	}
	for _, stale := range []string{
		`{"status":"error","has_snapshot":true,"devices":[{"name":"旧缓存","ipv4":["192.168.10.30"]}]}`,
		`{"status":"ok","has_snapshot":false,"devices":[{"name":"无快照","ipv4":["192.168.10.31"]}]}`,
	} {
		reply, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", json.RawMessage(stale))
		if callErr != nil || string(reply) != stale {
			t.Fatalf("失败或未采集时不得导入旧结果: %s %v", reply, callErr)
		}
	}
	devices, err := store.ListDevicesForNode(ctx, "network:home", "peer-owrt")
	if err != nil || len(devices) != 2 {
		t.Fatalf("历史快照被错误导入: %+v %v", devices, err)
	}
}

func TestOpenWrtADBScanImportsSameIPOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, storage.DeviceInput{
		Name: "OWRT", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-owrt",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"status":"partial","has_snapshot":true,"count":3,"devices":[
		{"instance":"安卓电视","address":"192.168.10.25","port":5555},
		{"instance":"安卓电视另一端口","address":"192.168.10.25","port":37105},
		{"instance":"链接本地地址","address":"fe80::1","port":5555}
	]}`)
	result, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "adb", raw)
	if callErr != nil {
		t.Fatal(callErr)
	}
	for _, want := range []string{`"created":1`, `"existing":1`, `"skipped":1`} {
		if !strings.Contains(string(result), want) {
			t.Fatalf("ADB 应按设备地址合并端点: %s", result)
		}
	}
	devices, err := store.ListDevicesForNode(ctx, "network:home", "peer-owrt")
	if err != nil || len(devices) != 2 || devices[1].Note != "来源：OpenWrt ADB 发现" {
		t.Fatalf("ADB 新设备保存错误: %+v %v", devices, err)
	}
}

func TestOpenWrtDiscoveryImportRejectsMalformedPayload(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, storage.DeviceInput{
		Name: "OWRT", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-owrt",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		`{"status":"ok","has_snapshot":true,"count":1}`,
		`{"status":"ok","has_snapshot":true,"devices":"not-an-array"}`,
		`{"status":"ok","has_snapshot":true,"devices":[{"ipv4":3}]}`,
	} {
		_, e := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", json.RawMessage(invalid))
		if e == nil || e.code != "openwrt_discovery_invalid_response" {
			t.Fatalf("非法结构应拒绝自动入库: %s -> %v", invalid, e)
		}
	}
}

func TestOpenWrtDiscoveryImportSynchronizesOnlineAndOfflineStatus(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, storage.DeviceInput{
		Name: "OWRT", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-owrt",
	})
	if err != nil {
		t.Fatal(err)
	}
	first := json.RawMessage(`{"status":"ok","has_snapshot":true,"truncated":false,"devices":[
		{"name":"电脑","ipv4":["192.168.10.20"],"mac":"02:11:22:33:44:55","observation":"recent"},
		{"name":"电视","ipv4":["192.168.10.21"],"mac":"02:11:22:33:44:56","observation":"lease"}
	]}`)
	if _, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", first); callErr != nil {
		t.Fatal(callErr)
	}
	devices, err := store.ListDevicesForNode(ctx, "network:home", "peer-owrt")
	if err != nil || len(devices) != 3 {
		t.Fatalf("首次扫描应登记两台设备: %+v %v", devices, err)
	}
	statusByIP := func(ip string) string {
		t.Helper()
		items, listErr := store.ListDevicesForNode(ctx, "network:home", "peer-owrt")
		if listErr != nil {
			t.Fatal(listErr)
		}
		for _, item := range items {
			if item.IPAddress == ip {
				return item.DiscoveryStatus
			}
		}
		t.Fatalf("找不到设备 IP %s", ip)
		return ""
	}
	if statusByIP("192.168.10.20") != "online" || statusByIP("192.168.10.21") != "online" {
		t.Fatal("本次 recent/lease 观测到的设备必须在线")
	}

	// 完整扫描中电视缺席，之前的观测状态改为离线。
	second := json.RawMessage(`{"status":"ok","has_snapshot":true,"truncated":false,"devices":[
		{"name":"电脑","ipv4":["192.168.10.20"],"mac":"02:11:22:33:44:55","observation":"recent"}
	]}`)
	if _, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", second); callErr != nil {
		t.Fatal(callErr)
	}
	if statusByIP("192.168.10.20") != "online" || statusByIP("192.168.10.21") != "offline" {
		t.Fatal("完整扫描应将本次出现的设备置为在线、缺席设备置为离线")
	}

	// 部分扫描无法确认缺席设备离线；但明确观测到的设备仍可更新为在线。
	partial := json.RawMessage(`{"status":"partial","has_snapshot":true,"truncated":false,"devices":[
		{"name":"电脑","ipv4":["192.168.10.20"],"mac":"02:11:22:33:44:55","observation":"recent"}
	]}`)
	if _, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", partial); callErr != nil {
		t.Fatal(callErr)
	}
	if statusByIP("192.168.10.21") != "offline" {
		t.Fatal("部分扫描不能把缺席设备重新判定为在线或覆盖其离线状态")
	}

	// 即使 status=ok，结果被截断也不能将未返回设备判离线。
	truncated := json.RawMessage(`{"status":"ok","has_snapshot":true,"truncated":true,"devices":[
		{"name":"电脑","ipv4":["192.168.10.20"],"mac":"02:11:22:33:44:55","observation":"recent"}
	]}`)
	if _, callErr := importOpenWrtDiscoverySnapshot(ctx, store, gateway.ID, "devices", truncated); callErr != nil {
		t.Fatal(callErr)
	}
	if statusByIP("192.168.10.21") != "offline" {
		t.Fatal("截断扫描不应批量修改缺席设备状态")
	}
}

func TestOpenWrtDiscoveryScanReturnsDetailedDiagnostics(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "discovery-diagnostics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, storage.DeviceInput{
		Name: "上海-OWRT", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-local",
	})
	if err != nil {
		t.Fatal(err)
	}
	mapper, found := openWrtNetworkIPMapperFromRoute(routeDataForNetworkIP(), gateway.NetworkKey, gateway.PeerID)
	if !found {
		t.Fatal("测试路由映射缺失")
	}
	raw := json.RawMessage(`{"status":"ok","has_snapshot":true,"source":"easy-discovery","count":1,"devices":[
		{"name":"电脑","ipv4":["192.168.10.20"],"mac":"02:11:22:33:44:55","observation":"recent"}
	]}`)
	routeDiagnostics := openWrtNetworkIPDiagnostics{
		TargetNetworkKey: gateway.NetworkKey,
		TargetPeerID:     gateway.PeerID,
		NetworkEntries:   1,
		HostsQueried:     1,
		HostsSucceeded:   1,
		HostsMatched:     1,
		Available:        true,
		MatchLocation:    "node_info",
		MappingRules:     []string{"192.168.10.0/24 -> 10.10.30.0/24"},
		Reason:           "测试路由匹配成功",
		Hosts: []openWrtNetworkIPHostDiagnostic{{
			HostID: "host-entry", HostName: "云端入口", Status: "matched",
			MatchLocation: "node_info", MappingRules: []string{"192.168.10.0/24 -> 10.10.30.0/24"},
		}},
	}
	updated, callErr := importOpenWrtDiscoverySnapshotWithDiagnostics(ctx, store, gateway, "devices", raw, mapper, routeDiagnostics)
	if callErr != nil {
		t.Fatal(callErr)
	}
	var response struct {
		Import struct {
			Created  int `json:"created"`
			Existing int `json:"existing"`
			Skipped  int `json:"skipped"`
		} `json:"import"`
		Diagnostics struct {
			GeneratedAt string `json:"generated_at"`
			Gateway     struct {
				Name       string `json:"name"`
				NetworkKey string `json:"network_key"`
				PeerID     string `json:"peer_id"`
			} `json:"gateway"`
			Snapshot struct {
				CanImport   bool `json:"can_import"`
				DeviceCount int  `json:"device_count"`
			} `json:"snapshot"`
			Route struct {
				Available     bool     `json:"available"`
				MatchLocation string   `json:"match_location"`
				MappingRules  []string `json:"mapping_rules"`
				Hosts         []struct {
					HostName string `json:"host_name"`
				} `json:"hosts"`
			} `json:"route"`
			Import struct {
				Devices          []storage.DiscoveredDeviceSyncDiagnostic `json:"devices"`
				NetworkIPWritten int                                      `json:"network_ip_written"`
			} `json:"import"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(updated, &response); err != nil {
		t.Fatal(err)
	}
	if response.Import.Created != 1 || response.Import.Existing != 0 || response.Import.Skipped != 0 {
		t.Fatalf("原有入库统计契约应保持不变: %+v", response.Import)
	}
	diagnostics := response.Diagnostics
	if diagnostics.GeneratedAt == "" || diagnostics.Gateway.Name != "上海-OWRT" ||
		diagnostics.Gateway.NetworkKey != gateway.NetworkKey || diagnostics.Gateway.PeerID != gateway.PeerID ||
		!diagnostics.Snapshot.CanImport || diagnostics.Snapshot.DeviceCount != 1 ||
		!diagnostics.Route.Available || diagnostics.Route.MatchLocation != "node_info" ||
		len(diagnostics.Route.MappingRules) != 1 || len(diagnostics.Route.Hosts) != 1 ||
		diagnostics.Route.Hosts[0].HostName != "云端入口" ||
		len(diagnostics.Import.Devices) != 1 || diagnostics.Import.NetworkIPWritten != 1 {
		t.Fatalf("扫描诊断信息不完整: %+v", diagnostics)
	}
	device := diagnostics.Import.Devices[0]
	if device.Result != "created" || device.CurrentIPAddress != "192.168.10.20" ||
		device.CurrentNetworkIP != "10.10.30.20" || device.NetworkIPAction != "written" ||
		device.NetworkIPMappingStatus != "mapped" ||
		!device.AddressFieldsUpdated || !device.DatabaseUpdated || !device.DiscoveryStatusUpdated {
		t.Fatalf("逐设备数据库写入诊断不准确: %+v", device)
	}
}
