package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestCatalogSnapshotAtomicity(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureCatalogSchema(ctx); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreateCatalogSource(ctx, "Deploy", "git@github.com:CiBeDerORG/deploy.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	entry := CatalogApp{SourceID: source.ID, AppID: "sub2api", Name: "Sub2API", Summary: "服务", Category: "developer", Revision: "a", ManifestSHA256: "digest"}
	if err := s.SaveCatalogSnapshot(ctx, source.ID, "a", []CatalogApp{entry}); err != nil {
		t.Fatal(err)
	}
	conflict := entry
	conflict.SourceID = "different"
	conflict.Revision = "b"
	if s.SaveCatalogSnapshot(ctx, source.ID, "b", []CatalogApp{conflict}) == nil {
		t.Fatal("不一致快照被接受")
	}
	actual, err := s.CatalogApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 1 || actual[0].Revision != "a" {
		t.Fatalf("失败刷新污染成功快照: %+v", actual)
	}
	if actual[0].ManifestPath != "sub2api/app.yaml" || actual[0].CatalogOnly {
		t.Fatalf("旧调用应沿用标准清单路径和部署能力: %+v", actual[0])
	}
	if err := s.SetCatalogSyncError(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	sources, err := s.CatalogSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].CurrentRevision != "a" || sources[0].LastError == "" {
		t.Fatalf("错误刷新覆盖既有信息: %+v", sources)
	}
	if err := s.SaveCatalogSnapshot(ctx, source.ID, "c", nil); err != nil {
		t.Fatal(err)
	}
	actual, err = s.CatalogApps(ctx)
	if err != nil || len(actual) != 0 {
		t.Fatal("空快照未正确提交", err)
	}
	source, err = s.CatalogSourceByID(ctx, source.ID)
	if err != nil || source.CurrentRevision != "c" || source.LastError != "" {
		t.Fatal("成功刷新没有清除失败状态")
	}
	_, err = s.CatalogSourceByID(ctx, "missing")
	if !errors.Is(err, ErrSourceNotFound) {
		t.Fatal("未知来源错误类型不正确", err)
	}
}

func TestCatalogSchemaMigratesLegacySnapshot(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureCatalogSchema(ctx); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreateCatalogSource(ctx, "Deploy", "git@github.com:CiBeDerORG/deploy.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	entry := CatalogApp{SourceID: source.ID, AppID: "demo", Name: "演示应用", Revision: "a", ManifestSHA256: "digest"}
	if err := s.SaveCatalogSnapshot(ctx, source.ID, entry.Revision, []CatalogApp{entry}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"ALTER TABLE catalog_apps DROP COLUMN manifest_path",
		"ALTER TABLE catalog_apps DROP COLUMN catalog_only",
		"ALTER TABLE catalog_apps DROP COLUMN native_install",
	} {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.EnsureCatalogSchema(ctx); err != nil {
		t.Fatal(err)
	}
	apps, err := s.CatalogApps(ctx)
	entry.ManifestPath = "demo/app.yaml"
	if err != nil || len(apps) != 1 || apps[0] != entry {
		t.Fatalf("迁移后旧快照不一致: %+v, %v", apps, err)
	}
	entry.CatalogOnly = true
	entry.ManifestPath = "tools/macos/app.yml"
	entry.Revision = "b"
	if err := s.SaveCatalogSnapshot(ctx, source.ID, entry.Revision, []CatalogApp{entry}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureCatalogSchema(ctx); err != nil {
		t.Fatal(err)
	}
	apps, err = s.CatalogApps(ctx)
	if err != nil || len(apps) != 1 || apps[0] != entry {
		t.Fatalf("重新初始化后展示条目不一致: %+v, %v", apps, err)
	}
	// A 的回填若与 B 的成功刷新交错，不能改变已经切换到 B 的卡片。
	verified := entry
	verified.CatalogOnly, verified.NativeInstall, verified.Revision = false, "macos_homebrew", "a"
	if err := s.RestoreCatalogNativeInstall(ctx, verified); err != nil {
		t.Fatal(err)
	}
	found, err := s.CatalogAppByID(ctx, source.ID, entry.AppID)
	if err != nil || found != entry {
		t.Fatalf("按身份读取展示条目失败: %+v, %v", found, err)
	}
	if _, err := s.CatalogAppByID(ctx, "其他来源", entry.AppID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("跨来源查询应无匹配条目: %v", err)
	}
}
