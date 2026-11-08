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
  agent_started_at?: string
  agent_commit?: string
  agent_version?: string
  hostname: string
  platform: string
  arch: string
  software_root: string
  observed_at: string
  docker?: { available?: boolean; status?: string }
}
export type DockerPort = {
  protocol: 'tcp' | 'udp' | string
  host_ip?: string
  host_port: number
  container_port: number
  is_web?: boolean
}
export type DockerContainer = {
  container_id: string
  name: string
  image?: string
  state: string
  ports?: DockerPort[]
}
export type DockerResource = {
  resource_id: string
  name: string
  state: string
  kind: string
  managed: boolean
  compose_project?: string
  ports?: DockerPort[]
  containers?: DockerContainer[]
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
  icon_url?: string
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
export type AgentUpdateAction = 'update' | 'force-update'
export type AgentMaintenanceState = { state: 'waiting' | 'timeout'; action: AgentUpdateAction }
export type AppAction = 'install' | 'update' | 'uninstall' | 'adopt' | 'force_update'
export type ConfigField = {
  custom_input?: boolean
  pattern?: string
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
export type AppActionResponse = { action?: string; result?: string; message?: string; accepted?: boolean; tasks?: ExecutionTask[] }
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

export type ExecutionTask = {
  id: string
  parent_id?: string
  type: 'host' | 'application'
  host_id: string
  source_id?: string
  app_id?: string
  action: string
  trigger_source: string
  state: 'queued' | 'preparing' | 'running' | 'succeeded' | 'failed' | 'interrupted'
  message?: string
  created_at: string
  started_at?: string
  finished_at?: string
}
