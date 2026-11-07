package hosts

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

// TestRealAgentHTTPInterop 启动真实 Agent HTTP 服务，验证配对错误码、有效码和已使用码。
func TestRealAgentHTTPInterop(t *testing.T) {
	binary := os.Getenv("EASYNETTOPO_TEST_AGENT_BINARY")
	if binary == "" {
		t.Skip("仅在跨仓库集成测试中运行")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "agent-state")
	softwareRoot := filepath.Join(t.TempDir(), "software")
	for _, dir := range []string{dataDir, softwareRoot} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"EASYHOSTAGENT_DATA_DIR="+dataDir,
		"EASYHOSTAGENT_SOFTWARE_ROOT="+softwareRoot,
		"EASYHOSTAGENT_LISTEN_ADDRESS="+address,
		"EASYHOSTAGENT_TRANSPORT=http",
	)
	cli := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(absolute, args...)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Agent 本地命令执行失败：%v", err)
		}
		return string(output)
	}
	identity := cli("-identity")
	agentID := mustCLIValue(t, identity, "Agent ID", realAgentIDLine)
	pin := mustCLIValue(t, identity, "公钥指纹", realSPKILine)
	cmd := exec.Command(absolute)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	endpoint := "http://" + address
	client, err := NewClient(endpoint, agentID, pin)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		_, err = client.Identity(ctx)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("真实 HTTP Agent 身份接口不可用", ctx.Err())
		}
		time.Sleep(150 * time.Millisecond)
	}
	db, err := storage.Open(filepath.Join(t.TempDir(), "controller", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	host, err := svc.Register(ctx, "HTTP 测试主机", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	code := mustCLIValue(t, cli("-pairing-code"), "一次性配对码", realCodeLine)
	wrong := strings.Repeat("0", 32)
	if wrong == code {
		wrong = strings.Repeat("f", 32)
	}
	if _, err := svc.Pair(ctx, host.ID, agentID, pin, wrong); DiagnosePairing(err).Code != "invalid_pairing_code" {
		t.Fatalf("错误配对码没有返回精确原因：%+v", DiagnosePairing(err))
	}
	if _, err := svc.Pair(ctx, host.ID, agentID, pin, code); err != nil {
		t.Fatal("真实 HTTP 配对失败", err)
	}
	paired, err := svc.hosts.Get(ctx, host.ID)
	if err != nil || paired.AgentID != agentID {
		t.Fatal("真实 HTTP 配对未持久化主机绑定", err)
	}
	if _, err := svc.Pair(ctx, host.ID, agentID, pin, code); DiagnosePairing(err).Code != "pairing_window_closed" {
		t.Fatalf("已使用配对码没有返回精确原因：%+v", DiagnosePairing(err))
	}
	if err := svc.RequireManifestVersion(ctx, host.ID, 2); err != nil {
		t.Fatal("真实 Agent 未声明应用配置 v2 能力", err)
	}
	if _, err := svc.Query(ctx, host.ID, "host"); err != nil {
		t.Fatal("HTTP 配对后的鉴权查询失败", err)
	}
}
