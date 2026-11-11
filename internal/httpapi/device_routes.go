package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func deviceRoutes(mux *http.ServeMux, db *storage.Store, devices *storage.DeviceStore) {
	authorize := func(w http.ResponseWriter, r *http.Request) bool {
		c, err := r.Cookie(authCookie)
		if err != nil || !db.HasSession(r.Context(), c.Value) {
			http.Error(w, "需要管理员登录", http.StatusUnauthorized)
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
			parsed, err := url.Parse(raw)
			if err != nil || !strings.EqualFold(parsed.Host, r.Host) ||
				(parsed.Scheme != "http" && parsed.Scheme != "https") {
				http.Error(w, "非法 Origin", http.StatusForbidden)
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
	decode := func(w http.ResponseWriter, r *http.Request, target any) bool {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "请求必须是 JSON", http.StatusUnsupportedMediaType)
			return false
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			http.Error(w, "请求参数无效", http.StatusBadRequest)
			return false
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			http.Error(w, "仅允许一个 JSON 对象", http.StatusBadRequest)
			return false
		}
		return true
	}
	mapError := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, storage.ErrDeviceNotFound), errors.Is(err, storage.ErrDeviceGroupNotFound):
			reply(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": "设备或分组不存在"})
		case errors.Is(err, storage.ErrInvalidDevice):
			reply(w, http.StatusBadRequest, map[string]string{"code": "invalid_device", "message": "设备名称、类型名称、语义标签、IP、MAC 或网络节点关联信息不合法"})
		case errors.Is(err, storage.ErrDeviceGroupExists):
			reply(w, http.StatusConflict, map[string]string{"code": "device_group_exists", "message": "同名设备分组已存在"})
		case errors.Is(err, storage.ErrInvalidDeviceGroup):
			reply(w, http.StatusBadRequest, map[string]string{"code": "invalid_device_group", "message": "分组名称不能为空且不能超过 64 个字符"})
		case errors.Is(err, storage.ErrDeviceNodeConflict):
			reply(w, http.StatusConflict, map[string]string{"code": "device_node_conflict", "message": "该网络节点已经关联其他设备"})
		case errors.Is(err, storage.ErrDeviceNotOpenWrt):
			reply(w, http.StatusConflict, map[string]string{"code": "device_not_openwrt", "message": "只有标记为 OpenWrt 的设备才能配置发现服务"})
		case errors.Is(err, storage.ErrInvalidOpenWrtConfig):
			reply(w, http.StatusBadRequest, map[string]string{"code": "invalid_openwrt_config", "message": "OpenWrt RPCD 地址或专用账号配置不合法；请检查设备 IP、端口、用户名和密钥"})
		default:
			reply(w, http.StatusServiceUnavailable, map[string]string{"code": "device_storage_unavailable", "message": "设备数据操作失败"})
		}
	}

	mux.HandleFunc("/api/v1/device-groups", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			items, err := devices.ListDeviceGroups(r.Context())
			if err != nil {
				mapError(w, err)
				return
			}
			reply(w, http.StatusOK, map[string]any{"groups": items})
		case http.MethodPost:
			if !mutation(w, r) {
				return
			}
			var req struct {
				Name string `json:"name"`
			}
			if !decode(w, r, &req) {
				return
			}
			item, err := devices.CreateDeviceGroup(r.Context(), req.Name)
			if err != nil {
				mapError(w, err)
				return
			}
			reply(w, http.StatusCreated, item)
		default:
			methodNotAllowed(w, "GET, POST")
		}
	})
	mux.HandleFunc("/api/v1/device-groups/", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/device-groups/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			if !mutation(w, r) {
				return
			}
			var req struct {
				Name string `json:"name"`
			}
			if !decode(w, r, &req) {
				return
			}
			item, err := devices.UpdateDeviceGroup(r.Context(), id, req.Name)
			if err != nil {
				mapError(w, err)
				return
			}
			reply(w, http.StatusOK, item)
		case http.MethodDelete:
			if !mutation(w, r) {
				return
			}
			if err := devices.DeleteDeviceGroup(r.Context(), id); err != nil {
				mapError(w, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, "PATCH, DELETE")
		}
	})
	mux.HandleFunc("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			items, err := devices.ListDevices(r.Context())
			if err != nil {
				mapError(w, err)
				return
			}
			reply(w, http.StatusOK, map[string]any{"devices": items})
		case http.MethodPost:
			if !mutation(w, r) {
				return
			}
			var req storage.DeviceInput
			if !decode(w, r, &req) {
				return
			}
			item, err := devices.CreateDevice(r.Context(), req)
			if err != nil {
				mapError(w, err)
				return
			}
			reply(w, http.StatusCreated, item)
		default:
			methodNotAllowed(w, "GET, POST")
		}
	})
	mux.HandleFunc("/api/v1/devices/", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r) {
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), "/")
		if len(parts) == 2 && parts[0] == "network-nodes" && parts[1] == "sync" {
			switch r.Method {
			case http.MethodGet:
				enabled, err := devices.NetworkNodeSyncEnabled(r.Context())
				if err != nil {
					mapError(w, err)
					return
				}
				reply(w, http.StatusOK, map[string]bool{"enabled": enabled})
			case http.MethodPost:
				if !mutation(w, r) {
					return
				}
				var req struct {
					Nodes []storage.NetworkNodeDeviceInput `json:"nodes"`
				}
				if !decode(w, r, &req) {
					return
				}
				result, err := devices.SyncNetworkNodeDevices(r.Context(), req.Nodes)
				if err != nil {
					mapError(w, err)
					return
				}
				reply(w, http.StatusOK, result)
			default:
				methodNotAllowed(w, "GET, POST")
			}
			return
		}
		if len(parts) == 1 && parts[0] != "" {
			switch r.Method {
			case http.MethodPatch:
				if !mutation(w, r) {
					return
				}
				var req storage.DeviceInput
				if !decode(w, r, &req) {
					return
				}
				item, err := devices.UpdateDevice(r.Context(), parts[0], req)
				if err != nil {
					mapError(w, err)
					return
				}
				reply(w, http.StatusOK, item)
			case http.MethodDelete:
				if !mutation(w, r) {
					return
				}
				if err := devices.DeleteDevice(r.Context(), parts[0]); err != nil {
					mapError(w, err)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusNoContent)
			default:
				methodNotAllowed(w, "PATCH, DELETE")
			}
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "openwrt" {
			switch r.Method {
			case http.MethodGet:
				config, err := devices.DeviceOpenWrtConfig(r.Context(), parts[0])
				if err != nil {
					mapError(w, err)
					return
				}
				reply(w, http.StatusOK, config)
			case http.MethodPut:
				if !mutation(w, r) {
					return
				}
				var req struct {
					RPCDPort int    `json:"rpcd_port"`
					HTTPS    bool   `json:"https"`
					Username string `json:"username"`
					Password string `json:"password"`
				}
				if !decode(w, r, &req) {
					return
				}
				config, err := devices.SaveDeviceOpenWrtConfig(r.Context(), parts[0], req.RPCDPort, req.HTTPS, req.Username, req.Password)
				if err != nil {
					mapError(w, err)
					return
				}
				reply(w, http.StatusOK, config)
			case http.MethodDelete:
				if !mutation(w, r) {
					return
				}
				if err := devices.ClearDeviceOpenWrtConfig(r.Context(), parts[0]); err != nil {
					mapError(w, err)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusNoContent)
			default:
				methodNotAllowed(w, "GET, PUT, DELETE")
			}
			return
		}
		http.NotFound(w, r)
	})
}
