import type { DockerPort } from './types'

// Docker 可能同时报告 0.0.0.0 与 :: 绑定。这是两个不同的地址族，
// 不能统一缩写为 *；只消除地址、端口与协议完全相同的重复记录。
export function uniquePublishedPorts(ports?: DockerPort[]): DockerPort[] {
  const seen = new Set<string>()
  return (ports || []).filter(port => {
    const key = [port.host_ip ?? '', port.host_port, port.container_port, port.protocol].join('|')
    if (seen.has(key)) return false
    seen.add(key)
    return true
  }).sort((a, b) => {
    const family = (ip?: string) => ip?.includes(':') ? 1 : 0
    return a.host_port - b.host_port ||
      family(a.host_ip) - family(b.host_ip) ||
      (a.host_ip || '').localeCompare(b.host_ip || '') ||
      a.container_port - b.container_port ||
      a.protocol.localeCompare(b.protocol)
  })
}

export function formatPublishedPort(port: DockerPort): string {
  const ip = port.host_ip?.trim() || ''
  const family = ip.includes(':') ? 'IPv6' : ip ? 'IPv4' : '地址未提供'
  const address = ip.includes(':') ? '[' + ip + ']' : ip || '未知'
  return family + ' · ' + address + ':' + port.host_port +
    ' → ' + port.container_port + '/' + port.protocol
}
