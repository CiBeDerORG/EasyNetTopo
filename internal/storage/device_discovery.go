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
	NetworkIPStatus string
	MACAddress     string
	Manufacturer   string
	Model          string
	Source         string
	Observation    string
}

type DiscoveredDeviceSyncDiagnostic struct {
	DeviceID               string `json:"device_id,omitempty"`
	Name                   string `json:"name,omitempty"`
	Source                 string `json:"source,omitempty"`
	Observation            string `json:"observation,omitempty"`
	IPAddress              string `json:"ip_address,omitempty"`
	MACAddress             string `json:"mac_address,omitempty"`
	MatchedBy              string `json:"matched_by,omitempty"`
	Result                 string `json:"result"`
	Reason                 string `json:"reason,omitempty"`
	PreviousIPAddress      string `json:"previous_ip_address,omitempty"`
	CurrentIPAddress       string `json:"current_ip_address,omitempty"`
	PreviousNetworkIP      string `json:"previous_network_ip,omitempty"`
	CurrentNetworkIP       string `json:"current_network_ip,omitempty"`
	NetworkIPAction        string `json:"network_ip_action,omitempty"`
	NetworkIPMappingStatus string `json:"network_ip_mapping_status,omitempty"`
	AddressFieldsUpdated  bool   `json:"address_fields_updated"`
	DatabaseUpdated        bool   `json:"database_updated"`
	DiscoveryStatusUpdated bool   `json:"discovery_status_updated"`
}

type DiscoveredDeviceImportResult struct {
	Created              int                              `json:"created"`
	Existing             int                              `json:"existing"`
	Skipped              int                              `json:"skipped"`
	NetworkIPWritten     int                              `json:"network_ip_written"`
	NetworkIPCleared     int                              `json:"network_ip_cleared"`
	NetworkIPUnchanged   int                              `json:"network_ip_unchanged"`
	NetworkIPUnavailable int                              `json:"network_ip_unavailable"`
	NetworkIPUnmapped    int                              `json:"network_ip_unmapped"`
	Devices              []DiscoveredDeviceSyncDiagnostic `json:"devices"`
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
	result.Devices = make([]DiscoveredDeviceSyncDiagnostic, 0, len(candidates))
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
	rows, err := tx.QueryContext(ctx, "SELECT id,name,ip_address,network_ip,mac_address,COALESCE(gateway_group_id,'') FROM devices WHERE network_key=? AND peer_id=?", networkKey, peerID)
	if err != nil {
		return result, err
	}
	type identitySet map[string]struct{}
	ips, macs := make(map[string]identitySet), make(map[string]identitySet)
	deviceGatewayGroups := make(map[string]string)
	deviceNames := make(map[string]string)
	deviceIPs := make(map[string]string)
	deviceNetworkIPs := make(map[string]string)
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
		var id, name, ipAddress, networkIP, macAddress, groupID string
		if err := rows.Scan(&id, &name, &ipAddress, &networkIP, &macAddress, &groupID); err != nil {
			_ = rows.Close()
			return result, err
		}
		deviceGatewayGroups[id] = groupID
		deviceNames[id] = name
		deviceIPs[id] = ipAddress
		deviceNetworkIPs[id] = networkIP
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
		diagnostic := DiscoveredDeviceSyncDiagnostic{
			Name:        discoveredField(item.Name, 100),
			Source:      strings.TrimSpace(item.Source),
			Observation: strings.TrimSpace(item.Observation),
			IPAddress:   strings.TrimSpace(item.IPAddress),
			MACAddress:             strings.TrimSpace(item.MACAddress),
			NetworkIPMappingStatus: strings.TrimSpace(item.NetworkIPStatus),
			Result:                  "skipped",
		}
		recordSkipped := func(reason string) {
			diagnostic.Result = "skipped"
			diagnostic.Reason = reason
			result.Skipped++
			result.Devices = append(result.Devices, diagnostic)
		}

		source := strings.TrimSpace(item.Source)
		if source == "" {
			source = "devices"
		}
		if source != "devices" && source != "adb" {
			recordSkipped("不支持的发现来源")
			continue
		}
		diagnostic.Source = source
		observation := strings.TrimSpace(item.Observation)
		if observation == "" {
			observation = "recent"
		}
		diagnostic.Observation = observation
		if observation != "recent" && observation != "lease" && observation != "cached" {
			recordSkipped("发现状态值无效")
			continue
		}
		// cached 是扩展包保留的历史快照，不是本次观测；不能新增设备或刷新在线状态。
		if observation == "cached" {
			recordSkipped("历史缓存不是本次实时观测，不参与入库")
			continue
		}

		ipAddress := strings.TrimSpace(item.IPAddress)
		macAddress := strings.TrimSpace(item.MACAddress)
		if ipAddress == "" && macAddress == "" {
			recordSkipped("缺少可用于定位设备的 IP 和 MAC")
			continue
		}
		if ipAddress != "" {
			ip, err := netip.ParseAddr(ipAddress)
			if err != nil || !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() || ip.IsLoopback() || ip.Zone() != "" {
				recordSkipped("IP 地址格式无效或属于不可登记地址")
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
				recordSkipped("MAC 地址格式无效或属于组播地址")
				continue
			}
			macAddress = strings.ToLower(mac.String())
		}
		diagnostic.IPAddress = ipAddress
		diagnostic.MACAddress = macAddress

		ipMatch, ipAmbiguous := findMatch(ips, ipAddress)
		macMatch, macAmbiguous := findMatch(macs, macAddress)
		if ipAmbiguous || macAmbiguous || (ipMatch != "" && macMatch != "" && ipMatch != macMatch) {
			// 身份信息指向多个历史设备时不擅自合并，也不更新在线状态。
			recordSkipped("IP 与 MAC 匹配到多个不同历史设备，无法安全合并")
			continue
		}
		deviceID := ipMatch
		matchedBy := "new"
		if ipMatch != "" && macMatch != "" {
			matchedBy = "ip+mac"
		} else if ipMatch != "" {
			matchedBy = "ip"
		} else if macMatch != "" {
			deviceID = macMatch
			matchedBy = "mac"
		}
		diagnostic.MatchedBy = matchedBy

		created := false
		if deviceID != "" {
			result.Existing++
			diagnostic.Result = "existing"
			diagnostic.DeviceID = deviceID
			diagnostic.Name = deviceNames[deviceID]
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
				recordSkipped("设备字段校验失败")
				continue
			}
			deviceID, err = uuid()
			if err != nil {
				return result, err
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO devices (id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,manufacturer,model,ip_address,network_ip,mac_address,note,is_openwrt,network_key,peer_id,created_at,updated_at) VALUES(?,?,NULL,?,'通用设备','[]',0,0,?,?,?,?,?,?,0,?,?,?,?)",
				deviceID, input.Name, ownedGroupID, input.Manufacturer, input.Model, input.IPAddress, networkIP,
				input.MACAddress, input.Note, networkKey, peerID, now, now)
			if err != nil {
				return result, err
			}
			deviceGatewayGroups[deviceID] = ownedGroupID
			deviceNames[deviceID] = input.Name
			deviceIPs[deviceID] = ipAddress
			deviceNetworkIPs[deviceID] = networkIP
			addIdentity(ips, ipAddress, deviceID)
			addIdentity(macs, macAddress, deviceID)
			result.Created++
			created = true
			diagnostic.Result = "created"
			diagnostic.DeviceID = deviceID
			diagnostic.Name = input.Name
			diagnostic.AddressFieldsUpdated = true
			diagnostic.DatabaseUpdated = true
			diagnostic.Reason = "新设备已插入当前网关设备组"
		}

		oldIPAddress := deviceIPs[deviceID]
		oldNetworkIP := deviceNetworkIPs[deviceID]
		if created {
			diagnostic.CurrentIPAddress = ipAddress
			diagnostic.CurrentNetworkIP = networkIP
		} else {
			diagnostic.PreviousIPAddress = oldIPAddress
			diagnostic.PreviousNetworkIP = oldNetworkIP
			diagnostic.CurrentIPAddress = oldIPAddress
			diagnostic.CurrentNetworkIP = oldNetworkIP
		}

		updateIPAddress := ipAddress != "" && ipAddress != oldIPAddress
		updateNetworkIP := networkIPKnown || updateIPAddress
		if !created && (updateIPAddress || updateNetworkIP) {
			writeResult, err := tx.ExecContext(ctx, "UPDATE devices SET ip_address=CASE WHEN ?<>'' THEN ? ELSE ip_address END, network_ip=CASE WHEN ? THEN ? ELSE network_ip END, updated_at=? WHERE id=? AND ((?<>'' AND ip_address<>?) OR (? AND network_ip<>?))",
				ipAddress, ipAddress, updateNetworkIP, networkIP, now, deviceID,
				ipAddress, ipAddress, updateNetworkIP, networkIP)
			if err != nil {
				return result, err
			}
			affected, err := writeResult.RowsAffected()
			if err != nil {
				return result, err
			}
			diagnostic.AddressFieldsUpdated = affected > 0
			diagnostic.DatabaseUpdated = affected > 0
		}
		if updateIPAddress {
			removeIdentity(ips, oldIPAddress, deviceID)
			deviceIPs[deviceID] = ipAddress
			addIdentity(ips, ipAddress, deviceID)
			diagnostic.CurrentIPAddress = ipAddress
		}
		if updateNetworkIP {
			diagnostic.CurrentNetworkIP = networkIP
			deviceNetworkIPs[deviceID] = networkIP
		}

		switch {
		case ipAddress == "":
			diagnostic.NetworkIPAction = "no_ip"
		case created && networkIP != "":
			diagnostic.NetworkIPAction = "written"
		case created && networkIPKnown:
			diagnostic.NetworkIPAction = "unmapped"
		case created:
			diagnostic.NetworkIPAction = "unavailable"
		case networkIPKnown && networkIP == oldNetworkIP:
			diagnostic.NetworkIPAction = "unchanged"
		case networkIPKnown && networkIP != "":
			diagnostic.NetworkIPAction = "written"
		case networkIPKnown && oldNetworkIP != "":
			diagnostic.NetworkIPAction = "cleared"
		case networkIPKnown:
			diagnostic.NetworkIPAction = "unmapped"
		case updateIPAddress && oldNetworkIP != "":
			diagnostic.NetworkIPAction = "unavailable_cleared"
		case updateIPAddress:
			diagnostic.NetworkIPAction = "unavailable"
		default:
			diagnostic.NetworkIPAction = "unavailable_preserved"
		}
		switch diagnostic.NetworkIPAction {
		case "written":
			result.NetworkIPWritten++
		case "cleared", "unavailable_cleared":
			result.NetworkIPCleared++
		case "unchanged":
			result.NetworkIPUnchanged++
		case "unavailable", "unavailable_cleared", "unavailable_preserved":
			result.NetworkIPUnavailable++
		case "unmapped":
			result.NetworkIPUnmapped++
		}

		// 已存在但不属于当前网关组的直属设备或其他网关设备只参与去重，不改写其发现状态。
		if deviceGatewayGroups[deviceID] != ownedGroupID {
			diagnostic.Reason = "已匹配当前网络节点中的设备，但不属于本网关设备组；仅按既有规则同步 IP，不更新发现状态"
			result.Devices = append(result.Devices, diagnostic)
			continue
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO device_discovery_observations (device_id,gateway_device_id,source,status,last_seen_at,updated_at) VALUES(?,?,?,'online',?,?) ON CONFLICT(device_id,gateway_device_id,source) DO UPDATE SET status='online',last_seen_at=excluded.last_seen_at,updated_at=excluded.updated_at",
			deviceID, gatewayID, source, now, now)
		if err != nil {
			return result, err
		}
		diagnostic.DiscoveryStatusUpdated = true
		diagnostic.DatabaseUpdated = true
		if diagnostic.Reason == "" {
			if created {
				diagnostic.Reason = "新设备已登记，并更新为本次在线观测"
			} else {
				diagnostic.Reason = "按 " + matchedBy + " 命中历史设备，已执行本次同步"
			}
		}
		result.Devices = append(result.Devices, diagnostic)
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
