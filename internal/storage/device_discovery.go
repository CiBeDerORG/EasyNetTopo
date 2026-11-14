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
	Name           string
	IPAddress      string
	NetworkIP      string
	NetworkIPKnown bool
	MACAddress     string
	Manufacturer   string
	Model          string
	Source         string
	Observation    string
}

type DiscoveredDeviceImportResult struct {
	Created  int `json:"created"`
	Existing int `json:"existing"`
	Skipped  int `json:"skipped"`
}

// ImportGatewayDiscoveredDevices 保留原有导入入口；不做离线批量判定。
func (s *DeviceStore) ImportGatewayDiscoveredDevices(ctx context.Context, gatewayID string, candidates []DiscoveredDeviceInput) (DiscoveredDeviceImportResult, error) {
	return s.syncGatewayDiscoveredDevices(ctx, gatewayID, "", candidates, false)
}

// SyncGatewayDiscoveredDevices 将单一来源的一次扫描作为状态同步。
// 只有完整成功的快照才会把该来源之前观测到、但本次缺席的设备标记为离线。
func (s *DeviceStore) SyncGatewayDiscoveredDevices(ctx context.Context, gatewayID, source string, candidates []DiscoveredDeviceInput, complete bool) (DiscoveredDeviceImportResult, error) {
	if source != "devices" && source != "adb" {
		return DiscoveredDeviceImportResult{}, ErrInvalidDevice
	}
	for i := range candidates {
		candidates[i].Source = source
	}
	return s.syncGatewayDiscoveredDevices(ctx, gatewayID, source, candidates, complete)
}

func (s *DeviceStore) syncGatewayDiscoveredDevices(ctx context.Context, gatewayID, completeSource string, candidates []DiscoveredDeviceInput, complete bool) (DiscoveredDeviceImportResult, error) {
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
	rows, err := tx.QueryContext(ctx, `SELECT id,ip_address,mac_address,COALESCE(gateway_group_id,'') FROM devices
		WHERE network_key=? AND peer_id=?`, networkKey, peerID)
	if err != nil {
		return result, err
	}
	type identitySet map[string]struct{}
	ips, macs := make(map[string]identitySet), make(map[string]identitySet)
	deviceGatewayGroups := make(map[string]string)
	deviceIPs := make(map[string]string)
	addIdentity := func(index map[string]identitySet, key, id string) {
		if key == "" {
			return
		}
		if index[key] == nil {
			index[key] = make(identitySet)
		}
		index[key][id] = struct{}{}
	}
	removeIdentity := func(index map[string]identitySet, key, id string) {
		if key == "" || index[key] == nil {
			return
		}
		delete(index[key], id)
		if len(index[key]) == 0 {
			delete(index, key)
		}
	}
	for rows.Next() {
		var id, ipAddress, macAddress, groupID string
		if err := rows.Scan(&id, &ipAddress, &macAddress, &groupID); err != nil {
			_ = rows.Close()
			return result, err
		}
		deviceGatewayGroups[id] = groupID
		deviceIPs[id] = ipAddress
		addIdentity(ips, ipAddress, id)
		addIdentity(macs, strings.ToLower(macAddress), id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if complete && completeSource != "" {
		// 完整扫描开始时，先将当前网关组内所有设备置为该来源的离线；
		// 随后本次明确观测到的设备再更新为在线。网关组外设备不会被触碰。
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO device_discovery_observations
			(device_id,gateway_device_id,source,status,last_seen_at,updated_at)
			SELECT id,?,?,'offline','',? FROM devices WHERE gateway_group_id=?`,
			gatewayID, completeSource, now, ownedGroupID); err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE device_discovery_observations
			SET status='offline',updated_at=?
			WHERE gateway_device_id=? AND source=?
				AND device_id IN (SELECT id FROM devices WHERE gateway_group_id=?)`,
			now, gatewayID, completeSource, ownedGroupID); err != nil {
			return result, err
		}
	}

	findMatch := func(index map[string]identitySet, key string) (string, bool) {
		matches := index[key]
		if len(matches) == 0 {
			return "", false
		}
		if len(matches) != 1 {
			return "", true
		}
		for id := range matches {
			return id, false
		}
		return "", false
	}

	for _, item := range candidates {
		source := strings.TrimSpace(item.Source)
		if source == "" {
			source = "devices"
		}
		if source != "devices" && source != "adb" {
			result.Skipped++
			continue
		}
		observation := strings.TrimSpace(item.Observation)
		if observation == "" {
			observation = "recent"
		}
		if observation != "recent" && observation != "lease" && observation != "cached" {
			result.Skipped++
			continue
		}
		// cached 是扩展包保留的历史快照，不是本次观测；不能新增设备或刷新在线状态。
		if observation == "cached" {
			result.Skipped++
			continue
		}

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
		networkIP := ""
		networkIPKnown := item.NetworkIPKnown && ipAddress != ""
		if networkIPKnown && strings.TrimSpace(item.NetworkIP) != "" {
			mapped, parseErr := netip.ParseAddr(strings.TrimSpace(item.NetworkIP))
			if parseErr == nil && mapped.Is4() && mapped.IsGlobalUnicast() && !mapped.IsLinkLocalUnicast() && !mapped.IsLoopback() && mapped.Zone() == "" {
				networkIP = mapped.String()
			}
		}
		if macAddress != "" {
			mac, err := net.ParseMAC(macAddress)
			if err != nil || len(mac) != 6 || mac[0]&1 != 0 {
				result.Skipped++
				continue
			}
			macAddress = strings.ToLower(mac.String())
		}

		ipMatch, ipAmbiguous := findMatch(ips, ipAddress)
		macMatch, macAmbiguous := findMatch(macs, macAddress)
		if ipAmbiguous || macAmbiguous || (ipMatch != "" && macMatch != "" && ipMatch != macMatch) {
			// 身份信息指向多个历史设备时不擅自合并，也不更新在线状态。
			result.Skipped++
			continue
		}
		deviceID := ipMatch
		if deviceID == "" {
			deviceID = macMatch
		}
		if deviceID != "" {
			result.Existing++
		} else {
			name := discoveredField(item.Name, 100)
			if name == "" {
				if ipAddress != "" {
					name = ipAddress
				} else {
					name = macAddress
				}
			}
			note := "来源：OpenWrt 服务发现"
			if source == "adb" {
				note = "来源：OpenWrt ADB 发现"
			}
			input := DeviceInput{
				Name: name, IPAddress: ipAddress, MACAddress: macAddress,
				Manufacturer: discoveredField(item.Manufacturer, 100), Model: discoveredField(item.Model, 120),
				Note: note, NetworkKey: networkKey, PeerID: peerID, GatewayGroupID: ownedGroupID,
			}
			if err := normalizeDevice(&input); err != nil {
				result.Skipped++
				continue
			}
			deviceID, err = uuid()
			if err != nil {
				return result, err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO devices
				(id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,
				manufacturer,model,ip_address,network_ip,mac_address,note,is_openwrt,network_key,peer_id,created_at,updated_at)
				VALUES(?,?,NULL,?,'通用设备','[]',0,0,?,?,?,?,?,?,0,?,?,?,?)`,
				deviceID, input.Name, ownedGroupID, input.Manufacturer, input.Model, input.IPAddress, networkIP,
				input.MACAddress, input.Note, networkKey, peerID, now, now)
			if err != nil {
				return result, err
			}
			deviceGatewayGroups[deviceID] = ownedGroupID
			deviceIPs[deviceID] = ipAddress
			addIdentity(ips, ipAddress, deviceID)
			addIdentity(macs, macAddress, deviceID)
			result.Created++
		}

		// 已有设备通过 MAC 或 IP 匹配后，也必须同步本次观测到的内网 IP。
		// 组网 IP 与内网 IP 在同一条 UPDATE 中更新，避免出现新内网 IP 搭配旧组网 IP。
		// 如果内网 IP 已变化但本次路由映射暂不可用，清空旧组网 IP，避免保留错误映射；
		// 如果内网 IP 未变化且映射暂不可用，则保留已有组网 IP。
		oldIPAddress := deviceIPs[deviceID]
		updateIPAddress := ipAddress != "" && ipAddress != oldIPAddress
		updateNetworkIP := networkIPKnown || updateIPAddress
		if updateIPAddress || updateNetworkIP {
			if _, err := tx.ExecContext(ctx, `UPDATE devices SET
				ip_address=CASE WHEN ?<>'' THEN ? ELSE ip_address END,
				network_ip=CASE WHEN ? THEN ? ELSE network_ip END,
				updated_at=?
				WHERE id=? AND (
					(?<>'' AND ip_address<>?) OR (? AND network_ip<>?)
				)`,
				ipAddress, ipAddress, updateNetworkIP, networkIP, now, deviceID,
				ipAddress, ipAddress, updateNetworkIP, networkIP); err != nil {
				return result, err
			}
		}
		if updateIPAddress {
			removeIdentity(ips, oldIPAddress, deviceID)
			deviceIPs[deviceID] = ipAddress
			addIdentity(ips, ipAddress, deviceID)
		}

		// 已存在但不属于当前网关组的直属设备或其他网关设备只参与去重，不改写其发现状态。
		if deviceGatewayGroups[deviceID] != ownedGroupID {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO device_discovery_observations
			(device_id,gateway_device_id,source,status,last_seen_at,updated_at)
			VALUES(?,?,?,'online',?,?)
			ON CONFLICT(device_id,gateway_device_id,source) DO UPDATE SET
				status='online',last_seen_at=excluded.last_seen_at,updated_at=excluded.updated_at`,
			deviceID, gatewayID, source, now, now)
		if err != nil {
			return result, err
		}
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
