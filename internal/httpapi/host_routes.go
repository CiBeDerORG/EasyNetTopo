package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func hostRoutes(mux *http.ServeMux, db *storage.Store, service *hosts.Service, catalogSvc *catalog.Service) {
	authorize := func(w http.ResponseWriter, r *http.Request) bool {
		c, err := r.Cookie(authCookie)
		if err != nil || !db.HasSession(r.Context(), c.Value) {
			http.Error(w, "需要管理员登录", 401)
			return false
		}
		return true
	}
	mutation := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("X-ENT-Request") != "1" {
			http.Error(w, "缺少同源请求标记", 403)
			return false
		}
		if raw := r.Header.Get("Origin"); raw != "" {
			parsed, err := url.Parse(raw)
			if err != nil || !strings.EqualFold(parsed.Host, r.Host) || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				http.Error(w, "非法 Origin", 403)
				return false
			}
		}
		return true
	}
	jsonReply := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "请求必须是 JSON", 415)
			return false
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if err := d.Decode(v); err != nil {
			http.Error(w, "请求参数无效", 400)
			return false
		}
		var extra any
		if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
			http.Error(w, "仅允许一个 JSON 对象", 400)
			return false
		}
		return true
	}
	mux.HandleFunc("/api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			all, err := service.List(r.Context())
			if err != nil {
				http.Error(w, "无法读取主机列表", 503)
				return
			}
			jsonReply(w, 200, map[string]any{"hosts": all})
		case http.MethodPost:
			if !mutation(w, r) {
				return
			}
			var v struct {
				Name     string `json:"name"`
				Endpoint string `json:"endpoint"`
			}
			if !decode(w, r, &v) {
				return
			}
			h, err := service.Register(r.Context(), v.Name, v.Endpoint)
			if err != nil {
				http.Error(w, "主机名称或 IP:port 不合法", 400)
				return
			}
			jsonReply(w, 201, h)
		default:
			methodNotAllowed(w, "GET, POST")
		}
	})
	mux.HandleFunc("/api/v1/hosts/", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/hosts/"), "/")
		if len(parts) == 3 && parts[1] == "apps" && catalogSvc != nil {
			if r.Method != http.MethodPost {
				methodNotAllowed(w, "POST")
				return
			}
			if mutation(w, r) {
				appAction(w, r, parts[0], parts[2], service, catalogSvc)
			}
			return
		}
		if len(parts) != 2 || len(parts[0]) != 36 {
			http.NotFound(w, r)
			return
		}
		id, action := parts[0], parts[1]
		if action == "pair" {
			if r.Method != http.MethodPost {
				methodNotAllowed(w, "POST")
				return
			}
			if !mutation(w, r) {
				return
			}
			var req struct {
				AgentID     string `json:"agent_id"`
				SPKISHA256  string `json:"spki_sha256"`
				PairingCode string `json:"pairing_code"`
			}
			if !decode(w, r, &req) {
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			h, err := service.Pair(ctx, id, req.AgentID, req.SPKISHA256, req.PairingCode)
			if errors.Is(err, storage.ErrHostNotFound) {
				http.NotFound(w, r)
				return
			}
			if errors.Is(err, storage.ErrHostBinding) {
				slog.Warn("主机配对失败", "host_id", id, "stage", "binding", "code", "host_binding_conflict")
				jsonReply(w, http.StatusConflict, map[string]string{
					"code": "host_binding_conflict",
					"stage": "binding",
					"message": "主机或 Agent 已绑定其他身份；请核对现有绑定，不能直接覆盖",
				})
				return
			}
			if err != nil {
				diagnostic := hosts.DiagnosePairing(err)
				// 严格只记录分类信息。绝不记录配对码、Token、指纹或 HTTP 请求正文。
				slog.Warn("主机配对失败", "host_id", id, "stage", diagnostic.Stage, "code", diagnostic.Code, "agent_status", diagnostic.AgentStatus)
				jsonReply(w, diagnostic.Status, diagnostic)
				return
			}
			slog.Info("主机配对完成", "host_id", id, "stage", "saved")
			jsonReply(w, 200, h)
			return
		}
		if action != "snapshot" && action != "docker" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		kind := "host"
		if action == "docker" {
			kind = "docker"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		snap, err := service.Query(ctx, id, kind)
		if errors.Is(err, storage.ErrHostNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "Agent 状态暂时不可获取，不能推断主机资源已删除", 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(snap)
	})
}
