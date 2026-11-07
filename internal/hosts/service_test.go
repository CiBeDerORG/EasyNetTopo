package hosts

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestUpdateValidatesEndpointAndPersistsValidMetadata(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "state", "controller.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	host, err := service.Register(ctx, "Mac mini", "127.0.0.1:49200")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.Update(ctx, host.ID, "Mac mini m6", "http://host.docker.internal:7018")
	if err != nil {
		t.Fatalf("有效的 HTTP Agent 地址应允许保存：%v", err)
	}
	if updated.Name != "Mac mini m6" || updated.Endpoint != "http://host.docker.internal:7018" {
		t.Fatalf("主机资料未正确保存：%+v", updated)
	}

	current, err := service.hosts.Get(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Endpoint != "http://host.docker.internal:7018" {
		t.Fatalf("主机地址未持久化，实际为 %q", current.Endpoint)
	}
	if current.AgentID != "" || current.SPKISHA256 != "" {
		t.Fatalf("修改主机资料不应改变 Agent 身份：%+v", current)
	}
}
