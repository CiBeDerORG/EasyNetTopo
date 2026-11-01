package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

// NewHandler 建立最小 HTTP 路由，业务写操作将在完成认证后单独开放。
func NewHandler(webDir string, db *storage.Store, providers ...*catalog.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Status  string `json:"status"`
			Service string `json:"service"`
			Version int    `json:"api_version"`
		}{Status: "ok", Service: "easynettopo", Version: 1})
	})

	authRoutes(mux, db)
	if len(providers) != 0 && providers[0] != nil { catalogRoutes(mux, db, providers[0]) }

	// 没有声明的 /api 路由不能落入 SPA 回退逻辑。
	mux.Handle("/api/", http.NotFoundHandler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		index := filepath.Join(webDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "前端尚未构建", http.StatusServiceUnavailable)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/")
		target := filepath.Join(webDir, filepath.Clean(rel))
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			http.FileServer(http.Dir(webDir)).ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}
