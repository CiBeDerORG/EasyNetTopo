export type DeviceGroup = {
  id: string
  name: string
  device_count: number
  created_at: string
  updated_at: string
}

export type GatewayGroup = {
  id: string
  name: string
  network_key: string
  peer_id: string
  device_count: number
  gateway_device_id?: string
  gateway_device_name?: string
  created_at: string
  updated_at: string
}

export type Device = {
  id: string
  name: string
  group_id?: string
  group_name?: string
  gateway_group_id?: string
  gateway_group_name?: string
  owned_gateway_group_id?: string
  semantic_kinds: string[]
  is_node_device: boolean
  is_gateway: boolean
  manufacturer?: string
  model?: string
  ip_address?: string
  mac_address?: string
  note?: string
  network_key?: string
  peer_id?: string
  openwrt_configured: boolean
  openwrt_rpcd_port?: number
  openwrt_https?: boolean
  openwrt_username?: string
  created_at: string
  updated_at: string
}

export type DeviceInput = {
  name: string
  group_id: string
  gateway_group_id: string
  semantic_kinds: string[]
  is_gateway: boolean
  manufacturer: string
  model: string
  ip_address: string
  mac_address: string
  note: string
  network_key: string
  peer_id: string
}

export type NetworkNodeDeviceInput = {
  name: string
  network_key: string
  peer_id: string
}

export type NetworkNodeSyncStatus = { enabled: boolean }
export type NetworkNodeSyncResult = { created: number; existing: number; skipped: number; enabled: boolean }

export type DeviceOpenWrtConfig = {
  device_id: string
  configured: boolean
  rpcd_port: number
  https: boolean
  username: string
}
