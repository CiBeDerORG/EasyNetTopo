import assert from 'node:assert/strict'
import test from 'node:test'
import { networkRows, ipv4 } from '../src/features/network-nodes/data.ts'

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
