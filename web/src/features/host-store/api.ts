import { apiRequest } from '../../lib/api-client'
import type { AppAction, AppActionResponse, ExecutionTask, AppConfig, AppRef, AppStatus, AuthStatus, CatalogApp, CatalogSource, ControllerInfo, DockerInventory, Host, HostSnapshot } from './types'

export function getAuthStatus() {
  return apiRequest<AuthStatus>('/api/v1/auth/status')
}
export function login(username: string, password: string) {
  return apiRequest<{ username: string }>('/api/v1/auth/login', { method: 'POST', body: { username, password } })
}
export function logout() {
  return apiRequest<void>('/api/v1/auth/logout', { method: 'POST' })
}
export function getControllerInfo() {
  return apiRequest<ControllerInfo>('/api/v1/system/info')
}
export async function getHosts() {
  const result = await apiRequest<{ hosts: Host[] }>('/api/v1/hosts')
  return result.hosts || []
}
export function createHost(payload: { name: string; endpoint: string }) {
  return apiRequest<Host>('/api/v1/hosts', { method: 'POST', body: payload })
}
export function updateHost(id: string, payload: { name: string; endpoint: string }) {
  return apiRequest<Host>('/api/v1/hosts/' + encodeURIComponent(id), { method: 'PATCH', body: payload })
}
export function deleteHost(id: string) {
  return apiRequest<void>('/api/v1/hosts/' + encodeURIComponent(id), { method: 'DELETE' })
}
export function pairHost(id: string, payload: { agent_id: string; spki_sha256: string; pairing_code: string }) {
  return apiRequest<Host>('/api/v1/hosts/' + encodeURIComponent(id) + '/pair', { method: 'POST', body: payload })
}
export function queueNativeAgentUpdate(id: string, action: 'update' | 'force-update') {
  return apiRequest<{ state: 'queued'; action: string }>(
    '/api/v1/hosts/' + encodeURIComponent(id) + '/agent/' + action,
    { method: 'POST' }
  )
}
export function getHostSnapshot(id: string) {
  return apiRequest<HostSnapshot>('/api/v1/hosts/' + encodeURIComponent(id) + '/snapshot')
}
export function getDockerInventory(id: string) {
  return apiRequest<DockerInventory>('/api/v1/hosts/' + encodeURIComponent(id) + '/docker')
}
export async function getSources() {
  const result = await apiRequest<{ sources: CatalogSource[] }>('/api/v1/catalog/sources')
  return result.sources || []
}
export function createSource(payload: { name: string; url: string; branch: string }) {
  return apiRequest<CatalogSource>('/api/v1/catalog/sources', { method: 'POST', body: payload })
}
export function deleteSource(id: string) {
  return apiRequest<void>('/api/v1/catalog/sources/' + encodeURIComponent(id), { method: 'DELETE' })
}
export function refreshSource(id: string) {
  return apiRequest<{ source_id: string; revision: string; app_count: number }>(
    '/api/v1/catalog/sources/' + encodeURIComponent(id) + '/refresh', { method: 'POST' }
  )
}
export async function getApps() {
  const result = await apiRequest<{ apps: CatalogApp[]; install_enabled: boolean }>('/api/v1/catalog/apps')
  return result.apps || []
}
export function getAppConfig(app: AppRef) {
  return apiRequest<AppConfig>(
    '/api/v1/catalog/apps/' + encodeURIComponent(app.source_id) + '/' +
      encodeURIComponent(app.app_id) + '/config'
  )
}
export async function getAppStatuses(hostId: string, apps: AppRef[]) {
  const refs = apps.map(app => ({ source_id: app.source_id, app_id: app.app_id }))
  const result: AppStatus[] = []
  const total = Math.max(refs.length, 1)
  for (let start = 0; start < total; start += 256) {
    const response = await apiRequest<{ apps: AppStatus[] }>(
      '/api/v1/hosts/' + encodeURIComponent(hostId) + '/apps/status',
      { method: 'POST', body: { apps: refs.slice(start, start + 256), include_managed: start === 0 } }
    )
    result.push(...(response.apps || []))
  }
  return result
}
export function executeAppAction(hostId: string, action: AppAction, payload: AppRef & {
  revision?: string
  config?: Record<string, string>
}) {
  return apiRequest<AppActionResponse>(
    '/api/v1/hosts/' + encodeURIComponent(hostId) + '/apps/' + action,
    { method: 'POST', body: payload }
  )
}

export async function getExecutionTasks() {
  const response = await apiRequest<{ tasks: ExecutionTask[] }>('/api/v1/tasks')
  return response.tasks || []
}

export function resolveInterruptedExecutionTask(id: string) {
  return apiRequest<{ resolved: boolean; task_id: string; state: 'succeeded' | 'failed' }>(
    '/api/v1/tasks/' + encodeURIComponent(id) + '/resolve',
    { method: 'POST', body: { confirm_stopped: true } }
  )
}
