package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestNetworkNodeAttributesPersistAndValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "app.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	lat, lng := 31.23, 121.47
	input := NodeAttributes{
		NetworkKey: "home", PeerID: "12345", Alias: "客厅电脑", HasPublicIP: true, PublicIP: "8.8.8.8",
		Location: NodeLocation{Country: "中国", Province: "上海市", City: "上海市", County: "黄浦区", Latitude: &lat, Longitude: &lng},
	}
	if _, err := db.SaveNodeAttributes(ctx, input); err != nil {
		t.Fatal(err)
	}
	updated := input
	updated.Alias = "新名字"
	if _, err := db.SaveNodeAttributes(ctx, updated); err != nil {
		t.Fatal(err)
	}
	got, err := db.ListNodeAttributes(ctx)
	if err != nil || len(got) != 1 || got[0].Alias != "新名字" ||
		got[0].Location.Latitude == nil || *got[0].Location.Latitude != lat {
		t.Fatalf("更新节点属性错误: %+v, %v", got, err)
	}
	for _, bad := range []NodeAttributes{
		{PeerID: "1"},
		{NetworkKey: "home", PeerID: "—"},
		{NetworkKey: "home", PeerID: "1", HasPublicIP: true, PublicIP: "127.0.0.1"},
		{NetworkKey: "home", PeerID: "1", Location: NodeLocation{Latitude: &lat}},
	} {
		if _, err := db.SaveNodeAttributes(ctx, bad); !errors.Is(err, ErrInvalidNodeAttributes) {
			t.Fatalf("无效节点属性未拒绝: %+v, %v", bad, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err = reopened.ListNodeAttributes(ctx)
	if err != nil || len(got) != 1 || got[0].Alias != "新名字" || got[0].PublicIP != "8.8.8.8" {
		t.Fatalf("重启后属性丢失: %+v, %v", got, err)
	}
}

func TestDeleteNodeAttributesOnlyClearsNodeAttributeRecords(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	lat, lng := 31.23, 121.47
	if _, err := db.SaveNodeAttributes(ctx, NodeAttributes{
		NetworkKey: "network:home", PeerID: "peer-1", Alias: "客厅节点",
		Location: NodeLocation{Latitude: &lat, Longitude: &lng},
	}); err != nil {
		t.Fatal(err)
	}
	deviceStore, err := NewDeviceStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deviceStore.CreateDevice(ctx, DeviceInput{Name: "保留设备"}); err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteNodeAttributes(ctx); err != nil {
		t.Fatal(err)
	}
	attributes, err := db.ListNodeAttributes(ctx)
	if err != nil || len(attributes) != 0 {
		t.Fatalf("节点属性未清空: %+v %v", attributes, err)
	}
	devices, err := deviceStore.ListDevices(ctx)
	if err != nil || len(devices) != 1 || devices[0].Name != "保留设备" {
		t.Fatalf("清除节点属性误删设备: %+v %v", devices, err)
	}
}

func TestNodeAttributesOriginalHostnameSurvivesPeerRotation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "node-attributes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	lat, lng := 30.5, 120.5
	original := NodeAttributes{
		NetworkKey: "network:home", PeerID: "old-peer", Hostname: "FnOS", Alias: "家用 NAS",
		Location: NodeLocation{Latitude: &lat, Longitude: &lng},
	}
	if _, err = db.SaveNodeAttributes(ctx, original); err != nil {
		t.Fatal(err)
	}
	original.PeerID = "new-peer"
	if _, err = db.SaveNodeAttributes(ctx, original); err != nil {
		t.Fatalf("相同原始 hostname 应当更新运行时 Peer ID: %v", err)
	}
	attributes, err := db.ListNodeAttributes(ctx)
	if err != nil || len(attributes) != 1 || attributes[0].PeerID != "new-peer" ||
		attributes[0].Hostname != "FnOS" || attributes[0].Alias != "家用 NAS" {
		t.Fatalf("节点属性换绑后不应重复或丢失: %+v %v", attributes, err)
	}
	// 同网络内的相同原始主机名不应登记为两个不同身份。
	if _, err = db.SaveNodeAttributes(ctx, NodeAttributes{
		NetworkKey: "network:home", PeerID: "other-peer", Hostname: "fnos", Alias: "重复主机",
	}); err == nil {
		// 现有记录可以自动跟踪新的 Peer ID，但不能同时留下两个 hostname。
		attributes, err = db.ListNodeAttributes(ctx)
		if err != nil || len(attributes) != 1 || attributes[0].PeerID != "other-peer" {
			t.Fatalf("同主机名换绑不应创建重复属性: %+v %v", attributes, err)
		}
	}
}
