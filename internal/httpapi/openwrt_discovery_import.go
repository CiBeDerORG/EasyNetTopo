package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"strings"

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
	var snapshot struct {
		Status      string                   `json:"status"`
		HasSnapshot bool                     `json:"has_snapshot"`
		Truncated   bool                     `json:"truncated"`
		Devices     []openWrtDiscoveryRecord `json:"devices"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "发现返回的设备数据结构无效")
	}
	if snapshot.Status == "error" || !snapshot.HasSnapshot {
		return raw, nil
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
	summary, err := devices.SyncGatewayDiscoveredDevices(ctx, gatewayID, kind, items, complete)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "扫描已完成，但自动添加设备失败；请检查设备管理数据后重新扫描")
	}
	var body map[string]json.RawMessage
	if json.Unmarshal(raw, &body) != nil {
		return nil, discoveryError(http.StatusBadGateway, "openwrt_discovery_invalid_response", "发现结果无效")
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "无法生成设备导入统计")
	}
	body["import"] = encoded
	updated, err := json.Marshal(body)
	if err != nil {
		return nil, discoveryError(http.StatusServiceUnavailable, "openwrt_discovery_import_failed", "无法返回设备导入结果")
	}
	return updated, nil
}
