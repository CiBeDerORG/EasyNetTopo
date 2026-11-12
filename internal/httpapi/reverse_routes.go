package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func reverseRoutes(mux *http.ServeMux, db *storage.Store, service *hosts.Service) {
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
		h, err := service.AuthenticateReverseAgent(r.Context(), req.AgentID, token)
		if errors.Is(err, storage.ErrTemporaryHostExpired) {
			jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "临时主机已过期或已撤销"})
			return
		}
		if err != nil {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "Agent 身份或授权无效"})
			return
		}
		h, err = service.UpdateReversePoll(r.Context(), req.AgentID)
		if err != nil {
			jsonReply(w, http.StatusGone, map[string]string{"code": "temporary_host_expired", "message": "临时主机已过期或已撤销"})
			return
		}
		jsonReply(w, http.StatusOK, map[string]any{
			"status": "accepted", "controller_id": db.ControllerID(),
			"expires_at": h.ExpiresAt, "next_poll_at": h.NextPollAt,
			"next_poll_seconds": 10, "task": nil,
		})
	})
}
