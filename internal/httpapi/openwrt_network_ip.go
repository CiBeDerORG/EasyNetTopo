package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type proxyIPv4Mapping struct {
	source   netip.Prefix
	target   netip.Prefix
	explicit bool
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
	matchedNodes             map[string]struct{}
}

type openWrtNetworkIPDiagnostics struct {
	TargetNetworkKey string                           `json:"target_network_key"`
	PauseImport      bool                             `json:"import_paused,omitempty"`
	TargetHostname   string                           `json:"target_hostname,omitempty"`
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
	seen := make(map[string]int)
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
		if index, exists := seen[key]; exists {
			mappings[index].explicit = mappings[index].explicit || len(parts) == 2
			continue
		}
		seen[key] = len(mappings)
		mappings = append(mappings, proxyIPv4Mapping{source: source, target: target, explicit: len(parts) == 2})
	}
	return mappings
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

func findOpenWrtNetworkIPMapper(raw json.RawMessage, hostname, entryID string) (openWrtNetworkIPMapper, bool, openWrtNetworkIPHostDiagnostic) {
	diagnostic := openWrtNetworkIPHostDiagnostic{Status: "no_match", matchedNodes: make(map[string]struct{})}
	if hostname == "" {
		diagnostic.Message = "缺少原始主机名"
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

	var local, remote networkIPMappingEvidence
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

		identityScope := currentNetworkKey
		if strings.HasPrefix(identityScope, "instance:") {
			identityScope = "entry:" + entryID + ":" + identityScope
		}
		matches := func(node map[string]json.RawMessage, id string) bool {
			if id == "" || !strings.EqualFold(jsonString(node["hostname"]), hostname) {
				return false
			}
			diagnostic.matchedNodes[identityScope+"\x00"+id] = struct{}{}
			return true
		}
		if matches(nodeInfo, jsonString(nodeInfo["peer_id"])) {
			diagnostic.MatchingNetworkInstances++
			diagnostic.ProxyCIDRCount += rawProxyCIDRCount(nodeInfo["proxy_cidrs"])
			local.add(proxyIPv4Mappings(nodeInfo["proxy_cidrs"]))
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
			if !matches(route, routePeerID) {
				continue
			}
			diagnostic.MatchingPeerRoutes++
			diagnostic.ProxyCIDRCount += rawProxyCIDRCount(route["proxy_cidrs"])
			// 远端通告的单段只表示可达网段，无法确定网关上的原始网段。
			remote.add(slices.DeleteFunc(proxyIPv4Mappings(route["proxy_cidrs"]), func(rule proxyIPv4Mapping) bool {
				return !rule.explicit
			}))
		}
	}

	sort.Strings(diagnostic.ObservedNetworkKeys)
	sort.Strings(diagnostic.ObservedPeerIDs)
	if len(diagnostic.matchedNodes) > 1 {
		diagnostic.Status = "ambiguous"
		diagnostic.Message = "发现多个同名 EasyTier 节点，已暂停自动入库和组网 IP 转换"
		return openWrtNetworkIPMapper{}, false, diagnostic
	}
	evidence, location := preferredNetworkIPMapping(local, remote)
	if evidence.conflict {
		diagnostic.Status = "ambiguous"
		diagnostic.MatchLocation = location
		diagnostic.Message = "同一节点返回的代理规则不一致"
		return openWrtNetworkIPMapper{}, false, diagnostic
	}
	if !evidence.mapper.available {
		diagnostic.Message = "未找到原始主机名对应的节点"
		if len(diagnostic.matchedNodes) == 1 {
			diagnostic.Message = "已找到节点，但没有完整的代理网段映射"
		}
		return openWrtNetworkIPMapper{}, false, diagnostic
	}
	diagnostic.Status = "matched"
	diagnostic.MatchLocation = location
	diagnostic.MappingRules = mappingRuleLabels(evidence.mapper)
	diagnostic.ValidMappingCount = len(evidence.mapper.mappings)
	diagnostic.Message = "已找到目标节点代理网段"
	return evidence.mapper, true, diagnostic
}

type networkIPMappingEvidence struct {
	mapper   openWrtNetworkIPMapper
	conflict bool
}

func (e *networkIPMappingEvidence) add(mappings []proxyIPv4Mapping) {
	if len(mappings) == 0 {
		return
	}
	candidate := openWrtNetworkIPMapper{available: true, mappings: mappings}
	if e.mapper.available {
		e.conflict = e.conflict || mapperSignature(e.mapper) != mapperSignature(candidate)
		return
	}
	e.mapper = candidate
}

func preferredNetworkIPMapping(local, remote networkIPMappingEvidence) (networkIPMappingEvidence, string) {
	if local.mapper.available || local.conflict {
		return local, "node_info"
	}
	return remote, "peer_routes"
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
func resolveOpenWrtNetworkIPMapperWithDiagnostics(ctx context.Context, hostSvc *hosts.Service, device storage.Device) (openWrtNetworkIPMapper, openWrtNetworkIPDiagnostics) {
	diagnostics := openWrtNetworkIPDiagnostics{
		TargetNetworkKey: device.NetworkKey,
		TargetHostname:   device.NodeHostname,
		TargetPeerID:     device.PeerID,
		Hosts:            make([]openWrtNetworkIPHostDiagnostic, 0),
	}
	if hostSvc == nil {
		diagnostics.Reason = "主机服务不可用，无法读取 EasyTier 组网入口"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	if strings.TrimSpace(device.NodeHostname) == "" {
		diagnostics.PauseImport = true
		diagnostics.Reason = "网关设备缺少 EasyTier 原始主机名，请确认归属节点"
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
			mapper, found, parsed := findOpenWrtNetworkIPMapper(raw, device.NodeHostname, entry.ID)
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

	var local, remote networkIPMappingEvidence
	matchedNodes := make(map[string]struct{})
	for item := range results {
		for identity := range item.diagnostic.matchedNodes {
			matchedNodes[identity] = struct{}{}
		}
		diagnostics.Hosts = append(diagnostics.Hosts, item.diagnostic)
		switch item.diagnostic.Status {
		case "failed", "cancelled":
			diagnostics.HostsFailed++
		default:
			diagnostics.HostsSucceeded++
		}
		evidence := &remote
		if item.diagnostic.MatchLocation == "node_info" {
			evidence = &local
		}
		if item.diagnostic.Status == "ambiguous" {
			evidence.conflict = true
		}
		if item.found {
			diagnostics.HostsMatched++
			evidence.add(item.mapper.mappings)
		}
	}
	sort.Slice(diagnostics.Hosts, func(i, j int) bool {
		if diagnostics.Hosts[i].HostName == diagnostics.Hosts[j].HostName {
			return diagnostics.Hosts[i].HostID < diagnostics.Hosts[j].HostID
		}
		return diagnostics.Hosts[i].HostName < diagnostics.Hosts[j].HostName
	})
	if len(matchedNodes) > 1 {
		diagnostics.PauseImport = true
		diagnostics.Reason = "发现多个同名 EasyTier 节点，已暂停自动入库和组网 IP 转换"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	if diagnostics.HostsFailed > 0 {
		diagnostics.PauseImport = true
		diagnostics.Reason = "部分组网入口查询失败，无法确认原始主机名唯一"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	evidence, location := preferredNetworkIPMapping(local, remote)
	if evidence.conflict {
		diagnostics.Reason = "多个 EasyTier 组网入口返回了不同的代理网段，无法安全确定映射规则"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	if !evidence.mapper.available {
		diagnostics.Reason = "未找到目标原始主机名对应的完整代理网段"
		return openWrtNetworkIPMapper{}, diagnostics
	}
	diagnostics.Available = true
	diagnostics.MappingRules = mappingRuleLabels(evidence.mapper)
	diagnostics.MatchLocation = location
	diagnostics.Reason = "已找到目标网络节点代理网段"
	return evidence.mapper, diagnostics
}

func discoverySnapshotCanImport(raw json.RawMessage) bool {
	value := jsonObject(raw)
	if len(value) == 0 || jsonString(value["status"]) == "error" {
		return false
	}
	var hasSnapshot bool
	return json.Unmarshal(value["has_snapshot"], &hasSnapshot) == nil && hasSnapshot
}
