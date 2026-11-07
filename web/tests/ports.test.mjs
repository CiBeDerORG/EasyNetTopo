import assert from 'node:assert/strict'
import test from 'node:test'
import { formatPublishedPort, uniquePublishedPorts } from '../src/features/host-store/ports.ts'

test('双栈 Docker 端口的 IPv4 与 IPv6 必须可区分', () => {
  const source = [
    { host_ip: '::', host_port: 7004, container_port: 9000, protocol: 'tcp' },
    { host_ip: '0.0.0.0', host_port: 7004, container_port: 9000, protocol: 'tcp' },
    { host_ip: '::', host_port: 7004, container_port: 9000, protocol: 'tcp' }
  ]
  const displayed = uniquePublishedPorts(source).map(formatPublishedPort)
  assert.deepEqual(displayed, [
    'IPv4 · 0.0.0.0:7004 → 9000/tcp',
    'IPv6 · [::]:7004 → 9000/tcp'
  ])
  assert.equal(source.length, 3, '不能修改原始 Docker 采集记录')
})

test('指定绑定地址按实际 IP 展示，不能统称为公网或所有网卡', () => {
  assert.equal(formatPublishedPort({
    host_ip: '127.0.0.1', host_port: 7017, container_port: 8080, protocol: 'tcp'
  }), 'IPv4 · 127.0.0.1:7017 → 8080/tcp')
  assert.equal(formatPublishedPort({
    host_ip: '2001:db8::1', host_port: 7017, container_port: 8080, protocol: 'tcp'
  }), 'IPv6 · [2001:db8::1]:7017 → 8080/tcp')
  assert.equal(formatPublishedPort({
    host_port: 7017, container_port: 8080, protocol: 'tcp'
  }), '地址未提供 · 未知:7017 → 8080/tcp')
})

test('不同端口、容器端口及 TCP/UDP 不能被去重', () => {
  const ports = [
    { host_ip: '0.0.0.0', host_port: 80, container_port: 80, protocol: 'tcp' },
    { host_ip: '0.0.0.0', host_port: 80, container_port: 80, protocol: 'udp' },
    { host_ip: '0.0.0.0', host_port: 80, container_port: 8080, protocol: 'tcp' }
  ]
  assert.equal(uniquePublishedPorts(ports).length, 3)
  assert.deepEqual(uniquePublishedPorts([]), [])
})
