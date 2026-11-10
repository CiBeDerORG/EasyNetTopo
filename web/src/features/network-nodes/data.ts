type RecordValue = Record<string, unknown>
type NetworkInstance = { id: string; name: string; result: RecordValue }
export type ConnectionRow = {
  id: string; protocol: string; localAddress: string; remoteAddress: string
  latency: string; loss: string; rx: string; tx: string; isDefault: boolean
}
export type NetworkRow = {
  key: string; kind: string; instance: string; instanceID: string; id: string
  hostname: string; ip: string; cidr: string; subnets: string; version: string
  nextHop: string; cost: string; routeLabel: string; connection: string
  protocol: string; latency: string; directLatency: string; rx: string; tx: string
  publicIP: string; listeners: string; natType: string; loss: string
  flags: string[]; connections: ConnectionRow[]
  networkKey: string; candidatePublicIP: string
}

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
// 位置和手工属性须按组网作用域与节点 ID 关联，不能按临时路由行 key 保存。
function networkKey(instance: NetworkInstance): string {
  const name = display(record(instance.result.node_info).network_name)
  return name !== '—' ? 'network:' + name : 'instance:' + instance.id
}
// 对端连接地址可能是中继节点地址，以下只提取公网 IPv4 候选供用户确认。
export function publicIPv4Candidate(value: unknown): string {
  const raw = typeof value === 'string' ? value : ''
  let host = raw
  if (raw.includes('://')) {
    try { host = new URL(raw).hostname } catch { return '' }
  }
  const parts = host.split('.')
  if (parts.length !== 4 || parts.some(part => !/^\\d{1,3}$/.test(part) || Number(part) > 255)) return ''
  const [a, b, c] = parts.map(Number)
  if (a === 0 || a === 10 || a === 127 || a >= 224 || (a === 100 && b >= 64 && b <= 127) ||
      (a === 169 && b === 254) || (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && (b === 168 || (b === 0 && (c === 0 || c === 2 || c === 88)))) ||
      (a === 198 && (b === 18 || b === 19 || (b === 51 && c === 100))) ||
      (a === 203 && b === 0 && c === 113)) return ''
  return parts.map(Number).join('.')
}
function peerPublicIP(conns: ConnectionRow[]): string {
  const ordered = [...conns].sort((a, b) => Number(b.isDefault) - Number(a.isDefault))
  return ordered.map(conn => publicIPv4Candidate(conn.remoteAddress)).find(Boolean) || ''
}
function subnets(value: unknown): string {
  return list(value).map(display).filter(item => item !== '—').join('、') || '—'
}
function connections(value: unknown): RecordValue[] {
  return list(record(value).conns).map(record).filter(conn => conn.is_closed !== true)
}
function numeric(value: unknown): number | null {
  if (typeof value !== 'number' && (typeof value !== 'string' || !value.trim())) return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}
function natType(value: unknown): string {
  if (!value || typeof value !== 'object') return '—'
  const names = ['Unknown', 'OpenInternet', 'NoPAT', 'FullCone', 'Restricted', 'PortRestricted',
    'Symmetric', 'SymUdpFirewall', 'SymmetricEasyInc', 'SymmetricEasyDec']
  const labels = ['未知', '开放互联网', '无端口转换', '全锥形', '受限锥形', '端口受限锥形',
    '对称型', '对称 UDP 防火墙', '对称型递增', '对称型递减']
  // Protobuf JSON 可使用枚举名称或数值；省略枚举字段代表 Unknown。
  const raw = record(value).udp_nat_type ?? 0
  const index = numeric(raw) ?? names.indexOf(String(raw))
  return labels[index] ?? '未知'
}
function nodeFlags(value: unknown): string[] {
  const flags = record(value)
  return [flags.is_public_server === true ? '公共服务器' : '',
    flags.avoid_relay_data === true ? '避免数据中继' : ''].filter(Boolean)
}
function defaultConnectionId(value: unknown): string | null {
  if (typeof value === 'string') return value.toLowerCase()
  const uuid = record(value)
  const parts = ['part1', 'part2', 'part3', 'part4'].map(key => numeric(uuid[key] ?? 0))
  if (parts.some(part => part == null || !Number.isInteger(part) || part < 0 || part > 0xffffffff)
    || parts.every(part => part === 0)) return null
  const hex = parts.map(part => part!.toString(16).padStart(8, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}
function protocol(conn: RecordValue): string {
  const tunnel = record(conn.tunnel)
  const type = display(tunnel.tunnel_type)
  if (type === '—') return type
  const isIPv6 = [tunnel.local_addr, tunnel.remote_addr].some(address => {
    try { return new URL(String(record(address).url)).hostname.startsWith('[') }
    catch { return false }
  })
  return isIPv6 && !type.endsWith('6') ? type + '6' : type
}
function lossRate(conn: RecordValue): string {
  // 已上报连接的 Protobuf JSON 会省略零丢包率；没有连接时由调用方显示缺失。
  const loss = numeric(conn.loss_rate ?? 0)
  return loss == null || loss < 0 || loss > 1 ? '—' : `${Number((loss * 100).toFixed(1))}%`
}
function connectionRows(peer: RecordValue, items: RecordValue[]): ConnectionRow[] {
  const defaultId = defaultConnectionId(peer.default_conn_id)
  return items.map(conn => {
    const latency = numeric(record(conn.stats).latency_us)
    return {
      id: display(conn.conn_id), protocol: protocol(conn),
      localAddress: display(record(record(conn.tunnel).local_addr).url),
      remoteAddress: display(record(record(conn.tunnel).remote_addr).url),
      latency: latency == null || latency < 0 ? '—' : (latency / 1000).toFixed(2) + ' ms',
      loss: lossRate(conn), rx: totalBytes([conn], 'rx_bytes'), tx: totalBytes([conn], 'tx_bytes'),
      isDefault: defaultId != null && String(conn.conn_id).toLowerCase() === defaultId
    }
  })
}
function protocols(items: RecordValue[]): string {
  return [...new Set(items.map(protocol)
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
  const values = items.map(conn => numeric(record(conn.stats)[field]))
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value) && value >= 0)
  return values.length ? values.reduce((sum, value) => sum + value, 0).toLocaleString() + ' B' : '—'
}
function routeRows(instance: NetworkInstance): NetworkRow[] {
  if (!Array.isArray(instance.result.peer_routes)) {
    throw new Error('EasyTier route 返回结构无效：缺少 peer_routes 数组')
  }
  return instance.result.peer_routes.map(value => {
    const pair = record(value), route = record(pair.route), peer = record(pair.peer)
    const items = connections(peer)
    const details = connectionRows(peer, items)
    const id = display(route.peer_id ?? peer.peer_id)
    const hostname = display(route.hostname)
    const cost = display(route.cost)
    const costValue = typeof route.cost === 'number' ? route.cost : null
    const routeLabel = costValue === 1 ? 'P2P 直连路由'
      : costValue != null && costValue > 1 ? '中继 · 开销 ' + cost
      : '路由可见'
    return {
      key: '', kind: '网络路由', networkKey: networkKey(instance),
      candidatePublicIP: peerPublicIP(details),
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
      tx: totalBytes(items, 'tx_bytes'),
      publicIP: '—', listeners: '—',
      natType: natType(route.stun_info), flags: nodeFlags(route.feature_flag),
      loss: (details.find(conn => conn.isDefault) ?? details[0])?.loss ?? '—',
      connections: details
    }
  })
}
function localRow(instance: NetworkInstance): NetworkRow {
  const node = record(instance.result.node_info || instance.result)
  if (!Object.keys(node).length) throw new Error('EasyTier route 返回结构无效：缺少 node_info')
  return {
    key: '', kind: '入口实例', networkKey: networkKey(instance),
    candidatePublicIP: list(record(node.stun_info).public_ip).map(publicIPv4Candidate).find(Boolean) || '',
    routeLabel: '本机实例', connection: '本机',
    protocol: '—', latency: '—', directLatency: '—', nextHop: '—', cost: '—', rx: '—', tx: '—',
    loss: '—', connections: [], natType: natType(node.stun_info), flags: nodeFlags(node.feature_flag),
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
export function networkRows(value: unknown, capability: string): NetworkRow[] {
  return instances(value).flatMap(instance =>
    capability === 'node' ? [localRow(instance)] : routeRows(instance))
}

// 新列表完全基于一次 CLI route -v -o json 的结果，避免 peer -v 的数组格式歧义。
// 一个实例的一条路由对应一条记录；本机与路由表重复时优先保留本机行。
export function mergedNetworkRows(routeValue: unknown): NetworkRow[] {
  const rows: NetworkRow[] = []
  for (const instance of instances(routeValue)) {
    if (!instance.result.node_info || !Array.isArray(instance.result.peer_routes)) {
      throw new Error('EasyTier route 返回结构无效：需要 node_info 和 peer_routes')
    }
    const local = localRow(instance)
    const instanceKey = instance.id !== '—' ? instance.id : instance.name
    local.key = 'local:' + instanceKey + ':' + local.id
    rows.push(local)

    const seen = new Set<string>()
    for (const [index, route] of routeRows(instance).entries()) {
      if ((route.id !== '—' && route.id === local.id) ||
          (route.ip !== '—' && route.ip === local.ip)) continue
      const identity = route.id !== '—' ? route.id : route.ip !== '—' ? route.ip : '#' + index
      if (seen.has(identity)) continue
      seen.add(identity)
      route.key = 'route:' + instanceKey + ':' + identity
      rows.push(route)
    }
  }
  return rows.sort((a, b) => a.kind !== b.kind
    ? a.kind === '入口实例' ? -1 : 1
    : a.instance.localeCompare(b.instance, 'zh-CN') || a.hostname.localeCompare(b.hostname, 'zh-CN'))
}
