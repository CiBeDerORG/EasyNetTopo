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
		body = req
	case "install":
		var req struct {
			marketRef
			Revision string            `json:"revision"`
			Config   map[string]string `json:"config"`
		}
		if !decode(&req) {
			http.Error(w, "无效的安装参数", 400)
			return
		}
		pkg, err := source.BuildCandidateBundle(ctx, req.Source, req.App)
		if err != nil {
			http.Error(w, "安装包未通过清单校验", 422)
			return
		}
		if pkg.Revision != req.Revision {
			http.Error(w, "来源版本已变化，请重新打开安装表单", 409)
			return
		}
		if _, err = service.CacheBundle(ctx, hostID, pkg.SHA256, pkg.ZIP); err != nil {
			http.Error(w, "安装包未送达，请检查主机配对和 Agent 版本", 502)
			return
		}
		body = map[string]any{"source_id": req.Source, "app_id": req.App, "bundle_sha256": pkg.SHA256, "config": req.Config}
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
