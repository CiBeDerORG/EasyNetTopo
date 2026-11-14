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

func openWrtNetworkIPMapperFromRoute(raw json.RawMessage, networkKey, peerID string) (openWrtNetworkIPMapper, bool) {
	if strings.TrimSpace(networkKey) == "" || strings.TrimSpace(peerID) == "" {
		return openWrtNetworkIPMapper{}, false
	}
	payload := raw
	if envelope := jsonObject(raw); len(envelope["data"]) > 0 {
		payload = envelope["data"]
	}

	var instances []json.RawMessage
	if json.Unmarshal(payload, &instances) != nil {
		instances = []json.RawMessage{payload}
	}

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
		if currentNetworkKey != networkKey {
			continue
		}

		if jsonString(nodeInfo["peer_id"]) == peerID {
			if mapper, ok := mapperFromNetworkNode(nodeInfo); ok {
				if !mergeNetworkIPMapper(&selected, mapper) {
					return openWrtNetworkIPMapper{}, false
				}
			}
			continue
		}

		var peerRoutes []json.RawMessage
		if json.Unmarshal(result["peer_routes"], &peerRoutes) != nil {
			continue
		}
		for _, rawPeerRoute := range peerRoutes {
			pair := jsonObject(rawPeerRoute)
			route := jsonObject(pair["route"])
			peer := jsonObject(pair["peer"])
			routePeerID := jsonString(route["peer_id"])
			if routePeerID == "" {
				routePeerID = jsonString(peer["peer_id"])
			}
			if routePeerID != peerID {
				continue
			}
			if mapper, ok := mapperFromNetworkNode(route); ok {
				if !mergeNetworkIPMapper(&selected, mapper) {
					return openWrtNetworkIPMapper{}, false
				}
			}
		}
	}

	if selected == nil {
		return openWrtNetworkIPMapper{}, false
	}
	return *selected, true
}

func mergeNetworkIPMapper(selected **openWrtNetworkIPMapper, candidate openWrtNetworkIPMapper) bool {
	if *selected == nil {
		copy := candidate
		*selected = &copy
		return true
	}
	return mapperSignature(**selected) == mapperSignature(candidate)
}

func resolveOpenWrtNetworkIPMapper(ctx context.Context, hostSvc *hosts.Service, device storage.Device) openWrtNetworkIPMapper {
	if hostSvc == nil || device.NetworkKey == "" || device.PeerID == "" {
		return openWrtNetworkIPMapper{}
	}
	entries, err := hostSvc.List(ctx)
	if err != nil {
		return openWrtNetworkIPMapper{}
	}

	type result struct {
		mapper openWrtNetworkIPMapper
		found  bool
	}
	results := make(chan result, len(entries))
	semaphore := make(chan struct{}, 4)
	var wait sync.WaitGroup
	for _, entry := range entries {
		if !entry.NetworkEntry {
			continue
		}
		hostID := entry.ID
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			raw, queryErr := hostSvc.NetworkData(queryCtx, hostID, "route")
			if queryErr != nil {
				return
			}
			mapper, found := openWrtNetworkIPMapperFromRoute(raw, device.NetworkKey, device.PeerID)
			if found {
				results <- result{mapper: mapper, found: true}
			}
		}()
	}
	wait.Wait()
	close(results)

	var selected *openWrtNetworkIPMapper
	for item := range results {
		if !item.found {
			continue
		}
		if !mergeNetworkIPMapper(&selected, item.mapper) {
			return openWrtNetworkIPMapper{}
		}
	}
	if selected == nil {
		return openWrtNetworkIPMapper{}
	}
	return *selected
}

func discoverySnapshotCanImport(raw json.RawMessage) bool {
	value := jsonObject(raw)
	if len(value) == 0 || jsonString(value["status"]) == "error" {
		return false
	}
	var hasSnapshot bool
	return json.Unmarshal(value["has_snapshot"], &hasSnapshot) == nil && hasSnapshot
}
