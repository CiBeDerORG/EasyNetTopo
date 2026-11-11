import { apiRequest } from '../../lib/api-client'

export type NodeLocation = {
  country: string
  province: string
  city: string
  county: string
  latitude: number | null
  longitude: number | null
}

export type NodeAttributes = {
  network_key: string
  peer_id: string
  alias: string
  has_public_ip: boolean
  public_ip: string
  location: NodeLocation
}

export function listNodeAttributes() {
  return apiRequest<{ attributes: NodeAttributes[] }>('/api/v1/network-nodes/attributes')
}
export function saveNodeAttributes(value: NodeAttributes) {
  return apiRequest<NodeAttributes>('/api/v1/network-nodes/attributes', { method: 'PUT', body: value })
}
export function clearNodeAttributes() {
  return apiRequest<void>('/api/v1/network-nodes/attributes', { method: 'DELETE' })
}
