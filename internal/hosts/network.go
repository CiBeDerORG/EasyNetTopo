package hosts

import (
	"bytes"
	"encoding/json"
	"net/url"
)

// 仅输出展示所需的字段。官方 node 信息包含完整配置，禁止把密钥传到浏览器。
func filterNetworkData(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, key := range []string{"agent_id", "adapter_type", "resource_id", "capability", "data", "instance_id", "instance_name", "result", "node_info", "peer_routes", "route", "peer", "peer_id", "ipv4_addr", "proxy_cidrs", "hostname", "stun_info", "inst_id", "listeners", "version", "public_ip", "udp_nat_type", "tcp_nat_type", "address", "addr", "network_length", "next_hop_peer_id", "cost", "path_latency", "conns", "stats", "rx_bytes", "tx_bytes", "latency_us", "loss_rate", "is_closed", "tunnel", "tunnel_type", "network_name", "node_id", "direct_peers", "latency_ms"} {
		allowed[key] = true
	}
	for _, key := range []string{"feature_flag", "is_public_server", "avoid_relay_data", "default_conn_id", "conn_id", "part1", "part2", "part3", "part4"} {
		allowed[key] = true
	}
	var clean func(any) any
	clean = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, v := range x {
				if k == "local_addr" || k == "remote_addr" {
					out[k] = networkEndpoint(v)
					continue
				}
				if allowed[k] {
					out[k] = clean(v)
				}
			}
			return out
		case []any:
			for i := range x {
				x[i] = clean(x[i])
			}
			return x
		default:
			return v
		}
	}
	return json.Marshal(clean(value))
}

// 节点详情只需要连接端点，不向浏览器传递 URL 中的凭据、路径和查询参数。
func networkEndpoint(value any) map[string]string {
	address, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := address["url"].(string)
	if !ok {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return nil
	}
	return map[string]string{"url": (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()}
}
