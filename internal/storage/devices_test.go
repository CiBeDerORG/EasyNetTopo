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
		{Name: "bad OpenWrt", DeviceType: "other", IsOpenWrt: true, IPAddress: "192.168.1.1"},
		{Name: "public OpenWrt", DeviceType: "router", IsOpenWrt: true, IPAddress: "8.8.8.8"},
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
	router, err := store.CreateDevice(ctx, DeviceInput{Name: "OpenWrt", DeviceType: "router", IPAddress: "192.168.1.2", IsOpenWrt: true})
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
		Name: "OpenWrt", DeviceType: "router", IPAddress: "192.168.1.1", IsOpenWrt: true,
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
	if err := store.DeleteDevice(ctx, deletedID); !errors.Is(err, ErrNodeDeviceProtected) {
		t.Fatalf("不允许删除网络节点基础设备: %v", err)
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
	if third.Created != 2 || third.Existing != 2 || third.Skipped != 0 {
		t.Fatalf("自动补充新节点结果错误: %+v", third)
	}
	items, err = store.ListDevices(ctx)
	if err != nil || len(items) != 4 {
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
	if !foundEntry {
		t.Fatal("节点基础设备在同步后丢失")
	}
	if !foundPreviouslyDeletedAssetScope {
		t.Fatal("删除普通设备不应阻止该节点基础设备后续自动补充")
	}

	gatewayGroup, err := store.CreateGatewayGroup(ctx, "家庭网络", "network:home", "peer-new")
	if err != nil {
		t.Fatal(err)
	}
	custom, err := store.CreateDevice(ctx, DeviceInput{
		Name: "X86 软路由", DeviceType: "X86 软路由", SemanticKinds: []string{"router", "server"},
		IsGateway: true, GatewayGroupID: gatewayGroup.ID, NetworkKey: "network:home", PeerID: "peer-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !custom.IsGateway || custom.GatewayGroupID != gatewayGroup.ID || len(custom.SemanticKinds) != 2 ||
		custom.SemanticKinds[0] != "router" || custom.SemanticKinds[1] != "server" {
		t.Fatalf("自定义类型/多语义标签/网关自身设备保存错误: %+v", custom)
	}
	child, err := store.CreateDevice(ctx, DeviceInput{
		Name: "客厅电视", DeviceType: "电视", GatewayGroupID: gatewayGroup.ID,
		NetworkKey: "network:home", PeerID: "peer-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := store.ListGatewayGroups(ctx, "network:home", "peer-new")
	if err != nil || len(groups) != 1 || groups[0].DeviceCount != 2 || groups[0].GatewayDeviceID != custom.ID {
		t.Fatalf("网关组设备统计错误: %+v %v", groups, err)
	}
	if _, err := store.CreateDevice(ctx, DeviceInput{
		Name: "第二台网关", DeviceType: "路由器", IsGateway: true, GatewayGroupID: gatewayGroup.ID,
		NetworkKey: "network:home", PeerID: "peer-new",
	}); !errors.Is(err, ErrGatewayDeviceConflict) {
		t.Fatalf("同一网关组允许绑定多个网关自身设备: %v", err)
	}
	if err := store.DeleteGatewayGroup(ctx, gatewayGroup.ID); err != nil {
		t.Fatal(err)
	}
	updatedGateway, err := store.Device(ctx, custom.ID)
	if err != nil || updatedGateway.GatewayGroupID != "" || updatedGateway.IsGateway {
		t.Fatalf("删除网关组后设备未安全转为直属设备: %+v %v", updatedGateway, err)
	}
	updatedChild, err := store.Device(ctx, child.ID)
	if err != nil || updatedChild.GatewayGroupID != "" {
		t.Fatalf("删除网关组后下挂设备未保留: %+v %v", updatedChild, err)
	}
	if _, err := store.CreateDevice(ctx, DeviceInput{Name: "非法标签", DeviceType: "测试", SemanticKinds: []string{"unknown-kind"}}); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("非法语义标签未被拒绝: %v", err)
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
	gatewayGroup, err := store.CreateGatewayGroup(ctx, "家庭网关", "network:home", "peer-1")
	if err != nil {
		t.Fatal(err)
	}
	router, err := store.CreateDevice(ctx, DeviceInput{
		Name: "OpenWrt", GroupID: group.ID, GatewayGroupID: gatewayGroup.ID,
		DeviceType: "router", SemanticKinds: []string{"router", "openwrt"},
		IsOpenWrt: true, IPAddress: "192.168.1.1",
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
	if legacy.IsNodeDevice || len(legacy.SemanticKinds) != 0 {
		t.Fatalf("启动时不应自动改写历史设备标识或类型标签: %+v", legacy)
	}
}
