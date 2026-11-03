export type AuthStatus = { initialized: boolean; authenticated: boolean }
export type ControllerInfo = { controller_id: string }

export type Host = {
  id: string
  name: string
  endpoint: string
  agent_id: string
  spki_sha256: string
  paired_at?: string
}
export type HostHealthStatus = 'online' | 'checking' | 'error' | 'paired' | 'unpaired'

export type HostSnapshot = {
  agent_id: string
  hostname: string
  platform: string
  arch: string
  software_root: string
  observed_at: string
  docker?: { available?: boolean; status?: string }
}
export type DockerResource = {
  resource_id: string
  name: string
  state: string
  kind: string
  managed: boolean
}
export type DockerInventory = {
  agent_id: string
  observed_at: string
  status: string
  complete: boolean
  resources: DockerResource[]
  error?: string
}
export type CatalogSource = {
  id: string
  name: string
  url: string
  branch: string
  current_revision: string
  last_error?: string
  updated_at?: string
}
export type CatalogApp = {
  source_id: string
  app_id: string
  name: string
  summary: string
  category: string
  revision: string
  manifest_sha256: string
}
export type AppRef = { source_id: string; app_id: string }
export type AppStatus = AppRef & {
  managed: boolean
  allowed: boolean
  external?: boolean
  adoptable?: boolean
  force_updatable?: boolean
  mode?: string
  blocked_reason?: string
  state: string
  action?: string
  result?: string
  message?: string
}
export type AppAction = 'install' | 'update' | 'uninstall' | 'adopt' | 'force_update'
export type ConfigField = {
  key: string
  type: string
  label: string
  description?: string
  required: boolean
  editable?: boolean
  default?: string | boolean | number
  minimum?: number
  maximum?: number
  min_length?: number
  max_length?: number
}
export type AppConfig = { revision: string; config: ConfigField[] }
export type AppActionResponse = { action?: string; result?: string; message?: string }
export type AppPermissions = {
  showInstall: boolean
  canInstall: boolean
  showUpdate: boolean
  canUpdate: boolean
  showUninstall: boolean
  canUninstall: boolean
  showAdopt: boolean
  canAdopt: boolean
  showForceUpdate: boolean
  canForceUpdate: boolean
  grantRequired: boolean
  blockReason: string
}
