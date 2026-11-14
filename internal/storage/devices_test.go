package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceGroupsAndDevicesPersistAndAssociateNetworkNode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data", "app.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	group, err := store.CreateDeviceGroup(ctx, "家庭")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{{Name: "节点 1", NetworkKey: "network:home", PeerID: "peer-1"}}); err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateDevice(ctx, DeviceInput{
		Name: "客厅电视", GroupID: group.ID, DeviceType: "tv", Manufacturer: "小米", Model: "电视",
		IPAddress: "192.168.1.20", MACAddress: "AA:BB:CC:DD:EE:FF", NetworkKey: "network:home", PeerID: "peer-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if device.GroupName != "家庭" || device.MACAddress != "aa:bb:cc:dd:ee:ff" || device.PeerID != "peer-1" {
		t.Fatalf("设备字段错误: %+v", device)
	}
	second, err := store.CreateDevice(ctx, DeviceInput{Name: "客厅电视盒子", DeviceType: "tv", NetworkKey: "network:home", PeerID: "peer-1"})
	if err != nil || second.IsNodeDevice {
		t.Fatalf("同一节点下应允许创建普通设备: %+v %v", second, err)
	}
	nodeItems, err := store.ListDevicesForNode(ctx, "network:home", "peer-1")
	if err != nil || len(nodeItems) != 3 {
		t.Fatalf("节点范围筛选错误，应该包含 1 个节点设备和 2 个普通设备: %d %v", len(nodeItems), err)
	}
	groups, err := store.ListDeviceGroups(ctx)
	if err != nil || len(groups) != 1 || groups[0].DeviceCount != 1 {
		t.Fatalf("分组统计错误: %+v %v", groups, err)
	}
	updated, err := store.UpdateDevice(ctx, device.ID, DeviceInput{Name: "客厅电视盒子", DeviceType: "tv", IPAddress: "192.168.1.21"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "客厅电视盒子" || updated.GroupID != "" || updated.NetworkKey != "" {
		t.Fatalf("设备更新未正确覆盖关联字段: %+v", updated)
	}
	renamed, err := store.UpdateDeviceGroup(ctx, group.ID, "家庭娱乐")
	if err != nil || renamed.Name != "家庭娱乐" {
		t.Fatalf("分组重命名失败: %+v %v", renamed, err)
	}
	if err := store.DeleteDeviceGroup(ctx, group.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Device(ctx, device.ID); err != nil {
		t.Fatal("删除分组不应删除设备:", err)
	}
	if err := store.DeleteDevice(ctx, device.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Device(ctx, device.ID); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("设备删除失败: %v", err)
	}
}

func TestDeviceNodeScopeIndexMigrationAllowsMultipleAssetsPerNode(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE devices (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, group_id TEXT,
			device_type TEXT NOT NULL DEFAULT '通用设备', semantic_kinds TEXT NOT NULL DEFAULT '[]',
			is_node_device INTEGER NOT NULL DEFAULT 0, is_gateway INTEGER NOT NULL DEFAULT 0,
			manufacturer TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '', mac_address TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '', is_openwrt INTEGER NOT NULL DEFAULT 0,
			network_key TEXT NOT NULL DEFAULT '', peer_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_devices_network_node ON devices(network_key,peer_id)
			WHERE network_key<>'' AND peer_id<>''`,
		`INSERT INTO devices(id,name,device_type,is_node_device,network_key,peer_id,created_at,updated_at)
			VALUES('legacy-node','旧版节点设备','通用设备',1,'network:legacy','peer-legacy','2026-01-01','2026-01-01')`,
	} {
		if _, err := db.db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	child, err := store.CreateDevice(ctx, DeviceInput{
		Name: "迁移后新增设备", DeviceType: "电脑", NetworkKey: "network:legacy", PeerID: "peer-legacy",
	})
	if err != nil {
		t.Fatalf("旧版唯一索引迁移后，同一节点仍不能新增普通设备: %v", err)
	}
	if child.IsNodeDevice {
		t.Fatal("普通设备不应被自动标记为节点设备")
	}
	items, err := store.ListDevicesForNode(ctx, "network:legacy", "peer-legacy")
	if err != nil || len(items) != 2 {
		t.Fatalf("旧数据迁移后的节点范围查询错误: count=%d err=%v", len(items), err)
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO devices(id,name,device_type,is_node_device,network_key,peer_id,created_at,updated_at)
		VALUES('duplicate-node','重复节点设备','通用设备',1,'network:legacy','peer-legacy','2026-01-02','2026-01-02')`); err == nil {
		t.Fatal("迁移后应只允许每个网络节点有一条节点基础设备记录")
	}
}

func TestDeviceValidationAndOpenWrtCredentialSealing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []DeviceInput{
		{Name: "", DeviceType: "other"},
		{Name: "bad ip", DeviceType: "other", IPAddress: "not-an-ip"},
		{Name: "bad mac", DeviceType: "other", MACAddress: "invalid"},
		{Name: "invalid node link", DeviceType: "other", NetworkKey: "network:home"},
		{Name: "bad OpenWrt", DeviceType: "other", SemanticKinds: []string{"openwrt"}},
		{Name: "public OpenWrt", SemanticKinds: []string{"openwrt"}, IPAddress: "8.8.8.8"},
	} {
		if _, err := store.CreateDevice(ctx, input); !errors.Is(err, ErrInvalidDevice) {
			t.Fatalf("非法设备通过校验: %+v err=%v", input, err)
		}
	}
	normal, err := store.CreateDevice(ctx, DeviceInput{Name: "普通路由器", DeviceType: "router", IPAddress: "192.168.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeviceOpenWrtConfig(ctx, normal.ID); !errors.Is(err, ErrDeviceNotOpenWrt) {
		t.Fatalf("普通设备可读取 OpenWrt 配置: %v", err)
	}
	router, err := store.CreateDevice(ctx, DeviceInput{Name: "OpenWrt", SemanticKinds: []string{"openwrt"}, IPAddress: "192.168.1.2"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := store.SaveDeviceOpenWrtConfig(ctx, router.ID, 80, false, "easy-device", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if !config.Configured || config.Username != "easy-device" {
		t.Fatalf("OpenWrt 配置未保存: %+v", config)
	}
	secret, err := store.DeviceOpenWrtSecret(ctx, router.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret.Password != "secret-value" || secret.URL != "http://192.168.1.2:80/ubus" {
		t.Fatalf("OpenWrt 凭据解封错误: %+v", secret)
	}
	devices, err := store.ListDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devices {
		if d.ID == router.ID && !d.OpenWrtConfigured {
			t.Fatal("列表未显示已配置状态")
		}
	}
	if _, err := store.SaveDeviceOpenWrtConfig(ctx, router.ID, 8080, false, "easy-device", ""); !errors.Is(err, ErrInvalidOpenWrtConfig) {
		t.Fatal("修改配置却未重新提供密码:", err)
	}
	if _, err := store.SaveDeviceOpenWrtConfig(ctx, router.ID, 80, false, "easy-device", ""); err != nil {
		t.Fatal("相同配置应保留已保存凭据:", err)
	}
	if _, err := store.UpdateDevice(ctx, router.ID, DeviceInput{Name: "普通路由器", DeviceType: "router", IPAddress: "192.168.1.2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeviceOpenWrtConfig(ctx, router.ID); !errors.Is(err, ErrDeviceNotOpenWrt) {
		t.Fatalf("取消 OpenWrt 标识后仍可读取配置: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, ".device-openwrt-sealing-key"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("密钥文件权限错误: %v %v", info, err)
	}
}

func TestDeviceStoreDoesNotRegenerateMissingKeyWithSavedCredentials(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	router, err := store.CreateDevice(ctx, DeviceInput{
		Name: "OpenWrt", SemanticKinds: []string{"openwrt"}, IPAddress: "192.168.1.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveDeviceOpenWrtConfig(ctx, router.ID, 80, false, "easy-device", "secret-value"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".device-openwrt-sealing-key")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDeviceStore(ctx, db); err == nil {
		t.Fatal("凭据密钥丢失时不应生成新密钥并继续启动")
	}
}

func TestNetworkNodeDeviceSyncIsIdempotentAndPreservesDeletion(t *testing.T) {
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

	enabled, err := store.NetworkNodeSyncEnabled(ctx)
	if err != nil || enabled {
		t.Fatalf("自动补充初始状态错误: enabled=%v err=%v", enabled, err)
	}

	nodes := []NetworkNodeDeviceInput{
		{Name: "入口节点", NetworkKey: "network:home", PeerID: "peer-entry"},
		{Name: "远端节点", NetworkKey: "network:home", PeerID: "peer-remote"},
	}
	first, err := store.SyncNetworkNodeDevices(ctx, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created != 2 || first.Existing != 0 || !first.Enabled {
		t.Fatalf("首次初始化结果错误: %+v", first)
	}
	enabled, err = store.NetworkNodeSyncEnabled(ctx)
	if err != nil || !enabled {
		t.Fatalf("自动补充未持久化: enabled=%v err=%v", enabled, err)
	}

	second, err := store.SyncNetworkNodeDevices(ctx, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created != 0 || second.Existing != 2 {
		t.Fatalf("重复同步没有幂等: %+v", second)
	}
	items, err := store.ListDevices(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("设备数量错误: %d err=%v", len(items), err)
	}
	for _, item := range items {
		if !item.IsNodeDevice || item.IsGateway || item.DeviceType != "通用设备" {
			t.Fatalf("节点设备初始属性错误: %+v", item)
		}
	}

	var deletedID string
	for _, item := range items {
		if item.PeerID == "peer-entry" {
			deletedID = item.ID
		}
	}
	if deletedID == "" {
		t.Fatal("未找到待删除节点设备")
	}
	if err := store.DeleteDevice(ctx, deletedID); err != nil {
		t.Fatalf("应允许删除网络节点基础设备: %v", err)
	}
	preDeleted, err := store.CreateDevice(ctx, DeviceInput{
		Name: "提前删除的节点", DeviceType: "通用设备", NetworkKey: "network:home", PeerID: "peer-predeleted",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteDevice(ctx, preDeleted.ID); err != nil {
		t.Fatal(err)
	}

	withNew := append(nodes,
		NetworkNodeDeviceInput{Name: "新增节点", NetworkKey: "network:home", PeerID: "peer-new"},
		NetworkNodeDeviceInput{Name: "提前删除的节点", NetworkKey: "network:home", PeerID: "peer-predeleted"},
	)
	third, err := store.SyncNetworkNodeDevices(ctx, withNew)
	if err != nil {
		t.Fatal(err)
	}
	if third.Created != 2 || third.Existing != 1 || third.Skipped != 1 {
		t.Fatalf("自动补充新节点结果错误: %+v", third)
	}
	items, err = store.ListDevices(ctx)
	if err != nil || len(items) != 3 {
		t.Fatalf("节点基础设备或新节点创建结果错误: count=%d err=%v", len(items), err)
	}
	foundEntry, foundPreviouslyDeletedAssetScope := false, false
	for _, item := range items {
		if item.PeerID == "peer-entry" && item.IsNodeDevice {
			foundEntry = true
		}
		if item.PeerID == "peer-predeleted" && item.IsNodeDevice {
			foundPreviouslyDeletedAssetScope = true
		}
	}
	if foundEntry {
		t.Fatal("手动删除的节点基础设备不应被自动同步重新创建")
	}
	if !foundPreviouslyDeletedAssetScope {
		t.Fatal("删除普通设备不应阻止该节点基础设备后续自动补充")
	}

	custom, err := store.CreateDevice(ctx, DeviceInput{
		Name: "X86 软路由", DeviceType: "X86 软路由", SemanticKinds: []string{"router", "server"},
		IsGateway: true, NetworkKey: "network:home", PeerID: "peer-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !custom.IsGateway || custom.OwnedGatewayGroupID == "" || custom.GatewayGroupID != "" || len(custom.SemanticKinds) != 2 ||
		custom.SemanticKinds[0] != "router" || custom.SemanticKinds[1] != "server" {
		t.Fatalf("自定义类型/多语义标签/网关设备保存错误: %+v", custom)
	}
	child, err := store.CreateDevice(ctx, DeviceInput{
		Name: "客厅电视", DeviceType: "电视", GatewayGroupID: custom.OwnedGatewayGroupID,
		NetworkKey: "network:home", PeerID: "peer-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	childGateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "二级路由器", DeviceType: "路由器", IsGateway: true, GatewayGroupID: custom.OwnedGatewayGroupID,
		NetworkKey: "network:home", PeerID: "peer-new",
	})
	if err != nil || !childGateway.IsGateway || childGateway.OwnedGatewayGroupID == "" ||
		childGateway.GatewayGroupID != custom.OwnedGatewayGroupID {
		t.Fatalf("网关下挂设备也应能独立成为网关: %+v %v", childGateway, err)
	}
	grandchild, err := store.CreateDevice(ctx, DeviceInput{
		Name: "二级网关下的电视", GatewayGroupID: childGateway.OwnedGatewayGroupID,
		NetworkKey: "network:home", PeerID: "peer-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := store.ListGatewayGroups(ctx, "network:home", "peer-new")
	if err != nil || len(groups) != 2 {
		t.Fatalf("多层网关关系没有正确建立: %+v %v", groups, err)
	}
	for _, group := range groups {
		switch group.GatewayDeviceID {
		case custom.ID:
			if group.DeviceCount != 2 {
				t.Fatalf("一级网关下挂设备计数错误: %+v", group)
			}
		case childGateway.ID:
			if group.DeviceCount != 1 {
				t.Fatalf("二级网关下挂设备计数错误: %+v", group)
			}
		default:
			t.Fatalf("网关组所有者错误: %+v", group)
		}
	}
	if err := store.DeleteGatewayGroup(ctx, custom.OwnedGatewayGroupID); err != nil {
		t.Fatal(err)
	}
	updatedGateway, err := store.Device(ctx, custom.ID)
	if err != nil || updatedGateway.OwnedGatewayGroupID != "" || updatedGateway.IsGateway {
		t.Fatalf("关闭网关组后设备未安全转为普通设备: %+v %v", updatedGateway, err)
	}
	updatedChild, err := store.Device(ctx, child.ID)
	if err != nil || updatedChild.GatewayGroupID != "" {
		t.Fatalf("删除上级网关后普通下挂设备未提升为直属设备: %+v %v", updatedChild, err)
	}
	updatedChildGateway, err := store.Device(ctx, childGateway.ID)
	if err != nil || updatedChildGateway.GatewayGroupID != "" || !updatedChildGateway.IsGateway ||
		updatedChildGateway.OwnedGatewayGroupID == "" {
		t.Fatalf("删除上级网关后下级网关应保留自身网关能力: %+v %v", updatedChildGateway, err)
	}
	updatedGrandchild, err := store.Device(ctx, grandchild.ID)
	if err != nil || updatedGrandchild.GatewayGroupID != childGateway.OwnedGatewayGroupID {
		t.Fatalf("二级网关下挂设备关系不应被破坏: %+v %v", updatedGrandchild, err)
	}

	if _, err := store.CreateDevice(ctx, DeviceInput{Name: "非法标签", DeviceType: "测试", SemanticKinds: []string{"unknown-kind"}}); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("非法语义标签未被拒绝: %v", err)
	}
}

func TestNetworkNodePeerRotationKeepsDeviceIdentityAndChildren(t *testing.T) {
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
	initial := []NetworkNodeDeviceInput{{Name: "原始节点显示名", NetworkKey: "network:home", PeerID: "peer-old", Hostname: "FnOS"}}
	if result, err := store.SyncNetworkNodeDevices(ctx, initial); err != nil || result.Created != 1 {
		t.Fatalf("初始化失败: %+v %v", result, err)
	}
	original, err := store.ListNodeDevices(ctx)
	if err != nil || len(original) != 1 || original[0].NodeHostname != "FnOS" {
		t.Fatalf("未保存原始主机名: %+v %v", original, err)
	}
	originalID := original[0].ID
	lat, lng := 31.23, 121.47
	if _, err := db.SaveNodeAttributes(ctx, NodeAttributes{
		NetworkKey: "network:home", PeerID: "peer-old", Alias: "自定义节点别名",
		Location: NodeLocation{Latitude: &lat, Longitude: &lng},
	}); err != nil {
		t.Fatal(err)
	}
	gateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "保留的网关", IsGateway: true, NetworkKey: "network:home", PeerID: "peer-old",
		SemanticKinds: []string{"router", "openwrt"}, IPAddress: "192.168.10.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveDeviceOpenWrtConfig(ctx, gateway.ID, 80, false, "account", "secret"); err != nil {
		t.Fatal(err)
	}
	child, err := store.CreateDevice(ctx, DeviceInput{
		Name: "下挂设备", GatewayGroupID: gateway.OwnedGatewayGroupID,
		NetworkKey: "network:home", PeerID: "peer-old",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "新的显示名称不应覆盖原有", NetworkKey: "network:home", PeerID: "peer-new", Hostname: "fnos"},
	})
	if err != nil || result.Rebound != 1 || result.Created != 0 {
		t.Fatalf("相同原始主机名没有关联原节点: %+v %v", result, err)
	}
	nodes, err := store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 1 || nodes[0].ID != originalID ||
		nodes[0].PeerID != "peer-new" || nodes[0].Name != "原始节点显示名" {
		t.Fatalf("节点被重复创建或丢失自定义资料: %+v %v", nodes, err)
	}
	oldScope, err := store.ListDevicesForNode(ctx, "network:home", "peer-old")
	if err != nil || len(oldScope) != 0 {
		t.Fatalf("旧节点关联未迁出: %+v %v", oldScope, err)
	}
	newScope, err := store.ListDevicesForNode(ctx, "network:home", "peer-new")
	if err != nil || len(newScope) != 3 {
		t.Fatalf("节点下挂设备未同步换绑: %+v %v", newScope, err)
	}
	updatedChild, err := store.Device(ctx, child.ID)
	if err != nil || updatedChild.GatewayGroupID != gateway.OwnedGatewayGroupID {
		t.Fatalf("网关设备层级丢失: %+v %v", updatedChild, err)
	}
	groups, err := store.ListGatewayGroups(ctx, "network:home", "peer-new")
	if err != nil || len(groups) != 1 || groups[0].GatewayDeviceID != gateway.ID || groups[0].DeviceCount != 1 {
		t.Fatalf("网关组未跟随节点重新绑定: %+v %v", groups, err)
	}
	if secret, err := store.DeviceOpenWrtSecret(ctx, gateway.ID); err != nil || secret.Password != "secret" {
		t.Fatalf("保存的 OpenWrt 配置丢失: %+v %v", secret, err)
	}
	attrs, err := db.ListNodeAttributes(ctx)
	if err != nil || len(attrs) != 1 || attrs[0].PeerID != "peer-new" ||
		attrs[0].Hostname != "fnos" || attrs[0].Alias != "自定义节点别名" || attrs[0].Location.Latitude == nil {
		t.Fatalf("原有节点别名与地理信息没有继承: %+v %v", attrs, err)
	}
	again, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "其他显示名", NetworkKey: "network:home", PeerID: "peer-new", Hostname: "FnOS"},
	})
	if err != nil || again.Existing != 1 || again.Rebound != 0 {
		t.Fatalf("重复同步应保持幂等: %+v %v", again, err)
	}
}

func TestNetworkNodeHostnameLegacyMigrationAndAmbiguityGuards(t *testing.T) {
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
	// 旧版只有展示名称、没有 node_hostname，首次遇到同名原始 hostname 时应安全接管。
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "yun", NetworkKey: "network:alpha", PeerID: "old"},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.ListNodeDevices(ctx)
	if err != nil || len(before) != 1 || before[0].NodeHostname != "" {
		t.Fatalf("旧客户端兼容性异常: %+v %v", before, err)
	}
	result, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "展示名可变化", NetworkKey: "network:alpha", PeerID: "new", Hostname: "yun"},
	})
	if err != nil || result.Rebound != 1 {
		t.Fatalf("历史同名节点未被换绑: %+v %v", result, err)
	}
	after, err := store.ListNodeDevices(ctx)
	if err != nil || len(after) != 1 || after[0].ID != before[0].ID ||
		after[0].NodeHostname != "yun" || after[0].PeerID != "new" {
		t.Fatalf("旧记录迁移错误: %+v %v", after, err)
	}
	// 相同 hostname 允许存在于不同 network_key，不应该跨网络关联。
	result, err = store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "另一个网络", NetworkKey: "network:beta", PeerID: "beta-peer", Hostname: "yun"},
	})
	if err != nil || result.Created != 1 {
		t.Fatalf("跨网络同名节点误合并: %+v %v", result, err)
	}
	// 同一批数据报告两个不同 Peer ID 却使用相同原始主机名时不能猜测关联。
	if _, err = store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "节点 1", NetworkKey: "network:alpha", PeerID: "a", Hostname: "yun"},
		{Name: "节点 2", NetworkKey: "network:alpha", PeerID: "b", Hostname: "YUN"},
	}); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("相同网络同名节点冲突应拒绝: %v", err)
	}
	after, err = store.ListNodeDevices(ctx)
	if err != nil || len(after) != 2 {
		t.Fatalf("有歧义的同步不得改写数据: %+v %v", after, err)
	}
}

func TestNetworkNodeReconcileAlreadyCreatedLegacyDuplicate(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy-duplicates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	// 先模拟旧版同步：原始主机名没有独立保存，Peer ID 改变后产生多余的节点壳。
	old := []NetworkNodeDeviceInput{{Name: "FnOS", NetworkKey: "network:home", PeerID: "old"}}
	if _, err := store.SyncNetworkNodeDevices(ctx, old); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("初始化旧节点失败: %+v %v", nodes, err)
	}
	originalID := nodes[0].ID
	child, err := store.CreateDevice(ctx, DeviceInput{Name: "旧节点的下挂设备", NetworkKey: "network:home", PeerID: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "FnOS · 网络节点", NetworkKey: "network:home", PeerID: "new"},
	}); err != nil {
		t.Fatal(err)
	}
	nodes, err = store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 2 {
		t.Fatalf("未模拟出历史重复节点: %+v %v", nodes, err)
	}
	result, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "任意展示别名", NetworkKey: "network:home", PeerID: "new", Hostname: "FnOS"},
	})
	if err != nil || result.Rebound != 1 {
		t.Fatalf("未回收空白重复节点并沿用原历史设备: %+v %v", result, err)
	}
	nodes, err = store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 1 || nodes[0].ID != originalID || nodes[0].PeerID != "new" {
		t.Fatalf("原历史节点 ID 未保留: %+v %v", nodes, err)
	}
	c, err := store.Device(ctx, child.ID)
	if err != nil || c.PeerID != "new" {
		t.Fatalf("原有子设备丢失: %+v %v", c, err)
	}
}

func TestClearDeviceDataIsScopedAndDisablesAutomaticNodeSync(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	lat, lng := 31.23, 121.47
	if _, err := db.SaveNodeAttributes(ctx, NodeAttributes{
		NetworkKey: "network:home", PeerID: "peer-1", Alias: "保留节点属性",
		Location: NodeLocation{Latitude: &lat, Longitude: &lng},
	}); err != nil {
		t.Fatal(err)
	}
	group, err := store.CreateDeviceGroup(ctx, "家庭")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "网络节点", NetworkKey: "network:home", PeerID: "peer-1"},
	}); err != nil {
		t.Fatal(err)
	}
	router, err := store.CreateDevice(ctx, DeviceInput{
		Name: "OpenWrt", GroupID: group.ID, IsGateway: true,
		SemanticKinds: []string{"openwrt"}, IPAddress: "192.168.1.1",
		NetworkKey: "network:home", PeerID: "peer-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveDeviceOpenWrtConfig(ctx, router.ID, 80, false, "easy-device", "secret-value"); err != nil {
		t.Fatal(err)
	}
	controllerID := db.ControllerID()

	if err := store.ClearDeviceData(ctx); err != nil {
		t.Fatal(err)
	}
	devices, err := store.ListDevices(ctx)
	if err != nil || len(devices) != 0 {
		t.Fatalf("设备记录未清空: %+v %v", devices, err)
	}
	groups, err := store.ListDeviceGroups(ctx)
	if err != nil || len(groups) != 0 {
		t.Fatalf("设备分组未清空: %+v %v", groups, err)
	}
	gatewayGroups, err := store.ListGatewayGroups(ctx, "network:home", "peer-1")
	if err != nil || len(gatewayGroups) != 0 {
		t.Fatalf("网关组未清空: %+v %v", gatewayGroups, err)
	}
	enabled, err := store.NetworkNodeSyncEnabled(ctx)
	if err != nil || enabled {
		t.Fatalf("清空后自动补充仍启用: enabled=%v err=%v", enabled, err)
	}
	for table, want := range map[string]int{
		"device_openwrt_configs": 0,
		"network_node_sync_seen": 0,
	} {
		var count int
		if err := db.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != want {
			t.Fatalf("%s 未清空: count=%d err=%v", table, count, err)
		}
	}
	attributes, err := db.ListNodeAttributes(ctx)
	if err != nil || len(attributes) != 1 || attributes[0].Alias != "保留节点属性" {
		t.Fatalf("清除设备数据误删节点属性: %+v %v", attributes, err)
	}
	if db.ControllerID() != controllerID {
		t.Fatal("清除设备数据改变了控制端 ID")
	}
	if _, err := store.SyncNetworkNodeDevicesIfEnabled(ctx, []NetworkNodeDeviceInput{
		{Name: "网络节点", NetworkKey: "network:home", PeerID: "peer-1"},
	}); !errors.Is(err, ErrNetworkNodeAutoSyncDisabled) {
		t.Fatalf("清空后后台同步未被阻止: %v", err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "网络节点", NetworkKey: "network:home", PeerID: "peer-1"},
	}); err != nil {
		t.Fatalf("手动初始化应仍可重新创建节点设备: %v", err)
	}
}

func TestNewDeviceStoreDoesNotRewriteHistoricalDeviceRows(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, statement := range []string{
		`CREATE TABLE devices (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, group_id TEXT,
			device_type TEXT NOT NULL DEFAULT '通用设备', semantic_kinds TEXT NOT NULL DEFAULT '[]',
			is_node_device INTEGER NOT NULL DEFAULT 0, is_gateway INTEGER NOT NULL DEFAULT 0,
			manufacturer TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '', mac_address TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '', is_openwrt INTEGER NOT NULL DEFAULT 0,
			network_key TEXT NOT NULL DEFAULT '', peer_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`INSERT INTO devices(id,name,device_type,semantic_kinds,is_node_device,is_openwrt,network_key,peer_id,created_at,updated_at)
			VALUES('legacy','历史路由器','router','[]',0,1,'network:legacy','peer-legacy','2026-01-01','2026-01-01')`,
	} {
		if _, err := db.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.Device(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.IsNodeDevice || !hasSemanticKind(legacy.SemanticKinds, "openwrt") {
		t.Fatalf("历史 OpenWrt 标记应只在读取结果中兼容为语义标签: %+v", legacy)
	}
	var storedSemanticKinds string
	if err := db.db.QueryRowContext(ctx, "SELECT semantic_kinds FROM devices WHERE id='legacy'").Scan(&storedSemanticKinds); err != nil {
		t.Fatal(err)
	}
	if storedSemanticKinds != "[]" {
		t.Fatalf("启动时不应改写历史语义数据，实际值：%s", storedSemanticKinds)
	}
}

func TestDeviceGatewayOwnershipSupportsNestedGatewaysAndNodeDevices(t *testing.T) {
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
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "节点基础设备", NetworkKey: "network:home", PeerID: "peer-home"},
	}); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("节点基础设备初始化失败: %+v %v", nodes, err)
	}
	nodeGateway, err := store.UpdateDevice(ctx, nodes[0].ID, DeviceInput{
		Name: "节点基础设备", IsGateway: true, NetworkKey: "network:home", PeerID: "peer-home",
	})
	if err != nil || !nodeGateway.IsNodeDevice || !nodeGateway.IsGateway || nodeGateway.OwnedGatewayGroupID == "" {
		t.Fatalf("节点基础设备应能直接成为网关设备: %+v %v", nodeGateway, err)
	}

	childGateway, err := store.CreateDevice(ctx, DeviceInput{
		Name: "下挂路由器", IsGateway: true, GatewayGroupID: nodeGateway.OwnedGatewayGroupID,
		NetworkKey: "network:home", PeerID: "peer-home",
	})
	if err != nil || childGateway.OwnedGatewayGroupID == "" || childGateway.GatewayGroupID != nodeGateway.OwnedGatewayGroupID {
		t.Fatalf("下挂设备应能同时拥有自己的下挂层级: %+v %v", childGateway, err)
	}
	grandchild, err := store.CreateDevice(ctx, DeviceInput{
		Name: "客厅电视", GatewayGroupID: childGateway.OwnedGatewayGroupID,
		NetworkKey: "network:home", PeerID: "peer-home",
	})
	if err != nil {
		t.Fatal(err)
	}

	disabledNode, err := store.UpdateDevice(ctx, nodeGateway.ID, DeviceInput{
		Name: "节点基础设备", IsGateway: false, NetworkKey: "network:home", PeerID: "peer-home",
	})
	if err != nil || disabledNode.IsGateway || disabledNode.OwnedGatewayGroupID != "" {
		t.Fatalf("取消节点设备的网关属性失败: %+v %v", disabledNode, err)
	}
	movedChild, err := store.Device(ctx, childGateway.ID)
	if err != nil || movedChild.GatewayGroupID != "" || !movedChild.IsGateway || movedChild.OwnedGatewayGroupID == "" {
		t.Fatalf("取消节点设备网关属性后，下级网关应提升为直属设备: %+v %v", movedChild, err)
	}
	movedGrandchild, err := store.Device(ctx, grandchild.ID)
	if err != nil || movedGrandchild.GatewayGroupID != movedChild.OwnedGatewayGroupID {
		t.Fatalf("取消上级网关后更深层设备仍应归属于下级网关: %+v %v", movedGrandchild, err)
	}

	movedChild, err = store.UpdateDevice(ctx, childGateway.ID, DeviceInput{
		Name: "下挂路由器", IsGateway: true, NetworkKey: "network:other", PeerID: "peer-other",
	})
	if err != nil || movedChild.NetworkKey != "network:other" || movedChild.PeerID != "peer-other" {
		t.Fatalf("迁移普通网关设备失败: %+v %v", movedChild, err)
	}
	movedGrandchild, err = store.Device(ctx, grandchild.ID)
	if err != nil || movedGrandchild.NetworkKey != "network:other" || movedGrandchild.PeerID != "peer-other" {
		t.Fatalf("迁移网关时应递归迁移下挂设备: %+v %v", movedGrandchild, err)
	}
	groups, err := store.ListGatewayGroups(ctx, "network:other", "peer-other")
	if err != nil || len(groups) != 1 || groups[0].GatewayDeviceID != childGateway.ID || groups[0].DeviceCount != 1 {
		t.Fatalf("迁移后网关组范围和层级错误: %+v %v", groups, err)
	}
	disabledChild, err := store.UpdateDevice(ctx, childGateway.ID, DeviceInput{
		Name: "下挂路由器", IsGateway: false, NetworkKey: "network:other", PeerID: "peer-other",
	})
	if err != nil || disabledChild.IsGateway || disabledChild.OwnedGatewayGroupID != "" {
		t.Fatalf("取消普通网关设备属性失败: %+v %v", disabledChild, err)
	}
	movedGrandchild, err = store.Device(ctx, grandchild.ID)
	if err != nil || movedGrandchild.GatewayGroupID != "" {
		t.Fatalf("取消网关属性后，下挂设备应提升为直属设备: %+v %v", movedGrandchild, err)
	}
}

func TestLegacyGatewayGroupMigrationPromotesGatewayOwner(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy-gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE devices (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, group_id TEXT,
			gateway_group_id TEXT, device_type TEXT NOT NULL DEFAULT '通用设备',
			semantic_kinds TEXT NOT NULL DEFAULT '[]', is_node_device INTEGER NOT NULL DEFAULT 0,
			is_gateway INTEGER NOT NULL DEFAULT 0, manufacturer TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '', ip_address TEXT NOT NULL DEFAULT '',
			mac_address TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '',
			is_openwrt INTEGER NOT NULL DEFAULT 0, network_key TEXT NOT NULL DEFAULT '',
			peer_id TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE device_gateway_groups (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, network_key TEXT NOT NULL, peer_id TEXT NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_devices_gateway_group_gateway ON devices(gateway_group_id)
			WHERE gateway_group_id IS NOT NULL AND gateway_group_id<>'' AND is_gateway=1`,
		`INSERT INTO device_gateway_groups(id,name,network_key,peer_id,created_at,updated_at)
			VALUES('legacy-group','旧网关组','network:legacy','peer-legacy','2026-01-01','2026-01-01')`,
		`INSERT INTO devices(id,name,gateway_group_id,is_gateway,network_key,peer_id,created_at,updated_at)
			VALUES('legacy-gateway','旧网关','legacy-group',1,'network:legacy','peer-legacy','2026-01-01','2026-01-01')`,
		`INSERT INTO devices(id,name,gateway_group_id,is_gateway,network_key,peer_id,created_at,updated_at)
			VALUES('legacy-child','旧下挂设备','legacy-group',0,'network:legacy','peer-legacy','2026-01-01','2026-01-01')`,
	} {
		if _, err := db.db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := store.Device(ctx, "legacy-gateway")
	if err != nil || !gateway.IsGateway || gateway.GatewayGroupID != "" || gateway.OwnedGatewayGroupID != "legacy-group" {
		t.Fatalf("旧网关设备没有迁移为网关组所有者: %+v %v", gateway, err)
	}
	child, err := store.Device(ctx, "legacy-child")
	if err != nil || child.GatewayGroupID != "legacy-group" || child.GatewayGroupName != "旧网关" {
		t.Fatalf("旧下挂设备关系没有保留: %+v %v", child, err)
	}
	groups, err := store.ListGatewayGroups(ctx, "network:legacy", "peer-legacy")
	if err != nil || len(groups) != 1 || groups[0].GatewayDeviceID != "legacy-gateway" || groups[0].DeviceCount != 1 {
		t.Fatalf("旧网关组迁移后的所有者和设备数量错误: %+v %v", groups, err)
	}
}

func TestNodeAttributeHostnameBackfillOnUnchangedPeer(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "backfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SaveNodeAttributes(ctx, NodeAttributes{
		NetworkKey: "network:home", PeerID: "old-peer", Alias: "上海-腾讯云",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "yun", NetworkKey: "network:home", PeerID: "old-peer", Hostname: "yun"},
	}); err != nil {
		t.Fatal(err)
	}
	attrs, err := db.ListNodeAttributes(ctx)
	if err != nil || len(attrs) != 1 || attrs[0].Hostname != "yun" || attrs[0].Alias != "上海-腾讯云" {
		t.Fatalf("原 Peer ID 对应的手工节点属性应补全 hostname：%+v %v", attrs, err)
	}
}

func TestDeleteNodeBasePreservesOtherAssetsAndSuppressesPeerRotation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "delete-node.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "OWRT", NetworkKey: "network:home", PeerID: "peer-old", Hostname: "OWRT"},
	}); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("节点基础设备缺失: %+v %v", nodes, err)
	}
	gateway, err := store.UpdateDevice(ctx, nodes[0].ID, DeviceInput{
		Name: "上海-OWRT", NetworkKey: "network:home", PeerID: "peer-old",
		NodeHostname: "OWRT", IsGateway: true, SemanticKinds: []string{"router", "openwrt"},
		IPAddress: "192.168.10.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := store.CreateDevice(ctx, DeviceInput{
		Name: "下挂手机", NetworkKey: "network:home", PeerID: "peer-old",
		GatewayGroupID: gateway.OwnedGatewayGroupID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteDevice(ctx, gateway.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Device(ctx, gateway.ID); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("基础设备应删除: %v", err)
	}
	remaining, err := store.Device(ctx, child.ID)
	if err != nil || remaining.GatewayGroupID != "" || remaining.NetworkKey != "network:home" ||
		remaining.PeerID != "peer-old" || remaining.NodeHostname != "OWRT" {
		t.Fatalf("删除节点基础网关不得删除子设备及所属主机名: %+v %v", remaining, err)
	}
	result, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "OWRT", NetworkKey: "network:home", PeerID: "peer-new", Hostname: "OWRT"},
	})
	if err != nil || result.Created != 0 || result.Skipped != 1 {
		t.Fatalf("Peer ID 变化后不能重新创建手动删除的基础设备: %+v %v", result, err)
	}
	nodes, err = store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 0 {
		t.Fatalf("手动删除的节点基础设备仍存在: %+v %v", nodes, err)
	}
}

func TestMoveNodeBaseToSelectedHostnamePreservesGatewayAndChildren(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "move-node.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "历史上海-OWRT", NetworkKey: "network:home", PeerID: "old-peer", Hostname: "old-owrt"},
		{Name: "当前 OWRT", NetworkKey: "network:home", PeerID: "new-peer", Hostname: "OWRT"},
	}); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListNodeDevices(ctx)
	if err != nil || len(nodes) != 2 {
		t.Fatalf("需要两个独立节点作为初始数据: %+v %v", nodes, err)
	}
	var historical, fresh Device
	for _, node := range nodes {
		if node.NodeHostname == "old-owrt" {
			historical = node
		} else if node.NodeHostname == "OWRT" {
			fresh = node
		}
	}
	if historical.ID == "" || fresh.ID == "" {
		t.Fatal("找不到旧节点或新节点")
	}
	gateway, err := store.UpdateDevice(ctx, historical.ID, DeviceInput{
		Name: "上海-OWRT", IsGateway: true, NetworkKey: "network:home", PeerID: "old-peer",
		NodeHostname: "old-owrt", SemanticKinds: []string{"openwrt", "router"}, IPAddress: "192.168.10.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveDeviceOpenWrtConfig(ctx, gateway.ID, 80, false, "user", "password"); err != nil {
		t.Fatal(err)
	}
	child, err := store.CreateDevice(ctx, DeviceInput{
		Name: "手机", NetworkKey: "network:home", PeerID: "old-peer", GatewayGroupID: gateway.OwnedGatewayGroupID,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 目标已存在基础设备时禁止覆盖或静默删除。
	_, err = store.UpdateDevice(ctx, gateway.ID, DeviceInput{
		Name: "上海-OWRT", NetworkKey: "network:home", PeerID: "new-peer",
		NodeHostname: "OWRT", IsGateway: true, SemanticKinds: []string{"openwrt", "router"},
		IPAddress: "192.168.10.1",
	})
	if !errors.Is(err, ErrDeviceNodeConflict) {
		t.Fatalf("目标节点占用应返回安全冲突，而不是覆盖: %v", err)
	}
	if err := store.DeleteDevice(ctx, fresh.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := store.UpdateDevice(ctx, gateway.ID, DeviceInput{
		Name: "上海-OWRT", NetworkKey: "network:home", PeerID: "new-peer",
		NodeHostname: "OWRT", IsGateway: true, SemanticKinds: []string{"openwrt", "router"},
		IPAddress: "192.168.10.1",
	})
	if err != nil || moved.ID != historical.ID || moved.NodeHostname != "OWRT" ||
		moved.PeerID != "new-peer" || !moved.IsNodeDevice || !moved.IsGateway {
		t.Fatalf("基础设备改绑失败或丢失身份与网关属性: %+v %v", moved, err)
	}
	remaining, err := store.Device(ctx, child.ID)
	if err != nil || remaining.NetworkKey != "network:home" || remaining.PeerID != "new-peer" ||
		remaining.NodeHostname != "OWRT" || remaining.GatewayGroupID != gateway.OwnedGatewayGroupID {
		t.Fatalf("移动节点时下挂设备应跟随归属并绑定新原主机名: %+v %v", remaining, err)
	}
	groups, err := store.ListGatewayGroups(ctx, "network:home", "new-peer")
	if err != nil || len(groups) != 1 || groups[0].GatewayDeviceID != gateway.ID || groups[0].DeviceCount != 1 {
		t.Fatalf("迁移后网关组层级错误: %+v %v", groups, err)
	}
	cfg, err := store.DeviceOpenWrtSecret(ctx, gateway.ID)
	if err != nil || cfg.Password != "password" {
		t.Fatalf("迁移后 OpenWrt 密钥丢失: %+v %v", cfg, err)
	}
	result, err := store.SyncNetworkNodeDevices(ctx, []NetworkNodeDeviceInput{
		{Name: "OWRT", NetworkKey: "network:home", PeerID: "new-peer", Hostname: "OWRT"},
	})
	if err != nil || result.Existing != 1 || result.Skipped != 0 {
		t.Fatalf("手工改绑后应解除目标同步抑制: %+v %v", result, err)
	}
}

