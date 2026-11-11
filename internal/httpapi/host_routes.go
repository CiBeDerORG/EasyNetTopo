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
	"github.com/CiBeDerORG/EasyNetTopo/internal/tasks"
)

func hostRoutes(mux *http.ServeMux, db *storage.Store, service *hosts.Service, catalogSvc *catalog.Service, taskServices ...*tasks.Service) {
	var taskSvc *tasks.Service
	if len(taskServices) > 0 {
		taskSvc = taskServices[0]
	}
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
	// 人工节点属性独立于路由数据与 Agent；定时刷新不覆盖保存内容。
	mux.HandleFunc("/api/v1/network-nodes/attributes", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			attrs, err := db.ListNodeAttributes(r.Context())
			if err != nil {
				jsonReply(w, 503, map[string]string{"code": "storage_unavailable", "message": "读取节点属性失败"})
				return
			}
			jsonReply(w, 200, map[string]any{"attributes": attrs})
		case http.MethodPut:
			if !mutation(w, r) {
				return
			}
			var attr storage.NodeAttributes
			if !decode(w, r, &attr) {
				return
			}
			saved, err := db.SaveNodeAttributes(r.Context(), attr)
			if errors.Is(err, storage.ErrInvalidNodeAttributes) {
				jsonReply(w, 400, map[string]string{"code": "invalid_node_attributes", "message": "节点属性、位置或公网 IP 格式不正确"})
			} else if err != nil {
				jsonReply(w, 503, map[string]string{"code": "storage_unavailable", "message": "保存节点属性失败"})
			} else {
				jsonReply(w, 200, saved)
			}
		default:
			methodNotAllowed(w, "GET, PUT")
		}
	})
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
				http.Error(w, "主机名称或连接地址不合法", 400)
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
		if len(parts) == 2 && len(parts[0]) == 36 && parts[1] == "network-entry" {
			if r.Method != http.MethodPut {
				methodNotAllowed(w, "PUT")
				return
			}
			if !mutation(w, r) {
				return
			}
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if !decode(w, r, &req) {
				return
			}
			if req.Enabled == nil {
				http.Error(w, "必须指定入口状态", 400)
				return
			}
			h, err := service.SetNetworkEntry(r.Context(), parts[0], *req.Enabled)
			if errors.Is(err, storage.ErrHostNotFound) {
				http.NotFound(w, r)
			} else if errors.Is(err, storage.ErrHostBinding) {
				http.Error(w, "请先配对主机 Agent", 409)
			} else if err != nil {
				http.Error(w, "组网入口保存失败", 503)
			} else {
				jsonReply(w, 200, h)
			}
			return
		}
		if len(parts) == 3 && len(parts[0]) == 36 && parts[1] == "network" {
			if r.Method != http.MethodGet {
				methodNotAllowed(w, "GET")
				return
			}
			if parts[2] != "node" && parts[2] != "peer" && parts[2] != "route" && parts[2] != "peer-center" {
				http.NotFound(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			raw, err := service.NetworkData(ctx, parts[0], parts[2])
			if errors.Is(err, storage.ErrHostNotFound) {
				http.NotFound(w, r)
			} else if err != nil {
				jsonReply(w, 502, map[string]string{"message": "网络数据读取失败，请检查组网入口、Agent 的 EasyTier Driver 开关及容器运行状态"})
			} else {
				jsonReply(w, 200, raw)
			}
			return
		}
		if len(parts) == 2 && len(parts[0]) == 36 && parts[1] == "adapters" {
			if r.Method != http.MethodGet {
				methodNotAllowed(w, "GET")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			raw, err := service.Query(ctx, parts[0], "adapters")
			if errors.Is(err, storage.ErrHostNotFound) {
				http.NotFound(w, r)
			} else if err != nil {
				jsonReply(w, 502, map[string]string{"code": "adapter_unavailable", "message": "Agent Driver 设置暂时不可用；可能需要先更新 Agent"})
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(raw)
			}
			return
		}
		if len(parts) == 3 && len(parts[0]) == 36 && parts[1] == "adapters" {
			if parts[2] != "easytier" {
				http.NotFound(w, r)
				return
			}
			if r.Method != http.MethodPut {
				methodNotAllowed(w, "PUT")
				return
			}
			if !mutation(w, r) {
				return
			}
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if !decode(w, r, &req) || req.Enabled == nil {
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			raw, err := service.SetAdapter(ctx, parts[0], parts[2], *req.Enabled)
			if errors.Is(err, storage.ErrHostNotFound) {
				http.NotFound(w, r)
			} else if err != nil {
				jsonReply(w, 502, map[string]string{"code": "adapter_settings_failed", "message": "Driver 开关未保存，请核对 Agent 连接和版本"})
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(raw)
			}
			return
		}
		if len(parts) == 4 && len(parts[0]) == 36 && parts[1] == "adapters" && parts[2] == "easytier" && parts[3] == "port-forwards" {
			if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
				methodNotAllowed(w, "GET, POST, DELETE")
				return
			}
			if r.Method != http.MethodGet && !mutation(w, r) {
				return
			}
			var change *hosts.PortForwardChange
			var instanceID, instanceName string
			if r.Method == http.MethodGet {
				q := r.URL.Query()
				for key, values := range q {
					if (key != "instance_id" && key != "instance_name") || len(values) != 1 {
						jsonReply(w, 400, map[string]string{"code": "invalid_request", "message": "不支持的查询参数"})
						return
					}
				}
				instanceID, instanceName = q.Get("instance_id"), q.Get("instance_name")
			} else {
				change = &hosts.PortForwardChange{}
				if !decode(w, r, change) {
					return
				}
				if change.ResourceID != "compose:easytier" {
					jsonReply(w, 400, map[string]string{"code": "invalid_request", "message": "只允许操作 EasyTier Compose 资源"})
					return
				}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			raw, err := service.EasyTierPortForwards(ctx, parts[0], r.Method, instanceID, instanceName, change)
			switch {
			case errors.Is(err, storage.ErrHostNotFound):
				http.NotFound(w, r)
			case err != nil:
				var agentErr *hosts.AdapterHTTPError
				if errors.As(err, &agentErr) && (agentErr.Status == 400 || agentErr.Status == 403 || agentErr.Status == 422) {
					jsonReply(w, agentErr.Status, map[string]string{"code": agentErr.Code, "message": "EasyTier 端口转发操作被 Agent 拒绝"})
				} else {
					jsonReply(w, 502, map[string]string{"code": "adapter_unavailable", "message": "无法访问已配对主机上的 EasyTier Driver"})
				}
			default:
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(raw)
			}
			return
		}
		// Only local host metadata is editable. No Agent identity, fingerprint or
		// sealed credentials are accepted by this endpoint.
		if len(parts) == 1 && len(parts[0]) == 36 {
			id := parts[0]
			switch r.Method {
			case http.MethodPatch:
				if !mutation(w, r) {
					return
				}
				var req struct {
					Name     string `json:"name"`
					Endpoint string `json:"endpoint"`
				}
				if !decode(w, r, &req) {
					return
				}
				h, err := service.Update(r.Context(), id, req.Name, req.Endpoint)
				if errors.Is(err, storage.ErrHostNotFound) {
					http.NotFound(w, r)
					return
				}
				if errors.Is(err, hosts.ErrInvalidHostMetadata) {
					jsonReply(w, http.StatusBadRequest, map[string]string{
						"code": "invalid_host_metadata", "message": "主机名称或连接地址不能为空或过长",
					})
					return
				}
				if err != nil {
					http.Error(w, "主机资料保存失败", http.StatusServiceUnavailable)
					return
				}
				slog.Info("主机资料已更新（配对凭据未修改）", "host_id", id)
				jsonReply(w, http.StatusOK, h)
				return
			case http.MethodDelete:
				if !mutation(w, r) {
					return
				}
				err := service.Delete(r.Context(), id)
				if errors.Is(err, storage.ErrHostNotFound) {
					http.NotFound(w, r)
					return
				}
				if err != nil {
					http.Error(w, "主机记录删除失败", http.StatusServiceUnavailable)
					return
				}
				slog.Info("本地主机记录已删除（未清理远端 Agent 或应用）", "host_id", id)
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusNoContent)
				return
			default:
				methodNotAllowed(w, "PATCH, DELETE")
				return
			}
		}
		if len(parts) == 3 && parts[1] == "agent" && (parts[2] == "update" || parts[2] == "force-update") {
			if r.Method != http.MethodPost {
				methodNotAllowed(w, "POST")
				return
			}
			if !mutation(w, r) {
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			queued, err := service.NativeAgentUpdate(ctx, parts[0], parts[2])
			if errors.Is(err, storage.ErrHostNotFound) {
				http.NotFound(w, r)
				return
			}
			if err != nil {
				code, message := "agent_update_unavailable",
					"无法向 Agent 发送更新任务。请检查网络连接及目标主机上的 Agent 服务。"
				switch {
				case errors.Is(err, hosts.ErrAgentUpdateUnsupported):
					code, message = "agent_update_unsupported",
						"目标 Agent 版本过旧，不支持远程更新。请先在该主机终端从最新 deploy main 执行 sudo bash deploy.sh easyhostagent update，再重试。"
				case errors.Is(err, hosts.ErrAgentUpdateEnvironment):
					code, message = "agent_update_environment",
						"Agent 已收到指令，但本机原生部署目录权限、安装标记或 systemd-run 不满足更新要求。请在目标主机检查相关路径与系统日志。"
				case errors.Is(err, hosts.ErrAgentUpdateUnauthorized):
					code, message = "agent_update_authorization",
						"Agent 拒绝当前配对授权，请检查主机配对状态，必要时重新配对。"
				case errors.Is(err, hosts.ErrAgentUpdateBusy):
					code, message = "agent_update_busy",
						"Agent 正在执行其他应用操作，请在操作完成后重试。"
				}
				jsonReply(w, http.StatusBadGateway, map[string]string{"code": code, "message": message})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write(queued)
			return
		}
		if len(parts) == 3 && parts[1] == "apps" && catalogSvc != nil {
			if r.Method != http.MethodPost {
				methodNotAllowed(w, "POST")
				return
			}
			if !mutation(w, r) {
				return
			}
			if taskSvc != nil && parts[2] == "update" {
				enqueueManualUpdate(w, r, parts[0], taskSvc, catalogSvc)
			} else {
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
					"code":    "host_binding_conflict",
					"stage":   "binding",
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
