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
      const latency = stats.map(stat => Number(stat.latency_us)).filter(value => Number.isFinite(value) && value > 0)
      const pathLatency = Number(route.path_latency)
      const bestLatency = latency.length ? Math.min(...latency) / 1000 :
        route.path_latency != null && Number.isFinite(pathLatency) && pathLatency > 0 ? pathLatency : null
      return { instance: instance.name, id: display(route.peer_id ?? peer.peer_id), hostname: display(route.hostname), ip: ipv4(route.ipv4_addr), subnets: list(route.proxy_cidrs).map(display).join('、') || '—', version: display(route.version), nextHop: display(route.next_hop_peer_id), cost: display(route.cost), connection: connections.length ? '直连' : '未报告直连', protocol: [...new Set(connections.map(conn => display(record(conn.tunnel).tunnel_type)))].join('、') || '—', latency: bestLatency == null ? '—' : bestLatency.toFixed(2) + ' ms', rx: stats.length ? stats.reduce((sum, stat) => sum + Number(stat.rx_bytes || 0), 0).toLocaleString() + ' B' : '—', tx: stats.length ? stats.reduce((sum, stat) => sum + Number(stat.tx_bytes || 0), 0).toLocaleString() + ' B' : '—' }
    })
  })
}


// 将入口本机和路由表中的节点合并为一份清单；保留路由可见但未直连的节点。
// 与魔改 EasyTier 的节点详情一致：以路由身份为基础，叠加直连协议和延迟。
export function mergedNetworkRows(nodeValue: unknown, peerValue: unknown) {
  const local = nodeValue == null ? [] : networkRows(nodeValue, 'node')
  const remote = peerValue == null ? [] : networkRows(peerValue, 'peer')
  const singleLocal = local.length === 1 ? local[0] : undefined
  const rows: Array<Record<string, string>> = []
  const keyFor = (instance: string, id: string, ip: string, index: number) =>
    instance + ':' + (id !== '—' ? id : ip !== '—' ? ip : '#' + index)

  local.forEach((item, index) => {
    rows.push({
      ...item,
      key: 'local:' + keyFor(item.instance, item.id, item.ip, index),
      kind: '入口实例',
      routeLabel: '本机实例',
      connection: '本机',
      protocol: '—',
      latency: '—',
      rx: '—',
      tx: '—',
      nextHop: '—',
      cost: '—'
    })
  })

  remote.forEach((item, index) => {
    // CLI 单实例的 peer 响应可能不携带实例名称，沿用 node 响应中的名称。
    const instance = item.instance === '—' && singleLocal ? singleLocal.instance : item.instance
    const own = local.find(node => node.instance === instance) || (singleLocal && item.instance === '—' ? singleLocal : undefined)
    // 某些 EasyTier 版本会把本机也放入路由表，不重复显示。
    if (own && ((item.id !== '—' && item.id === own.id) ||
      (item.ip !== '—' && item.ip === own.ip))) return
    const cost = Number(item.cost)
    const routeLabel = item.cost !== '—' && Number.isFinite(cost)
      ? (cost === 1 ? 'P2P 直连' : cost > 1 ? '中继 · 开销 ' + item.cost : '路由可见')
      : (item.connection === '直连' ? '直连' : '路由可见')
    rows.push({
      ...item,
      instance,
      key: 'route:' + keyFor(instance, item.id, item.ip, index),
      kind: '网络路由',
      routeLabel,
      listeners: '—',
      publicIP: '—'
    })
  })
  return rows.sort((a, b) => (a.kind === b.kind ? a.hostname.localeCompare(b.hostname, 'zh-CN') : a.kind === '入口实例' ? -1 : 1))
}
