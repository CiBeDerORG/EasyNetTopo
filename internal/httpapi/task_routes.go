package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"github.com/CiBeDerORG/EasyNetTopo/internal/tasks"
)

type hookTarget struct {
	HostName    string `json:"host_name"`
	ServiceName string `json:"service_name"`
}

var triggerPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)

func taskRoutes(mux *http.ServeMux, db *storage.Store, runner *tasks.Service, hostSvc *hosts.Service, catalogSvc *catalog.Service) {
	jsonReply := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	mux.HandleFunc("GET /api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(authCookie)
		if err != nil || !db.HasSession(r.Context(), c.Value) {
			http.Error(w, "请先登录", http.StatusUnauthorized)
			return
		}
		result, err := runner.List(r.Context())
		if err != nil {
			http.Error(w, "无法读取任务", http.StatusServiceUnavailable)
			return
		}
		jsonReply(w, http.StatusOK, map[string]any{"tasks": result})
	})
	mux.HandleFunc("POST /api/hooks/deploy/update", func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("EASYNETTOPO_HOOK_TOKEN")
		if len(secret) < 32 {
			jsonReply(w, http.StatusServiceUnavailable, map[string]string{
				"code": "hook_not_configured", "message": "Hook Token 未配置或长度不足 32 字符",
			})
			return
		}
		auth := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
			jsonReply(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized"})
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "请求必须是 JSON", http.StatusUnsupportedMediaType)
			return
		}
		var req struct {
			TriggerSource string       `json:"trigger_source"`
			Targets       []hookTarget `json:"targets"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil || !triggerPattern.MatchString(req.TriggerSource) ||
			len(req.Targets) < 1 || len(req.Targets) > 64 {
			http.Error(w, "来源或任务目标参数无效", http.StatusBadRequest)
			return
		}
		var extra any
		if !errors.Is(decoder.Decode(&extra), io.EOF) {
			http.Error(w, "请求只能包含一个 JSON 对象", http.StatusBadRequest)
			return
		}
		registered, err := hostSvc.List(r.Context())
		if err != nil {
			http.Error(w, "主机列表不可用", http.StatusServiceUnavailable)
			return
		}
		apps, err := catalogSvc.Apps(r.Context())
		if err != nil {
			http.Error(w, "应用目录不可用", http.StatusServiceUnavailable)
			return
		}
		targets, err := resolveTargets(req.Targets, registered, apps)
		if err != nil {
			jsonReply(w, http.StatusUnprocessableEntity, map[string]string{
				"code": "invalid_targets", "message": err.Error(),
			})
			return
		}
		created, err := runner.Enqueue(r.Context(), req.TriggerSource, targets)
		if err != nil {
			http.Error(w, "任务创建失败", http.StatusServiceUnavailable)
			return
		}
		jsonReply(w, http.StatusAccepted, map[string]any{"accepted": true, "tasks": created})
	})
}

func resolveTargets(req []hookTarget, registered []storage.Host, apps []storage.CatalogApp) ([]storage.TaskTarget, error) {
	targets := make([]storage.TaskTarget, 0, len(req))
	for _, item := range req {
		if item.HostName == "" || item.ServiceName == "" ||
			strings.TrimSpace(item.HostName) != item.HostName ||
			strings.TrimSpace(item.ServiceName) != item.ServiceName {
			return nil, errors.New("主机名称或服务名称不能为空或包含首尾空白")
		}
		hostCount := 0
		var host storage.Host
		for _, h := range registered {
			if h.Name == item.HostName {
				host, hostCount = h, hostCount+1
			}
		}
		if hostCount != 1 || host.AgentID == "" {
			return nil, errors.New("目标主机不存在、名称重复或未配对: " + item.HostName)
		}
		appCount := 0
		var app storage.CatalogApp
		for _, a := range apps {
			if a.AppID == item.ServiceName || a.Name == item.ServiceName {
				app, appCount = a, appCount+1
			}
		}
		if appCount != 1 {
			return nil, errors.New("目标服务不存在或名称重复: " + item.ServiceName)
		}
		targets = append(targets, storage.TaskTarget{
			HostID: host.ID, SourceID: app.SourceID, AppID: app.AppID, Action: "update",
		})
	}
	return targets, nil
}

// enqueueManualUpdate 把现有页面的更新操作接入同一个持久化调度器。
// 不在 HTTP 请求中构建包、上传 ZIP 或等待远程脚本执行。
func enqueueManualUpdate(w http.ResponseWriter, r *http.Request, hostID string, runner *tasks.Service, catalogSvc *catalog.Service) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "需要 JSON", http.StatusUnsupportedMediaType)
		return
	}
	var req struct {
		Source   string `json:"source_id"`
		App      string `json:"app_id"`
		Revision string `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || req.Source == "" || req.App == "" || req.Revision == "" {
		http.Error(w, "无效的应用标识", http.StatusBadRequest)
		return
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		http.Error(w, "请求只能包含一个 JSON 对象", http.StatusBadRequest)
		return
	}
	apps, err := catalogSvc.Apps(r.Context())
	if err != nil {
		http.Error(w, "应用目录不可用", http.StatusServiceUnavailable)
		return
	}
	valid := false
	for _, app := range apps {
		if app.SourceID == req.Source && app.AppID == req.App {
			valid = app.Revision == req.Revision
			break
		}
	}
	if !valid {
		http.Error(w, "应用来源或版本已变化，请刷新后重试", http.StatusConflict)
		return
	}
	created, err := runner.Enqueue(r.Context(), "manual", []storage.TaskTarget{
		{HostID: hostID, SourceID: req.Source, AppID: req.App, Action: "update"},
	})
	if err != nil {
		http.Error(w, "创建更新任务失败", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true, "tasks": created})
}
