import { Box, Connection, Link, VideoPlay } from '@element-plus/icons-vue'
import { publicIPv4Candidate } from '../network-nodes/data'
import type { DeviceService } from './types'

export const serviceIcons = [
  { value: 'web', label: '网页', component: Link },
  { value: 'storage', label: '存储', component: Box },
  { value: 'terminal', label: '远程连接', component: Connection },
  { value: 'media', label: '影音', component: VideoPlay }
]
export function serviceIcon(value: string) {
  return serviceIcons.find(icon => icon.value === value)?.component || Connection
}

export function serviceURL(ip: string, port: number | undefined, existing: string) {
  if (!ip || port == null || !Number.isInteger(port) || port < 1 || port > 65535) return ''
  const validIPv4 = /^\d{1,3}(?:\.\d{1,3}){3}$/.test(ip) && ip.split('.').every(octet => Number(octet) <= 255)
  if (!validIPv4 && !/^[0-9a-f:.]+:[0-9a-f:.]+$/i.test(ip)) return ''
  const host = validIPv4 ? ip : '[' + ip + ']'
  const scheme = /^https:\/\//i.test(existing) ? 'https' : 'http'
  try {
    const normalized = new URL(scheme + '://' + host).hostname
    if (['0.0.0.0', '[::]'].includes(normalized)) return ''
    return scheme + '://' + normalized + ':' + port
  } catch {
    return ''
  }
}

export function publicAccessURL(ip: string, listen: string, existing: string) {
  const colon = listen.lastIndexOf(':')
  if (colon < 0) return ''
  const bind = listen.slice(0, colon).replace(/^\[|\]$/g, '')
  if (bind === '::1' || bind.startsWith('127.')) return ''
  return serviceURL(publicIPv4Candidate(bind) || ip, Number(listen.slice(colon + 1)), existing)
}

export function activePublicURL(service: DeviceService | null, entryPublicIps: Record<string, Record<string, string>>, existing = service?.access_url ?? '') {
  const cfg = service?.forward
  if (!cfg?.enabled) return ''
  return publicAccessURL(entryPublicIps[cfg.host_id]?.[cfg.instance_id] || '', cfg.listen, existing)
}
