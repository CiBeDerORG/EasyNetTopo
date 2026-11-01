package storage

import (
	"bytes"
	"context"
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
