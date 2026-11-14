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
  network_ip?: string
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

export type OpenWrtNetworkIPHostDiagnostic = {
  host_id: string
  host_name: string
  status: 'matched' | 'no_match' | 'failed' | 'cancelled' | 'ambiguous' | string
  message?: string
  instance_count: number
  matching_network_instances: number
  peer_route_count: number
  matching_peer_routes: number
  proxy_cidr_count: number
  valid_mapping_count: number
  observed_network_keys?: string[]
  observed_peer_ids?: string[]
  match_location?: 'node_info' | 'peer_routes' | string
  mapping_rules?: string[]
}

export type OpenWrtNetworkIPDiagnostics = {
  target_network_key: string
  target_peer_id: string
  network_entries: number
  hosts_queried: number
  hosts_succeeded: number
  hosts_failed: number
  hosts_matched: number
  available: boolean
  match_location?: string
  mapping_rules?: string[]
  reason?: string
  hosts: OpenWrtNetworkIPHostDiagnostic[]
}

export type OpenWrtDeviceSyncDiagnostic = {
  device_id?: string
  name?: string
  source?: string
  observation?: string
  ip_address?: string
  mac_address?: string
  matched_by?: string
  result: 'created' | 'existing' | 'skipped' | string
  reason?: string
  previous_ip_address?: string
  current_ip_address?: string
  previous_network_ip?: string
  current_network_ip?: string
  network_ip_action?: string
  network_ip_mapping_status?: string
  address_fields_updated: boolean
  database_updated: boolean
  discovery_status_updated: boolean
}

export type OpenWrtDiscoveryDiagnostics = {
  generated_at: string
  gateway: {
    device_id?: string
    name?: string
    ip_address?: string
    network_key?: string
    peer_id?: string
  }
  snapshot: {
    status: string
    has_snapshot: boolean
    truncated: boolean
    source?: string
    count: number
    device_count: number
    can_import: boolean
    message?: string
  }
  route: OpenWrtNetworkIPDiagnostics
  import: {
    created: number
    existing: number
    skipped: number
    network_ip_written: number
    network_ip_cleared: number
    network_ip_unchanged: number
    network_ip_unavailable: number
    network_ip_unmapped: number
    devices: OpenWrtDeviceSyncDiagnostic[]
  }
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
  diagnostics?: OpenWrtDiscoveryDiagnostics
}
