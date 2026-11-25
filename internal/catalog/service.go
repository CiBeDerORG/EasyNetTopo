package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type Service struct {
	store    *storage.Store
	cacheDir string
	mu       sync.Mutex
}

func NewService(ctx context.Context, db *storage.Store) (*Service, error) {
	if err := db.EnsureCatalogSchema(ctx); err != nil {
		return nil, err
	}
	cache := filepath.Join(db.DataDir(), "git-cache")
	if err := os.MkdirAll(cache, 0700); err != nil {
		return nil, err
	}
	return &Service{store: db, cacheDir: cache}, nil
}
func (s *Service) Create(ctx context.Context, name, remote, branch string) (storage.CatalogSource, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 80 || strings.ContainsAny(name, "\r\n\x00") {
		return storage.CatalogSource{}, errors.New("来源名称必须为 1–80 个字符")
	}
	if err := AllowedRemote(remote); err != nil {
		return storage.CatalogSource{}, err
	}
	if err := validateBranch(branch); err != nil {
		return storage.CatalogSource{}, err
	}
	return s.store.CreateCatalogSource(ctx, name, remote, branch)
}
func (s *Service) Sources(ctx context.Context) ([]storage.CatalogSource, error) {
	return s.store.CatalogSources(ctx)
}
func (s *Service) App(ctx context.Context, sourceID, applicationID string) (storage.CatalogApp, error) {
	return s.store.CatalogAppByID(ctx, sourceID, applicationID)
}

func (s *Service) Apps(ctx context.Context) ([]storage.CatalogApp, error) {
	apps, err := s.store.CatalogApps(ctx)
	for i := range apps {
		apps[i].IconURL = fmt.Sprintf("/api/v1/catalog/apps/%s/%s/icon?revision=%s", apps[i].SourceID, apps[i].AppID, apps[i].Revision)
	}
	return apps, err
}

// Delete 删除一个 Git 来源及其应用索引，不会向 Agent 发送卸载指令。
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.DeleteCatalogSource(ctx, id); err != nil {
		return err
	}
	// 缓存仅属于该来源；即使磁盘清理失败，数据库删除也已经提交。
	_ = os.RemoveAll(filepath.Join(s.cacheDir, id+".git"))
	return nil
}

func (s *Service) Refresh(ctx context.Context, id string) (int, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source, err := s.store.CatalogSourceByID(ctx, id)
	if err != nil {
		return 0, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	fail := func(cause error) (int, string, error) {
		_ = s.store.SetCatalogSyncError(context.Background(), id)
		return 0, "", fmt.Errorf("应用源同步失败（旧成功快照保留）: %w", cause)
	}
	if err := AllowedRemote(source.URL); err != nil {
		return fail(err)
	}
	repository, err := refreshGit(ctx, filepath.Join(s.cacheDir, source.ID+".git"), source.URL, source.Branch)
	if err != nil {
		return fail(errors.New("无法访问 Git 来源"))
	}
	apps, err := readCatalogApps(ctx, id, repository)
	if err != nil {
		return fail(err)
	}
	// 全部有效应用都被 enabled:false 停用也是一次有效同步，允许清空商店索引。
	// 提前保留固定 Commit 的 Git 对象，后续 Fetch 不应使已展示版本消失。
	if err := repository.pin(ctx); err != nil {
		return fail(errors.New("无法保留固定 Git Commit"))
	}
	if err := s.store.SaveCatalogSnapshot(ctx, id, repository.revision, apps); err != nil {
		return fail(err)
	}
	return len(apps), repository.revision, nil
}

// 应用 ID 和上架开关取自固定版本的配置，与清单所在目录无关。
func readCatalogApps(ctx context.Context, sourceID string, repo *gitRepository) ([]storage.CatalogApp, error) {
	apps := make([]storage.CatalogApp, 0)
	seen := make(map[string]bool)
	candidates := 0
	for name, entry := range repo.objects {
		base := path.Base(name)
		if (base != "app.yaml" && base != "app.yml") || (entry.Mode != "100644" && entry.Mode != "100755") {
			continue
		}
		candidates++
		if candidates > 300 {
			return nil, errors.New("应用描述文件数量超过上限")
		}
		raw, err := repo.contents(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("读取应用配置 %q 失败: %w", name, err)
		}
		m, err := readCatalogMetadata(raw)
		if err != nil {
			continue
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("应用配置中存在重复 id %q", m.ID)
		}
		seen[m.ID] = true
		if !m.IsEnabled() {
			continue
		}
		// 原生安装只选择 Agent 内置的固定动作，来源清单不能指定命令或配方。
		nativeInstall := ""
		if !m.CatalogOnly && m.Version == 0 && m.ID == "maestro" && m.NativeInstall == "macos_homebrew" {
			nativeInstall = m.NativeInstall
		}
		apps = append(apps, storage.CatalogApp{
			SourceID: sourceID, AppID: m.ID, Name: m.Name, Summary: m.Summary, Category: m.Category,
			Revision: repo.revision, ManifestSHA256: manifestDigest(raw), ManifestPath: name,
			NativeInstall: nativeInstall,
			CatalogOnly:   m.CatalogOnly || (nativeInstall == "" && m.Version != 1 && m.Version != 2),
		})
	}
	if len(seen) == 0 {
		return nil, errors.New("仓库中没有可读取的应用配置")
	}
	return apps, nil
}

// 历史版本的清单可能已经移动，需要从对应 Commit 按配置 ID 重新定位。
func manifestPathAtRevision(ctx context.Context, repo *gitRepository, sourceID, appID string, current storage.CatalogApp) (string, error) {
	if current.Revision == repo.revision {
		return current.ManifestPath, nil
	}
	apps, err := readCatalogApps(ctx, sourceID, repo)
	if err != nil {
		return "", err
	}
	for _, app := range apps {
		if app.AppID == appID {
			return app.ManifestPath, nil
		}
	}
	return "", errors.New("固定版本中找不到启用的应用配置")
}
