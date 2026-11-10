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
