package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
)

type marketRef struct {
	Source string `json:"source_id"`
	App    string `json:"app_id"`
}

// EasyNetTopo 是控制端自身；保留后台自动关联所需的 adopt，
// 但禁止从应用市场 API 绕过前端执行安装、卸载或强制更新。
func marketActionAllowed(appID, action string) bool {
	return appID != "easynettopo" || action == "update" || action == "adopt" || action == "status"
}

func appAction(w http.ResponseWriter, r *http.Request, hostID, name string, service *hosts.Service, source *catalog.Service) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "需要 JSON", 415)
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	dec.DisallowUnknownFields()
	decode := func(v any) bool { return dec.Decode(v) == nil && dec.Decode(&struct{}{}) == io.EOF }
	ctx := r.Context()
	var body any
	switch name {
	case "status":
		var req struct {
			Apps           []marketRef `json:"apps"`
			IncludeManaged bool        `json:"include_managed"`
		}
		if !decode(&req) || len(req.Apps) > 256 {
			http.Error(w, "无效的批量查询", 400)
			return
		}
		body = req
	case "uninstall":
		var req marketRef
		if !decode(&req) {
			http.Error(w, "无效的应用标识", 400)
			return
		}
		if !marketActionAllowed(req.App, name) {
			http.Error(w, "EasyNetTopo 仅支持更新", http.StatusForbidden)
			return
		}
		body = req
	case "install", "update", "adopt", "force_update":
		var req struct {
			marketRef
			Revision string            `json:"revision"`
			Config   map[string]string `json:"config"`
		}
		if !decode(&req) || name != "install" && len(req.Config) != 0 {
			http.Error(w, "无效的安装或更新参数", 400)
			return
		}
		if !marketActionAllowed(req.App, name) {
			http.Error(w, "EasyNetTopo 仅支持更新", http.StatusForbidden)
			return
		}
		pkg, err := source.BuildCandidateBundle(ctx, req.Source, req.App)
		if err != nil {
			http.Error(w, "安装包未通过清单校验", 422)
			return
		}
		if pkg.Revision != req.Revision {
			http.Error(w, "来源版本已变化，请刷新应用商店并重新操作", 409)
			return
		}
		if err = service.RequireManifestVersion(ctx, hostID, pkg.SchemaVersion); err != nil {
			http.Error(w, "目标 Agent 尚不支持此应用配置版本，或身份校验失败，请更新 Agent 后重试", 422)
			return
		}
		if name == "install" {
			allowed := map[string]bool{}
			for _, field := range pkg.Config {
				key, _ := field["key"].(string)
				allowed[key] = field["custom_input"] != false && field["editable"] != false
			}
			for key := range req.Config {
				if !allowed[key] {
					http.Error(w, "请求包含未声明、隐藏或只读的配置字段", 422)
					return
				}
			}
		}
		if _, err = service.CacheBundle(ctx, hostID, pkg.SHA256, pkg.ZIP); err != nil {
			http.Error(w, "安装包未送达，请检查主机配对和 Agent 版本", 502)
			return
		}
		body = map[string]any{"source_id": req.Source, "app_id": req.App, "bundle_sha256": pkg.SHA256}
		if name == "install" {
			body.(map[string]any)["config"] = req.Config
		}
		if name == "force_update" {
			// A dedicated route, not a generic update bypass: UI and Agent
			// both require an explicit recovery confirmation.
			body.(map[string]any)["confirm_external"] = true
		}
	default:
		http.NotFound(w, r)
		return
	}
	raw, err := service.AppAction(ctx, hostID, name, body)
	if err != nil {
		var failure *hosts.AppError
		if errors.As(err, &failure) {
			http.Error(w, failure.Message, failure.Status)
		} else {
			http.Error(w, "无法联系已配对主机，请检查连接与 Agent 版本", 502)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}
