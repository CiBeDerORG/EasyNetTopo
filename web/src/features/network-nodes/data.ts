type RecordValue = Record<string, unknown>
type NetworkInstance = { id: string; name: string; result: RecordValue }

export function record(value: unknown): RecordValue {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as RecordValue : {}
}
export function list(value: unknown): unknown[] { return Array.isArray(value) ? value : [] }
export function display(value: unknown): string {
  return typeof value === 'string' && value.trim() || typeof value === 'number' && Number.isFinite(value)
    ? String(value) : '—'
}
export function ipv4(value: unknown): string {
  if (typeof value === 'string') return value || '—'
  const address = record(record(value).address).addr
  if (typeof address !== 'number' || !Number.isInteger(address)) return '—'
  return [24, 16, 8, 0].map(shift => (address >>> shift) & 255).join('.')
}
export function ipv4Cidr(value: unknown): string {
  const ip = ipv4(value)
  if (ip === '—') return ip
  const length = record(value).network_length
  return typeof length === 'number' && Number.isInteger(length) && length >= 0 && length <= 32
    ? ip + '/' + length : ip
}
function instanceLabel(name: unknown, id: string): string {
  const label = display(name)
  return label !== '—' ? label : id !== '—' ? '实例 ' + id.slice(0, 8) : '默认实例'
}

// CLI 单实例输出对象，多实例输出 { instance_id, instance_name, result } 数组。
// 单实例中不能把 node_info.hostname（主机名）冒充网络实例名。
export function instances(value: unknown): NetworkInstance[] {
  if (Array.isArray(value)) {
    return value.map(item => {
      const row = record(item)
      const result = record(row.result)
      if (!Object.keys(result).length) throw new Error('EasyTier 多实例返回数据缺少有效 result')
      const info = record(result.node_info || result)
      const id = display(row.instance_id ?? info.inst_id)
      return { id, name: instanceLabel(row.instance_name, id), result }
    })
  }
  const result = record(value)
  if (!Object.keys(result).length) throw new Error('EasyTier 未返回有效网络信息')
  const info = record(result.node_info || result)
  const id = display(info.inst_id)
  return [{ id, name: instanceLabel(info.network_name, id), result }]
}
function subnets(value: unknown): string {
  return list(value).map(display).filter(item => item !== '—').join('、') || '—'
}
function connections(value: unknown): RecordValue[] {
  return list(record(value).conns).map(record).filter(conn => conn.is_closed !== true)
}
function protocols(items: RecordValue[]): string {
  return [...new Set(items.map(item => display(record(item.tunnel).tunnel_type))
    .filter(item => item !== '—'))].join('、') || '—'
}
function durationMs(items: RecordValue[]): number | null {
  const values = items.map(conn => record(conn.stats).latency_us)
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value) && value > 0)
  return values.length ? Math.min(...values) / 1000 : null
}
function pathLatency(route: RecordValue, items: RecordValue[]): string {
  const path = route.path_latency
  // CLI 路由 path_latency 单位为 ms；缺失时才使用直连连接的 us 统计。
  const ms = typeof path === 'number' && Number.isFinite(path) && path > 0
    ? path : durationMs(items)
  return ms == null ? '—' : ms.toFixed(2) + ' ms'
}
function totalBytes(items: RecordValue[], field: 'rx_bytes' | 'tx_bytes'): string {
  const values = items.map(conn => record(conn.stats)[field])
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value) && value >= 0)
  return values.length ? values.reduce((sum, value) => sum + value, 0).toLocaleString() + ' B' : '—'
}
function routeRows(instance: NetworkInstance): Record<string, string>[] {
  if (!Array.isArray(instance.result.peer_routes)) {
    throw new Error('EasyTier route 返回结构无效：缺少 peer_routes 数组')
  }
  return instance.result.peer_routes.map(value => {
    const pair = record(value), route = record(pair.route), peer = record(pair.peer)
    const items = connections(peer)
    const id = display(route.peer_id ?? peer.peer_id)
    const hostname = display(route.hostname)
    const cost = display(route.cost)
    const costValue = typeof route.cost === 'number' ? route.cost : null
    const routeLabel = costValue === 1 ? 'P2P 直连路由'
      : costValue != null && costValue > 1 ? '中继 · 开销 ' + cost
      : '路由可见'
    return {
      instance: instance.name,
      instanceID: instance.id,
      id,
      hostname: hostname !== '—' ? hostname : id !== '—' ? '节点 #' + id : '未知节点',
      ip: ipv4(route.ipv4_addr),
      cidr: ipv4Cidr(route.ipv4_addr),
      subnets: subnets(route.proxy_cidrs),
      version: display(route.version),
      nextHop: display(route.next_hop_peer_id),
      cost,
      routeLabel,
      connection: items.length ? '发现直连连接' : '未报告直连连接',
      protocol: protocols(items),
      latency: pathLatency(route, items),
      directLatency: durationMs(items) == null ? '—' : durationMs(items)!.toFixed(2) + ' ms',
      rx: totalBytes(items, 'rx_bytes'),
      tx: totalBytes(items, 'tx_bytes')
    }
  })
}
function localRow(instance: NetworkInstance): Record<string, string> {
  const node = record(instance.result.node_info || instance.result)
  if (!Object.keys(node).length) throw new Error('EasyTier route 返回结构无效：缺少 node_info')
  return {
    instance: instance.name,
    instanceID: instance.id,
    id: display(node.peer_id),
    hostname: display(node.hostname),
    ip: ipv4(node.ipv4_addr),
    cidr: ipv4Cidr(node.ipv4_addr),
    subnets: subnets(node.proxy_cidrs),
    version: display(node.version),
    publicIP: subnets(record(node.stun_info).public_ip),
    listeners: subnets(node.listeners)
  }
}

// 保留原有 node / route 查询的独立列表，供迁移前后核对。
export function networkRows(value: unknown, capability: string): Record<string, string>[] {
  return instances(value).flatMap(instance =>
    capability === 'node' ? [localRow(instance)] : routeRows(instance))
}

// 新列表完全基于一次 CLI route -v -o json 的结果，避免 peer -v 的数组格式歧义。
// 一个实例的一条路由对应一条记录；本机与路由表重复时优先保留本机行。
export function mergedNetworkRows(routeValue: unknown): Record<string, string>[] {
  const rows: Record<string, string>[] = []
  for (const instance of instances(routeValue)) {
    if (!instance.result.node_info || !Array.isArray(instance.result.peer_routes)) {
      throw new Error('EasyTier route 返回结构无效：需要 node_info 和 peer_routes')
    }
    const local = localRow(instance)
    const instanceKey = instance.id !== '—' ? instance.id : instance.name
    local.key = 'local:' + instanceKey + ':' + local.id
    local.kind = '入口实例'
    local.routeLabel = '本机实例'
    local.connection = '本机'
    local.protocol = '—'
    local.latency = '—'
    local.directLatency = '—'
    local.nextHop = '—'
    local.cost = '—'
    local.rx = '—'
    local.tx = '—'
    rows.push(local)

    const seen = new Set<string>()
    for (const [index, route] of routeRows(instance).entries()) {
      if ((route.id !== '—' && route.id === local.id) ||
          (route.ip !== '—' && route.ip === local.ip)) continue
      const identity = route.id !== '—' ? route.id : route.ip !== '—' ? route.ip : '#' + index
      if (seen.has(identity)) continue
      seen.add(identity)
      route.key = 'route:' + instanceKey + ':' + identity
      route.kind = '网络路由'
      route.publicIP = '—'
      route.listeners = '—'
      rows.push(route)
    }
  }
  return rows.sort((a, b) => a.kind !== b.kind
    ? a.kind === '入口实例' ? -1 : 1
    : a.instance.localeCompare(b.instance, 'zh-CN') || a.hostname.localeCompare(b.hostname, 'zh-CN'))
}
