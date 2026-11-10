import assert from 'node:assert/strict'
import test from 'node:test'
import { networkRows, mergedNetworkRows, ipv4 } from '../src/features/network-nodes/data.ts'

test('官方单实例节点与多实例节点均可展示', () => {
  const node = { hostname: 'yun', peer_id: 123, ipv4_addr: '10.0.0.1', proxy_cidrs: [], inst_id: 'a' }
  assert.equal(networkRows(node, 'node')[0].hostname, 'yun')
  assert.equal(networkRows([{ instance_id: 'a', instance_name: '网络甲', result: node }, { instance_id: 'b', instance_name: '网络乙', result: node }], 'node').length, 2)
})
test('子网路由与直连统计取自不同字段，不把缺失数据当作零', () => {
  const pair = { route: { hostname: 'OWRT', ipv4_addr: { address: { addr: 167772162 }, network_length: 24 }, proxy_cidrs: ['192.168.10.0/24'], next_hop_peer_id: 12, cost: 1 }, peer: { conns: [{ stats: { rx_bytes: 42, tx_bytes: 84, latency_us: 1500 }, tunnel: { tunnel_type: 'udp' } }] } }
  const row = networkRows({ peer_routes: [pair] }, 'peer')[0]
  assert.equal(row.ip, '10.0.0.2')
  assert.equal(row.subnets, '192.168.10.0/24')
  assert.equal(row.latency, '1.50 ms')
  assert.equal(row.connection, '直连')
  assert.equal(networkRows({ peer_routes: [{ route: pair.route }] }, 'route')[0].latency, '—')
  assert.equal(ipv4(null), '—')
})
test('空网络与未知响应格式明确区分', () => {
  assert.deepEqual(networkRows([], 'node'), [])
  assert.deepEqual(networkRows({ peer_routes: [] }, 'peer'), [])
  assert.throws(() => networkRows({ unknown: [] }, 'peer'))
  assert.throws(() => networkRows([{ unknown: 1 }], 'node'))
})

test('魔改版节点清单：入口、路由与直连信息合并在同一列表', () => {
  const node = { hostname: 'yun', inst_id: 'instance-1', peer_id: 1, ipv4_addr: '10.0.0.1', version: '2.6.4' }
  const route = { peer_routes: [
    { route: { hostname: 'yun', peer_id: 1, ipv4_addr: '10.0.0.1', cost: 0 } },
    { route: { hostname: 'OWRT', peer_id: 2, ipv4_addr: '10.0.0.2', cost: 1 }, peer: {
      peer_id: 2, conns: [{ stats: { latency_us: 1800 }, tunnel: { tunnel_type: 'udp' } }]
    } },
    { route: { hostname: '远端节点', peer_id: 3, ipv4_addr: '10.0.0.3', cost: 3, path_latency: 24.5 } }
  ] }
  const rows = mergedNetworkRows(node, route)
  assert.equal(rows.length, 3, '本机节点不应被重复列为路由')
  assert.equal(rows[0].kind, '入口实例')
  assert.equal(rows[0].hostname, 'yun')
  const direct = rows.find(row => row.hostname === 'OWRT')
  const relay = rows.find(row => row.hostname === '远端节点')
  assert.equal(direct?.routeLabel, 'P2P 直连')
  assert.equal(direct?.protocol, 'udp')
  assert.equal(direct?.latency, '1.80 ms')
  assert.equal(relay?.routeLabel, '中继 · 开销 3')
  assert.equal(relay?.protocol, '—')
  assert.equal(relay?.latency, '24.50 ms')
  assert.equal(relay?.connection, '未报告直连')
})

test('入口或网络路由单独采集成功时仍可展示，并保留多实例身份', () => {
  const peer = { peer_routes: [{ route: { hostname: '远端', peer_id: 9, ipv4_addr: '10.0.0.9', cost: 2 } }] }
  assert.equal(mergedNetworkRows(null, peer).length, 1)
  assert.equal(mergedNetworkRows({ hostname: '入口', peer_id: 1 }, null).length, 1)
  assert.deepEqual(mergedNetworkRows(null, null), [])
  const nodes = [
    { instance_id: 'A', instance_name: '甲网络', result: { hostname: '甲', peer_id: 1 } },
    { instance_id: 'B', instance_name: '乙网络', result: { hostname: '乙', peer_id: 2 } }
  ]
  const peers = [
    { instance_id: 'A', instance_name: '甲网络', result: peer },
    { instance_id: 'B', instance_name: '乙网络', result: peer }
  ]
  const rows = mergedNetworkRows(nodes, peers)
  assert.equal(rows.length, 4)
  assert.deepEqual(rows.filter(row => row.kind === '网络路由').map(row => row.instance).sort(), ['乙网络', '甲网络'])
  assert.equal(new Set(rows.map(row => row.key)).size, rows.length)
})
