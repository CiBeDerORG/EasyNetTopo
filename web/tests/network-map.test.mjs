import test from 'node:test'
import assert from 'node:assert/strict'
import { buildMapTopology, curveSeparationMeters, linkCurveLanes, trafficRates } from '../src/features/network-map/topology.ts'

const route = {
  node_info: { hostname: 'yun', peer_id: 100, inst_id: 'a', network_name: 'prod', ipv4_addr: '10.10.10.1' },
  peer_routes: [
    { route: { peer_id: 200, hostname: 'FnOS', cost: 1, next_hop_peer_id: 999, path_latency: 20 },
      peer: { conns: [{ stats: { rx_bytes: 400, tx_bytes: 600, latency_us: 18000 }, tunnel: { tunnel_type: 'udp' } }] } },
    { route: { peer_id: 300, hostname: 'OWRT', cost: 3, next_hop_peer_id: 200, path_latency: 67 } },
    { route: { peer_id: 400, hostname: 'Mac', cost: 0, path_latency: 0 } }
  ]
}
const attribute = (id, latitude, longitude) => ({
  network_key: 'network:prod', peer_id: String(id), alias: '',
  has_public_ip: false, public_ip: '',
  location: { country: '中国', province: '', city: '', county: '', latitude, longitude }
})

const mapNode = (key, latitude, longitude) => ({
  key, id: key, instanceID: 'a', instance: 'prod', networkKey: 'network:prod',
  hostname: key, label: key, ip: '', latitude, longitude, location: '',
  publicIP: '', natType: '', flags: [], version: '', subnets: '', visible: true
})
const mapLink = (key, from, to) => ({
  key, from, to, via: null, kind: 'p2p', source: 'peer-center',
  latencyMs: null, lossPercent: null, cost: null, protocol: '—', nextHop: '—',
  rxBytes: null, txBytes: null, rxBps: null, txBps: null
})

test('route 数据构建本机、P2P、中继及未知路由，且没有位置时不猜测坐标', () => {
  const graph = buildMapTopology(route, [attribute(100, 31, 121), attribute(200, 35, 139)], 'entry')
  assert.equal(graph.nodes.length, 4)
  assert.deepEqual(graph.links.map(link => link.kind).sort(), ['p2p', 'relay', 'unknown'].sort())
  const direct = graph.links.find(link => link.kind === 'p2p')
  assert.ok(direct)
  assert.equal(direct.latencyMs, 20)
  assert.equal(direct.rxBytes, 400)
  assert.equal(direct.txBytes, 600)
  assert.equal(graph.links.find(link => link.kind === 'relay')?.via, graph.nodes.find(node => node.id === '200')?.key)
  assert.equal(graph.nodes[2].latitude, null)
  assert.equal(graph.nodes[3].longitude, null)
})

test('相近方向的直连线路按真实方位分配车道，不会因 key 排序颠倒', () => {
  const nodes = [
    mapNode('source', 0, 0),
    mapNode('target-north', 1, 10),
    mapNode('target-south', -1, 10),
    mapNode('target-other', 10, 0)
  ]
  const links = [
    { ...mapLink('north', 'source', 'target-north'), source: 'route' },
    { ...mapLink('south', 'source', 'target-south'), source: 'route' },
    mapLink('other-direction', 'source', 'target-other')
  ]
  const lanes = linkCurveLanes(nodes, links)
  assert.equal(lanes.get('north'), -0.5)
  assert.equal(lanes.get('south'), 0.5)
  assert.equal(lanes.has('other-direction'), false)
})

test('反向链路也保持相对于共同节点的地理车道顺序', () => {
  const nodes = [
    mapNode('source', 0, 0),
    mapNode('target-north', 1, 10),
    mapNode('target-south', -1, 10)
  ]
  const links = [
    mapLink('north', 'source', 'target-north'),
    mapLink('south-reversed', 'target-south', 'source')
  ]
  const lanes = linkCurveLanes(nodes, links)
  assert.equal(lanes.get('north'), -0.5)
  assert.equal(lanes.get('south-reversed'), -0.5)
  // 乘以方向后，两条线相对于共同 source 的车道仍然是从北到南递增。
  assert.ok((lanes.get('north') || 0) < -(lanes.get('south-reversed') || 0))
})

test('线路互斥距离随相机高度缩放，全球视角大、节点近景小', () => {
  assert.equal(curveSeparationMeters(25_500_000), 240_000)
  assert.equal(curveSeparationMeters(780_000), 7_800)
  assert.equal(curveSeparationMeters(50_000), 900)
})

test('Peer Center 的真实直连优先级高于同一节点对的入口路由推断', () => {
  const peerCenter = [
    { node_id: '200', direct_peers: [{ node_id: '300', latency_ms: 11 }] },
    { node_id: '300', direct_peers: [{ node_id: '200', latency_ms: 11 }, { node_id: '100', latency_ms: 6 }] }
  ]
  const graph = buildMapTopology(route, [], 'entry', peerCenter)
  assert.equal(graph.links.length, 4)
  assert.equal(graph.links.filter(link => link.kind === 'relay').length, 0)
  const extra = graph.links.filter(link => link.source === 'peer-center')
  assert.equal(extra.length, 2)
  assert.ok(extra.every(link => link.kind === 'p2p'))
  assert.ok(extra.some(link => link.latencyMs === 11))
  const local = graph.nodes.find(node => node.id === '100')
  const relayPeer = graph.nodes.find(node => node.id === '300')
  assert.ok(local && relayPeer)
  const overridden = graph.links.find(link =>
    (link.from === local.key && link.to === relayPeer.key) ||
    (link.from === relayPeer.key && link.to === local.key))
  assert.equal(overridden?.kind, 'p2p')
  assert.equal(overridden?.source, 'peer-center')
})

test('没有实例标识的全局拓扑不能跨两个网络实例乱连', () => {
  const graph = buildMapTopology([
    { instance_id: 'a', instance_name: 'alpha', result: route },
    { instance_id: 'b', instance_name: 'beta', result: route }
  ], [], 'entry', [{ node_id: '100', direct_peers: [{ node_id: '200', latency_ms: 3 }] }])
  assert.equal(graph.nodes.length, 8)
  assert.equal(graph.links.length, 6)
  assert.equal(graph.links.filter(link => link.source === 'peer-center').length, 0)
})

test('公网 IP 候选并不被当成节点的已确认地理位置或真实公网 IP', () => {
  const node = buildMapTopology(route, [attribute(200, 0, 0)], 'entry').nodes[1]
  assert.equal(node.latitude, 0)
  assert.equal(node.longitude, 0)
  assert.equal(node.publicIP, '')
})

test('累计流量必须有连续采样，回退或重置不产生负速率', () => {
  assert.deepEqual(trafficRates(undefined, {rx: 500, tx: 100, time: 10000}),
    { rxBps: null, txBps: null })
  assert.deepEqual(trafficRates({rx: 500, tx: 100, time: 10000},
    {rx: 1524, tx: 612, time: 11000}), {rxBps: 1024, txBps: 512})
  assert.deepEqual(trafficRates({rx: 500, tx: 100, time: 10000},
    {rx: 0, tx: null, time: 11000}), {rxBps: null, txBps: null})
})
