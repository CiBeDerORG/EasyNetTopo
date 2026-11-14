package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type openWrtDiscoveryRecord struct {
	Name        string   `json:"name"`
	Hostname    string   `json:"hostname"`
	MAC         string   `json:"mac"`
	IPv4        []string `json:"ipv4"`
	IPv6        []string `json:"ipv6"`
	Vendor      string   `json:"vendor"`
	Model       string   `json:"model"`
	Instance    string   `json:"instance"`
	Host        string   `json:"host"`
	Address     string   `json:"address"`
	Observation string   `json:"observation"`
}

func firstDiscoveryIP(candidates ...string) string {
	for _, candidate := range candidates {
		ip, err := netip.ParseAddr(strings.TrimSpace(candidate))
		if err == nil && ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && ip.Zone() == "" {
			return ip.String()
		}
	}
	return ""
}

// 仅 POST scan 的有效新快照自动导入；GET list 和失败时留下的旧快照都不能写设备库。
func importOpenWrtDiscoverySnapshot(ctx context.Context, devices *storage.DeviceStore, gatewayID, kind string, raw json.RawMessage) (json.RawMessage, *openWrtDiscoveryError) {
	return importOpenWrtDiscoverySnapshotWithNetworkIP(ctx, devices, gatewayID, kind, raw, openWrtNetworkIPMapper{})
}

type openWrtDiscoveryGatewayDiagnostic struct {
	DeviceID   string `json:"device_id,omitempty"`
	Name       string `json:"name,omitempty"`
	IPAddress  string `json:"ip_address,omitempty"`
	NetworkKey string `json:"network_key,omitempty"`
	PeerID     string `json:"peer_id,omitempty"`
}

type openWrtDiscoverySnapshotDiagnostic struct {
	Status      string `json:"status"`
	HasSnapshot bool   `json:"has_snapshot"`
	Truncated   bool   `json:"truncated"`
	Source      string `json:"source,omitempty"`
	Count       int    `json:"count"`
	DeviceCount int    `json:"device_count"`
	CanImport   bool   `json:"can_import"`
	Message     string `json:"message,omitempty"`
}

type openWrtDiscoveryDiagnostics struct {
	GeneratedAt string                               `json:"generated_at"`
	Gateway     openWrtDiscoveryGatewayDiagnostic    `json:"gateway"`
	Snapshot    openWrtDiscoverySnapshotDiagnostic   `json:"snapshot"`
	Route       openWrtNetworkIPDiagnostics          `json:"route"`
	Import      storage.DiscoveredDeviceImportResult `json:"import"`
}

func attachOpenWrtDiscoveryDiagnostics(raw json.RawMessage, diagnostics openWrtDiscoveryDiagnostics) (json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("发现结果必须是 JSON 对象")
	}
	diagnostics.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	encoded, err := json.Marshal(diagnostics)
	if err != nil {
		return nil, err
	}
	body["diagnostics"] = encoded
	updated, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func importOpenWrtDiscoverySnapshotWithNetworkIP(ctx context.Context, devices *storage.DeviceStore, gatewayID, kind string, raw json.RawMessage, mapper openWrtNetworkIPMapper) (json.RawMessage, *openWrtDiscoveryError) {
	return importOpenWrtDiscoverySnapshotInternal(ctx, devices, storage.Device{ID: gatewayID}, kind, raw, mapper, openWrtNetworkIPDiagnostics{}, false)
}

func importOpenWrtDiscoverySnapshotWithDiagnostics(ctx context.Context, devices *storage.DeviceStore, gateway storage.Device, kind string, raw json.RawMessage, mapper openWrtNetworkIPMapper, routeDiagnostics openWrtNetworkIPDiagnostics) (json.RawMessage, *openWrtDiscoveryError) {
	return importOpenWrtDiscoverySnapshotInternal(ctx, devices, gateway, kind, raw, mapper, routeDiagnostics, true)
}

func importOpenWrtDiscoverySnapshotInternal(ctx context.Context, devices *storage.DeviceStore, gateway storage.Device, kind string, raw json.RawMessage, mapper openWrtNetworkIPMapper, routeDiagnostics openWrtNetworkIPDiagnostics, includeDiagnostics bool) (json.RawMessage, *openWrtDiscoveryError) {
	var snapshot struct {
		Status      string                   `json:"status"`
		HasSnapshot bool                     `json:"has_snapshot"`
		Truncated   bool                     `json:"truncated"`
		Source      string                   `json:"source"`
		Count       int                      `json:"count"`
		Message     string                   `json:"message"`
		Devices     []openWrtDiscoveryRecord `json:"devices"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "发现返回的设备数据结构无效")
	}
	diagnostics := openWrtDiscoveryDiagnostics{
		Gateway: openWrtDiscoveryGatewayDiagnostic{
			DeviceID: gateway.ID, Name: gateway.Name, IPAddress: gateway.IPAddress,
			NetworkKey: gateway.NetworkKey, PeerID: gateway.PeerID,
		},
		Snapshot: openWrtDiscoverySnapshotDiagnostic{
			Status: snapshot.Status, HasSnapshot: snapshot.HasSnapshot, Truncated: snapshot.Truncated,
			Source: snapshot.Source, Count: snapshot.Count, DeviceCount: len(snapshot.Devices),
			CanImport: snapshot.Status != "error" && snapshot.HasSnapshot, Message: snapshot.Message,
		},
		Route:  routeDiagnostics,
		Import: storage.DiscoveredDeviceImportResult{
			Devices: make([]storage.DiscoveredDeviceSyncDiagnostic, 0),
		},
	}
	if diagnostics.Route.Hosts == nil {
		diagnostics.Route.Hosts = make([]openWrtNetworkIPHostDiagnostic, 0)
	}
	if diagnostics.Snapshot.Count == 0 {
		diagnostics.Snapshot.Count = len(snapshot.Devices)
	}
	if snapshot.Status == "error" || !snapshot.HasSnapshot {
		if !includeDiagnostics {
			return raw, nil
		}
		if diagnostics.Route.Reason == "" {
			diagnostics.Route.Reason = "快照无效或不存在，按现有规则跳过 EasyTier 路由映射和设备入库"
		}
		updated, err := attachOpenWrtDiscoveryDiagnostics(raw, diagnostics)
		if err != nil {
			return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "无法附加扫描诊断信息")
		}
		return updated, nil
	}
	// 避免格式异常或不可信设备返回的畸形超量列表造成大规模写入。
	if snapshot.Devices == nil || len(snapshot.Devices) > 256 {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "发现结果缺少有效设备列表或超过最大条目数")
	}
	items := make([]storage.DiscoveredDeviceInput, 0, len(snapshot.Devices))
	for _, record := range snapshot.Devices {
		candidate := storage.DiscoveredDeviceInput{Source: kind, Observation: record.Observation}
		if kind == "adb" {
			candidate.Observation = "recent"
			candidate.Name = record.Instance
			if candidate.Name == "" {
				candidate.Name = record.Host
			}
			candidate.IPAddress = firstDiscoveryIP(record.Address)
		} else {
			candidate.Name = record.Name
			if candidate.Name == "" {
				candidate.Name = record.Hostname
			}
			candidate.MACAddress = record.MAC
			candidate.IPAddress = firstDiscoveryIP(append(record.IPv4, record.IPv6...)...)
			candidate.Manufacturer = record.Vendor
			candidate.Model = record.Model
		}
		candidate.NetworkIP, candidate.NetworkIPKnown = mapper.mapAddress(candidate.IPAddress)
		candidate.NetworkIPStatus = mapper.diagnosticStatus(candidate.IPAddress, candidate.NetworkIP, candidate.NetworkIPKnown)
		// 忽略广播和组播 MAC；只有真实可定位的 MAC/IP 才允许自动登记。
		if candidate.MACAddress != "" {
			mac, err := net.ParseMAC(candidate.MACAddress)
			if err == nil && len(mac) == 6 && (mac[0]&1) != 0 {
				candidate.MACAddress = ""
			}
		}
		items = append(items, candidate)
	}
	// partial 或 truncated 快照只能确认本次明确观测到的设备在线，不能据此把缺席设备判为离线。
	complete := snapshot.Status == "ok" && !snapshot.Truncated
	summary, err := devices.SyncGatewayDiscoveredDevices(ctx, gateway.ID, kind, items, complete)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "扫描已完成，但自动添加设备失败；请检查设备管理数据后重新扫描")
	}
	diagnostics.Import = summary
	var body map[string]json.RawMessage
	if json.Unmarshal(raw, &body) != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "发现结果无效")
	}
	importSummary := struct {
		Created  int `json:"created"`
		Existing int `json:"existing"`
		Skipped  int `json:"skipped"`
	}{Created: summary.Created, Existing: summary.Existing, Skipped: summary.Skipped}
	encoded, err := json.Marshal(importSummary)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "无法生成设备导入统计")
	}
	body["import"] = encoded
	intermediate, err := json.Marshal(body)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "无法生成设备导入结果")
	}
	if !includeDiagnostics {
		return intermediate, nil
	}
	updated, err := attachOpenWrtDiscoveryDiagnostics(intermediate, diagnostics)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "无法生成扫描诊断信息")
	}
	return updated, nil
}