package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostTokenEncryptedAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data", "app.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hosts, err := NewHostStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	h, err := hosts.Create(ctx, "FnOS", "127.0.0.1:7014")
	if err != nil {
		t.Fatal(err)
	}
	agentID := "11111111-1111-4111-8111-111111111111"
	pin := strings.Repeat("a", 64)
	token := strings.Repeat("b", 64)
	if err := hosts.SavePairing(ctx, h, agentID, pin, token); err != nil {
		t.Fatal(err)
	}
	assigned, err := hosts.Get(ctx, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if assigned.AgentID != agentID || assigned.SPKISHA256 != pin {
		t.Fatal("绑定身份不正确")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(token)) {
		t.Fatal("Token 明文被写入 SQLite")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hosts, err = NewHostStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := hosts.Credentials(ctx, assigned)
	if err != nil || decrypted != token {
		t.Fatal("重启后无法解密 Agent Token", err)
	}
	keyInfo, err := os.Stat(filepath.Join(filepath.Dir(path), ".agent-sealing-key"))
	if err != nil || keyInfo.Mode().Perm() != 0600 {
		t.Fatal("密钥文件权限不是 0600", err)
	}
	if err := hosts.AssertAgentNotBoundElsewhere(ctx, "another-host", agentID); err == nil {
		t.Fatal("没有拒绝同一个 Agent 绑定第二台主机")
	}
}

func TestDeletePairedHostOnlyRemovesLocalRecord(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "data", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hosts, err := NewHostStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	first, err := hosts.Create(ctx, "删除目标", "http://127.0.0.1:7018")
	if err != nil {
		t.Fatal(err)
	}
	other, err := hosts.Create(ctx, "保留目标", "http://127.0.0.1:7019")
	if err != nil {
		t.Fatal(err)
	}
	agentID := "11111111-1111-4111-8111-111111111111"
	pin := strings.Repeat("a", 64)
	token := strings.Repeat("b", 64)
	if err := hosts.SavePairing(ctx, first, agentID, pin, token); err != nil {
		t.Fatal(err)
	}
	if err := hosts.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := hosts.Get(ctx, first.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatal("本地登记和密封凭据应随主机一起删除", err)
	}
	if _, err := hosts.Get(ctx, other.ID); err != nil {
		t.Fatal("删除不能影响其他主机", err)
	}
	if err := hosts.AssertAgentNotBoundElsewhere(ctx, other.ID, agentID); err != nil {
		t.Fatal("已删除的主机不应继续占用控制端 Agent 唯一绑定", err)
	}
	if err := hosts.Delete(ctx, first.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatal("重复删除应返回主机不存在", err)
	}
	if updated, err := hosts.UpdateMetadata(ctx, other.ID, "新名称", "http://127.0.0.1:7020"); err != nil || updated.Name != "新名称" {
		t.Fatal("剩余主机应仍可编辑", err)
	}
}

func TestUpdatePairedHostPreservesCredentials(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "data", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hosts, err := NewHostStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	h, err := hosts.Create(ctx, "fnOS", "http://127.0.0.1:7018")
	if err != nil {
		t.Fatal(err)
	}
	agentID := "11111111-1111-4111-8111-111111111111"
	pin := strings.Repeat("a", 64)
	token := strings.Repeat("b", 64)
	if err := hosts.SavePairing(ctx, h, agentID, pin, token); err != nil {
		t.Fatal(err)
	}
	before, err := hosts.Get(ctx, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sealedBefore []byte
	if err := db.db.QueryRowContext(ctx, "SELECT token_sealed FROM hosts WHERE id=?", h.ID).Scan(&sealedBefore); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, endpoint string
	}{
		// Same values are an idempotent edit.
		{"fnOS", "http://127.0.0.1:7018"},
		// Metadata change must not re-pair.
		{"家用 fnOS", "http://127.0.0.2:7018"},
	} {
		updated, err := hosts.UpdateMetadata(ctx, h.ID, tc.name, tc.endpoint)
		if err != nil {
			t.Fatal(err)
		}
		if updated.ID != before.ID || updated.Name != tc.name || updated.Endpoint != tc.endpoint ||
			updated.AgentID != before.AgentID || updated.SPKISHA256 != before.SPKISHA256 ||
			updated.PairedAt != before.PairedAt {
			t.Fatalf("编辑修改了配对记录或未更新名称/地址：before=%+v updated=%+v", before, updated)
		}
		actualToken, err := hosts.Credentials(ctx, updated)
		if err != nil || actualToken != token {
			t.Fatalf("编辑导致已有 Token 丢失或重置: %v", err)
		}
		var sealedAfter []byte
		if err := db.db.QueryRowContext(ctx, "SELECT token_sealed FROM hosts WHERE id=?", h.ID).Scan(&sealedAfter); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(sealedBefore, sealedAfter) {
			t.Fatal("编辑不得重新加密或修改已保存的 Token")
		}
	}
	if _, err := hosts.UpdateMetadata(ctx, "00000000-0000-4000-8000-000000000000", "missing", "http://127.0.0.1:7018"); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("不存在的主机不应被自动创建: %v", err)
	}
}
