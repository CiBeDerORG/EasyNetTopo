export type DeviceGroup = {
  id: string
  name: string
  device_count: number
  created_at: string
  updated_at: string
}

export type DeviceType = 'router' | 'computer' | 'phone' | 'tablet' | 'tv' | 'nas' | 'other'

export type Device = {
  id: string
  name: string
  group_id?: string
  group_name?: string
  device_type: DeviceType
  manufacturer?: string
  model?: string
  ip_address?: string
  mac_address?: string
  note?: string
  is_openwrt: boolean
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
  device_type: DeviceType
  manufacturer: string
  model: string
  ip_address: string
  mac_address: string
  note: string
  is_openwrt: boolean
  network_key: string
  peer_id: string
}

export type DeviceOpenWrtConfig = {
  device_id: string
  configured: boolean
  rpcd_port: number
  https: boolean
  username: string
}
