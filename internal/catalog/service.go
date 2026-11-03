package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
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
func (s *Service) Apps(ctx context.Context) ([]storage.CatalogApp, error) {
	return s.store.CatalogApps(ctx)
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
	apps := make([]storage.CatalogApp, 0)
	candidates := 0
	for name, entry := range repository.objects {
		if entry.Mode != "100644" && entry.Mode != "100755" {
			continue
		}
		parts := strings.Split(name, "/")
		if len(parts) != 2 || parts[1] != "app.yaml" || !appID.MatchString(parts[0]) {
			continue
		}
		candidates++
		if candidates > 300 {
			return fail(errors.New("应用描述文件数量超过上限"))
		}
		raw, err := repository.contents(ctx, name)
		if err != nil {
			continue
		}
		manifest, digest, err := previewManifest(parts[0], raw, repository.objects)
		if err != nil {
			continue
		}
		apps = append(apps, storage.CatalogApp{
			SourceID: id, AppID: manifest.ID, Name: manifest.Name,
			Summary: manifest.Summary, Category: manifest.Category,
			Revision: repository.revision, ManifestSHA256: digest,
		})
	}
	if len(apps) == 0 {
		return fail(errors.New("仓库中没有受支持且校验通过的应用描述"))
	}
	// 提前保留固定 Commit 的 Git 对象，后续 Fetch 不应使已展示版本消失。
	if err := repository.pin(ctx); err != nil {
		return fail(errors.New("无法保留固定 Git Commit"))
	}
	if err := s.store.SaveCatalogSnapshot(ctx, id, repository.revision, apps); err != nil {
		return fail(err)
	}
	return len(apps), repository.revision, nil
}
