import type { QueryClient } from '@tanstack/vue-query'
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
  hostname?: string
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

export function updateNodeAttributesCache(client: QueryClient, saved: NodeAttributes) {
  const key = ['network-node-attributes']
  client.setQueryData<{ attributes: NodeAttributes[] }>(key, previous => {
    if (!previous) return previous
    const hostname = saved.hostname?.toLocaleLowerCase()
    const otherNodes = previous.attributes.filter(item => {
      if (item.network_key !== saved.network_key) return true
      if (item.peer_id === saved.peer_id) return false
      return !hostname || item.hostname?.toLocaleLowerCase() !== hostname
    })
    return { attributes: [...otherNodes, saved] }
  })
  void client.invalidateQueries({ queryKey: key })
}
