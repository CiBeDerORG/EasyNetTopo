import test from 'node:test'
import assert from 'node:assert/strict'
import { buildMapTopology, trafficRates } from '../src/features/network-map/topology.ts'

const route = {
  node_info: { hostname: 'yun', peer_id: 100, inst_id: 'a', network_name: 'prod', ipv4_addr: '10.10.10.1' },
  peer_routes: [
    { route: { peer_id: 200, hostname: 'FnOS', cost: 1, next_hop_peer_id: 200, path_latency: 20 },
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

test('route 数据构建本机、P2P、中继及未知路由，且没有位置时不猜测坐标', () => {
  const graph = buildMapTopology(route, [attribute(100, 31, 121), attribute(200, 35, 139)], 'entry')
  assert.equal(graph.nodes.length, 4)
  assert.deepEqual(graph.links.map(link => link.kind).sort(), ['p2p', 'relay', 'unknown'].sort())
  const direct = graph.links[0]
  assert.equal(direct.latencyMs, 20)
  assert.equal(direct.rxBytes, 400)
  assert.equal(direct.txBytes, 600)
  assert.equal(graph.links.find(link => link.kind === 'relay')?.via, graph.nodes.find(node => node.id === '200')?.key)
  assert.equal(graph.nodes[2].latitude, null)
  assert.equal(graph.nodes[3].longitude, null)
})

test('Peer Center 补充真正的远端直连边，不覆盖入口路由的转发关系', () => {
  const peerCenter = [
    { node_id: '200', direct_peers: [{ node_id: '300', latency_ms: 11 }] },
    { node_id: '300', direct_peers: [{ node_id: '200', latency_ms: 11 }, { node_id: '100', latency_ms: 6 }] }
  ]
  const graph = buildMapTopology(route, [], 'entry', peerCenter)
  assert.equal(graph.links.length, 5)
  assert.equal(graph.links.filter(link => link.kind === 'relay').length, 1)
  const extra = graph.links.filter(link => link.source === 'peer-center')
  assert.equal(extra.length, 2)
  assert.ok(extra.every(link => link.kind === 'p2p'))
  assert.ok(extra.some(link => link.latencyMs === 11))
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
