package httpapi

import (
	"encoding/json"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const authCookie = "easynettopo_session"

type attemptLimit struct {
	mu     sync.Mutex
	failed []time.Time
}

func (l *attemptLimit) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	recent := l.failed[:0]
	for _, t := range l.failed {
		if now.Sub(t) < 10*time.Minute {
			recent = append(recent, t)
		}
	}
	l.failed = recent
	if len(recent) >= 8 {
		return false
	}
	l.failed = append(l.failed, now)
	return true
}

func authRoutes(mux *http.ServeMux, store *storage.Store) {
	limiter := &attemptLimit{}
	jsonReply := func(w http.ResponseWriter, status int, val any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(val)
	}
	session := func(r *http.Request) bool {
		c, err := r.Cookie(authCookie)
		return err == nil && store.HasSession(r.Context(), c.Value)
	}
	mutation := func(w http.ResponseWriter, r *http.Request) bool {
		// 禁止普通跨站 HTML 表单；同源前端的请求必须带专用请求头。
		if r.Header.Get("X-ENT-Request") != "1" {
			http.Error(w, "请求来源未授权", 403)
			return false
		}
		if raw := r.Header.Get("Origin"); raw != "" {
			u, err := url.Parse(raw)
			if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "http" && u.Scheme != "https") {
				http.Error(w, "非法 Origin", 403)
				return false
			}
		}
		return true
	}
	mux.HandleFunc("/api/v1/auth/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			methodNotAllowed(w, "GET")
			return
		}
		ready, err := store.Initialized(r.Context())
		if err != nil {
			http.Error(w, "数据库不可用", 503)
			return
		}
		jsonReply(w, 200, map[string]bool{"initialized": ready, "authenticated": session(r)})
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			methodNotAllowed(w, "POST")
			return
		}
		if !mutation(w, r) {
			return
		}
		if !limiter.allow() {
			http.Error(w, "登录尝试过多", 429)
			return
		}
		var v struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4097))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&v) != nil || len(v.Username) > 128 || len(v.Password) > 72 || decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "请求格式错误", 400)
			return
		}
		if store.Verify(r.Context(), v.Username, v.Password) != nil {
			http.Error(w, "账号或密码错误", 401)
			return
		}
		token, expiry, err := store.NewSession(r.Context())
		if err != nil {
			http.Error(w, "登录服务不可用", 503)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: authCookie, Value: token, Path: "/api/v1/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, Expires: expiry})
		jsonReply(w, 200, map[string]string{"username": v.Username})
	})
	mux.HandleFunc("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			methodNotAllowed(w, "POST")
			return
		}
		if !mutation(w, r) {
			return
		}
		if c, err := r.Cookie(authCookie); err == nil {
			_ = store.Revoke(r.Context(), c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: authCookie, Path: "/api/v1/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: -1})
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/v1/system/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			methodNotAllowed(w, "GET")
			return
		}
		if !session(r) {
			http.Error(w, "请先登录", 401)
			return
		}
		jsonReply(w, 200, map[string]string{"controller_id": store.ControllerID()})
	})
}
func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, "Method Not Allowed", 405)
}
