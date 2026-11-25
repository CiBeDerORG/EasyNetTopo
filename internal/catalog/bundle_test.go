package catalog

import (
	"bytes"
	"context"
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
	manifestPath := "catalog/packages/app.yml"
	write(manifestPath, manifestRaw)
	nativePath := "macOS/开发工具/app.yaml"
	nativeMode := "catalog_only: true"
	if independent {
		nativeMode = "native_install: macos_homebrew"
	}
	write(nativePath, []byte("id: maestro\nname: Maestro\nenabled: true\n"+nativeMode+"\ncategory: developer\nsummary: Mac 原生工具\nicon: assets/missing.svg\n"))
	write("hidden/app.yml", []byte("id: hidden-tool\nname: 已停用\nenabled: false\n"))
	write("broken-package/app.yaml", []byte("id: incomplete\nname: 缺少包资源的应用\nschema_version: 2\nruntime: {kind: docker_compose}\n"))
	if independent {
		write("catalog/packages/assets/icon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="12"/></svg>`))
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
	repo, err := pinnedBundleRepository(ctx, cache, revision)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := readCatalogApps(ctx, source.ID, repo)
	if err != nil || len(apps) != 3 {
		t.Fatalf("应收录 Compose、原生工具和缺少包资源的配置：%v, %v", apps, err)
	}
	if err := db.SaveCatalogSnapshot(ctx, source.ID, revision, apps); err != nil {
		t.Fatal(err)
	}
	native, err := db.CatalogAppByID(ctx, source.ID, "maestro")
	if err != nil || native.CatalogOnly == independent || (native.NativeInstall == "macos_homebrew") != independent || native.ManifestPath != nativePath {
		t.Fatalf("原生工具未按配置 ID 和实际路径上架：%+v, %v", native, err)
	}
	if _, _, err := service.Icon(ctx, source.ID, "maestro", revision); err == nil {
		t.Fatal("缺失的图标被当作有效资源")
	}
	for _, id := range []string{"maestro", "incomplete", "hidden-tool"} {
		if _, err := service.BuildCandidateBundle(ctx, source.ID, id); err == nil {
			t.Fatalf("应用 %s 意外获得可执行的候选包", id)
		}
	}
	// 同一固定版本中两个同名 ID 会使路径、开关产生歧义。
	repo.objects["second/app.yaml"] = repo.objects[nativePath]
	if _, err := readCatalogApps(ctx, source.ID, repo); err == nil {
		t.Fatal("重复 ID 被收录")
	}
	repo.objects = map[string]gitObject{"app.yaml": repo.objects["hidden/app.yml"]}
	if apps, err := readCatalogApps(ctx, source.ID, repo); err != nil || len(apps) != 0 {
		t.Fatalf("全部停用应生成空快照：%v, %v", apps, err)
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
	git("mv", "catalog/packages", "catalog/moved")
	manifestPath = "catalog/moved/app.yml"
	write(manifestPath, nextManifest)
	git("add", ".")
	git("commit", "-qm", "测试：更新当前目录快照")
	nextRevision := git("rev-parse", "HEAD")
	git("push", "-q", cache, "HEAD:refs/easynettopo/catalog/"+nextRevision)
	if err := db.SaveCatalogSnapshot(ctx, source.ID, nextRevision, []storage.CatalogApp{{
		SourceID: source.ID, AppID: app, Revision: nextRevision, ManifestSHA256: manifestDigest(nextManifest), ManifestPath: manifestPath,
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
		SourceID: source.ID, AppID: app, Revision: nextRevision, ManifestSHA256: manifestDigest(nextManifest), ManifestPath: manifestPath,
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
