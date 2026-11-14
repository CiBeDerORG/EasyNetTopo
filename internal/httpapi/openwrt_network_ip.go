package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type proxyIPv4Mapping struct {
	source netip.Prefix
	target netip.Prefix
}

type openWrtNetworkIPMapper struct {
	available bool
	mappings  []proxyIPv4Mapping
}

type openWrtNetworkIPHostDiagnostic struct {
	HostID                   string   `json:"host_id"`
	HostName                 string   `json:"host_name"`
	Status                   string   `json:"status"`
	Message                  string   `json:"message,omitempty"`
	InstanceCount            int      `json:"instance_count"`
	MatchingNetworkInstances int      `json:"matching_network_instances"`
	PeerRouteCount           int      `json:"peer_route_count"`
	MatchingPeerRoutes       int      `json:"matching_peer_routes"`
	ProxyCIDRCount           int      `json:"proxy_cidr_count"`
	ValidMappingCount        int      `json:"valid_mapping_count"`
	ObservedNetworkKeys      []string `json:"observed_network_keys,omitempty"`
	ObservedPeerIDs          []string `json:"observed_peer_ids,omitempty"`
	MatchLocation            string   `json:"match_location,omitempty"`
	MappingRules             []string `json:"mapping_rules,omitempty"`
}

type openWrtNetworkIPDiagnostics struct {
	TargetNetworkKey string                           `json:"target_network_key"`
	TargetPeerID     string                           `json:"target_peer_id"`
	NetworkEntries   int                              `json:"network_entries"`
	HostsQueried     int                              `json:"hosts_queried"`
	HostsSucceeded   int                              `json:"hosts_succeeded"`
	HostsFailed      int                              `json:"hosts_failed"`
	HostsMatched     int                              `json:"hosts_matched"`
	Available        bool                             `json:"available"`
	MatchLocation    string                           `json:"match_location,omitempty"`
	MappingRules     []string                         `json:"mapping_rules,omitempty"`
	Reason           string                           `json:"reason,omitempty"`
	Hosts            []openWrtNetworkIPHostDiagnostic `json:"hosts"`
}

func (m openWrtNetworkIPMapper) mapAddress(raw string) (string, bool) {
	if !m.available {
		return "", false
	}
	address, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || !address.Is4() {
		return "", false
	}

	matches := make([]proxyIPv4Mapping, 0, 1)
	for _, mapping := range m.mappings {
		if mapping.source.IsValid() && mapping.source.Addr().Is4() && mapping.source.Contains(address) {
			matches = append(matches, mapping)
		}
	}
	if len(matches) != 1 {
		return "", true
	}

	mapping := matches[0]
	source := mapping.source.Masked()
	target := mapping.target.Masked()
	if !source.IsValid() || !target.IsValid() || !source.Addr().Is4() || !target.Addr().Is4() ||
		source.Bits() != target.Bits() {
		return "", true
	}

	sourceBase := ipv4ToUint32(source.Addr().As4())
	targetBase := ipv4ToUint32(target.Addr().As4())
	offset := ipv4ToUint32(address.As4()) - sourceBase
	hostCount := uint64(1) << uint(32-source.Bits())
	// /31 和 /32 可以表示点到点或单主机代理，不排除其首尾地址。
	if source.Bits() <= 30 && (offset == 0 || uint64(offset) >= hostCount-1) {
		return "", true
	}

	targetValue := uint64(targetBase) + uint64(offset)
	if targetValue > 0xffffffff {
		return "", true
	}
	mapped := uint32ToIPv4(uint32(targetValue))
	if !target.Contains(mapped) {
		return "", true
	}
	return mapped.String(), true
}

func ipv4ToUint32(value [4]byte) uint32 {
	return uint32(value[0])<<24 | uint32(value[1])<<16 | uint32(value[2])<<8 | uint32(value[3])
}

func uint32ToIPv4(value uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value),
	})
}

func jsonObject(raw json.RawMessage) map[string]json.RawMessage {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return value
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}

func proxyIPv4Mappings(raw json.RawMessage) []proxyIPv4Mapping {
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		var single string
		if json.Unmarshal(raw, &single) != nil {
			return nil
		}
		values = []string{single}
	}

	mappings := make([]proxyIPv4Mapping, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		parts := strings.Split(strings.TrimSpace(value), "->")
		if len(parts) < 1 || len(parts) > 2 {
			continue
		}
		source, err := netip.ParsePrefix(strings.TrimSpace(parts[0]))
		if err != nil || !source.Addr().Is4() {
			continue
		}
		source = source.Masked()
		target := source
		if len(parts) == 2 {
			target, err = netip.ParsePrefix(strings.TrimSpace(parts[1]))
			if err != nil || !target.Addr().Is4() {
				continue
			}
			target = target.Masked()
		}
		// EasyTier 要求映射前后的 CIDR 大小一致；不合法规则不参与映射。
		if source.Bits() != target.Bits() {
			continue
		}
		key := source.String() + "->" + target.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		mappings = append(mappings, proxyIPv4Mapping{source: source, target: target})
	}
	return mappings
}

func mapperFromNetworkNode(node map[string]json.RawMessage) (openWrtNetworkIPMapper, bool) {
	mappings := proxyIPv4Mappings(node["proxy_cidrs"])
	if len(mappings) == 0 {
		return openWrtNetworkIPMapper{}, false
	}
	return openWrtNetworkIPMapper{available: true, mappings: mappings}, true
}

func mapperSignature(mapper openWrtNetworkIPMapper) string {
	mappings := make([]string, 0, len(mapper.mappings))
	for _, mapping := range mapper.mappings {
		mappings = append(mappings, mapping.source.String()+"->"+mapping.target.String())
	}
	sort.Strings(mappings)
	return strings.Join(mappings, ",")
}

func (m openWrtNetworkIPMapper) diagnosticStatus(raw, mapped string, known bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "no_ip"
	}
	address, err := netip.ParseAddr(raw)
	if err != nil {
		return "invalid_address"
	}
	if !address.Is4() {
		return "ipv4_only"
	}
	if !m.available {
		return "mapper_unavailable"
	}
	matches := 0
	for _, mapping := range m.mappings {
		if mapping.source.Contains(address) {
			matches++
		}
	}
	if matches == 0 {
		return "no_matching_rule"
	}
	if matches > 1 {
		return "ambiguous_rule"
	}
	if !known || strings.TrimSpace(mapped) == "" {
		return "matched_unmapped"
	}
	return "mapped"
}

func openWrtNetworkIPMapperFromRoute(raw json.RawMessage, networkKey, peerID string) (openWrtNetworkIPMapper, bool) {
	mapper, found, _ := openWrtNetworkIPMapperFromRouteWithDiagnostics(raw, networkKey, peerID)
	return mapper, found
}

func mappingRuleLabels(mapper openWrtNetworkIPMapper) []string {
	rules := make([]string, 0, len(mapper.mappings))
	for _, mapping := range mapper.mappings {
		rules = append(rules, mapping.source.String()+" -> "+mapping.target.String())
	}
	sort.Strings(rules)
	return rules
}

func appendUniqueDiagnosticValue(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func rawProxyCIDRCount(raw json.RawMessage) int {
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) == nil {
		return len(values)
	}
	if len(raw) > 0 && string(raw) != "null" {
		return 1
	}
	return 0
}

func openWrtNetworkIPMapperFromRouteWithDiagnostics(raw json.RawMessage, networkKey, peerID string) (openWrtNetworkIPMapper, bool, openWrtNetworkIPHostDiagnostic) {
	diagnostic := openWrtNetworkIPHostDiagnostic{Status: "no_match"}
	if strings.TrimSpace(networkKey) == "" || strings.TrimSpace(peerID) == "" {
		diagnostic.Message = "目标 network_key 或 peer_id 为空，无法匹配路由"
		return openWrtNetworkIPMapper{}, false, diagnostic
	}
	payload := raw
	if envelope := jsonObject(raw); len(envelope["data"]) > 0 {
		payload = envelope["data"]
	}

	var instances []json.RawMessage
	if json.Unmarshal(payload, &instances) != nil {
		instances = []json.RawMessage{payload}
	}
	diagnostic.InstanceCount = len(instances)

	var selected *openWrtNetworkIPMapper
	for _, rawInstance := range instances {
		instance := jsonObject(rawInstance)
		result := jsonObject(instance["result"])
		if len(result) == 0 {
			result = instance
		}
		nodeInfo := jsonObject(result["node_info"])
		if len(nodeInfo) == 0 {
			nodeInfo = result
		}

		instanceID := jsonString(instance["instance_id"])
		if instanceID == "" {
			instanceID = jsonString(nodeInfo["inst_id"])
		}
		currentNetworkKey := "instance:" + instanceID
		if networkName := jsonString(nodeInfo["network_name"]); networkName != "" && networkName != "—" {
			currentNetworkKey = "network:" + networkName
		}
		diagnostic.ObservedNetworkKeys = appendUniqueDiagnosticValue(diagnostic.ObservedNetworkKeys, currentNetworkKey)
		diagnostic.ObservedPeerIDs = appendUniqueDiagnosticValue(diagnostic.ObservedPeerIDs, jsonString(nodeInfo["peer_id"]))
		if currentNetworkKey != networkKey {
			continue
		}
		diagnostic.MatchingNetworkInstances++

		if jsonString(nodeInfo["peer_id"]) == peerID {
			diagnostic.MatchLocation = "node_info"
			diagnostic.ProxyCIDRCount += rawProxyCIDRCount(nodeInfo["proxy_cidrs"])
			mapper, ok := mapperFromNetworkNode(nodeInfo)
			if ok {
				diagnostic.ValidMappingCount += len(mapper.mappings)
				if !mergeNetworkIPMapper(&selected, mapper) {
					diagnostic.Status = "ambiguous"
					diagnostic.Message = "同一目标节点匹配到多组不一致的代理网段，已拒绝选择"
					return openWrtNetworkIPMapper{}, false, diagnostic
				}
				diagnostic.MappingRules = mappingRuleLabels(mapper)
				diagnostic.Message = "已通过 node_info.peer_id 匹配目标网络节点"
			} else {
				diagnostic.Message = "peer_id 已匹配 node_info，但 proxy_cidrs 中没有有效 IPv4 映射规则"
			}
			continue
		}

		var peerRoutes []json.RawMessage
		if json.Unmarshal(result["peer_routes"], &peerRoutes) != nil {
			continue
		}
		diagnostic.PeerRouteCount += len(peerRoutes)
		for _, rawPeerRoute := range peerRoutes {
			pair := jsonObject(rawPeerRoute)
			route := jsonObject(pair["route"])
			peer := jsonObject(pair["peer"])
			routePeerID := jsonString(route["peer_id"])
			if routePeerID == "" {
				routePeerID = jsonString(peer["peer_id"])
			}
			diagnostic.ObservedPeerIDs = appendUniqueDiagnosticValue(diagnostic.ObservedPeerIDs, routePeerID)
			if routePeerID != peerID {
				continue
			}
			diagnostic.MatchingPeerRoutes++
			diagnostic.MatchLocation = "peer_routes"
			diagnostic.ProxyCIDRCount += rawProxyCIDRCount(route["proxy_cidrs"])
			mapper, ok := mapperFromNetworkNode(route)
			if ok {
				diagnostic.ValidMappingCount += len(mapper.mappings)
				if !mergeNetworkIPMapper(&selected, mapper) {
					diagnostic.Status = "ambiguous"
					diagnostic.Message = "同一目标节点匹配到多组不一致的代理网段，已拒绝选择"
					return openWrtNetworkIPMapper{}, false, diagnostic
				}
				diagnostic.MappingRules = mappingRuleLabels(mapper)
				diagnostic.Message = "已通过 peer_routes.route.peer_id 匹配目标网络节点"
			} else {
				diagnostic.Message = "peer_id 已匹配 peer_routes，但 route.proxy_cidrs 中没有有效 IPv4 映射规则"
			}
		}
	}

	sort.Strings(diagnostic.ObservedNetworkKeys)
	sort.Strings(diagnostic.ObservedPeerIDs)
	if selected == nil {
		if diagnostic.MatchingNetworkInstances == 0 {
			diagnostic.Message = "没有找到匹配的 network_key：" + networkKey
		} else if diagnostic.MatchLocation == "" {
			diagnostic.Message = "network_key 已匹配，但 node_info.peer_id 和 peer_routes.peer_id 均未匹配目标节点"
		} else if diagnostic.Message == "" {
			diagnostic.Message = "找到目标节点，但没有可用的代理网段"
		}
		return openWrtNetworkIPMapper{}, false, diagnostic
	}
	diagnostic.Status = "matched"
	diagnostic.MappingRules = mappingRuleLabels(*selected)
	diagnostic.ValidMappingCount = len(selected.mappings)
	diagnostic.Message = "已找到目标节点代理网段"
	return *selected, true, diagnostic
}
func mergeNetworkIPMapper(selected **openWrtNetworkIPMapper, candidate openWrtNetworkIPMapper) bool {
	if *selected == nil {
		copy := candidate
		*selected = &copy
		return true
	}
	return mapperSignature(**selected) == mapperSignature(candidate)
}

func safeOpenWrtDiagnosticError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "未配对"), strings.Contains(message, "not paired"):
		return "组网入口主机尚未配对"
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return "查询超时"
	case strings.Contains(message, "connection refused"):
		return "连接被拒绝"
	case strings.Contains(message, "no such host"):
		return "DNS 解析失败"
	case strings.Contains(message, "certificate"), strings.Contains(message, "tls"):
		return "TLS 证书校验失败"
	case strings.Contains(message, "401"), strings.Contains(message, "unauthorized"):
		return "Agent 身份认证失败（HTTP 401）"
	case strings.Contains(message, "403"), strings.Contains(message, "forbidden"):
		return "Agent 拒绝请求（HTTP 403）"
	case strings.Contains(message, "404"):
		return "Agent 未提供 route 查询接口（HTTP 404）"
	case strings.Contains(message, "context canceled"):
		return "查询已取消"
	default:
		return "route 数据请求失败（详细错误已隐藏）"
	}
}
func resolveOpenWrtNetworkIPMapper(ctx context.Context, hostSvc *hosts.Service, device storage.Device) openWrtNetworkIPMapper {
	mapper, _ := resolveOpenWrtNetworkIPMapperWithDiagnostics(ctx, hostSvc, device)
	return mapper
}

func resolveOpenWrtNetworkIPMapperWithDiagnostics(ctx context.Context, hostSvc *hosts.Service, device storage.Device) (openWrtNetworkIPMapper, openWrtNetworkIPDiagnostics) {
	diagnostics := openWrtNetworkIPDiagnostics{
		TargetNetworkKey: device.NetworkKey,
		TargetPeerID:     device.PeerID,
		Hosts:            make([]openWrtNetworkIPHostDiagnostic, 0),
	}
	if hostSvc == nil {
		diagnostics.Reason = "主机服务不可用，无法读取 EasyTier 组网入口"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	if strings.TrimSpace(device.NetworkKey) == "" || strings.TrimSpace(device.PeerID) == "" {
		diagnostics.Reason = "网关设备缺少 network_key 或 peer_id，无法定位对应网络节点"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	entries, err := hostSvc.List(ctx)
	if err != nil {
		diagnostics.Reason = "读取主机列表失败：" + safeOpenWrtDiagnosticError(err)
		return openWrtNetworkIPMapper{}, diagnostics
	}
	networkEntries := make([]storage.Host, 0, len(entries))
	for _, entry := range entries {
		if entry.NetworkEntry {
			networkEntries = append(networkEntries, entry)
		}
	}
	diagnostics.NetworkEntries = len(networkEntries)
	diagnostics.HostsQueried = len(networkEntries)
	if len(networkEntries) == 0 {
		diagnostics.Reason = "没有已登记的 EasyTier 组网入口"
		return openWrtNetworkIPMapper{}, diagnostics
	}

	type queryResult struct {
		diagnostic openWrtNetworkIPHostDiagnostic
		mapper     openWrtNetworkIPMapper
		found      bool
	}
	results := make(chan queryResult, len(networkEntries))
	semaphore := make(chan struct{}, 4)
	var wait sync.WaitGroup
	for _, entry := range networkEntries {
		wait.Add(1)
		go func(entry storage.Host) {
			defer wait.Done()
			hostDiagnostic := openWrtNetworkIPHostDiagnostic{
				HostID:   entry.ID,
				HostName: entry.Name,
				Status:   "failed",
			}
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				hostDiagnostic.Status = "cancelled"
				hostDiagnostic.Message = "扫描上下文已取消，未发起路由查询"
				results <- queryResult{diagnostic: hostDiagnostic}
				return
			}
			defer func() { <-semaphore }()

			queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			raw, queryErr := hostSvc.NetworkData(queryCtx, entry.ID, "route")
			if queryErr != nil {
				hostDiagnostic.Message = safeOpenWrtDiagnosticError(queryErr)
				results <- queryResult{diagnostic: hostDiagnostic}
				return
			}
			mapper, found, parsed := openWrtNetworkIPMapperFromRouteWithDiagnostics(raw, device.NetworkKey, device.PeerID)
			parsed.HostID = entry.ID
			parsed.HostName = entry.Name
			if found {
				parsed.Status = "matched"
				results <- queryResult{diagnostic: parsed, mapper: mapper, found: true}
				return
			}
			if parsed.Status == "" {
				parsed.Status = "no_match"
			}
			results <- queryResult{diagnostic: parsed}
		}(entry)
	}
	wait.Wait()
	close(results)

	var selected *openWrtNetworkIPMapper
	conflict := false
	for item := range results {
		diagnostics.Hosts = append(diagnostics.Hosts, item.diagnostic)
		switch item.diagnostic.Status {
		case "failed", "cancelled":
			diagnostics.HostsFailed++
		default:
			diagnostics.HostsSucceeded++
		}
		if !item.found {
			continue
		}
		diagnostics.HostsMatched++
		if !mergeNetworkIPMapper(&selected, item.mapper) {
			conflict = true
		}
	}
	sort.Slice(diagnostics.Hosts, func(i, j int) bool {
		if diagnostics.Hosts[i].HostName == diagnostics.Hosts[j].HostName {
			return diagnostics.Hosts[i].HostID < diagnostics.Hosts[j].HostID
		}
		return diagnostics.Hosts[i].HostName < diagnostics.Hosts[j].HostName
	})
	if conflict {
		diagnostics.Reason = "多个 EasyTier 组网入口返回了不同的代理网段，无法安全确定映射规则"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	if selected == nil {
		diagnostics.Reason = "所有组网入口查询完成，但没有找到匹配目标 network_key / peer_id 的有效代理网段"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	diagnostics.Available = true
	diagnostics.MappingRules = mappingRuleLabels(*selected)
	for _, host := range diagnostics.Hosts {
		if host.Status == "matched" {
			diagnostics.MatchLocation = host.MatchLocation
			break
		}
	}
	diagnostics.Reason = "已找到目标网络节点代理网段"
	return *selected, diagnostics
}
func discoverySnapshotCanImport(raw json.RawMessage) bool {
	value := jsonObject(raw)
	if len(value) == 0 || jsonString(value["status"]) == "error" {
		return false
	}
	var hasSnapshot bool
	return json.Unmarshal(value["has_snapshot"], &hasSnapshot) == nil && hasSnapshot
}
