package httpapi

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type serviceForwardAgent interface {
	NetworkData(context.Context, string, string) (json.RawMessage, error)
	EasyTierPortForwards(context.Context, string, string, string, string, *hosts.PortForwardChange) (json.RawMessage, error)
}

type serviceForwardInput struct {
	Enabled         bool   `json:"enabled"`
	HostID          string `json:"host_id"`
	InstanceID      string `json:"instance_id"`
	Listen          string `json:"listen"`
	AllowPublicBind bool   `json:"allow_public_bind"`
}

var (
	serviceForwardUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)
	errForwardInput    = errors.New("请选择公网入口和网络实例，填写有效监听地址及端口，并确认公网监听")
	errForwardTarget   = errors.New("所选实例未找到设备对应的有效组网 IP")
	errForwardConflict = errors.New("监听地址、端口和协议已被其他转发规则占用")
	errForwardNotReady = errors.New("无法读取 EasyTier 实际规则，请检查公网入口或 Agent")
)

func unwrapForwardData(raw json.RawMessage) json.RawMessage {
	if data := jsonObject(raw)["data"]; len(data) != 0 {
		return data
	}
	return raw
}

func forwardNodeIP(raw json.RawMessage) netip.Addr {
	if ip, err := netip.ParseAddr(jsonString(raw)); err == nil {
		return ip.Unmap()
	}
	if prefix, err := netip.ParsePrefix(jsonString(raw)); err == nil {
		return prefix.Addr().Unmap()
	}
	var address struct {
		Address *struct{ Addr uint32 } `json:"address"`
	}
	if json.Unmarshal(raw, &address) == nil && address.Address != nil {
		return uint32ToIPv4(address.Address.Addr)
	}
	return netip.Addr{}
}

func serviceForwardTarget(raw json.RawMessage, instanceID string, device storage.Device) (netip.Addr, error) {
	payload := unwrapForwardData(raw)
	var instances []json.RawMessage
	if json.Unmarshal(payload, &instances) != nil {
		instances = []json.RawMessage{payload}
	}
	if instances == nil {
		return netip.Addr{}, errForwardNotReady
	}
	configured, _ := netip.ParseAddr(device.NetworkIP)
	managementIP, _ := netip.ParseAddr(device.IPAddress)
	if strings.TrimSpace(device.NodeHostname) == "" {
		return netip.Addr{}, errForwardTarget
	}
	var target netip.Addr
	matchedInstance := false
	for _, rawInstance := range instances {
		instance := jsonObject(rawInstance)
		result := jsonObject(instance["result"])
		if result == nil {
			result = instance
		}
		local := jsonObject(result["node_info"])
		if local == nil {
			return netip.Addr{}, errForwardNotReady
		}
		id := jsonString(instance["instance_id"])
		if id == "" {
			id = jsonString(local["inst_id"])
		}
		if !strings.EqualFold(id, instanceID) {
			continue
		}
		if matchedInstance {
			return netip.Addr{}, errForwardNotReady
		}
		matchedInstance = true
		var peers []json.RawMessage
		if json.Unmarshal(result["peer_routes"], &peers) != nil {
			return netip.Addr{}, errForwardNotReady
		}
		nodes := map[string]map[string]json.RawMessage{}
		if strings.EqualFold(jsonString(local["hostname"]), device.NodeHostname) {
			nodes[jsonString(local["peer_id"])] = local
		}
		for _, item := range peers {
			pair := jsonObject(item)
			route := jsonObject(pair["route"])
			if !strings.EqualFold(jsonString(route["hostname"]), device.NodeHostname) {
				continue
			}
			peerID := jsonString(route["peer_id"])
			if peerID == "" {
				peerID = jsonString(jsonObject(pair["peer"])["peer_id"])
			}
			if _, seen := nodes[peerID]; !seen {
				nodes[peerID] = route
			}
		}
		if len(nodes) != 1 {
			return netip.Addr{}, errForwardTarget
		}
		for peerID, node := range nodes {
			if peerID == "" {
				return netip.Addr{}, errForwardTarget
			}
			nodeIP := forwardNodeIP(node["ipv4_addr"])
			if configured.IsValid() {
				configured = configured.Unmap()
				if configured == nodeIP {
					target = configured
				}
				for _, mapping := range proxyIPv4Mappings(node["proxy_cidrs"]) {
					if mapping.target.Contains(configured) {
						target = configured
						break
					}
				}
			} else if device.IsNodeDevice || (managementIP.IsValid() && managementIP.Unmap() == nodeIP) {
				target = nodeIP
			}
		}
	}
	if !target.IsGlobalUnicast() || target.IsLoopback() || target.IsLinkLocalUnicast() || target.Zone() != "" {
		return netip.Addr{}, errForwardTarget
	}
	return target, nil
}

type forwardSocket struct {
	Port uint16 `json:"port"`
	IP   struct {
		IPv4 *struct {
			Addr uint32 `json:"addr"`
		} `json:"Ipv4"`
		IPv6 *struct{ Part1, Part2, Part3, Part4 uint32 } `json:"Ipv6"`
	} `json:"ip"`
}

func (s forwardSocket) address() netip.AddrPort {
	if s.IP.IPv4 != nil {
		return netip.AddrPortFrom(uint32ToIPv4(s.IP.IPv4.Addr), s.Port)
	}
	if ip := s.IP.IPv6; ip != nil {
		var bytes [16]byte
		for i, part := range [...]uint32{ip.Part1, ip.Part2, ip.Part3, ip.Part4} {
			binary.BigEndian.PutUint32(bytes[i*4:], part)
		}
		return netip.AddrPortFrom(netip.AddrFrom16(bytes).Unmap(), s.Port)
	}
	return netip.AddrPort{}
}

type liveDeviceForward struct {
	instance, protocol  string
	listen, destination netip.AddrPort
}

func parseLiveDeviceForwards(raw json.RawMessage, instanceID string) ([]liveDeviceForward, error) {
	payload := unwrapForwardData(raw)
	var instances []json.RawMessage
	if json.Unmarshal(payload, &instances) != nil {
		instances = []json.RawMessage{payload}
	}
	if instances == nil || (instanceID != "" && len(instances) != 1) {
		return nil, errForwardNotReady
	}
	var rules []liveDeviceForward
	for _, rawInstance := range instances {
		instance := jsonObject(rawInstance)
		result := rawInstance
		id := instanceID
		if len(instance["result"]) != 0 {
			id, result = jsonString(instance["instance_id"]), instance["result"]
			if id == "" || (instanceID != "" && !strings.EqualFold(id, instanceID)) {
				return nil, errForwardNotReady
			}
		}
		var value struct {
			Rules *[]struct {
				Listen      forwardSocket `json:"bind_addr"`
				Destination forwardSocket `json:"dst_addr"`
				Protocol    *int          `json:"socket_type"`
			} `json:"cfgs"`
		}
		if json.Unmarshal(result, &value) != nil || value.Rules == nil {
			return nil, errForwardNotReady
		}
		for _, item := range *value.Rules {
			listen, dest := item.Listen.address(), item.Destination.address()
			if !listen.IsValid() || !dest.IsValid() || listen.Port() == 0 || dest.Port() == 0 ||
				item.Protocol == nil || *item.Protocol < 0 || *item.Protocol > 1 {
				return nil, errForwardNotReady
			}
			rules = append(rules, liveDeviceForward{id, [...]string{"tcp", "udp"}[*item.Protocol], listen, dest})
		}
	}
	return rules, nil
}

func (r liveDeviceForward) matches(cfg storage.DeviceServiceForward, protocol string) bool {
	return strings.EqualFold(r.instance, cfg.InstanceID) && r.protocol == protocol &&
		r.listen.String() == cfg.Listen && r.destination.String() == cfg.Destination
}

func readDeviceForwardRules(ctx context.Context, agent serviceForwardAgent, hostID, instanceID string) ([]liveDeviceForward, error) {
	raw, err := agent.EasyTierPortForwards(ctx, hostID, http.MethodGet, instanceID, "", nil)
	if err != nil {
		return nil, errForwardNotReady
	}
	return parseLiveDeviceForwards(raw, instanceID)
}

func sendDeviceForward(ctx context.Context, agent serviceForwardAgent, method, protocol string, cfg storage.DeviceServiceForward) error {
	listen, err := netip.ParseAddrPort(cfg.Listen)
	if err != nil {
		return errForwardInput
	}
	result, err := agent.EasyTierPortForwards(ctx, cfg.HostID, method, "", "", &hosts.PortForwardChange{
		ResourceID: "compose:easytier", InstanceID: cfg.InstanceID, Protocol: protocol,
		Listen: cfg.Listen, Destination: cfg.Destination,
		AllowPublicBind: listen.Addr().IsUnspecified(),
	})
	var response struct{ Applied bool }
	if err != nil || json.Unmarshal(result, &response) != nil || !response.Applied {
		return errors.New("Agent 未确认端口转发命令执行成功，请检查监听端口和实例状态")
	}
	return nil
}

func currentServiceForwardTarget(ctx context.Context, agent serviceForwardAgent, cfg storage.DeviceServiceForward, device storage.Device) (netip.Addr, error) {
	raw, err := agent.NetworkData(ctx, cfg.HostID, "route")
	if err != nil {
		return netip.Addr{}, errForwardNotReady
	}
	return serviceForwardTarget(raw, cfg.InstanceID, device)
}

func checkDeviceServiceForward(ctx context.Context, agent serviceForwardAgent, device storage.Device, service storage.DeviceService) (*storage.DeviceServiceForward, error) {
	cfg := service.Forward
	if cfg == nil || !cfg.Enabled {
		return cfg, nil
	}
	rules, err := readDeviceForwardRules(ctx, agent, cfg.HostID, cfg.InstanceID)
	if err != nil {
		return nil, err
	}
	updated := *cfg
	for _, protocol := range cfg.Protocols {
		if !slices.ContainsFunc(rules, func(rule liveDeviceForward) bool { return rule.matches(*cfg, protocol) }) {
			updated.Enabled = false
			return &updated, nil
		}
	}
	target, err := currentServiceForwardTarget(ctx, agent, *cfg, device)
	if err != nil && !errors.Is(err, errForwardTarget) {
		return nil, err
	}
	updated.Enabled = err == nil && netip.AddrPortFrom(target, uint16(service.Port)).String() == cfg.Destination
	return &updated, nil
}

func updateDeviceServiceForward(ctx context.Context, agent serviceForwardAgent, device storage.Device, service storage.DeviceService, input serviceForwardInput) (*storage.DeviceServiceForward, error) {
	if !input.Enabled {
		if service.Forward == nil {
			return nil, nil
		}
		cfg := *service.Forward
		rules, err := readDeviceForwardRules(ctx, agent, cfg.HostID, cfg.InstanceID)
		if err != nil {
			return nil, err
		}
		for _, protocol := range cfg.Protocols {
			if slices.ContainsFunc(rules, func(rule liveDeviceForward) bool { return rule.matches(cfg, protocol) }) {
				if err := sendDeviceForward(ctx, agent, http.MethodDelete, protocol, cfg); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	}

	listen, err := netip.ParseAddrPort(strings.TrimSpace(input.Listen))
	if err == nil {
		listen = netip.AddrPortFrom(listen.Addr().Unmap(), listen.Port())
	}
	if err != nil || !serviceForwardUUID.MatchString(input.HostID) || !serviceForwardUUID.MatchString(input.InstanceID) ||
		listen.Port() == 0 || listen.Addr().Zone() != "" || listen.Addr().IsMulticast() ||
		(listen.Addr().IsUnspecified() && !input.AllowPublicBind) {
		return nil, errForwardInput
	}
	cfg := storage.DeviceServiceForward{
		Enabled: true, HostID: input.HostID, InstanceID: input.InstanceID,
		Listen: listen.String(), Protocols: service.Protocols,
	}
	ip, err := currentServiceForwardTarget(ctx, agent, cfg, device)
	if err != nil {
		return nil, err
	}
	cfg.Destination = netip.AddrPortFrom(ip, uint16(service.Port)).String()
	previous := service.Forward
	sameEndpoint := previous != nil && previous.HostID == cfg.HostID && strings.EqualFold(previous.InstanceID, cfg.InstanceID) &&
		previous.Listen == cfg.Listen && previous.Destination == cfg.Destination
	if previous != nil && !sameEndpoint {
		return nil, errors.New("请先移除原转发，再更换公网入口、监听地址或目标地址")
	}
	var owned []liveDeviceForward
	if sameEndpoint {
		owned, err = readDeviceForwardRules(ctx, agent, cfg.HostID, cfg.InstanceID)
		if err != nil {
			return nil, err
		}
	}
	all, err := readDeviceForwardRules(ctx, agent, cfg.HostID, "")
	if err != nil {
		return nil, err
	}
	var pending []string
	for _, protocol := range cfg.Protocols {
		if sameEndpoint && slices.Contains(previous.Protocols, protocol) &&
			slices.ContainsFunc(owned, func(rule liveDeviceForward) bool { return rule.matches(cfg, protocol) }) {
			continue
		}
		for _, rule := range all {
			if rule.protocol == protocol && rule.listen.Port() == listen.Port() &&
				(rule.listen.Addr() == listen.Addr() || rule.listen.Addr().IsUnspecified() || listen.Addr().IsUnspecified()) {
				return nil, errForwardConflict
			}
		}
		pending = append(pending, protocol)
	}
	var added []string
	for _, protocol := range pending {
		if err := sendDeviceForward(ctx, agent, http.MethodPost, protocol, cfg); err != nil {
			if len(added) == 0 {
				return nil, err
			}
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			for _, done := range added {
				if removeErr := sendDeviceForward(cleanup, agent, http.MethodDelete, done, cfg); removeErr != nil {
					return nil, fmt.Errorf("转发未完整开启，%s 规则也未确认撤销，请检查公网入口", strings.ToUpper(done))
				}
			}
			return nil, err
		}
		added = append(added, protocol)
	}
	return &cfg, nil
}
