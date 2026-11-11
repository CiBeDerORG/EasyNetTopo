import assert from 'node:assert/strict'
import test from 'node:test'
import { gatewayIPFromProxySubnets, suggestedDirectGatewayIP } from '../src/features/devices/gateway-ip.ts'

test('从代理网段计算第一个可用 IPv4 主机地址', () => {
  assert.equal(gatewayIPFromProxySubnets('192.168.10.0/24'), '192.168.10.1')
  assert.equal(gatewayIPFromProxySubnets('10.20.0.0/16'), '10.20.0.1')
  assert.equal(gatewayIPFromProxySubnets('192.168.10.123/24'), '192.168.10.1')
  assert.equal(gatewayIPFromProxySubnets('192.168.10.4/30'), '192.168.10.5')
  assert.equal(gatewayIPFromProxySubnets('2001:db8::/64、192.168.20.0/24'), '192.168.20.1')
  assert.equal(gatewayIPFromProxySubnets('192.168.10.0/24、192.168.20.0/24'), '192.168.10.1')
})

test('无效或缺少可用 IPv4 主机地址时不推断', () => {
  for (const value of [
    '', '—', 'unknown', '192.168.10.0/31', '192.168.10.1/32',
    '0.0.0.0/0', '127.0.0.0/8', '169.254.1.0/24',
    '192.168.999.0/24', '192.168.010.0/24', '2001:db8::/64'
  ]) {
    assert.equal(gatewayIPFromProxySubnets(value), '', value)
  }
})

test('自动填写只针对当前网络节点的直属网关设备，不覆盖原 IP', () => {
  const nodes = [
    { networkKey: 'network:home', peerId: 'peer-a', row: { subnets: '192.168.10.0/24' } },
    { networkKey: 'network:home', peerId: 'peer-b', row: { subnets: '192.168.20.0/24' } },
    { networkKey: 'network:other', peerId: 'peer-a', row: { subnets: '10.10.0.0/16' } }
  ]
  const device = {
    is_gateway: true, gateway_group_id: '', ip_address: '',
    network_key: 'network:home', peer_id: 'peer-a'
  }
  assert.equal(suggestedDirectGatewayIP(device, nodes), '192.168.10.1')
  assert.equal(suggestedDirectGatewayIP({ ...device, peer_id: 'peer-b' }, nodes), '192.168.20.1')
  assert.equal(suggestedDirectGatewayIP({ ...device, network_key: 'network:other' }, nodes), '10.10.0.1')
  assert.equal(suggestedDirectGatewayIP({ ...device, peer_id: 'unknown' }, nodes), '')
  assert.equal(suggestedDirectGatewayIP({ ...device, is_gateway: false }, nodes), '')
  assert.equal(suggestedDirectGatewayIP({ ...device, gateway_group_id: 'parent-gateway' }, nodes), '')
  assert.equal(suggestedDirectGatewayIP({ ...device, ip_address: '192.168.10.250' }, nodes), '')
  assert.equal(suggestedDirectGatewayIP({ ...device, ip_address: ' 192.168.10.250 ' }, nodes), '')
  assert.equal(suggestedDirectGatewayIP(device, [{ networkKey: 'network:home', peerId: 'peer-a' }]), '')
})
