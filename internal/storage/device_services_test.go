package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestDeviceServiceMigrationAndState(t *testing.T) {
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
	device, err := store.CreateDevice(ctx, DeviceInput{Name: "设备"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := store.CreateDeviceService(ctx, device.ID, DeviceServiceInput{Name: "服务", Protocols: []string{"tcp"}, Port: 80})
	if err != nil {
		t.Fatal(err)
	}
	// 模拟升级前的表。既有服务仍能读取，转发与导航均默认为关闭。
	if _, err := db.db.ExecContext(ctx, "ALTER TABLE device_services DROP COLUMN forward_config"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, "ALTER TABLE device_services DROP COLUMN navigation_enabled"); err != nil {
		t.Fatal(err)
	}
	store, err = NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.DeviceService(ctx, device.ID, service.ID)
	if err != nil || legacy.ID != service.ID || legacy.Forward != nil || legacy.NavigationEnabled {
		t.Fatalf("旧服务迁移错误: %+v %v", legacy, err)
	}
	cfg := &DeviceServiceForward{Enabled: true, HostID: "host", InstanceID: "instance", Listen: "0.0.0.0:18080",
		Destination: "10.10.30.162:80", Protocols: []string{"tcp"}}
	enabled, err := store.SaveDeviceServiceForward(ctx, device.ID, service.ID, cfg)
	if err != nil || enabled.Forward == nil || !enabled.Forward.Enabled || enabled.Forward.Destination != cfg.Destination {
		t.Fatalf("启用状态未保存在服务记录: %+v %v", enabled, err)
	}
	pinned, err := store.SetDeviceServiceNavigation(ctx, device.ID, service.ID, true)
	if err != nil || !pinned.NavigationEnabled || pinned.Forward == nil || !pinned.Forward.Enabled {
		t.Fatalf("保存导航时丢失转发配置或导航状态: %+v %v", pinned, err)
	}
	store, err = NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := store.ListNavigationServices(ctx)
	if err != nil || len(persisted) != 1 || persisted[0].DeviceID != device.ID || persisted[0].ID != service.ID {
		t.Fatalf("重新初始化后未能读取已开启导航的服务: %+v %v", persisted, err)
	}
	input := service.DeviceServiceInput
	input.Port = 81
	if _, err := store.UpdateDeviceService(ctx, device.ID, service.ID, input); !errors.Is(err, ErrDeviceServiceForwardEnabled) {
		t.Fatalf("启用时未保护正在转发的服务端口: %v", err)
	}
	cfg.Enabled = false
	disabled, err := store.SaveDeviceServiceForward(ctx, device.ID, service.ID, cfg)
	if err != nil || !disabled.NavigationEnabled || disabled.Forward == nil || disabled.Forward.Enabled || disabled.Forward.Listen != cfg.Listen {
		t.Fatalf("转发失效没有保留原规则信息: %+v %v", disabled, err)
	}
}
