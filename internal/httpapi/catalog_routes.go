package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"gopkg.in/yaml.v3"
)

func catalogRoutes(mux *http.ServeMux, db *storage.Store, svc *catalog.Service, enabled bool) {
	requireAdmin := func(w http.ResponseWriter, r *http.Request) bool {
		cookie, err := r.Cookie(authCookie)
		if err != nil || !db.HasSession(r.Context(), cookie.Value) {
			http.Error(w, "请先登录", http.StatusUnauthorized)
			return false
		}
		return true
	}
	mutation := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("X-ENT-Request") != "1" {
			http.Error(w, "缺少同源请求标记", http.StatusForbidden)
			return false
		}
		if raw := r.Header.Get("Origin"); raw != "" {
			u, err := url.Parse(raw)
			if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "http" && u.Scheme != "https") {
				http.Error(w, "非法请求来源", http.StatusForbidden)
				return false
			}
		}
		return true
	}
	reply := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	mux.HandleFunc("/api/v1/catalog/sources", func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			sources, err := svc.Sources(r.Context())
			if err != nil {
				http.Error(w, "无法读取应用源", 503)
				return
			}
			reply(w, 200, map[string]any{"sources": sources})
		case http.MethodPost:
			if !mutation(w, r) {
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "需要 JSON", 415)
				return
			}
			var v struct {
				Name   string `json:"name"`
				URL    string `json:"url"`
				Branch string `json:"branch"`
			}
			dec := json.NewDecoder(io.LimitReader(r.Body, 4097))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&v); err != nil || dec.Decode(&struct{}{}) != io.EOF {
				http.Error(w, "来源请求参数不合法", 400)
				return
			}
			source, err := svc.Create(r.Context(), v.Name, v.URL, v.Branch)
			if err != nil {
				http.Error(w, "来源地址或参数不合法、或该来源已经存在", 400)
				return
			}
			reply(w, 201, source)
		default:
			methodNotAllowed(w, "GET, POST")
		}
	})
	mux.HandleFunc("/api/v1/catalog/sources/", func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			methodNotAllowed(w, "POST, DELETE")
			return
		}
		if !mutation(w, r) {
			return
		}
		segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/catalog/sources/"), "/")
		if len(segments[0]) != 36 {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodDelete && len(segments) == 1 {
			if err := svc.Delete(r.Context(), segments[0]); err != nil {
				if errors.Is(err, storage.ErrSourceNotFound) {
					http.NotFound(w, r)
					return
				}
				http.Error(w, "删除 Git 来源失败", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost || len(segments) != 2 || segments[1] != "refresh" {
			http.NotFound(w, r)
			return
		}
		count, commit, err := svc.Refresh(r.Context(), segments[0])
		if err != nil {
			if errors.Is(err, storage.ErrSourceNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Git 刷新失败，保留上一次成功快照；请检查服务器 Git 授权与应用清单", 502)
			return
		}
		reply(w, 200, map[string]any{"source_id": segments[0], "revision": commit, "app_count": count})
	})
	mux.HandleFunc("/api/v1/catalog/apps", func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		apps, err := svc.Apps(r.Context())
		if err != nil {
			http.Error(w, "无法读取应用目录", 503)
			return
		}
		reply(w, 200, map[string]any{"apps": apps, "install_enabled": enabled})
	})
	mux.HandleFunc("GET /api/v1/catalog/apps/{source}/{app}/config", func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		pkg, err := svc.BuildCandidateBundle(r.Context(), r.PathValue("source"), r.PathValue("app"))
		if err != nil {
			http.Error(w, "当前应用未通过安装包校验，请检查来源清单与配置模板", 422)
			return
		}
		var data struct {
			Config  []map[string]any
			Actions map[string]struct{ Strategy string }
		}
		if yaml.Unmarshal(pkg.Manifest, &data) != nil || data.Actions["uninstall"].Strategy == "" {
			http.Error(w, "清单必须声明安装和卸载方式", 422)
			return
		}
		reply(w, 200, map[string]any{"revision": pkg.Revision, "config": data.Config})
	})
}
