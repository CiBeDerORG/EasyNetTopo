package catalog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestRestoreNativeInstallerFromPinnedSnapshot(t *testing.T) {
	const manifestPath = "tools/mac/app.yaml"
	const declaration = "native_install: macos_homebrew\n"
	for _, tc := range []struct {
		name      string
		metadata  string
		path      string
		badDigest bool
		unpinned  bool
		duplicate bool
		restore   bool
	}{
		{name: "已有新列的旧快照", metadata: declaration, restore: true},
		{name: "固定引用缺失", metadata: declaration, unpinned: true},
		{name: "清单摘要不匹配", metadata: declaration, badDigest: true},
		{name: "清单路径不匹配", metadata: declaration, path: "old/app.yml"},
		{name: "相同ID出现两次", metadata: declaration, duplicate: true},
		{name: "显式仅展示", metadata: "catalog_only: true\n"},
		{name: "已禁用", metadata: declaration + "enabled: false\n"},
		{name: "未知安装器", metadata: "native_install: unknown\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			source, err := service.Create(ctx, "Deploy", "https://github.com/example/deploy.git", "main")
			if err != nil {
				t.Fatal(err)
			}

			work := filepath.Join(dir, "work")
			if err := os.MkdirAll(filepath.Join(work, "tools", "mac"), 0700); err != nil {
				t.Fatal(err)
			}
			raw := []byte("id: maestro\nname: Maestro\n" + tc.metadata)
			if err := os.WriteFile(filepath.Join(work, manifestPath), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if tc.duplicate {
				if err := os.WriteFile(filepath.Join(work, "app.yml"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", work, "-c", "user.name=测试", "-c", "user.email=test@example.invalid"}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v：%v, %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			git("init", "-q")
			git("add", ".")
			git("commit", "-qm", "固定测试清单")
			revision := git("rev-parse", "HEAD")
			cache := filepath.Join(service.cacheDir, source.ID+".git")
			git("clone", "--bare", work, cache)
			if !tc.unpinned {
				git("--git-dir="+cache, "update-ref", "refs/easynettopo/catalog/"+revision, revision)
			}
			old := storage.CatalogApp{
				SourceID: source.ID, AppID: "maestro", Name: "Maestro", Revision: revision,
				ManifestPath: manifestPath, ManifestSHA256: manifestDigest(raw), CatalogOnly: true,
			}
			if tc.path != "" {
				old.ManifestPath = tc.path
			}
			if tc.badDigest {
				old.ManifestSHA256 = strings.Repeat("0", 64)
			}
			if err := db.SaveCatalogSnapshot(ctx, source.ID, revision, []storage.CatalogApp{old}); err != nil {
				t.Fatal(err)
			}
			catalogSource, err := db.CatalogSourceByID(ctx, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := old
			if tc.restore {
				want.NativeInstall = "macos_homebrew"
				want.CatalogOnly = false
			}
			for i := 0; i < 2; i++ {
				restarted, err := NewService(ctx, db)
				if err != nil {
					t.Fatalf("控制台重启失败：%v", err)
				}
				got, err := restarted.App(ctx, source.ID, old.AppID)
				if err != nil || got != want {
					t.Fatalf("升级后卡片状态错误：%+v，期望 %+v，错误 %v", got, want, err)
				}
				currentSource, err := db.CatalogSourceByID(ctx, source.ID)
				if err != nil || currentSource != catalogSource {
					t.Fatalf("恢复程序改写了 Git 来源：%+v，错误 %v", currentSource, err)
				}
			}
		})
	}
}
