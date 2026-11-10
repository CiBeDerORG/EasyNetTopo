import assert from 'node:assert/strict'
import test from 'node:test'
import { networkRows, mergedNetworkRows, ipv4, ipv4Cidr } from '../src/features/network-nodes/data.ts'

// 根据真实 yun 组网入口的 CLI -v -o json route 结构构造脱敏样例。
const yun = {
  node_info: {
    hostname: 'yun', inst_id: 'instance-yun', peer_id: 3268205868,
    ipv4_addr: '10.10.10.1/24', version: '2.6.4-8428a89d',
    listeners: ['tcp://0.0.0.0:7005'], stun_info: { public_ip: [] }
  },
  peer_routes: [
    { route: {
      hostname: 'FnOS', peer_id: 354390789,
      ipv4_addr: { address: { addr: 168430082 }, network_length: 24 },
      cost: 1, path_latency: 17, next_hop_peer_id: 354390789,
      proxy_cidrs: [], version: '2.7.0-9eb9e8ac'
    }, peer: { conns: [
      { tunnel: { tunnel_type: 'udp' }, stats: { latency_us: 16000, rx_bytes: 42 } },
      { tunnel: { tunnel_type: 'tcp' }, stats: { latency_us: 17000, tx_bytes: 84 } }
    ] } },
    { route: {
      hostname: 'OWRT', peer_id: 3985186286,
      ipv4_addr: { address: { addr: 168430083 }, network_length: 24 },
      cost: 1, path_latency: 5, next_hop_peer_id: 3985186286,
      proxy_cidrs: ['192.168.10.0/24'], version: '2.6.4-8428a89d'
    }, peer: { conns: [
      { tunnel: { tunnel_type: 'tcp' } },
      { tunnel: { tunnel_type: 'udp' } }
    ] } }
  ]
}

test('真实单实例 route 的本机与两条路由生成三个节点，网络名称不冒充主机名', () => {
  const rows = mergedNetworkRows(yun)
  assert.equal(rows.length, 3)
  assert.deepEqual(rows.map(row => row.hostname), ['yun', 'FnOS', 'OWRT'])
  assert.equal(rows[0].kind, '入口实例')
  assert.equal(rows[0].cidr, '10.10.10.1/24')
  assert.equal(rows[0].instance, '实例 instance')
  assert.equal(rows[0].instanceID, 'instance-yun')

  const fnos = rows[1]
  assert.equal(fnos.cidr, '10.10.10.2/24')
  assert.equal(fnos.routeLabel, 'P2P 直连路由')
  assert.equal(fnos.protocol, 'udp、tcp')
  assert.equal(fnos.latency, '17.00 ms')
  assert.equal(fnos.directLatency, '16.00 ms')
  assert.equal(fnos.rx, '42 B')
  assert.equal(fnos.tx, '84 B')

  const owrt = rows[2]
  assert.equal(owrt.cidr, '10.10.10.3/24')
  assert.equal(owrt.latency, '5.00 ms')
  assert.equal(owrt.subnets, '192.168.10.0/24')
  assert.equal(owrt.protocol, 'tcp、udp')
  assert.equal(owrt.rx, '—', '没有流量统计不应显示为零')
})

test('原入口实例与网络路由仍能独立查询并解析', () => {
  assert.equal(networkRows(yun.node_info, 'node')[0].hostname, 'yun')
  assert.equal(networkRows(yun, 'route').length, 2)
  assert.equal(networkRows(yun, 'route')[1].hostname, 'OWRT')
  assert.equal(networkRows(yun, 'route')[0].latency, '17.00 ms')
  assert.equal(networkRows(yun, 'node')[0].listeners, 'tcp://0.0.0.0:7005')
  assert.equal(ipv4({ address: { addr: 168430082 }, network_length: 24 }), '10.10.10.2')
  assert.equal(ipv4Cidr({ address: { addr: 168430082 }, network_length: 24 }), '10.10.10.2/24')
})

test('多实例 route 包装数组按实例 ID 隔离相同节点，使用显式实例名', () => {
  const a = { instance_id: 'a', instance_name: '网络甲', result: yun }
  const b = { instance_id: 'b', instance_name: '网络乙', result: yun }
  const rows = mergedNetworkRows([a, b])
  assert.equal(rows.length, 6)
  assert.deepEqual(rows.filter(row => row.kind === '入口实例').map(row => row.instance), ['网络乙', '网络甲'].sort((x,y)=>x.localeCompare(y,'zh-CN')))
  assert.equal(new Set(rows.map(row => row.key)).size, 6)
  assert.equal(rows.filter(row => row.hostname === 'FnOS').length, 2)
  assert.equal(networkRows([a,b], 'route').length, 4)
  assert.equal(networkRows([a,b], 'node').length, 2)
})

test('中继路由保留路径延迟；关闭连接不算直连', () => {
  const routeValue = {
    node_info: yun.node_info,
    peer_routes: [{ route: {
      hostname: '中继后节点', peer_id: 10, ipv4_addr: '10.10.10.4/24',
      cost: 3, path_latency: 24.5
    }, peer: { conns: [{ is_closed: true, tunnel: { tunnel_type: 'tcp' } }] } }]
  }
  const row = mergedNetworkRows(routeValue)[1]
  assert.equal(row.routeLabel, '中继 · 开销 3')
  assert.equal(row.latency, '24.50 ms')
  assert.equal(row.connection, '未报告直连连接')
  assert.equal(row.protocol, '—')
  assert.equal(row.directLatency, '—')
})

test('路由回报本机时去重，缺失路由延迟可从直连统计补齐', () => {
  const routeValue = {
    node_info: yun.node_info,
    peer_routes: [
      { route: { hostname: 'yun', peer_id: yun.node_info.peer_id, ipv4_addr: '10.10.10.1/24' } },
      { route: { hostname: 'FnOS', peer_id: 2, ipv4_addr: '10.10.10.2/24', cost: 1 },
        peer: { conns: [
          { stats: { latency_us: 1500 }, tunnel: { tunnel_type: 'udp' } },
          { stats: { latency_us: 0 }, is_closed: true }
        ] } }
    ]
  }
  const rows = mergedNetworkRows(routeValue)
  assert.equal(rows.length, 2)
  assert.equal(rows[1].latency, '1.50 ms')
})

test('空路由只显示本机；空多实例无节点；不受支持的结构明确报错', () => {
  assert.equal(mergedNetworkRows({ node_info: yun.node_info, peer_routes: [] }).length, 1)
  assert.deepEqual(mergedNetworkRows([]), [])
  assert.deepEqual(networkRows([], 'node'), [])
  assert.equal(ipv4(null), '—')
  assert.throws(() => mergedNetworkRows({ peer_routes: [] }), /node_info/)
  assert.throws(() => mergedNetworkRows({ node_info: yun.node_info }), /peer_routes/)
  assert.throws(() => mergedNetworkRows([{ route: {} }]), /多实例/)
  assert.throws(() => networkRows({ unknown: true }, 'route'), /peer_routes/)
})
