type RecordValue = Record<string, unknown>
export function record(value: unknown): RecordValue {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as RecordValue : {}
}
export function list(value: unknown): unknown[] { return Array.isArray(value) ? value : [] }
export function display(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' ? String(value) : '—'
}
export function ipv4(value: unknown): string {
  if (typeof value === 'string') return value || '—'
  const address = record(record(value).address).addr
  if (typeof address !== 'number') return '—'
  return [24, 16, 8, 0].map(shift => (address >>> shift) & 255).join('.')
}
export function instances(value: unknown): { id: string; name: string; result: RecordValue }[] {
  if (Array.isArray(value)) return value.map(item => {
    const row = record(item)
    if (!row.result || typeof row.result !== 'object') throw new Error('EasyTier 多实例数据格式不受支持，请检查版本')
    return { id: display(row.instance_id), name: display(row.instance_name), result: record(row.result) }
  })
  const result = record(value)
  if (!Object.keys(result).length) throw new Error('EasyTier 未返回有效网络信息')
  const node = record(result.node_info || result)
  return [{ id: display(node.inst_id), name: display(node.hostname), result }]
}
export function networkRows(value: unknown, capability: string) {
  return instances(value).flatMap<Record<string, string>>(instance => {
    if (capability === 'node') {
      const node = record(instance.result.node_info || instance.result)
      return [{ instance: instance.name, id: display(node.peer_id), hostname: display(node.hostname), ip: ipv4(node.ipv4_addr), subnets: list(node.proxy_cidrs).map(display).join('、') || '—', version: display(node.version), listeners: list(node.listeners).map(display).join('、') || '—', publicIP: list(record(node.stun_info).public_ip).map(display).join('、') || '—' }]
    }
    if (!Array.isArray(instance.result.peer_routes)) throw new Error('EasyTier 未返回支持的对等节点/路由结构')
    return instance.result.peer_routes.map(value => {
      const pair = record(value), route = record(pair.route), peer = record(pair.peer)
      const connections = list(peer.conns).map(record).filter(conn => !conn.is_closed)
      const stats = connections.map(conn => record(conn.stats))
      const latency = stats.map(stat => Number(stat.latency_us)).filter(Number.isFinite)
      return { instance: instance.name, id: display(route.peer_id ?? peer.peer_id), hostname: display(route.hostname), ip: ipv4(route.ipv4_addr), subnets: list(route.proxy_cidrs).map(display).join('、') || '—', version: display(route.version), nextHop: display(route.next_hop_peer_id), cost: display(route.cost), connection: connections.length ? '直连' : '未报告直连', protocol: [...new Set(connections.map(conn => display(record(conn.tunnel).tunnel_type)))].join('、') || '—', latency: latency.length ? (Math.min(...latency) / 1000).toFixed(2) + ' ms' : '—', rx: stats.length ? stats.reduce((sum, stat) => sum + Number(stat.rx_bytes || 0), 0).toLocaleString() + ' B' : '—', tx: stats.length ? stats.reduce((sum, stat) => sum + Number(stat.tx_bytes || 0), 0).toLocaleString() + ' B' : '—' }
    })
  })
}
