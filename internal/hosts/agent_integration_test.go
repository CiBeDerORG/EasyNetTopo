package hosts

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

var (
	realAgentIDLine = regexp.MustCompile(`(?m)^Agent ID: ([0-9a-f-]{36})$`)
	realSPKILine    = regexp.MustCompile(`(?m)^TLS SPKI SHA-256: ([0-9a-f]{64})$`)
	realCodeLine    = regexp.MustCompile(`(?m)^One-time pairing code: ([0-9a-f]{32})$`)
)

func mustCLIValue(t *testing.T, output, name string, pattern *regexp.Regexp) string {
	t.Helper()
	m := pattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(m) != 2 {
		t.Fatalf("%s is missing from real Agent CLI output", name)
	}
	return m[1]
}

// TestRealAgentInterop uses a *real compiled EasyHostAgent*, not httptest.
// It runs only in the explicit cross-repository CI job. No Docker writes,
// secrets from the runner, privileged operations or NAS access are required.
func TestRealAgentInterop(t *testing.T) {
	binary := os.Getenv("EASYNETTOPO_TEST_AGENT_BINARY")
	if binary == "" {
		t.Skip("set EASYNETTOPO_TEST_AGENT_BINARY to run cross-repository integration")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "agent-state")
	softwareRoot := filepath.Join(t.TempDir(), "apps")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(softwareRoot, 0700); err != nil {
		t.Fatal(err)
	}
	available, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := available.Addr().String()
	if err := available.Close(); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"EASYHOSTAGENT_DATA_DIR="+dataDir,
		"EASYHOSTAGENT_SOFTWARE_ROOT="+softwareRoot,
		"EASYHOSTAGENT_LISTEN_ADDRESS="+endpoint,
	)
	cli := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(absolute, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("real Agent local CLI %v failed: %v (output suppressed)", args, err)
		}
		return string(out)
	}
	identityOutput := cli("-identity")
	agentID := mustCLIValue(t, identityOutput, "Agent ID", realAgentIDLine)
	pin := mustCLIValue(t, identityOutput, "TLS SPKI SHA-256", realSPKILine)
	if err := ValidateIdentity(agentID, pin); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(absolute)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	client, err := NewClient(endpoint, agentID, pin)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var identity Identity
	for {
		identity, err = client.Identity(ctx)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("real Agent HTTPS identity not ready: %v", ctx.Err())
		}
		time.Sleep(150 * time.Millisecond)
	}
	if identity.AgentID != agentID || identity.SPKISHA256 != pin || identity.ProtocolVersion != 1 {
		t.Fatal("real Agent CLI and HTTPS identities are not identical")
	}
	// A pinned but false fingerprint must not be accepted, even when identity
	// JSON is readable via the network.
	wrong, err := NewClient(endpoint, agentID, strings.Repeat("0", 64))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.Identity(ctx); err == nil {
		t.Fatal("real Agent accepted an incorrect TLS public key fingerprint")
	}
	db, err := storage.Open(filepath.Join(t.TempDir(), "controller", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	host, err := service.Register(ctx, "Integration Agent", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	codeOutput := cli("-pairing-code")
	code := mustCLIValue(t, codeOutput, "one-time code", realCodeLine)
	if fromCLI := mustCLIValue(t, codeOutput, "repeated Agent ID", realAgentIDLine); fromCLI != agentID {
		t.Fatal("Agent identity changed when opening pairing window")
	}
	paired, err := service.Pair(ctx, host.ID, agentID, pin, code)
	if err != nil {
		t.Fatalf("real Agent pairing failed: %v", err)
	}
	if paired.AgentID != agentID || paired.SPKISHA256 != pin {
		t.Fatal("persisted real Agent binding is inconsistent")
	}
	token, err := service.hosts.Credentials(ctx, paired)
	if err != nil {
		t.Fatal(err)
	}
	hostRaw, err := service.Query(ctx, host.ID, "host")
	if err != nil {
		t.Fatalf("real Agent /v1/host request failed: %v", err)
	}
	if !strings.Contains(string(hostRaw), softwareRoot) {
		t.Fatal("Agent host snapshot did not preserve configured software root")
	}
	inventoryRaw, err := service.Query(ctx, host.ID, "docker")
	if err != nil {
		t.Fatalf("real Agent /v1/docker/inventory request failed: %v", err)
	}
	if !strings.Contains(string(inventoryRaw), `"complete":`) {
		t.Fatal("real Docker inventory omitted completeness flag")
	}
	// The reviewed M3 Agent implementation must accept a safe immutable ZIP,
	// preserve the exact digest and reject malformed ZIP bytes. No execution.
	var bundle bytes.Buffer
	archive := zip.NewWriter(&bundle)
	file, err := archive.Create("sub2api/deploy/docker-compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("services: {}\\n")); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	bundleHash := sha256.Sum256(bundle.Bytes())
	digest := hex.EncodeToString(bundleHash[:])
	has, err := client.HasBundle(ctx, db.ControllerID(), token, digest)
	if err != nil || has {
		t.Fatalf("unexpected pre-upload bundle state: %v %v", has, err)
	}
	receipt, err := client.UploadBundle(ctx, db.ControllerID(), token, bundle.Bytes())
	if err != nil || receipt.AlreadyExists || receipt.SHA256 != digest {
		t.Fatalf("real Agent first bundle upload failed: %+v %v", receipt, err)
	}
	has, err = client.HasBundle(ctx, db.ControllerID(), token, digest)
	if err != nil || !has {
		t.Fatalf("real Agent could not find uploaded bundle: %v %v", has, err)
	}
	receipt, err = client.UploadBundle(ctx, db.ControllerID(), token, bundle.Bytes())
	if err != nil || !receipt.AlreadyExists {
		t.Fatalf("real Agent idempotent bundle upload failed: %+v %v", receipt, err)
	}
	// 控制端服务层须使用已有配对凭据、固定 TLS 公钥并复核缓存状态。
	cached, err := service.CacheBundle(ctx, host.ID, digest, bundle.Bytes())
	if err != nil || !cached.AlreadyExists || cached.AgentID != agentID || cached.SHA256 != digest {
		t.Fatalf("paired host candidate cache failed: %+v %v", cached, err)
	}
	if _, err := service.CacheBundle(ctx, host.ID, strings.Repeat("0", 64), bundle.Bytes()); err == nil {
		t.Fatal("paired host accepted mismatched local candidate digest")
	}
	if _, err := client.UploadBundle(ctx, db.ControllerID(), token, []byte("not a ZIP archive")); err == nil {
		t.Fatal("real Agent accepted a non-ZIP bundle with a correct SHA-256")
	}
	// Real grant replacement must revoke controller A only after controller B
	// finishes pairing; opening a new pairing window keeps old credentials.
	codeOutput = cli("-pairing-code")
	secondCode := mustCLIValue(t, codeOutput, "replacement code", realCodeLine)
	if _, err := client.Host(ctx, db.ControllerID(), token); err != nil {
		t.Fatalf("old grant was revoked before replacement: %v", err)
	}
	secondController := "22222222-2222-4222-8222-222222222222"
	newGrant, err := client.Pair(ctx, secondController, host.ID, secondCode)
	if err != nil {
		t.Fatalf("real Agent controller replacement failed: %v", err)
	}
	if _, err := client.Host(ctx, db.ControllerID(), token); err == nil {
		t.Fatal("real Agent still accepted old controller after successful re-pair")
	}
	if _, err := client.Host(ctx, secondController, newGrant.AccessToken); err != nil {
		t.Fatal(fmt.Errorf("real Agent rejected newly paired controller: %w", err))
	}
}
