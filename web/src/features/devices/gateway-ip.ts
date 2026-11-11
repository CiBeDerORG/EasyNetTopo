// 代理网段来自 EasyTier route.proxy_cidrs，由网络节点层拼接为「、」分隔的字符串。
// 推导的是建议管理 IP，不能作为设备实际在线或地址已分配的证据。
export function gatewayIPFromProxySubnets(subnets: string): string {
  for (const cidr of subnets.split(/[、，,;\s]+/)) {
    const match = /^(\d{1,3}(?:\.\d{1,3}){3})\/(\d{1,2})$/.exec(cidr)
    if (!match) continue
    const octets = match[1].split('.')
    if (octets.some(part => !/^(0|[1-9]\d{0,2})$/.test(part) || Number(part) > 255)) continue
    const prefix = Number(match[2])
    // /31 与 /32 不具备传统网关主机地址；/0 范围过大，不自动推断。
    if (prefix < 1 || prefix > 30) continue

    const address = octets.reduce((value, part) => value * 256 + Number(part), 0)
    const mask = (0xffffffff << (32 - prefix)) >>> 0
    const host = (((address >>> 0) & mask) >>> 0) + 1
    const ip = [24, 16, 8, 0].map(shift => (host >>> shift) & 255)
    // 排除非正常主机 IP；对非 RFC1918 代理网段仍不擅自改变网段的含义。
    if (ip[0] === 0 || ip[0] === 127 || ip[0] >= 224 ||
        (ip[0] === 169 && ip[1] === 254)) continue
    return ip.join('.')
  }
  return ''
}

type GatewayDeviceDraft = {
  is_gateway: boolean
  gateway_group_id?: string
  ip_address?: string
  network_key?: string
  peer_id?: string
}

type ProxySubnetNode = {
  networkKey: string
  peerId: string
  row?: { subnets: string }
}

// 只给当前网络节点的直属网关设备补缺失的管理 IP；不跨节点匹配，不覆盖已填写值。
export function suggestedDirectGatewayIP(
  device: GatewayDeviceDraft,
  nodes: ProxySubnetNode[]
): string {
  if (!device.is_gateway || device.gateway_group_id || device.ip_address?.trim() ||
      !device.network_key || !device.peer_id) return ''
  const node = nodes.find(item => item.networkKey === device.network_key && item.peerId === device.peer_id)
  return node?.row ? gatewayIPFromProxySubnets(node.row.subnets) : ''
}
