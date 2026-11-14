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
  node_hostname?: string
  openwrt_configured: boolean
  openwrt_rpcd_port?: number
  openwrt_https?: boolean
  openwrt_username?: string
  discovery_status: 'unknown' | 'online' | 'offline'
  discovery_last_seen_at?: string
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
  node_hostname?: string
}

export type NetworkNodeDeviceInput = {
  name: string
  network_key: string
  peer_id: string
  hostname: string
}

export type NetworkNodeSyncStatus = { enabled: boolean }
export type NetworkNodeSyncResult = { created: number; existing: number; rebound: number; skipped: number; enabled: boolean }

export type DeviceOpenWrtConfig = {
  device_id: string
  configured: boolean
  rpcd_port: number
  https: boolean
  username: string
}

export type OpenWrtDiscoveryKind = 'devices' | 'adb'

export type OpenWrtDiscoveryDevice = {
  id?: string
  name?: string
  hostname?: string | null
  mac?: string | null
  ipv4?: string[]
  ipv6?: string[]
  iface?: string
  network?: string
  vendor?: string | null
  model?: string | null
  observation?: 'recent' | 'lease' | 'cached'
  last_seen?: number | null
  instance?: string
  protocol?: string
  service?: string
  host?: string
  address?: string
  port?: number
}

export type OpenWrtDiscoverySnapshot = {
  status: 'ok' | 'partial' | 'error'
  has_snapshot?: boolean
  scanned_at?: number | null
  last_attempt_at?: number | null
  source?: string
  count?: number
  truncated?: boolean
  message?: string
  errors?: Array<{ source?: string; code?: string; message?: string }>
  devices?: OpenWrtDiscoveryDevice[]
  import?: {
    created: number
    existing: number
    skipped: number
  }
}
