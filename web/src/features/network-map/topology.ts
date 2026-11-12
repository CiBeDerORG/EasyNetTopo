import type { NodeAttributes } from '../network-nodes/attributes'
import { mergedNetworkRows, record, list, type NetworkRow } from '../network-nodes/data.ts'
import { networkNodeAttributeKey } from '../network-nodes/identity.ts'

export type MapLinkKind = 'p2p' | 'relay' | 'unknown'
export type MapNode = {
  key: string
  id: string
  instanceID: string
  instance: string
  networkKey: string
  hostname: string
  label: string
  ip: string
  latitude: number | null
  longitude: number | null
  location: string
  publicIP: string
  natType: string
  flags: string[]
  version: string
  subnets: string
  visible: boolean
}
export type MapLink = {
  key: string
  from: string
  to: string
  via: string | null
  kind: MapLinkKind
  source: 'route' | 'peer-center'
  latencyMs: number | null
  lossPercent: number | null
  cost: number | null
  protocol: string
  nextHop: string
  rxBytes: number | null
  txBytes: number | null
  rxBps: number | null
  txBps: number | null
}
export type MapTopology = { nodes: MapNode[]; links: MapLink[] }

const CURVE_PARALLEL_ANGLE_DEGREES = 18

function bearingDegrees(from: MapNode, to: MapNode): number {
  const radians = Math.PI / 180
  const lat1 = from.latitude! * radians
  const lat2 = to.latitude! * radians
  const deltaLon = (to.longitude! - from.longitude!) * radians
  const y = Math.sin(deltaLon) * Math.cos(lat2)
  const x = Math.cos(lat1) * Math.sin(lat2) -
    Math.sin(lat1) * Math.cos(lat2) * Math.cos(deltaLon)
  return Math.atan2(y, x) * 180 / Math.PI
}

function angularDistance(a: number, b: number): number {
  const difference = Math.abs(a - b) % 360
  return Math.min(difference, 360 - difference)
}

// 同一节点发往相近方向的直连线路使用对称曲率分车道，避免相邻节点间的线段重叠。
// 返回值以 link.from -> link.to 的方向为基准；反向链路会自动翻转符号。
export function linkCurveLanes(nodes: MapNode[], links: MapLink[]): Map<string, number> {
  type Candidate = { link: MapLink; bearing: number; direction: number }
  const byKey = new Map(nodes.map(node => [node.key, node]))
  const groups = new Map<string, Candidate[]>()

  const add = (anchor: MapNode, other: MapNode, link: MapLink, direction: number) => {
    if (anchor.latitude === null || anchor.longitude === null ||
        other.latitude === null || other.longitude === null) return
    const candidates = groups.get(anchor.key) || []
    candidates.push({ link, bearing: bearingDegrees(anchor, other), direction })
    groups.set(anchor.key, candidates)
  }

  for (const link of links) {
    if (link.kind !== 'p2p' || link.via) continue
    const from = byKey.get(link.from)
    const to = byKey.get(link.to)
    if (!from || !to) continue
    add(from, to, link, 1)
    add(to, from, link, -1)
  }

  const clusters: Candidate[][] = []
  for (const candidates of groups.values()) {
    const parents = candidates.map((_, index) => index)
    const root = (index: number): number => {
      while (parents[index] !== index) {
        parents[index] = parents[parents[index]]
        index = parents[index]
      }
      return index
    }
    for (let i = 0; i < candidates.length; i++) {
      for (let j = i + 1; j < candidates.length; j++) {
        if (angularDistance(candidates[i].bearing, candidates[j].bearing) >
            CURVE_PARALLEL_ANGLE_DEGREES) continue
        const left = root(i), right = root(j)
        if (left !== right) parents[right] = left
      }
    }
    const components = new Map<number, Candidate[]>()
    for (let i = 0; i < candidates.length; i++) {
      const key = root(i)
      const component = components.get(key) || []
      component.push(candidates[i])
      components.set(key, component)
    }
    for (const component of components.values()) {
      if (component.length > 1) clusters.push(component)
    }
  }

  // 优先处理更大的平行线路组，避免三角形拓扑中的同一条边被重复分配。
  clusters.sort((a, b) => b.length - a.length)
  const lanes = new Map<string, number>()
  for (const cluster of clusters) {
    const pending = cluster.filter(item => !lanes.has(item.link.key))
    if (pending.length < 2) continue
    pending.sort((a, b) => a.link.key.localeCompare(b.link.key))
    const half = (pending.length - 1) / 2
    pending.forEach((item, index) => {
      const lane = half === 0 ? 0 : (index - half) / half
      if (lane !== 0) lanes.set(item.link.key, lane * item.direction)
    })
  }
  return lanes
}

function numerical(value: unknown): number | null {
  if (value === null || value === undefined || value === '—' || value === '') return null
  const result = Number(value)
  return Number.isFinite(result) ? result : null
}
function metric(value: string): number | null {
  const match = value.match(/^([\d,.]+)\s*(ms|%|B)?$/)
  return match ? numerical(match[1].replaceAll(',', '')) : null
}
function kindOf(row: NetworkRow): MapLinkKind {
  // EasyTier route.cost 是路由类型的首要依据：
  // 1 表示直连，>1 表示经过中继。next_hop_peer_id 仅在 cost 缺失时兜底，
  // 不能先于 cost 判断，否则会把部分直连错误画成虚线。
  const cost = numerical(row.cost)
  if (cost === 1) return 'p2p'
  if (cost !== null && cost > 1) return 'relay'
  if (cost === null && row.nextHop !== '—') {
    return row.nextHop === row.id ? 'p2p' : 'relay'
  }
  return 'unknown'
}
function isValidLocation(attr?: NodeAttributes): boolean {
  if (!attr) return false
  const lat = attr.location.latitude
  const lon = attr.location.longitude
  return lat !== null && lon !== null && Number.isFinite(lat) && Number.isFinite(lon) &&
    lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

type PeerCenterRow = {
  node_id?: unknown
  direct_peers?: unknown
}
type PeerCenterGroup = { instanceID: string; rows: PeerCenterRow[] }

// CLI 单实例直接返回数组；多实例返回 { instance_id, result } 数组。
// 无法确定多实例的所属关系时不猜测跨网络拓扑。
function peerCenterGroups(value: unknown, instances: string[]): PeerCenterGroup[] {
  if (!Array.isArray(value)) return []
  if (value.every(item => record(item).node_id !== undefined)) {
    return instances.length === 1 ? [{ instanceID: instances[0], rows: value.map(record) }] : []
  }
  return value.flatMap(item => {
    const wrapper = record(item)
    const id = String(wrapper.instance_id ?? '')
    return instances.includes(id) && Array.isArray(wrapper.result)
      ? [{ instanceID: id, rows: list(wrapper.result).map(record) }]
      : []
  })
}

export function buildMapTopology(
  routeData: unknown,
  attributes: NodeAttributes[],
  entryHostID: string,
  peerCenterData?: unknown
): MapTopology {
  const rows = mergedNetworkRows(routeData)
  const attrs = new Map(attributes.map(item => [item.network_key + '|' + item.peer_id, item]))
  const nodes: MapNode[] = rows.map(row => {
    const attr = attrs.get(networkNodeAttributeKey(row, entryHostID) + '|' + row.id)
    const positioned = isValidLocation(attr)
    return {
      key: row.key, id: row.id, instanceID: row.instanceID, instance: row.instance,
      networkKey: row.networkKey, hostname: row.hostname,
      label: attr?.alias || row.hostname, ip: row.cidr,
      latitude: positioned ? attr!.location.latitude : null,
      longitude: positioned ? attr!.location.longitude : null,
      location: attr ? [attr.location.country, attr.location.province, attr.location.city, attr.location.county]
        .filter(Boolean).join(' · ') : '',
      publicIP: attr?.has_public_ip ? attr.public_ip : '',
      natType: row.natType, flags: row.flags, version: row.version, subnets: row.subnets,
      visible: true
    }
  })
  const locals = new Map(rows.filter(row => row.kind === '入口实例').map(row => [row.instanceID, row]))
  const links: MapLink[] = []
  const pairKey = (a: string, b: string) => [a, b].sort().join('|')
  for (const row of rows) {
    if (row.kind === '入口实例') continue
    const local = locals.get(row.instanceID)
    if (!local || local.key === row.key) continue
    const kind = kindOf(row)
    const viaRow = rows.find(candidate => candidate.instanceID === row.instanceID &&
      candidate.id === row.nextHop && candidate.key !== row.key && candidate.key !== local.key)
    const link: MapLink = {
      key: 'route:' + local.key + ':' + row.key,
      from: local.key, to: row.key, via: kind === 'relay' ? viaRow?.key || null : null,
      kind, source: 'route', latencyMs: metric(row.latency), lossPercent: metric(row.loss),
      cost: numerical(row.cost), protocol: row.protocol, nextHop: row.nextHop,
      rxBytes: metric(row.rx), txBytes: metric(row.tx), rxBps: null, txBps: null
    }
    links.push(link)
  }

  const instanceIDs = [...new Set(rows.map(row => row.instanceID))]
  const seenPeerCenterPairs = new Set<string>()
  for (const group of peerCenterGroups(peerCenterData, instanceIDs)) {
    const byID = new Map(nodes.filter(node => node.instanceID === group.instanceID)
      .map(node => [node.id, node]))
    for (const entry of group.rows) {
      const from = byID.get(String(entry.node_id ?? ''))
      if (!from) continue
      for (const peer of list(entry.direct_peers)) {
        const data = record(peer)
        const to = byID.get(String(data.node_id ?? ''))
        if (!to || to.key === from.key) continue
        const pair = pairKey(from.key, to.key)
        if (seenPeerCenterPairs.has(pair)) continue
        seenPeerCenterPairs.add(pair)

        // Peer Center 来自 EasyTier 全局 Peer Map，表示两个节点确实建立了直连。
        // 如果入口 Route 对同一节点对推断出中继，真实直连关系优先，避免虚线覆盖实线。
        const existing = links.find(link => pairKey(link.from, link.to) === pair)
        if (existing) {
          existing.kind = 'p2p'
          existing.source = 'peer-center'
          existing.via = null
          existing.latencyMs = numerical(data.latency_ms) ?? existing.latencyMs
          existing.cost = null
          continue
        }
        links.push({
          key: 'peer-center:' + pair, from: from.key, to: to.key,
          via: null, kind: 'p2p', source: 'peer-center',
          latencyMs: numerical(data.latency_ms), lossPercent: null, cost: null,
          protocol: '—', nextHop: '—', rxBytes: null, txBytes: null, rxBps: null, txBps: null
        })
      }
    }
  }
  return { nodes, links }
}

export type TrafficSample = { rx: number | null; tx: number | null; time: number }
export function trafficRates(
  previous: TrafficSample | undefined,
  current: TrafficSample
): { rxBps: number | null; txBps: number | null } {
  if (!previous || current.time <= previous.time) return { rxBps: null, txBps: null }
  const elapsed = (current.time - previous.time) / 1000
  const rate = (now: number | null, before: number | null): number | null =>
    now !== null && before !== null && now >= before && elapsed > 0
      ? (now - before) / elapsed : null
  return { rxBps: rate(current.rx, previous.rx), txBps: rate(current.tx, previous.tx) }
}
