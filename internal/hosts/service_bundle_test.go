package hosts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestCacheBundleRefusesUnpairedHostsAndForgedDigest(t *testing.T) {
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
	host, err := service.Register(ctx, "Unpaired Host", "127.0.0.1:49200")
	if err != nil {
		t.Fatal(err)
	}
	archive := []byte("not even a ZIP")
	hash := sha256.Sum256(archive)
	digest := hex.EncodeToString(hash[:])
	if _, err := service.CacheBundle(ctx, host.ID, strings.Repeat("0", 64), archive); err == nil {
		t.Fatal("未验证来源摘要就发送候选包")
	}
	if _, err := service.CacheBundle(ctx, host.ID, digest, archive); err == nil {
		t.Fatal("未配对主机收到了缓存请求")
	}
	if _, err := service.CacheBundle(ctx, "not-a-host-id", digest, archive); err == nil {
		t.Fatal("非法主机 ID 被允许")
	}
	if _, err := service.CacheBundle(ctx, host.ID, digest, nil); err == nil {
		t.Fatal("空候选包被允许")
	}
}
