package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

// 测试使用真正的本地 Git Commit 和裸仓库对象，不得从工作区读取文件。
func TestCandidateBundleUsesPinnedSnapshot(t *testing.T) {
	t.Run("v1", func(t *testing.T) { testCandidateBundleUsesPinnedSnapshot(t, false) })
	t.Run("v2", func(t *testing.T) { testCandidateBundleUsesPinnedSnapshot(t, true) })
}

func testCandidateBundleUsesPinnedSnapshot(t *testing.T, independent bool) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "controller", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Create(ctx, "Deploy", "https://github.com/CiBeDerORG/deploy.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	workdir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workdir, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", workdir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("config", "user.name", "Catalog Test")
	git("config", "user.email", "test@example.invalid")
	manifestRaw, resources, envExample, secretsExample := syntheticDeployFixture(t)
	app := "sub2api"
	if independent {
		manifestRaw, resources, envExample, secretsExample = independentConfigurationFixture(t)
		app = "unregistered-demo"
	}
	write := func(name string, data []byte) {
		t.Helper()
		full := filepath.Join(workdir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(app+"/app.yaml", manifestRaw)
	if independent {
		write(app+"/assets/icon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="12"/></svg>`))
	}
	for _, resource := range resources {
		write(resource.Source, resource.Content)
	}
	write(".env.example", envExample)
	write(".secrets.example", secretsExample)
	git("add", ".")
	git("commit", "-qm", "测试：固定候选部署资源")
	revision := git("rev-parse", "HEAD")
	cache := filepath.Join(service.cacheDir, source.ID+".git")
	cmd := exec.Command("git", "clone", "--bare", "-q", workdir, cache)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v %s", err, output)
	}
	if _, err := gitCommand(ctx, 512, "--git-dir="+cache, "update-ref", "refs/easynettopo/catalog/"+revision, revision); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(manifestRaw)
	if err := db.SaveCatalogSnapshot(ctx, source.ID, revision, []storage.CatalogApp{{
		SourceID: source.ID, AppID: app, Name: "Sub2API", Summary: "Service",
		Category: "developer", Revision: revision, ManifestSHA256: hex.EncodeToString(hash[:]),
	}}); err != nil {
		t.Fatal(err)
	}
	first, err := service.BuildCandidateBundle(ctx, source.ID, app)
	if err != nil {
		t.Fatal(err)
	}
	if independent {
		icon, mime, err := service.Icon(ctx, source.ID, app, revision)
		if err != nil || len(icon) == 0 || mime != "image/svg+xml" {
			t.Fatalf("固定版本图标读取失败：%v", err)
		}
		write(app+"/config.yaml", []byte("fields: invalid"))
	}
	// 修改当前工作目录不会改变已固定的 Git 对象。
	write(resources[0].Source, []byte("uncommitted and untrusted replacement"))
	second, err := service.BuildCandidateBundle(ctx, source.ID, app)
	if err != nil || !bytes.Equal(first.ZIP, second.ZIP) || first.SHA256 != second.SHA256 {
		t.Fatalf("未提交的工作区文件影响已锁定的包: %v", err)
	}
	// 模拟用户确认 A 后目录刷新到 B。两个版本均合法，包摘要确实不同。
	git("restore", ".")
	nextManifest := append(bytes.Clone(manifestRaw), []byte("\n# 新一版清单\n")...)
	write(app+"/app.yaml", nextManifest)
	git("add", ".")
	git("commit", "-qm", "测试：更新当前目录快照")
	nextRevision := git("rev-parse", "HEAD")
	git("push", "-q", cache, "HEAD:refs/easynettopo/catalog/"+nextRevision)
	nextHash := sha256.Sum256(nextManifest)
	if err := db.SaveCatalogSnapshot(ctx, source.ID, nextRevision, []storage.CatalogApp{{
		SourceID: source.ID, AppID: app, Revision: nextRevision, ManifestSHA256: hex.EncodeToString(nextHash[:]),
	}}); err != nil {
		t.Fatal(err)
	}
	current, err := service.BuildUpdateBundle(ctx, source.ID, app, "")
	if err != nil || current.Revision != nextRevision || current.SHA256 == first.SHA256 {
		t.Fatalf("跟随当前目录的更新应构建 B：%s，%v", current.Revision, err)
	}
	pinned, err := service.BuildUpdateBundle(ctx, source.ID, app, revision)
	if err != nil || pinned.Revision != revision || !bytes.Equal(pinned.ZIP, first.ZIP) {
		t.Fatalf("排队的手动更新未使用当时确认的 A：%s，%v", pinned.Revision, err)
	}
	if err := db.SaveCatalogSnapshot(ctx, source.ID, nextRevision, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BuildUpdateBundle(ctx, source.ID, app, revision); err == nil {
		t.Fatal("当前成功目录已撤回应用，却仍用旧引用创建了包")
	}
	if err := db.SaveCatalogSnapshot(ctx, source.ID, nextRevision, []storage.CatalogApp{{
		SourceID: source.ID, AppID: app, Revision: nextRevision, ManifestSHA256: hex.EncodeToString(nextHash[:]),
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := gitCommand(ctx, 512, "--git-dir="+cache, "update-ref", "-d", "refs/easynettopo/catalog/"+revision); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BuildUpdateBundle(ctx, source.ID, app, revision); err == nil {
		t.Fatal("A 的固定引用丢失后仍构建了包")
	}
	if _, err := gitCommand(ctx, 512, "--git-dir="+cache, "update-ref", "-d", "refs/easynettopo/catalog/"+nextRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BuildCandidateBundle(ctx, source.ID, app); err == nil {
		t.Fatal("未固定的当前 Git Commit 被用于构建")
	}
	if _, err := service.BuildCandidateBundle(ctx, "../not-a-source-id", "sub2api"); err == nil {
		t.Fatal("非法来源 ID 被接受")
	}
}
