package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"
)

// DiscoveredDeviceInput 是经过接口层字段提取的设备候选，不能由浏览器任意指定所属网关。
type DiscoveredDeviceInput struct {
	Name         string
	IPAddress    string
	MACAddress   string
	Manufacturer string
	Model        string
	Source       string
}

type DiscoveredDeviceImportResult struct {
	Created  int `json:"created"`
	Existing int `json:"existing"`
	Skipped  int `json:"skipped"`
}

// ImportGatewayDiscoveredDevices 在一个事务中对同一节点做保守去重。
// 只创建新设备，不更改已有设备的名称、网关归属、语义标签或手工维护的字段。
func (s *DeviceStore) ImportGatewayDiscoveredDevices(ctx context.Context, gatewayID string, candidates []DiscoveredDeviceInput) (DiscoveredDeviceImportResult, error) {
	var result DiscoveredDeviceImportResult
	if len(candidates) > 256 {
		return result, ErrInvalidDevice
	}

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	var networkKey, peerID, ownedGroupID, kinds string
	var isGateway, legacyOpenWrt bool
	err = tx.QueryRowContext(ctx, `SELECT d.network_key,d.peer_id,d.is_gateway,d.semantic_kinds,d.is_openwrt,
		COALESCE((SELECT gg.id FROM device_gateway_groups gg
			WHERE gg.gateway_device_id=d.id AND gg.network_key=d.network_key AND gg.peer_id=d.peer_id LIMIT 1),'')
		FROM devices d WHERE d.id=?`, gatewayID).
		Scan(&networkKey, &peerID, &isGateway, &kinds, &legacyOpenWrt, &ownedGroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrDeviceNotFound
	}
	if err != nil {
		return result, err
	}
	var semanticKinds []string
	if err := json.Unmarshal([]byte(kinds), &semanticKinds); err != nil {
		return result, err
	}
	if !isGateway || (!hasSemanticKind(semanticKinds, "openwrt") && !legacyOpenWrt) ||
		networkKey == "" || peerID == "" || ownedGroupID == "" {
		return result, ErrInvalidDevice
	}

	// 整个网络节点内的设备都参与去重，包括直属设备、手工录入设备和其他网关下的设备。
	rows, err := tx.QueryContext(ctx, `SELECT ip_address,mac_address FROM devices
		WHERE network_key=? AND peer_id=?`, networkKey, peerID)
	if err != nil {
		return result, err
	}
	ips, macs := make(map[string]bool), make(map[string]bool)
	for rows.Next() {
		var ip, mac string
		if err := rows.Scan(&ip, &mac); err != nil {
			rows.Close()
			return result, err
		}
		if ip != "" {
			ips[ip] = true
		}
		if mac != "" {
			macs[strings.ToLower(mac)] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range candidates {
		ipAddress := strings.TrimSpace(item.IPAddress)
		macAddress := strings.TrimSpace(item.MACAddress)
		if ipAddress == "" && macAddress == "" {
			result.Skipped++
			continue
		}
		if ipAddress != "" {
			ip, err := netip.ParseAddr(ipAddress)
			if err != nil || !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() || ip.IsLoopback() || ip.Zone() != "" {
				result.Skipped++
				continue
			}
			ipAddress = ip.String()
		}
		if macAddress != "" {
			mac, err := net.ParseMAC(macAddress)
			if err != nil || len(mac) != 6 {
				result.Skipped++
				continue
			}
			macAddress = strings.ToLower(mac.String())
		}
		if (ipAddress != "" && ips[ipAddress]) || (macAddress != "" && macs[macAddress]) {
			result.Existing++
			continue
		}

		name := discoveredField(item.Name, 100)
		if name == "" {
			if ipAddress != "" {
				name = ipAddress
			} else {
				name = macAddress
			}
		}
		source := "OpenWrt 服务发现"
		if item.Source == "adb" {
			source = "OpenWrt ADB 发现"
		}
		input := DeviceInput{
			Name: name, IPAddress: ipAddress, MACAddress: macAddress,
			Manufacturer: discoveredField(item.Manufacturer, 100), Model: discoveredField(item.Model, 120),
			Note: "来源：" + source, NetworkKey: networkKey, PeerID: peerID, GatewayGroupID: ownedGroupID,
		}
		if err := normalizeDevice(&input); err != nil {
			result.Skipped++
			continue
		}
		id, err := uuid()
		if err != nil {
			return result, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO devices
			(id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,
			manufacturer,model,ip_address,mac_address,note,is_openwrt,network_key,peer_id,created_at,updated_at)
			VALUES(?,?,NULL,?,'通用设备','[]',0,0,?,?,?,?,?,0,?,?,?,?)`,
			id, input.Name, ownedGroupID, input.Manufacturer, input.Model, input.IPAddress,
			input.MACAddress, input.Note, networkKey, peerID, now, now)
		if err != nil {
			return result, err
		}
		if ipAddress != "" {
			ips[ipAddress] = true
		}
		if macAddress != "" {
			macs[macAddress] = true
		}
		result.Created++
	}
	if err := tx.Commit(); err != nil {
		return DiscoveredDeviceImportResult{}, err
	}
	return result, nil
}

// 外部扫描字段只作为展示信息：控制字符去除、限制长度，缺字段时不猜测厂商或型号。
func discoveredField(value string, maxRunes int) string {
	value = strings.TrimSpace(strings.Map(func(char rune) rune {
		if char == '\u0000' || char == '\n' || char == '\r' || char == '\t' {
			return ' '
		}
		return char
	}, value))
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes])
	}
	return strings.TrimSpace(value)
}
