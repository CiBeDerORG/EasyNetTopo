package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"github.com/CiBeDerORG/EasyNetTopo/internal/tasks"
)

func reverseRoutes(mux *http.ServeMux, db *storage.Store, service *hosts.Service, taskSvc *tasks.Service, catalogSvc *catalog.Service) {
	jsonReply := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	admin := func(w http.ResponseWriter, r *http.Request) bool {
		cookie, err := r.Cookie(authCookie)
		if err != nil || !db.HasSession(r.Context(), cookie.Value) {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "需要管理员登录"})
			return false
		}
		return true
	}
	mutation := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("X-ENT-Request") != "1" {
			jsonReply(w, http.StatusForbidden, map[string]string{"code": "same_origin_required", "message": "缺少同源请求标记"})
			return false
		}
		if raw := r.Header.Get("Origin"); raw != "" {
			parsed, err := url.Parse(raw)
			if err != nil || !strings.EqualFold(parsed.Host, r.Host) ||
				(parsed.Scheme != "http" && parsed.Scheme != "https") {
				jsonReply(w, http.StatusForbidden, map[string]string{"code": "invalid_origin", "message": "非法请求来源"})
				return false
			}
		}
		return true
	}
	decode := func(w http.ResponseWriter, r *http.Request, dst any, max int64) bool {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			jsonReply(w, http.StatusUnsupportedMediaType, map[string]string{"code": "content_type", "message": "需要 JSON"})
			return false
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, max))
		decoder.DisallowUnknownFields()
		if decoder.Decode(dst) != nil {
			jsonReply(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "请求参数无效"})
			return false
		}
		var extra any
		if !errors.Is(decoder.Decode(&extra), io.EOF) {
			jsonReply(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "message": "请求只能包含一个 JSON 对象"})
			return false
		}
		return true
	}

	mux.HandleFunc("GET /api/v1/reverse/discovery", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) {
			return
		}
		state, err := service.Discovery(r.Context())
		if err != nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "storage_unavailable", "message": "读取主机发现状态失败"})
			return
		}
		items, err := service.ListDiscoveries(r.Context())
		if err != nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "storage_unavailable", "message": "读取待发现主机失败"})
			return
		}
		jsonReply(w, http.StatusOK, map[string]any{"discovery": state, "discoveries": items})
	})
	mux.HandleFunc("POST /api/v1/reverse/discovery/open", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) || !mutation(w, r) {
			return
		}
		token, state, err := service.OpenDiscovery(r.Context())
		if err != nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "discovery_open_failed", "message": "开启主机发现失败"})
			return
		}
		// Token 仅在开启时返回一次；服务端只保存 SHA-256 摘要。
		jsonReply(w, http.StatusCreated, map[string]any{"discovery": state, "bootstrap_token": token})
	})
	mux.HandleFunc("POST /api/v1/reverse/discovery/close", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) || !mutation(w, r) {
			return
		}
		if err := service.CloseDiscovery(r.Context()); err != nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "discovery_close_failed", "message": "关闭主机发现失败"})
			return
		}
		jsonReply(w, http.StatusOK, map[string]bool{"enabled": false})
	})
	mux.HandleFunc("POST /api/v1/reverse/discoveries/{agent_id}/accept", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) || !mutation(w, r) {
			return
		}
		h, err := service.AcceptDiscovery(r.Context(), r.PathValue("agent_id"))
		switch {
		case errors.Is(err, storage.ErrHostNotFound):
			jsonReply(w, http.StatusNotFound, map[string]string{"code": "discovery_not_found", "message": "待发现主机不存在"})
		case errors.Is(err, storage.ErrHostBinding):
			jsonReply(w, http.StatusConflict, map[string]string{"code": "agent_already_bound", "message": "该 Agent 已绑定其他主机"})
		case err != nil:
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "accept_failed", "message": "接受主机发现失败"})
		default:
			jsonReply(w, http.StatusCreated, h)
		}
	})
	mux.HandleFunc("DELETE /api/v1/reverse/hosts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) || !mutation(w, r) {
			return
		}
		err := service.RevokeTemporary(r.Context(), r.PathValue("id"))
		if errors.Is(err, storage.ErrTemporaryHostExpired) || errors.Is(err, storage.ErrHostNotFound) {
			jsonReply(w, http.StatusNotFound, map[string]string{"code": "temporary_host_not_found", "message": "临时主机不存在、已过期或已剔除"})
			return
		}
		if err != nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "revoke_failed", "message": "剔除临时主机失败"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Agent 初次报到使用发现窗口的一次性 Bootstrap Token。
	mux.HandleFunc("POST /api/v1/agent/discovery", func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || len(token) != 64 {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "发现凭据无效"})
			return
		}
		var req struct {
			AgentID        string `json:"agent_id"`
			Hostname       string `json:"hostname"`
			SPKISHA256     string `json:"spki_sha256"`
			Platform       string `json:"platform"`
			Arch           string `json:"arch"`
			Version        string `json:"version"`
			CertificateDER string `json:"certificate_der"`
		}
		if !decode(w, r, &req, 24<<10) {
			return
		}
		cert, err := base64.StdEncoding.DecodeString(req.CertificateDER)
		if err != nil {
			jsonReply(w, http.StatusBadRequest, map[string]string{"code": "invalid_certificate", "message": "Agent 身份证书格式无效"})
			return
		}
		state, controllerID, agentToken, err := service.RegisterDiscovery(r.Context(), token, storage.DiscoveredAgent{
			AgentID: req.AgentID, Hostname: req.Hostname, SPKISHA256: req.SPKISHA256,
			Platform: req.Platform, Arch: req.Arch, Version: req.Version,
		}, cert)
		if errors.Is(err, storage.ErrDiscoveryClosed) {
			jsonReply(w, http.StatusForbidden, map[string]string{"code": "discovery_closed", "message": "主机发现未开启或已过期"})
			return
		}
		if errors.Is(err, storage.ErrDiscoveryIdentity) || errors.Is(err, storage.ErrHostBinding) {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "identity_rejected", "message": "Agent 身份核验失败"})
			return
		}
		if err != nil {
			jsonReply(w, http.StatusBadRequest, map[string]string{"code": "invalid_discovery", "message": "主机发现信息无效"})
			return
		}
		response := map[string]any{"status": state, "next_poll_seconds": 10}
		if state == "accepted" {
			response["controller_id"] = controllerID
			response["agent_token"] = agentToken
		}
		jsonReply(w, http.StatusOK, response)
	})

	// 已接受的 Agent 使用独立 Token 轮询。临时主机有效期不会因轮询而延长。
	mux.HandleFunc("POST /api/v1/agent/poll", func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || len(token) != 64 {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 授权无效"})
			return
		}
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if !decode(w, r, &req, 2048) {
			return
		}
		h, err := service.AuthenticateReverseTaskAgent(r.Context(), req.AgentID, token)
		if err != nil {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 身份或授权无效"})
			return
		}
		active := service.TemporaryActive(h)
		if active {
			h, err = service.UpdateReversePoll(r.Context(), req.AgentID)
			if err != nil {
				jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "临时主机已过期或已撤销"})
				return
			}
		}
		var taskValue any
		if taskSvc != nil {
			payload, claimErr := taskSvc.ClaimReverseTask(r.Context(), req.AgentID)
			if claimErr != nil {
				jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "task_claim_failed", "message": "领取预安装任务失败"})
				return
			}
			if payload != nil {
				taskValue = map[string]any{
					"id": payload.Task.ID, "state": payload.Task.State, "source_id": payload.Task.SourceID, "app_id": payload.Task.AppID,
					"action": payload.Task.Action, "revision": payload.Revision,
					"bundle_sha256": payload.BundleSHA, "config": payload.Config,
					"bundle_url": "/api/v1/agent/tasks/" + payload.Task.ID + "/bundle",
				}
			}
		}
		if !active && taskValue == nil {
			jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "临时主机已到期，且没有待收尾任务"})
			return
		}
		jsonReply(w, http.StatusOK, map[string]any{
			"status": "accepted", "controller_id": db.ControllerID(),
			"expires_at": h.ExpiresAt, "next_poll_at": h.NextPollAt,
			"next_poll_seconds": 10, "task": taskValue,
		})
	})
	// 临时主机只允许通过预安装入口创建 install 任务；不会直接连接内网 Agent。
	mux.HandleFunc("POST /api/v1/hosts/{id}/preinstall", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) || !mutation(w, r) {
			return
		}
		if taskSvc == nil || catalogSvc == nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "preinstall_unavailable", "message": "预安装服务尚未初始化"})
			return
		}
		var req struct {
			Source   string            `json:"source_id"`
			App      string            `json:"app_id"`
			Revision string            `json:"revision"`
			Config   map[string]string `json:"config"`
		}
		if !decode(w, r, &req, 256<<10) || req.Source == "" || req.App == "" || req.Revision == "" {
			return
		}
		if req.App == "easynettopo" {
			jsonReply(w, http.StatusForbidden, map[string]string{"code": "app_not_allowed", "message": "临时主机不允许预安装 EasyNetTopo"})
			return
		}
		host, err := service.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			jsonReply(w, http.StatusNotFound, map[string]string{"code": "host_not_found", "message": "临时主机不存在"})
			return
		}
		if host.HostType != "temporary" || host.ConnectionMode != "reverse" || !service.TemporaryActive(host) {
			jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "该主机不是有效的临时主机"})
			return
		}
		pkg, err := catalogSvc.BuildCandidateBundle(r.Context(), req.Source, req.App)
		if err != nil || pkg.Revision != req.Revision {
			jsonReply(w, http.StatusConflict, map[string]string{"code": "catalog_revision_changed", "message": "应用来源或版本已变化，请刷新应用市场后重试"})
			return
		}
		fieldsByKey := make(map[string]map[string]any, len(pkg.Config))
		for _, field := range pkg.Config {
			key, _ := field["key"].(string)
			if key != "" {
				fieldsByKey[key] = field
			}
		}
		if len(req.Config) > len(fieldsByKey) {
			jsonReply(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_config", "message": "安装配置字段数量无效"})
			return
		}
		for key := range req.Config {
			field, exists := fieldsByKey[key]
			if !exists || !catalogConfigFieldInputAllowed(key, field, fieldsByKey, req.Config) {
				jsonReply(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_config", "message": "请求包含未声明、隐藏、只读或选项值无效的配置字段"})
				return
			}
		}
		if missing := catalogRequiredConfigFieldMissing(fieldsByKey, req.Config); missing != "" {
			jsonReply(w, http.StatusUnprocessableEntity, map[string]string{"code": "required_config_missing", "message": "缺少必填配置字段: " + missing})
			return
		}
		created, err := taskSvc.EnqueueTemporaryInstall(r.Context(), host.ID, req.Source, req.App, pkg.Revision, pkg.SHA256, req.Config, pkg.ZIP)
		if errors.Is(err, storage.ErrReverseTaskUnavailable) {
			jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "临时主机已过期或已撤销"})
			return
		}
		if errors.Is(err, storage.ErrReverseTaskConflict) {
			jsonReply(w, http.StatusConflict, map[string]string{"code": "preinstall_conflict", "message": "该主机已有相同应用的预安装任务"})
			return
		}
		if err != nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "preinstall_failed", "message": "创建预安装任务失败"})
			return
		}
		jsonReply(w, http.StatusAccepted, map[string]any{"accepted": true, "tasks": created})
	})

	// ACK 只允许在临时主机尚未到期时完成；执行开始后的状态回传可在自然到期后继续。
	mux.HandleFunc("POST /api/v1/agent/tasks/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || len(token) != 64 {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 授权无效"})
			return
		}
		var req struct { AgentID string `json:"agent_id"` }
		if !decode(w, r, &req, 2048) {
			return
		}
		if _, err := service.AuthenticateReverseAgent(r.Context(), req.AgentID, token); err != nil {
			jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "临时主机授权无效或已过期"})
			return
		}
		if taskSvc == nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "task_unavailable", "message": "任务服务不可用"})
			return
		}
		task, err := taskSvc.AcknowledgeReverseTask(r.Context(), req.AgentID, r.PathValue("id"))
		if err != nil {
			jsonReply(w, http.StatusConflict, map[string]string{"code": "task_ack_rejected", "message": "任务领取确认失败或任务已失效"})
			return
		}
		jsonReply(w, http.StatusOK, map[string]any{"accepted": true, "task": task})
	})

	mux.HandleFunc("POST /api/v1/agent/tasks/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || len(token) != 64 {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 授权无效"})
			return
		}
		var req struct {
			AgentID string `json:"agent_id"`
			State   string `json:"state"`
			Message string `json:"message"`
		}
		if !decode(w, r, &req, 4096) {
			return
		}
		if req.State != "running" && req.State != "succeeded" && req.State != "failed" && req.State != "interrupted" {
			jsonReply(w, http.StatusBadRequest, map[string]string{"code": "invalid_task_state", "message": "任务状态无效"})
			return
		}
		if _, err := service.AuthenticateReverseTaskAgent(r.Context(), req.AgentID, token); err != nil {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 身份或任务授权无效"})
			return
		}
		if taskSvc == nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "task_unavailable", "message": "任务服务不可用"})
			return
		}
		if err := taskSvc.UpdateReverseTaskState(r.Context(), req.AgentID, r.PathValue("id"), req.State, req.Message); err != nil {
			jsonReply(w, http.StatusConflict, map[string]string{"code": "task_status_rejected", "message": "任务状态回传失败"})
			return
		}
		jsonReply(w, http.StatusOK, map[string]bool{"accepted": true})
	})

	mux.HandleFunc("GET /api/v1/agent/tasks/{id}/bundle", func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		agentID := r.Header.Get("X-EHA-Agent-ID")
		if !ok || len(token) != 64 || agentID == "" {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 授权无效"})
			return
		}
		if _, err := service.AuthenticateReverseTaskAgent(r.Context(), agentID, token); err != nil {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 身份或任务授权无效"})
			return
		}
		if taskSvc == nil {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{"code": "bundle_unavailable", "message": "Bundle 服务不可用"})
			return
		}
		data, digest, err := taskSvc.ReverseBundle(r.Context(), agentID, r.PathValue("id"))
		if err != nil {
			jsonReply(w, http.StatusNotFound, map[string]string{"code": "bundle_not_found", "message": "任务安装包不存在或无权访问"})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Bundle-SHA256", digest)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+r.PathValue("id")+".zip\"")
		_, _ = w.Write(data)
	})

}
