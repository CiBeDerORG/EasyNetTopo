import { apiRequest } from '../../lib/api-client'
import type {
  Device, DeviceGroup, DeviceInput, DeviceOpenWrtConfig,
  NetworkNodeDeviceInput, NetworkNodeSyncResult, NetworkNodeSyncStatus
} from './types'

export async function getDevices() {
  const result = await apiRequest<{ devices: Device[] }>('/api/v1/devices')
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

export function getNetworkNodeSyncStatus() {
  return apiRequest<NetworkNodeSyncStatus>('/api/v1/devices/network-nodes/sync')
}
export function syncNetworkNodeDevices(nodes: NetworkNodeDeviceInput[]) {
  return apiRequest<NetworkNodeSyncResult>('/api/v1/devices/network-nodes/sync', { method: 'POST', body: { nodes } })
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
