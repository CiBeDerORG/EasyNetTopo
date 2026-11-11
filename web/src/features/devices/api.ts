import { apiRequest } from '../../lib/api-client'
import type {
  Device, DeviceGroup, DeviceInput, DeviceOpenWrtConfig, GatewayGroup,
  NetworkNodeDeviceInput, NetworkNodeSyncResult, NetworkNodeSyncStatus,
  OpenWrtDiscoveryKind, OpenWrtDiscoverySnapshot
} from './types'

export async function getNodeDevices() {
  const result = await apiRequest<{ node_devices: Device[] }>('/api/v1/devices/network-nodes')
  return result.node_devices || []
}
export async function getDevices(scope?: { networkKey: string; peerId: string } | { unassigned: true }) {
  const params = new URLSearchParams()
  if (scope && 'unassigned' in scope) params.set('unassigned', 'true')
  else if (scope) {
    params.set('network_key', scope.networkKey)
    params.set('peer_id', scope.peerId)
  }
  const query = params.toString()
  const suffix = query ? '?' + query : ''
  const result = await apiRequest<{ devices: Device[] }>('/api/v1/devices' + suffix)
  return result.devices || []
}
export function createDevice(payload: DeviceInput) {
  return apiRequest<Device>('/api/v1/devices', { method: 'POST', body: payload })
}
export function updateDevice(id: string, payload: DeviceInput) {
  return apiRequest<Device>('/api/v1/devices/' + encodeURIComponent(id), { method: 'PATCH', body: payload })
}
export function deleteDevice(id: string) {
  return apiRequest<void>('/api/v1/devices/' + encodeURIComponent(id), { method: 'DELETE' })
}

export async function getDeviceGroups() {
  const result = await apiRequest<{ groups: DeviceGroup[] }>('/api/v1/device-groups')
  return result.groups || []
}
export function createDeviceGroup(name: string) {
  return apiRequest<DeviceGroup>('/api/v1/device-groups', { method: 'POST', body: { name } })
}
export function updateDeviceGroup(id: string, name: string) {
  return apiRequest<DeviceGroup>('/api/v1/device-groups/' + encodeURIComponent(id), { method: 'PATCH', body: { name } })
}
export function deleteDeviceGroup(id: string) {
  return apiRequest<void>('/api/v1/device-groups/' + encodeURIComponent(id), { method: 'DELETE' })
}

export async function getGatewayGroups(networkKey: string, peerId: string) {
  const params = new URLSearchParams({ network_key: networkKey, peer_id: peerId })
  const result = await apiRequest<{ groups: GatewayGroup[] }>('/api/v1/gateway-groups?' + params.toString())
  return result.groups || []
}
export function createGatewayGroup(name: string, networkKey: string, peerId: string) {
  return apiRequest<GatewayGroup>('/api/v1/gateway-groups', {
    method: 'POST', body: { name, network_key: networkKey, peer_id: peerId }
  })
}
export function updateGatewayGroup(id: string, name: string) {
  return apiRequest<GatewayGroup>('/api/v1/gateway-groups/' + encodeURIComponent(id), {
    method: 'PATCH', body: { name }
  })
}
export function deleteGatewayGroup(id: string) {
  return apiRequest<void>('/api/v1/gateway-groups/' + encodeURIComponent(id), { method: 'DELETE' })
}

export function getNetworkNodeSyncStatus() {
  return apiRequest<NetworkNodeSyncStatus>('/api/v1/devices/network-nodes/sync')
}
export function syncNetworkNodeDevices(nodes: NetworkNodeDeviceInput[], enableAutoSync = true) {
  return apiRequest<NetworkNodeSyncResult>('/api/v1/devices/network-nodes/sync', {
    method: 'POST', body: { nodes, enable_auto_sync: enableAutoSync }
  })
}
export function clearDeviceData() {
  return apiRequest<void>('/api/v1/devices', { method: 'DELETE' })
}

export function getDeviceOpenWrtConfig(id: string) {
  return apiRequest<DeviceOpenWrtConfig>('/api/v1/devices/' + encodeURIComponent(id) + '/openwrt')
}
export function saveDeviceOpenWrtConfig(id: string, payload: {
  rpcd_port: number
  https: boolean
  username: string
  password: string
}) {
  return apiRequest<DeviceOpenWrtConfig>('/api/v1/devices/' + encodeURIComponent(id) + '/openwrt', {
    method: 'PUT', body: payload
  })
}
export function clearDeviceOpenWrtConfig(id: string) {
  return apiRequest<void>('/api/v1/devices/' + encodeURIComponent(id) + '/openwrt', { method: 'DELETE' })
}

function discoveryPath(id: string, kind: OpenWrtDiscoveryKind) {
  return '/api/v1/devices/' + encodeURIComponent(id) + '/openwrt/discovery/' + kind
}

export function getOpenWrtDiscoverySnapshot(id: string, kind: OpenWrtDiscoveryKind) {
  return apiRequest<OpenWrtDiscoverySnapshot>(discoveryPath(id, kind), { timeoutMs: 40_000 })
}

export function scanOpenWrtDiscovery(id: string, kind: OpenWrtDiscoveryKind) {
  return apiRequest<OpenWrtDiscoverySnapshot>(discoveryPath(id, kind), { method: 'POST', timeoutMs: 40_000 })
}
