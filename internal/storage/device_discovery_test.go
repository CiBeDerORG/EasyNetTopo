package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestGatewayDiscoveryAutoImportCreatesChildrenAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "网关", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", MACAddress: "02:00:00:00:00:01",
		NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gateway.OwnedGatewayGroupID == "" {
		t.Fatal("网关未创建自身网关组")
	}
	manual, err := store.CreateDevice(ctx, DeviceInput{
		Name: "手工设备", IPAddress: "192.168.10.12", Note: "保留用户输入",
		NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherNode, err := store.CreateDevice(ctx, DeviceInput{
		Name: "其他节点设备", IPAddress: "192.168.10.20",
		NetworkKey: "network:home", PeerID: "peer-b",
	})
	if err != nil {
		t.Fatal(err)
	}

	items := []DiscoveredDeviceInput{
		{Name: "网关自己", IPAddress: "192.168.10.1", MACAddress: "02:00:00:00:00:01"},
		{Name: "手工已有", IPAddress: "192.168.10.12"},
		{Name: "新设备甲", IPAddress: "192.168.10.20", MACAddress: "02:00:00:00:00:20", Manufacturer: "某品牌", Source: "devices"},
		{Name: "同一 MAC 不同 IP", IPAddress: "192.168.10.25", MACAddress: "02:00:00:00:00:20"},
		{Name: "ADB 终端", IPAddress: "192.168.10.21", Source: "adb"},
		{Name: "ADB 重复地址", IPAddress: "192.168.10.21", Source: "adb"},
		{Name: "空记录"},
		{Name: "无效地址", IPAddress: "fe80::2"},
	}
	result, err := store.ImportGatewayDiscoveredDevices(ctx, gateway.ID, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created != 2 || result.Existing != 4 || result.Skipped != 2 {
		t.Fatalf("自动导入计数错误: %+v", result)
	}
	nodeDevices, err := store.ListDevicesForNode(ctx, "network:home", "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodeDevices) != 4 {
		t.Fatalf("应有网关、手工设备和两个新设备，实际: %+v", nodeDevices)
	}
	if updated, err := store.Device(ctx, manual.ID); err != nil || updated.Name != manual.Name || updated.Note != manual.Note || updated.GatewayGroupID != manual.GatewayGroupID {
		t.Fatalf("已有手工设备被覆盖或移动: %+v %v", updated, err)
	}
	found := map[string]Device{}
	for _, d := range nodeDevices {
		found[d.IPAddress] = d
	}
	for _, ip := range []string{"192.168.10.20", "192.168.10.21"} {
		d := found[ip]
		if d.GatewayGroupID != gateway.OwnedGatewayGroupID || d.IsGateway || d.IsNodeDevice ||
			d.NetworkKey != gateway.NetworkKey || d.PeerID != gateway.PeerID {
			t.Fatalf("新设备未正确归属于网关: %+v", d)
		}
	}
	if found["192.168.10.20"].MACAddress != "02:00:00:00:00:20" ||
		found["192.168.10.20"].Manufacturer != "某品牌" {
		t.Fatalf("设备属性没有保存: %+v", found["192.168.10.20"])
	}
	if found["192.168.10.21"].Note != "来源：OpenWrt ADB 发现" {
		t.Fatalf("ADB 来源标识不正确: %+v", found["192.168.10.21"])
	}
	again, err := store.ImportGatewayDiscoveredDevices(ctx, gateway.ID, []DiscoveredDeviceInput{
		{Name: "重复扫描的新名字", IPAddress: "192.168.10.20", MACAddress: "02:00:00:00:00:20"},
		{Name: "ADB 重复", IPAddress: "192.168.10.21", Source: "adb"},
	})
	if err != nil || again.Created != 0 || again.Existing != 2 {
		t.Fatalf("重复扫描不应重复创建设备: %+v %v", again, err)
	}
	if updated, err := store.Device(ctx, found["192.168.10.20"].ID); err != nil || updated.Name != "新设备甲" {
		t.Fatalf("重复扫描覆盖了手工可编辑设备属性: %+v %v", updated, err)
	}
	other, err := store.Device(ctx, otherNode.ID)
	if err != nil || other.Name != "其他节点设备" {
		t.Fatalf("扫描修改其他网络节点设备: %+v %v", other, err)
	}
}

func TestGatewayDiscoveryAutoImportRejectsWrongScopeAndMalformedRecords(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	normal, err := store.CreateDevice(ctx, DeviceInput{
		Name: "普通设备", IPAddress: "192.168.10.30",
		NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportGatewayDiscoveredDevices(ctx, normal.ID, []DiscoveredDeviceInput{
		{Name: "错误添加", IPAddress: "192.168.10.31"},
	}); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("普通设备不应拥有扫描导入权: %v", err)
	}
	gateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "OpenWrt 网关", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.ImportGatewayDiscoveredDevices(ctx, gateway.ID, []DiscoveredDeviceInput{
		{Name: "非法 MAC", MACAddress: "not-a-mac"},
		{Name: "只有名称"},
		{Name: "无效 IP", IPAddress: "192.168.10.999"},
		{IPAddress: "192.168.10.23"},
		{Name: "电视\n盒子", IPAddress: "192.168.10.24"},
	})
	if err != nil || result.Created != 2 || result.Skipped != 3 {
		t.Fatalf("应跳过无定位信息和非法字段: %+v %v", result, err)
	}
	items, err := store.ListDevicesForNode(ctx, "network:home", "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, device := range items {
		if device.IPAddress == "192.168.10.23" && device.Name != "192.168.10.23" {
			t.Fatalf("缺失名称时应以 IP 命名: %+v", device)
		}
		if device.IPAddress == "192.168.10.24" && device.Name != "电视 盒子" {
			t.Fatalf("发现名称需要去除控制字符: %+v", device)
		}
	}
}


func TestGatewayDiscoveryRefreshesExistingDeviceIPAndNetworkIP(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "refresh-existing-device.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "OpenWrt", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	existing, err := store.CreateDevice(ctx, DeviceInput{
		Name: "历史设备", IPAddress: "192.168.10.20", MACAddress: "02:00:00:00:00:20",
		NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}

	syncScan := func(ip, networkIP string) {
		t.Helper()
		result, syncErr := store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "devices", []DiscoveredDeviceInput{
			{
				Name: "重新发现的设备", IPAddress: ip, MACAddress: "02:00:00:00:00:20",
				NetworkIP: networkIP, NetworkIPKnown: true, Observation: "recent",
			},
		}, false)
		if syncErr != nil || result.Existing != 1 || result.Created != 0 {
			t.Fatalf("重新扫描应匹配并更新历史设备: result=%+v err=%v", result, syncErr)
		}
	}

	// 历史设备原来没有组网 IP：首次扫描应同时补齐内网 IP 和组网 IP。
	syncScan("192.168.10.88", "10.10.30.88")
	updated, err := store.Device(ctx, existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IPAddress != "192.168.10.88" || updated.NetworkIP != "10.10.30.88" {
		t.Fatalf("首次重新发现没有同时补齐两个 IP: %+v", updated)
	}

	// DHCP 地址再次变化：同一 MAC 命中的历史设备必须同时更新两个地址。
	syncScan("192.168.10.99", "10.10.30.99")
	updated, err = store.Device(ctx, existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IPAddress != "192.168.10.99" || updated.NetworkIP != "10.10.30.99" {
		t.Fatalf("重复扫描没有同步更新内网 IP 和组网 IP: %+v", updated)
	}
}
