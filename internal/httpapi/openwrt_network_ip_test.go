package httpapi

import (
	"context"
	"encoding/json"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func ipv4AddressValue(value uint32) map[string]any {
	return map[string]any{
		"address": map[string]any{"addr": value},
		"network_length": 24,
	}
}

func routeDataForNetworkIP() json.RawMessage {
	value := map[string]any{
		"agent_id": "agent",
		"data": []any{
			map[string]any{
				"instance_id": "instance-home",
				"result": map[string]any{
					"node_info": map[string]any{
						"network_name": "home",
						"peer_id": "peer-local",
						"ipv4_addr": ipv4AddressValue(168435201), // 10.10.30.1
						"proxy_cidrs": []string{"192.168.10.0/24"},
					},
					"peer_routes": []any{
						map[string]any{
							"route": map[string]any{
								"peer_id": "peer-remote",
								"ipv4_addr": ipv4AddressValue(168437761), // 10.10.40.1
								"proxy_cidrs": []string{"192.168.20.0/24"},
							},
							"peer": map[string]any{"peer_id": "peer-remote"},
						},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(value)
	return raw
}

func TestOpenWrtNetworkIPMapperMapsLocalAndPeerRoutes(t *testing.T) {
	raw := routeDataForNetworkIP()
	local, found := openWrtNetworkIPMapperFromRoute(raw, "network:home", "peer-local")
	if !found {
		t.Fatal("应从本机 node_info 找到网络节点代理网段")
	}
	if mapped, known := local.mapAddress("192.168.10.162"); !known || mapped != "10.10.30.162" {
		t.Fatalf("本机节点组网 IP 映射错误: mapped=%q known=%v", mapped, known)
	}
	remote, found := openWrtNetworkIPMapperFromRoute(raw, "network:home", "peer-remote")
	if !found {
		t.Fatal("应从 peer_routes 找到远端网络节点代理网段")
	}
	if mapped, known := remote.mapAddress("192.168.20.88"); !known || mapped != "10.10.40.88" {
		t.Fatalf("远端节点组网 IP 映射错误: mapped=%q known=%v", mapped, known)
	}
	if _, found := openWrtNetworkIPMapperFromRoute(raw, "network:other", "peer-local"); found {
		t.Fatal("不应跨 network_key 匹配代理网段")
	}
}

func TestOpenWrtNetworkIPMapperRejectsAmbiguousAndInvalidAddresses(t *testing.T) {
	mapper, found := openWrtNetworkIPMapperFromRoute(routeDataForNetworkIP(), "network:home", "peer-local")
	if !found {
		t.Fatal("测试映射规则缺失")
	}
	for _, address := range []string{
		"192.168.10.0",
		"192.168.10.255",
		"192.168.11.10",
		"2001:db8::10",
		"not-an-ip",
	} {
		if mapped, known := mapper.mapAddress(address); mapped != "" || (address == "192.168.10.0" && !known) {
			t.Fatalf("异常地址不应被映射: address=%q mapped=%q known=%v", address, mapped, known)
		}
	}

	mismatched := mapper
	mismatched.targetPrefix = netip.MustParsePrefix("10.10.30.1/25")
	if mapped, known := mismatched.mapAddress("192.168.10.162"); !known || mapped != "" {
		t.Fatalf("代理网段与虚拟网段掩码不一致时不应映射: mapped=%q known=%v", mapped, known)
	}

	mapper.proxyPrefixes = append(mapper.proxyPrefixes, netip.MustParsePrefix("192.168.10.128/25"))
	if mapped, known := mapper.mapAddress("192.168.10.162"); !known || mapped != "" {
		t.Fatalf("重叠代理网段应视为歧义而不映射: mapped=%q known=%v", mapped, known)
	}
}

func TestOpenWrtDiscoveryImportPersistsAndRefreshesNetworkIP(t *testing.T) {
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
		Name: "OpenWrt", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-local",
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(map[string]any{
		"status": "ok",
		"has_snapshot": true,
		"devices": []any{
			map[string]any{
				"name": "电脑",
				"ipv4": []string{"192.168.10.162"},
				"mac": "02:11:22:33:44:55",
				"observation": "recent",
			},
		},
	})
	mapper, found := openWrtNetworkIPMapperFromRoute(routeDataForNetworkIP(), "network:home", "peer-local")
	if !found {
		t.Fatal("未能创建测试映射")
	}
	if _, callErr := importOpenWrtDiscoverySnapshotWithNetworkIP(ctx, store, gateway.ID, "devices", snapshot, mapper); callErr != nil {
		t.Fatal(callErr)
	}
	findDevice := func() storage.Device {
		t.Helper()
		items, listErr := store.ListDevicesForNode(ctx, "network:home", "peer-local")
		if listErr != nil {
			t.Fatal(listErr)
		}
		for _, item := range items {
			if item.IPAddress == "192.168.10.162" {
				return item
			}
		}
		t.Fatal("没有找到主动发现设备")
		return storage.Device{}
	}
	device := findDevice()
	if device.NetworkIP != "10.10.30.162" {
		t.Fatalf("首次扫描未保存组网 IP: %+v", device)
	}

	updatedRoute := routeDataForNetworkIP()
	var routeValue map[string]any
	if err := json.Unmarshal(updatedRoute, &routeValue); err != nil {
		t.Fatal(err)
	}
	data := routeValue["data"].([]any)
	instance := data[0].(map[string]any)
	result := instance["result"].(map[string]any)
	nodeInfo := result["node_info"].(map[string]any)
	nodeInfo["ipv4_addr"] = ipv4AddressValue(168437761) // 10.10.40.1
	updatedRoute, _ = json.Marshal(routeValue)
	updatedMapper, found := openWrtNetworkIPMapperFromRoute(updatedRoute, "network:home", "peer-local")
	if !found {
		t.Fatal("未能创建更新后的测试映射")
	}
	if _, callErr := importOpenWrtDiscoverySnapshotWithNetworkIP(ctx, store, gateway.ID, "devices", snapshot, updatedMapper); callErr != nil {
		t.Fatal(callErr)
	}
	device = findDevice()
	if device.NetworkIP != "10.10.40.162" {
		t.Fatalf("重复发现没有刷新组网 IP: %+v", device)
	}

	if _, callErr := importOpenWrtDiscoverySnapshotWithNetworkIP(ctx, store, gateway.ID, "devices", snapshot, openWrtNetworkIPMapper{}); callErr != nil {
		t.Fatal(callErr)
	}
	device = findDevice()
	if device.NetworkIP != "10.10.40.162" {
		t.Fatalf("路由数据暂时不可用时不应清空历史组网 IP: %+v", device)
	}

	outsideSubnet, _ := json.Marshal(map[string]any{
		"status": "ok",
		"has_snapshot": true,
		"devices": []any{
			map[string]any{
				"name": "电脑",
				"ipv4": []string{"192.168.11.162"},
				"mac": "02:11:22:33:44:55",
				"observation": "recent",
			},
		},
	})
	if _, callErr := importOpenWrtDiscoverySnapshotWithNetworkIP(ctx, store, gateway.ID, "devices", outsideSubnet, mapper); callErr != nil {
		t.Fatal(callErr)
	}
	device = findDevice()
	if device.NetworkIP != "" {
		t.Fatalf("规则已确认但设备地址不属于代理网段时应清除过期组网 IP: %+v", device)
	}
}
