package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGatewayDiscoveryStatusUsesCurrentObservationAndSourceScopedOffline(t *testing.T) {
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
		Name: "OpenWrt A", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherGateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "OpenWrt B", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.2", NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	createChild := func(name, ip string, groupID string) Device {
		t.Helper()
		device, createErr := store.CreateDevice(ctx, DeviceInput{
			Name: name, IPAddress: ip, GatewayGroupID: groupID,
			NetworkKey: "network:home", PeerID: "peer-a",
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return device
	}
	first := createChild("设备 A", "192.168.10.10", gateway.OwnedGatewayGroupID)
	second := createChild("设备 B", "192.168.10.11", gateway.OwnedGatewayGroupID)
	manual := createChild("手工设备", "192.168.10.12", gateway.OwnedGatewayGroupID)
	other := createChild("其他网关设备", "192.168.10.13", otherGateway.OwnedGatewayGroupID)

	for _, device := range []Device{first, second, manual, other} {
		if device.DiscoveryStatus != "unknown" {
			t.Fatalf("未发现设备初始状态应为 unknown: %+v", device)
		}
	}

	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "devices", []DiscoveredDeviceInput{
		{Name: "设备 A", IPAddress: first.IPAddress, Observation: "recent"},
		{Name: "设备 B", IPAddress: second.IPAddress, Observation: "lease"},
		{Name: "其他网关设备", IPAddress: other.IPAddress, Observation: "recent"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus := func(deviceID, expected string) Device {
		t.Helper()
		device, readErr := store.Device(ctx, deviceID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if device.DiscoveryStatus != expected {
			t.Fatalf("设备 %s 发现状态应为 %s，实际 %+v", device.Name, expected, device)
		}
		return device
	}
	assertStatus(first.ID, "online")
	assertStatus(second.ID, "online")
	assertStatus(manual.ID, "offline")
	assertStatus(other.ID, "unknown")

	// 完整服务发现中，只有本次观测到的 A 在线，之前由同一来源发现的 B 离线。
	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "devices", []DiscoveredDeviceInput{
		{Name: "设备 A", IPAddress: first.IPAddress, Observation: "recent"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(first.ID, "online")
	assertStatus(second.ID, "offline")

	// ADB 来源仍观测到 B，因此聚合状态必须保持在线。
	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "adb", []DiscoveredDeviceInput{
		{Name: "ADB 设备 B", IPAddress: second.IPAddress},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(second.ID, "online")

	// ADB 完整扫描中 B 缺席后，服务发现也已标记离线，最终聚合为离线。
	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "adb", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(second.ID, "offline")

	// 其他 OpenWrt 网关的观测不受当前网关扫描影响。
	_, err = store.SyncGatewayDiscoveredDevices(ctx, otherGateway.ID, "devices", []DiscoveredDeviceInput{
		{Name: "其他网关设备", IPAddress: other.IPAddress},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(other.ID, "online")
	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "devices", []DiscoveredDeviceInput{
		{Name: "设备 A", IPAddress: first.IPAddress, Observation: "recent"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(other.ID, "online")

	// 不完整扫描只能确认本次观测到的设备在线，不能将缺席设备判为离线。
	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "devices", []DiscoveredDeviceInput{
		{Name: "设备 A", IPAddress: first.IPAddress, Observation: "recent"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(first.ID, "online")

	lastSeen := assertStatus(first.ID, "online").DiscoveryLastSeenAt
	if lastSeen == "" {
		t.Fatal("在线设备应记录最近观测时间")
	}
	if _, err := store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "invalid", nil, true); err == nil {
		t.Fatal("不允许未知发现来源")
	}
}

func TestGatewayDiscoveryCachedRecordsAreNotOnlineOrImported(t *testing.T) {
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
		Name: "OpenWrt", SemanticKinds: []string{"openwrt"}, IsGateway: true,
		IPAddress: "192.168.10.1", NetworkKey: "network:home", PeerID: "peer-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SyncGatewayDiscoveredDevices(ctx, gateway.ID, "devices", []DiscoveredDeviceInput{
		{Name: "历史缓存设备", IPAddress: "192.168.10.20", Observation: "cached"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := store.ListDevicesForNode(ctx, "network:home", "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("缓存设备不能自动创建: %+v", devices)
	}
}
