package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

// CatalogSource 是来源记录而非执行权限；URL 中禁止包含用户凭据。
type CatalogSource struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Branch          string `json:"branch"`
	CurrentRevision string `json:"current_revision"`
	LastError       string `json:"last_error,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type CatalogApp struct {
	IconURL        string `json:"icon_url,omitempty"`
	SourceID       string `json:"source_id"`
	AppID          string `json:"app_id"`
	Name           string `json:"name"`
	Summary        string `json:"summary"`
	Category       string `json:"category"`
	Revision       string `json:"revision"`
	ManifestSHA256 string `json:"manifest_sha256"`
	ManifestPath   string `json:"-"`
	CatalogOnly    bool   `json:"catalog_only,omitempty"`
}

var ErrSourceNotFound = errors.New("Git 应用源不存在")

func (s *Store) DataDir() string { return filepath.Dir(s.path) }

// EnsureCatalogSchema 创建只读商店索引。所有刷新必须在单一 SQL 事务中提交。
func (s *Store) EnsureCatalogSchema(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{
		"CREATE TABLE IF NOT EXISTS catalog_sources(id TEXT PRIMARY KEY,name TEXT NOT NULL,url TEXT NOT NULL,branch TEXT NOT NULL,current_revision TEXT NOT NULL DEFAULT '',last_error TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL DEFAULT '',UNIQUE(url,branch))",
		"CREATE TABLE IF NOT EXISTS catalog_apps(source_id TEXT NOT NULL REFERENCES catalog_sources(id),app_id TEXT NOT NULL,name TEXT NOT NULL,summary TEXT NOT NULL,category TEXT NOT NULL,revision TEXT NOT NULL,manifest_sha256 TEXT NOT NULL,manifest_path TEXT NOT NULL DEFAULT '',catalog_only INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(source_id,app_id))",
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("初始化应用目录失败: %w", err)
		}
	}
	var hasPath, hasCatalogOnly bool
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE name='manifest_path'),
		COUNT(*) FILTER (WHERE name='catalog_only') FROM pragma_table_info('catalog_apps')`).Scan(&hasPath, &hasCatalogOnly); err != nil {
		return err
	}
	if !hasPath {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE catalog_apps ADD COLUMN manifest_path TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		// 旧快照只收录 <应用 ID>/app.yaml，迁移时可直接恢复原路径。
		if _, err := tx.ExecContext(ctx, "UPDATE catalog_apps SET manifest_path=app_id || '/app.yaml'"); err != nil {
			return err
		}
	}
	if !hasCatalogOnly {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE catalog_apps ADD COLUMN catalog_only INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CreateCatalogSource(ctx context.Context, name, url, branch string) (CatalogSource, error) {
	id, err := uuid()
	if err != nil {
		return CatalogSource{}, err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO catalog_sources(id,name,url,branch) VALUES(?,?,?,?)", id, name, url, branch)
	if err != nil {
		return CatalogSource{}, fmt.Errorf("无法创建 Git 应用源（可能已存在同地址和分支）: %w", err)
	}
	return CatalogSource{ID: id, Name: name, URL: url, Branch: branch}, nil
}
func (s *Store) CatalogSources(ctx context.Context) ([]CatalogSource, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,url,branch,current_revision,last_error,updated_at FROM catalog_sources ORDER BY name,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]CatalogSource, 0)
	for rows.Next() {
		var v CatalogSource
		if err := rows.Scan(&v.ID, &v.Name, &v.URL, &v.Branch, &v.CurrentRevision, &v.LastError, &v.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Store) CatalogSourceByID(ctx context.Context, id string) (CatalogSource, error) {
	var v CatalogSource
	err := s.db.QueryRowContext(ctx, "SELECT id,name,url,branch,current_revision,last_error,updated_at FROM catalog_sources WHERE id=?", id).
		Scan(&v.ID, &v.Name, &v.URL, &v.Branch, &v.CurrentRevision, &v.LastError, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrSourceNotFound
	}
	return v, err
}

// DeleteCatalogSource 原子地删除指定来源及其商店快照，不影响其他来源和已部署的应用。
func (s *Store) DeleteCatalogSource(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM catalog_apps WHERE source_id=?", id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM catalog_sources WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrSourceNotFound
	}
	return tx.Commit()
}

const selectCatalogApps = "SELECT source_id,app_id,name,summary,category,revision,manifest_sha256,manifest_path,catalog_only FROM catalog_apps"

func scanCatalogApp(row interface{ Scan(...any) error }) (CatalogApp, error) {
	var app CatalogApp
	err := row.Scan(&app.SourceID, &app.AppID, &app.Name, &app.Summary, &app.Category, &app.Revision, &app.ManifestSHA256, &app.ManifestPath, &app.CatalogOnly)
	return app, err
}

func (s *Store) CatalogAppByID(ctx context.Context, sourceID, appID string) (CatalogApp, error) {
	return scanCatalogApp(s.db.QueryRowContext(ctx, selectCatalogApps+" WHERE source_id=? AND app_id=?", sourceID, appID))
}

func (s *Store) CatalogApps(ctx context.Context) ([]CatalogApp, error) {
	rows, err := s.db.QueryContext(ctx, selectCatalogApps+" ORDER BY name,source_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	apps := make([]CatalogApp, 0)
	for rows.Next() {
		v, err := scanCatalogApp(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, v)
	}
	return apps, rows.Err()
}
func (s *Store) SetCatalogSyncError(ctx context.Context, id string) error {
	// 不记录 Git 的原始报错，避免泄露 SSH 配置、执行环境或远程凭据。
	result, err := s.db.ExecContext(ctx, "UPDATE catalog_sources SET last_error='同步失败，请检查 Git 访问和仓库应用清单',updated_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrSourceNotFound
	}
	return nil
}
func (s *Store) SaveCatalogSnapshot(ctx context.Context, id, revision string, apps []CatalogApp) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM catalog_sources WHERE id=?", id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrSourceNotFound
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM catalog_apps WHERE source_id=?", id); err != nil {
		return err
	}
	for _, v := range apps {
		if v.SourceID != id || v.Revision != revision {
			return errors.New("应用快照身份不匹配")
		}
		if v.ManifestPath == "" {
			v.ManifestPath = v.AppID + "/app.yaml"
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO catalog_apps(source_id,app_id,name,summary,category,revision,manifest_sha256,manifest_path,catalog_only) VALUES(?,?,?,?,?,?,?,?,?)",
			id, v.AppID, v.Name, v.Summary, v.Category, revision, v.ManifestSHA256, v.ManifestPath, v.CatalogOnly); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE catalog_sources SET current_revision=?,last_error='',updated_at=? WHERE id=?", revision, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		return err
	}
	return tx.Commit()
}
