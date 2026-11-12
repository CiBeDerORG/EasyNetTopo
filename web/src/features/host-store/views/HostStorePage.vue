<script setup lang="ts">
import { useRoute } from 'vue-router'
import NetworkNodesPane from '../../network-nodes/NetworkNodesPane.vue'
import NetworkMapPane from '../../network-map/NetworkMapPane.vue'
import DevicesPane from '../../devices/DevicesPane.vue'
import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, SwitchButton } from '@element-plus/icons-vue'
import { formatApiError } from '../../../lib/api-client'
import { clearDeviceData as clearDeviceDataApi } from '../../devices/api'
import { clearNodeAttributes } from '../../network-nodes/attributes'
import { useHostStoreUiStore } from '../../../stores/ui'
import { queryClient as appQueryClient } from '../../../app/query-client'
import {
  createHost,
  createSource,
  deleteHost as deleteHostApi,
  deleteSource,
  executeAppAction,
  getAppConfig,
  getAgentDiscovery,
  openAgentDiscovery,
  closeAgentDiscovery,
  acceptDiscoveredAgent,
  getAuthStatus,
  getControllerInfo,
  login,
  logout as logoutApi,
  pairHost as pairHostApi,
  preinstallTemporary,
  revokeTemporaryHost,
  queueNativeAgentUpdate,
  refreshSource,
  resolveInterruptedExecutionTask,
  updateHost,
  setNetworkEntry
} from '../api'
import { appKey, canRecoverFailedUninstall, getAppPermissions } from '../rules'
import { useHostStoreData } from '../composables/useHostStoreData'
import { currentAppTask, taskActive } from '../task-status'
import SidebarNav from '../components/SidebarNav.vue'
import HostListPane from '../components/HostListPane.vue'
import StorePane from '../components/StorePane.vue'
import AddHostDialog from '../components/AddHostDialog.vue'
import HostEditorDialog from '../components/HostEditorDialog.vue'
import SourceSettingsDialog from '../components/SourceSettingsDialog.vue'
import DockerResourceDrawer from '../components/DockerResourceDrawer.vue'
import AppConfigDialog from '../components/AppConfigDialog.vue'
import type {
  AgentUpdateAction,
  AppAction,
  AppConfig,
  CatalogApp,
  CatalogSource,
  ExecutionTask,
  Host,
  HostHealthStatus,
  AgentDiscoveryState,
  DiscoveredAgent
} from '../types'

const route = useRoute()
const isNetworkPage = computed(() => route.path === '/network-nodes')
const isDevicesPage = computed(() => route.path === '/devices')
const isMapPage = computed(() => route.path === '/network-map')
const isStorePage = computed(() => !isNetworkPage.value && !isDevicesPage.value && !isMapPage.value)
const ui = useHostStoreUiStore()
const queryClient = useQueryClient()
const authQuery = useQuery({
  queryKey: ['auth-status'],
  queryFn: getAuthStatus,
  retry: 0,
  refetchOnWindowFocus: false
})
const authenticated = computed(() => authQuery.data.value?.authenticated ?? false)
const initialized = computed(() => authQuery.data.value?.initialized ?? false)
const authLoading = computed(() => authQuery.isPending.value)
const authError = computed(() => authQuery.isError.value ? errorMessage(authQuery.error.value) : '')

const loginForm = reactive({ username: 'admin', password: '' })
const loginMutation = useMutation({
  mutationFn: () => login(loginForm.username.trim(), loginForm.password),
  onSuccess: async () => {
    loginForm.password = ''
    await authQuery.refetch()
    ElMessage.success('登录成功')
  },
  onError: error => ElMessage.error(errorMessage(error))
})
// Vue 模板对普通对象内的 Ref 不自动解包，先转换为顶层布尔响应式状态。
const loginPending = computed(() => loginMutation.isPending.value)
const controllerQuery = useQuery({
  queryKey: ['controller-info'],
  queryFn: getControllerInfo,
  enabled: authenticated,
  retry: 0
})
const controllerId = computed(() => controllerQuery.data.value?.controller_id || '')
const discoveryOpen = ref(false)
const discoveryLoading = ref(false)
const discoverySaving = ref(false)
const discoveryState = ref<AgentDiscoveryState>({ enabled: false })
const discoveredAgents = ref<DiscoveredAgent[]>([])
const discoveryToken = ref('')
let discoveryTimer: number | undefined
watch(discoveryOpen, open => {
  if (discoveryTimer !== undefined) {
    window.clearInterval(discoveryTimer)
    discoveryTimer = undefined
  }
  if (open) discoveryTimer = window.setInterval(() => { void refreshDiscovery() }, 5000)
})
onUnmounted(() => {
  if (discoveryTimer !== undefined) window.clearInterval(discoveryTimer)
})
async function refreshDiscovery() {
  if (discoveryLoading.value) return
  discoveryLoading.value = true
  try {
    const result = await getAgentDiscovery()
    discoveryState.value = result.discovery || { enabled: false }
    discoveredAgents.value = result.discoveries || []
  } catch (error) {
    ElMessage.error('读取主机发现状态失败：' + errorMessage(error))
  } finally {
    discoveryLoading.value = false
  }
}
async function showDiscovery() {
  discoveryOpen.value = true
  await refreshDiscovery()
}
async function startDiscovery() {
  if (discoverySaving.value) return
  discoverySaving.value = true
  try {
    const result = await openAgentDiscovery()
    discoveryState.value = result.discovery
    discoveryToken.value = result.bootstrap_token
    await refreshDiscovery()
    ElMessage.success('主机发现已开启，有效期 30 分钟。请将接入凭据配置到内网 EasyHostAgent。')
  } catch (error) {
    ElMessage.error('开启主机发现失败：' + errorMessage(error))
  } finally {
    discoverySaving.value = false
  }
}
async function stopDiscovery() {
  if (discoverySaving.value) return
  discoverySaving.value = true
  try {
    await closeAgentDiscovery()
    discoveryState.value = { enabled: false }
    ElMessage.success('已停止接收新的内网主机发现请求。')
  } catch (error) {
    ElMessage.error('关闭主机发现失败：' + errorMessage(error))
  } finally {
    discoverySaving.value = false
  }
}
async function acceptDiscovery(item: DiscoveredAgent) {
  if (discoverySaving.value) return
  discoverySaving.value = true
  try {
    const host = await acceptDiscoveredAgent(item.agent_id)
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
    await data.hostsQuery.refetch()
    ui.selectHost(host.id)
    await refreshDiscovery()
    ElMessage.success('已加入临时主机，有效期固定为 30 分钟。')
  } catch (error) {
    ElMessage.error('加入临时主机失败：' + errorMessage(error))
  } finally {
    discoverySaving.value = false
  }
}
async function copyDiscoveryToken() {
  if (!discoveryToken.value) return
  try {
    await navigator.clipboard.writeText(discoveryToken.value)
    ElMessage.success('接入凭据已复制。')
  } catch {
    ElMessage.warning('复制失败，请手动选择凭据复制。')
  }
}
function changeThemeMode(value: string) {
  if (value === 'system' || value === 'light' || value === 'dark') ui.setThemeMode(value)
}

const selectedHostId = computed(() => ui.selectedHostId)
const pendingAction = ref<{
  hostId: string
  appKey: string
  action: AppAction
  appName: string
  errors: number
} | null>(null)
const shouldPoll = computed(() => !!pendingAction.value)
const dockerOpen = ref(false)
const agentMaintenance = ref<{
  hostId: string; action: AgentUpdateAction; state: 'waiting' | 'timeout'; startedAt: string
} | null>(null)
const agentUpdating = computed(() => agentMaintenance.value?.hostId === selectedHostId.value &&
  agentMaintenance.value.state === 'waiting')
const agentMaintenanceForHost = computed(() => {
  const job = agentMaintenance.value
  return job?.hostId === selectedHostId.value ? { state: job.state, action: job.action } : null
})
const agentUpdateBusy = ref(false)
let upgradeDeadline: ReturnType<typeof setTimeout> | null = null
const data = useHostStoreData(selectedHostId, authenticated, shouldPoll, agentUpdating, isStorePage)

const hosts = data.hosts
const executionTasks = data.tasks
const sources = data.sources
const selectedHost = data.selectedHost
const rows = data.rows
const visibleApps = data.visibleApps
const snapshot = computed(() => data.snapshotQuery.data.value || null)
const snapshotLoading = computed(() => data.snapshotQuery.isFetching.value)
const snapshotError = computed(() =>
  data.snapshotQuery.isError.value ? errorMessage(data.snapshotQuery.error.value) : ''
)
const statusError = computed(() =>
  data.statusQuery.isError.value ? errorMessage(data.statusQuery.error.value) : ''
)
const marketLoading = computed(() => data.statusQuery.isFetching.value || data.statusQuery.isPending.value)
const dockerInventory = computed(() => data.dockerQuery.data.value)
const dockerLoading = computed(() => data.dockerQuery.isFetching.value)
const dockerError = computed(() =>
  data.dockerQuery.isError.value ? errorMessage(data.dockerQuery.error.value) : ''
)
const healthStatus = computed<HostHealthStatus>(() => {
  if (selectedHost.value?.host_type === 'temporary') {
    const seen = selectedHost.value.last_seen_at ? new Date(selectedHost.value.last_seen_at).getTime() : 0
    return seen && Date.now() - seen < 60_000 ? 'online' : 'error'
  }
  if (!selectedHost.value?.agent_id) return 'unpaired'
  if (agentUpdating.value) return 'checking'
  if (agentMaintenanceForHost.value?.state === 'timeout') return 'error'
  if (snapshotLoading.value) return 'checking'
  if (snapshotError.value) return 'error'
  if (snapshot.value) return 'online'
  return 'paired'
})
const refreshing = computed(() =>
  data.hostsQuery.isFetching.value || data.snapshotQuery.isFetching.value ||
  data.statusQuery.isFetching.value
)
const pendingKey = computed(() => pendingAction.value?.appKey || '')

// 仅保存当前浏览器已核实无资源的卡片显示偏好，不删除服务端任务审计记录。
const recoveredStorageKey = 'easynettopo-recovered-uninstall-cards'
function loadRecoveredUninstalls(): string[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(recoveredStorageKey) || '[]')
    return Array.isArray(value) ? value.filter((item): item is string =>
      typeof item === 'string' && item.length <= 300).slice(-256) : []
  } catch { return [] }
}
const recoveredUninstallCards = ref<string[]>(loadRecoveredUninstalls())
function setRecovered(hostId: string, key: string, enabled: boolean) {
  const marker = hostId + '|' + key
  const next = recoveredUninstallCards.value.filter(item => item !== marker)
  if (enabled) next.push(marker)
  recoveredUninstallCards.value = next.slice(-256)
  try { localStorage.setItem(recoveredStorageKey, JSON.stringify(recoveredUninstallCards.value)) }
  catch { /* 本地存储不可用时保留本次会话状态 */ }
}
async function recoverFailedUninstall(hostId: string, app: CatalogApp): Promise<boolean> {
  const [statusResult, dockerResult] = await Promise.allSettled([
    data.statusQuery.refetch(), data.dockerQuery.refetch()
  ])
  if (hostId !== selectedHostId.value ||
      statusResult.status !== 'fulfilled' || statusResult.value.isError ||
      dockerResult.status !== 'fulfilled' || dockerResult.value.isError) return false
  const row = statusResult.value.data?.find(item => appKey(item) === appKey(app))
  const inventory = dockerResult.value.data
  const task = currentAppTask(data.tasks.value, hostId, app.source_id, app.app_id)
  if (task && (taskActive(task) || task.state === 'interrupted')) return false
  if (!canRecoverFailedUninstall(row, inventory, app.app_id)) return false
  setRecovered(hostId, appKey(app), true)
  return true
}

const addHostOpen = ref(false)
const addHostSaving = ref(false)
const editorOpen = ref(false)
const editorHost = ref<Host | null>(null)
const editorSaving = ref(false)
const pairing = ref(false)
const sourceSettingsOpen = ref(false)
const sourceSaving = ref(false)
const busySourceId = ref('')
const deletingSourceId = ref('')
const actionBusyKey = ref('')
const configBusy = ref(false)
const dataManagementDialogOpen = ref(false)
const dataClearBusy = ref<'' | 'nodes' | 'devices'>('')
const configDialogOpen = ref(false)
const configState = ref<{ app: CatalogApp; config: AppConfig } | null>(null)

const configApp = computed(() => configState.value?.app || null)
const configRevision = computed(() => configState.value?.config.revision || '')
const configFields = computed(() => configState.value?.config.config || [])

// 仅收到原生 Agent 重启后的新进程标识，才认为主机重新连通。
// 未观察到重启则持续等待，超时只显示故障排查信息，不误报更新成功。
watch(() => data.snapshotQuery.dataUpdatedAt.value, () => {
  const job = agentMaintenance.value
  if (!job || job.state !== 'waiting' || job.hostId !== selectedHostId.value ||
    data.snapshotQuery.isError.value) return
  const started = data.snapshotQuery.data.value?.agent_started_at
  if (!started || started === job.startedAt) return
  if (upgradeDeadline) clearTimeout(upgradeDeadline)
  upgradeDeadline = null
  agentMaintenance.value = null
  ElMessage.info('EasyHostAgent 已重启并恢复连接；请核对发布版本及主机日志确认更新结果。')
})

async function requestAgentUpdate(action: AgentUpdateAction) {
  const host = selectedHost.value
  const startedAt = snapshot.value?.agent_started_at
  if (!host?.agent_id || !startedAt || snapshotError.value || agentUpdateBusy.value ||
    agentMaintenanceForHost.value?.state === 'waiting') {
    ElMessage.warning('需先连接支持远程更新的 EasyHostAgent，并确认主机快照可用。')
    return
  }
  try {
    await ElMessageBox.confirm(
      '即将在主机「' + host.name + '」上执行 EasyHostAgent ' +
      (action === 'force-update' ? '强制更新（重新下载并校验所配置 Release）' : '更新') +
      '。系统会延迟启动独立更新任务并重启 Agent，期间短暂断联属正常现象。超过 10 分钟仍未观察到重启并重新连接，页面仅提示开发人员检查日志。是否继续？',
      '确认更新 EasyHostAgent',
      { type: 'warning', confirmButtonText: '发送更新指令', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  agentUpdateBusy.value = true
  try {
    const response = await queueNativeAgentUpdate(host.id, action)
    if (response.state !== 'queued') throw new Error('更新任务未得到 Agent 确认')
    if (upgradeDeadline) clearTimeout(upgradeDeadline)
    agentMaintenance.value = { hostId: host.id, action, state: 'waiting', startedAt }
    upgradeDeadline = setTimeout(() => {
      if (agentMaintenance.value?.hostId === host.id &&
        agentMaintenance.value.state === 'waiting') {
        agentMaintenance.value = { ...agentMaintenance.value, state: 'timeout' }
      }
      upgradeDeadline = null
    }, 10 * 60 * 1000)
    ElMessage.info('更新指令已受理。Agent 将自动重启，短暂断联不视为失败。')
  } catch (error) {
    ElMessage.error('发送 EasyHostAgent 更新指令失败：' + errorMessage(error))
  } finally {
    agentUpdateBusy.value = false
  }
}

onUnmounted(() => {
  if (upgradeDeadline) clearTimeout(upgradeDeadline)
})


// 仅自动关联来自可信固定 Git 清单、且 Compose 路径和脚本通过 Agent 核验的原有部署。
const autoLinkAttempts = new Set<string>()
const autoLinking = ref(false)
watch(() => data.statusQuery.data.value, async rows => {
  const host = selectedHost.value
  if (!rows?.length || !host?.agent_id || autoLinking.value || pendingAction.value || actionBusyKey.value || agentUpdating.value) return
  const apps = new Map(data.catalogApps.value.map(app => [appKey(app), app]))
  const candidates = rows.filter(row => row.external && row.adoptable &&
    row.blocked_reason === 'adoption_required')
  if (!candidates.length) return
  autoLinking.value = true
  let updated = false
  try {
    for (const row of candidates) {
      if (selectedHostId.value !== host.id) break
      const app = apps.get(appKey(row))
      if (!app?.revision) continue
      const key = host.id + ':' + appKey(app) + ':' + app.revision
      if (autoLinkAttempts.has(key)) continue
      autoLinkAttempts.add(key)
      try {
        await executeAppAction(host.id, 'adopt', {
          source_id: app.source_id, app_id: app.app_id, revision: app.revision
        })
        updated = true
      } catch (error) {
        if (selectedHostId.value === host.id) {
          ElMessage.warning(app.name + ' 自动关联失败：' + errorMessage(error))
        }
      }
    }
  } finally {
    autoLinking.value = false
    if (updated && selectedHostId.value === host.id) await data.statusQuery.refetch()
  }
}, { immediate: true })


watch(hosts, items => {
  if (!items.some(host => host.id === ui.selectedHostId)) {
    ui.selectHost(items[0]?.id || '')
  }
}, { immediate: true })

watch(selectedHostId, () => {
  pendingAction.value = null
  dockerOpen.value = false
  configDialogOpen.value = false
})

watch(() => data.statusQuery.data.value, async result => {
  const job = pendingAction.value
  if (!job || job.hostId !== selectedHostId.value || !result) return
  const row = result.find(item => appKey(item) === job.appKey)
  const expectedAction = job.action === 'force_update' ? 'update' : job.action
  if (!row || row.action !== expectedAction || !row.result || row.result === 'running') return

  pendingAction.value = null
  // 安装/更新脚本执行结束后重新采集 Docker 端口映射，供卡片和资源抽屉共用。
  void data.dockerQuery.refetch()
  if (row.result === 'failed') {
    const app = data.catalogApps.value.find(item => appKey(item) === job.appKey)
    const restored = job.action === 'uninstall' && app
      ? await recoverFailedUninstall(job.hostId, app) : false
    if (restored) ElMessage.warning(job.appName + ' 卸载失败，但确认远端已无应用资源；卡片已恢复待安装状态')
    else ElMessage.error(job.appName + ' ' + actionName(job.action) + '失败：' +
      (row.message || '请检查 Agent 执行状态') +
      (job.action === 'uninstall' ? '；仍有资源或状态未核实，已保留实际安装信息' : ''))
  } else if (row.result === 'unknown') {
    ElMessage.warning(job.appName + ' 操作结果未知，请核对目标主机实际状态')
  } else if (row.result === 'succeeded' &&
    (job.action === 'uninstall' ? row.state === 'not_installed' : row.state === 'running')) {
    ElMessage.success(job.appName + ' ' + actionName(job.action) + '成功')
  } else {
    ElMessage.warning(job.appName + ' 脚本执行完成，但当前状态为「' + stateName(row.state) + '」，请核查')
  }
})

watch(() => data.statusQuery.errorUpdatedAt.value, (updatedAt, previous) => {
  const job = pendingAction.value
  if (!updatedAt || updatedAt === previous || !job) return
  job.errors += 1
  if (job.errors >= 3) {
    pendingAction.value = null
    ElMessage.warning('无法获取操作最终结果。恢复主机连接后请刷新状态，不能据此判断操作成功或失败。')
  }
})

function errorMessage(error: unknown) {
  return formatApiError(error)
}
function actionName(action: AppAction) {
  return ({ install: '安装', update: '更新', uninstall: '卸载', adopt: '接管', force_update: '强制更新' } as Record<AppAction, string>)[action]
}
function stateName(state: string) {
  return ({
    running: '运行中',
    stopped: '已停止',
    partial: '部分运行',
    not_installed: '未安装',
    unknown: '未知',
    missing: '容器缺失',
    external: '外部项目'
  } as Record<string, string>)[state] || state
}

async function resolveTask(task: ExecutionTask) {
  if (task.state !== 'interrupted') return
  const nativeAgent = task.source_id === 'native' && task.app_id === 'easyhostagent'
  try {
    await ElMessageBox.confirm(
      nativeAgent
        ? '请先确认目标主机的原生更新任务已经停止。系统将通过已配对 Agent 的启动标识验证是否在本次任务后重启：已重启则标记成功；否则按人工核实关闭为失败，并解除后续任务阻塞。'
        : '请先在目标主机检查 Docker / Agent，确认本次更新脚本确实已经停止。此操作只将无法确认的任务标记为失败，不代表更新成功，并会允许后续排队任务继续执行。',
      '确认解除主机任务阻塞',
      { type: 'warning', confirmButtonText: '已核实，解除阻塞', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  try {
    const result = await resolveInterruptedExecutionTask(task.id)
    await data.tasksQuery.refetch()
    if (result.state === 'succeeded') {
      ElMessage.success('已确认 Agent 重新启动并恢复连接，更新任务标记为成功，主机执行锁已释放。')
    } else {
      ElMessage.success('已关闭中断任务，后续队列可以继续执行。')
    }
  } catch (error) {
    ElMessage.error('解除任务阻塞失败：' + errorMessage(error))
  }
}

async function refreshAll() {
  autoLinkAttempts.clear()
  const tasks: Promise<unknown>[] = [data.hostsQuery.refetch(), data.tasksQuery.refetch()]
  if (selectedHost.value?.agent_id && selectedHost.value.connection_mode !== 'reverse') {
    tasks.push(data.snapshotQuery.refetch(), data.statusQuery.refetch())
  }
  await Promise.allSettled(tasks)
}

async function openEditor(host: Host) {
  // 编辑前刷新主机记录，避免用过期缓存判断 Agent 地址是否已保存。
  try {
    await data.hostsQuery.refetch()
  } catch {
    // 刷新失败时仍可编辑当前记录；保存与配对仍由服务端校验。
  }
  editorHost.value = data.hosts.value.find(item => item.id === host.id) || host
  editorOpen.value = true
}
async function openDocker(host: Host) {
  ui.selectHost(host.id)
  // 先让切换主机的清理逻辑完成，避免旧主机状态将新打开的抽屉立即关闭。
  await nextTick()
  dockerOpen.value = true
}

async function submitNewHost(payload: { name: string; endpoint: string }) {
  addHostSaving.value = true
  try {
    const host = await createHost(payload)
    await data.hostsQuery.refetch()
    ui.selectHost(host.id)
    addHostOpen.value = false
    editorHost.value = host
    editorOpen.value = true
    ElMessage.success('主机登记成功。请继续核验 Agent 身份并完成配对。')
  } catch (error) {
    ElMessage.error('添加主机失败：' + errorMessage(error))
  } finally {
    addHostSaving.value = false
  }
}

const networkSaving = ref(false)
async function toggleNetworkEntry(host: Host) {
  if (networkSaving.value) return
  networkSaving.value = true
  try {
    await setNetworkEntry(host.id, !host.network_entry)
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
    ElMessage.success(host.network_entry ? '已取消组网入口' : '已设为组网入口，可在网络节点菜单查看数据')
  } catch (error) { ElMessage.error(errorMessage(error)) }
  finally { networkSaving.value = false }
}

async function saveHost(payload: { name: string; endpoint: string }) {
  const host = editorHost.value
  if (!host) return
  editorSaving.value = true
  try {
    const updated = await updateHost(host.id, payload)
    // PATCH 响应是服务端已保存的权威记录，先立即同步编辑器，避免旧 endpoint
    // 继续触发 endpointDirty 并阻止配对。
    editorHost.value = updated
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
    await data.hostsQuery.refetch()
    editorHost.value = data.hosts.value.find(item => item.id === host.id) || updated
    ElMessage.success('主机资料已保存，原有配对凭据保持不变。')
  } catch (error) {
    ElMessage.error('保存主机失败：' + errorMessage(error))
  } finally {
    editorSaving.value = false
  }
}

async function pairHost(payload: { agent_id: string; spki_sha256: string; pairing_code: string }) {
  const host = editorHost.value
  if (!host) return
  if (host.agent_id) {
    try {
      await ElMessageBox.confirm(
        '重新配对成功会替换 Agent 当前唯一有效控制端授权。请确认 Agent UUID 与 TLS 指纹已从目标主机可信终端核验。',
        '确认重新配对',
        { type: 'warning', confirmButtonText: '继续配对', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
  }
  pairing.value = true
  try {
    await pairHostApi(host.id, payload)
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
    await queryClient.invalidateQueries({ queryKey: ['host-snapshot', host.id] })
    await queryClient.invalidateQueries({ queryKey: ['host-app-status', host.id] })
    editorOpen.value = false
    ElMessage.success('Agent 配对成功，可信来源中的应用无需单独授权。')
  } catch (error) {
    ElMessage.error('配对失败：' + errorMessage(error))
  } finally {
    pairing.value = false
  }
}

async function removeHost(host: Host) {
  const temporary = host.host_type === 'temporary'
  const warning = temporary
    ? '该操作会立即撤销临时接入授权、取消尚未领取的任务，并从主机列表剔除。已经开始的任务会标记为中断，远端执行结果可能未知。'
    : host.agent_id
      ? '该操作只删除 EasyNetTopo 本地登记和已保存凭据，不会撤销 Agent 端授权、停止 Agent 或删除 Docker 应用。'
      : '该操作只删除 EasyNetTopo 的主机登记，不影响 Agent 或 Docker 应用。'
  try {
    await ElMessageBox.confirm(
      '确定从 EasyNetTopo 主机列表' + (temporary ? '剔除临时主机「' : '删除「') + host.name + '」？\n\n' + warning,
      temporary ? '剔除临时主机' : '删除主机',
      { type: 'warning', confirmButtonText: temporary ? '确认剔除' : '确认删除', cancelButtonText: '取消' }
    )
    if (temporary) await revokeTemporaryHost(host.id)
    else await deleteHostApi(host.id)
    if (ui.selectedHostId === host.id) ui.selectHost('')
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
    ElMessage.success(temporary ? '临时主机已剔除。' : '主机已从 EasyNetTopo 删除。远端 Agent 和 Docker 应用未受影响。')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error('删除主机失败：' + errorMessage(error))
    }
  }
}

async function addSource(payload: { name: string; url: string; branch: string }) {
  sourceSaving.value = true
  try {
    await createSource(payload)
    await queryClient.invalidateQueries({ queryKey: ['catalog-sources'] })
    await queryClient.invalidateQueries({ queryKey: ['catalog-apps'] })
    ElMessage.success('应用源已添加，请手动同步以读取应用目录。')
  } catch (error) {
    ElMessage.error('添加应用源失败：' + errorMessage(error))
  } finally {
    sourceSaving.value = false
  }
}

async function syncSource(source: CatalogSource) {
  busySourceId.value = source.id
  try {
    const result = await refreshSource(source.id)
    await queryClient.invalidateQueries({ queryKey: ['catalog-sources'] })
    await queryClient.invalidateQueries({ queryKey: ['catalog-apps'] })
    await queryClient.invalidateQueries({ queryKey: ['host-app-status'] })
    ElMessage.success('应用源同步完成，发现 ' + result.app_count + ' 个应用。')
  } catch (error) {
    await queryClient.invalidateQueries({ queryKey: ['catalog-sources'] })
    ElMessage.error('应用源同步失败：' + errorMessage(error))
  } finally {
    busySourceId.value = ''
  }
}

async function removeSource(source: CatalogSource) {
  try {
    await ElMessageBox.confirm(
      '确认删除应用源「' + source.name + '」？这只会移除应用目录来源，不会卸载任何主机上已安装的应用。',
      '删除应用源',
      { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' }
    )
    deletingSourceId.value = source.id
    await deleteSource(source.id)
    await queryClient.invalidateQueries({ queryKey: ['catalog-sources'] })
    await queryClient.invalidateQueries({ queryKey: ['catalog-apps'] })
    await queryClient.invalidateQueries({ queryKey: ['host-app-status'] })
    ElMessage.success('应用源已删除，已安装应用不受影响。')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error('删除应用源失败：' + errorMessage(error))
    }
  } finally {
    deletingSourceId.value = ''
  }
}

async function openInstallConfig(app: CatalogApp) {
  if (app.app_id === 'easynettopo') return
  if (!selectedHost.value?.agent_id) {
    ElMessage.warning('请先选择并配对一台主机。')
    return
  }
  configBusy.value = true
  try {
    const config = await getAppConfig(app)
    configState.value = { app, config }
    configDialogOpen.value = true
  } catch (error) {
    ElMessage.error('无法读取安装配置：' + errorMessage(error))
  } finally {
    configBusy.value = false
  }
}

function permissionsFor(app: CatalogApp) {
  return getAppPermissions(app, data.statusByKey.value[appKey(app)], snapshot.value?.platform)
}

async function handleAppAction(app: CatalogApp, action: AppAction) {
  const permissions = permissionsFor(app)
  if (action === 'update' && !permissions.canUpdate) return
  if (action === 'uninstall' && !permissions.canUninstall) return
  if (action === 'adopt' && !permissions.canAdopt) return
  if (action === 'force_update' && !permissions.canForceUpdate) return

  if (action === 'uninstall') {
    try {
      await ElMessageBox.confirm(
        '将在主机「' + (selectedHost.value?.name || '') + '」上卸载「' + app.name +
          '」。默认保留持久化数据。请确认目标主机和应用正确。',
        '确认卸载',
        { type: 'warning', confirmButtonText: '确认卸载', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
  } else if (action === 'force_update') {
    try {
      await ElMessageBox.confirm(
        '仅适用于归属明确的受管应用，或已核验原 Deploy 目录的旧项目。Agent 会校验固定来源应用包；已有受管项目重新执行更新脚本，缺失旧脚本时安全修复后再更新。\n\n不会主动清除 .env、.secrets 和持久化数据；如项目归属不符、路径不安全或主机配对失效，Agent 会拒绝执行。更新可能重建容器，请事先备份数据。',
        '确认强制更新',
        { type: 'warning', confirmButtonText: '确认强制更新', cancelButtonText: '取消' }
      )
    } catch {
      return
    }

  }
  await runAppAction(app, action)
}

async function runAppAction(
  app: CatalogApp,
  action: AppAction,
  install?: { revision: string; config: Record<string, string> }
) {
  if (app.app_id === 'easynettopo' && action !== 'update') return
  const host = selectedHost.value
  if (!host?.agent_id) {
    ElMessage.warning('请先选择并配对一台主机。')
    return
  }
  const key = appKey(app)
  if (host.host_type === 'temporary' && (action !== 'install' || !install)) {
    ElMessage.warning('临时主机仅支持预安装，请通过应用市场配置安装参数。')
    return
  }
  setRecovered(host.id, key, false)
  actionBusyKey.value = key
  try {
    const payload = action === 'install' && install
      ? { source_id: app.source_id, app_id: app.app_id, revision: install.revision, config: install.config }
      : action === 'update' || action === 'adopt' || action === 'force_update'
        ? { source_id: app.source_id, app_id: app.app_id, revision: app.revision }
        : { source_id: app.source_id, app_id: app.app_id }
    if (host.host_type === 'temporary') {
      await preinstallTemporary(host.id, payload as {
        source_id: string; app_id: string; revision: string; config: Record<string, string>
      })
      configDialogOpen.value = false
      await data.tasksQuery.refetch()
      ElMessage.success(app.name + ' 预安装参数已保存，等待 EasyHostAgent 主动领取任务。')
      return
    }
    const accepted = await executeAppAction(host.id, action, payload)
    configDialogOpen.value = false
    if (action === 'update' && accepted.accepted) {
      ElMessage.info(app.name + ' 更新任务已加入队列，后台将自动执行。')
      await data.tasksQuery.refetch()
      return
    }
    if (action === 'uninstall' && accepted.result === 'failed') {
      const restored = await recoverFailedUninstall(host.id, app)
      ElMessage.warning(restored
        ? app.name + ' 卸载失败，但远端资源已不存在；已清理卡片旧任务并恢复待安装'
        : app.name + ' 卸载失败，远端资源仍存在或状态未确认；保留真实状态和失败信息')
      return
    }
    if (accepted.result === 'running') {
      pendingAction.value = { hostId: host.id, appKey: key, action, appName: app.name, errors: 0 }
      ElMessage.info(app.name + ' ' + actionName(action) + '请求已受理，正在等待 Agent 执行结果。')
    } else if (accepted.result === 'succeeded') {
      ElMessage.success(app.name + ' ' + actionName(action) + '已完成。')
    } else {
      ElMessage.warning(app.name + ' 已受理，但最终结果尚未确认，请刷新状态核对。')
    }
    await data.statusQuery.refetch()
    if (accepted.result === 'succeeded') await data.dockerQuery.refetch()
  } catch (error) {
    const restored = action === 'uninstall' && await recoverFailedUninstall(host.id, app)
    if (restored) ElMessage.warning(app.name + ' 卸载指令失败，但远端已确认无应用资源；卡片已恢复待安装')
    else ElMessage.error(app.name + ' ' + actionName(action) + '失败：' + errorMessage(error) +
      (action === 'uninstall' ? '；资源仍存在或状态未确认，保留实际状态' : ''))
    if (action !== 'uninstall') await data.statusQuery.refetch()
  } finally {
    actionBusyKey.value = ''
  }
}

async function submitInstallConfig(payload: { revision: string; config: Record<string, string> }) {
  const app = configState.value?.app
  if (!app) return
  await runAppAction(app, 'install', payload)
}

function showAppInfo(app: CatalogApp) {
  const content = [
    '应用：' + app.name,
    '应用 ID：' + app.app_id,
    '来源：' + (sources.value.find(source => source.id === app.source_id)?.name || app.source_id),
    '版本：' + (app.revision || '本机保留记录'),
    '清单摘要：' + (app.manifest_sha256 || '无')
  ].join('\n')
  ElMessageBox.alert(content, '应用信息', { confirmButtonText: '关闭', type: 'info' })
}

async function submitLogin() {
  if (loginPending.value || !loginForm.username.trim() || !loginForm.password) return
  loginMutation.mutate()
}
async function clearNetworkNodeData() {
  if (dataClearBusy.value) return
  try {
    await ElMessageBox.confirm(
      '将删除 EasyNetTopo 本地保存的全部网络节点属性，包括节点别名、公网 IP 标记及地址、地理位置和经纬度。不会删除实时 EasyTier 节点、设备记录、主机或 Agent 配对。此操作不可恢复，是否继续？',
      '确认清除节点数据',
      { type: 'warning', confirmButtonText: '确认清除', cancelButtonText: '取消' }
    )
  } catch { return }

  dataClearBusy.value = 'nodes'
  try {
    await clearNodeAttributes()
    await queryClient.invalidateQueries({ queryKey: ['network-node-attributes'] })
    dataManagementDialogOpen.value = false
    ElMessage.success('EasyNetTopo 网络节点数据已清除。')
  } catch (error) {
    ElMessage.error('清除网络节点数据失败：' + errorMessage(error))
  } finally {
    dataClearBusy.value = ''
  }
}

async function clearAllDeviceData() {
  if (dataClearBusy.value) return
  try {
    await ElMessageBox.confirm(
      '将删除 EasyNetTopo 本地全部设备记录，包括网络节点基础设备、普通设备、历史设备、设备分组、网关组、OpenWrt RPCD 配置和节点同步记录，并关闭节点自动补充。不会删除网络节点属性、主机登记、Agent 配对或远端资源。此操作不可恢复，是否继续？',
      '确认清除设备数据',
      { type: 'warning', confirmButtonText: '确认清除全部设备', cancelButtonText: '取消' }
    )
  } catch { return }

  dataClearBusy.value = 'devices'
  try {
    await clearDeviceDataApi()
    queryClient.setQueryData(['device-network-node-sync'], { enabled: false })
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-node-devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-network-node-sync'] }),
      queryClient.invalidateQueries({ queryKey: ['device-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['gateway-groups'] })
    ])
    dataManagementDialogOpen.value = false
    ElMessage.success('EasyNetTopo 设备数据已全部清除；需要时可重新初始化网络节点。')
  } catch (error) {
    await queryClient.invalidateQueries({ queryKey: ['device-network-node-sync'] })
    ElMessage.error('清除设备数据失败：' + errorMessage(error))
  } finally {
    dataClearBusy.value = ''
  }
}

async function logout() {
  try {
    await logoutApi()
    appQueryClient.clear()
    ui.selectHost('')
    await authQuery.refetch()
    ElMessage.success('已退出登录。')
  } catch (error) {
    ElMessage.error('退出登录失败：' + errorMessage(error))
  }
}
</script>

<template>
  <main v-if="authLoading" class="auth-screen">
    <section class="auth-card">
      <div class="auth-brand">EasyNetTopo</div>
      <el-skeleton :rows="3" animated />
      <p>正在连接控制端…</p>
    </section>
  </main>

  <main v-else-if="authError" class="auth-screen">
    <section class="auth-card">
      <div class="auth-brand">EasyNetTopo</div>
      <el-result icon="error" title="控制端暂时不可用" :sub-title="authError">
        <template #extra>
          <el-button type="primary" @click="authQuery.refetch()">重试连接</el-button>
        </template>
      </el-result>
    </section>
  </main>

  <main v-else-if="!initialized" class="auth-screen">
    <section class="auth-card">
      <div class="auth-brand">EasyNetTopo</div>
      <el-result icon="info" title="首次初始化" sub-title="请先在服务器可信终端运行 easynettopo admin init。">
        <template #extra>
          <el-button type="primary" @click="authQuery.refetch()">刷新状态</el-button>
        </template>
      </el-result>
    </section>
  </main>

  <main v-else-if="!authenticated" class="auth-screen">
    <section class="auth-card login-card">
      <div class="auth-brand">EasyNetTopo</div>
      <p class="auth-kicker">易网拓扑</p>
      <h1>管理员登录</h1>
      <p class="auth-description">登录后管理已配对主机与 Docker 应用。</p>
      <el-form label-position="top" @submit.prevent="submitLogin">
        <el-form-item label="账号">
          <el-input v-model="loginForm.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="loginForm.password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-button class="login-button" type="primary" native-type="submit" :loading="loginPending" :disabled="loginPending || !loginForm.username.trim() || !loginForm.password">
          登录控制端
        </el-button>
      </el-form>
    </section>
  </main>

  <div v-else class="ent-shell" :class="{ 'is-collapsed': ui.sidebarCollapsed }">
    <SidebarNav :collapsed="ui.sidebarCollapsed" @toggle="ui.toggleSidebar()" />
    <main class="workspace">
      <header class="topbar">
        <div>
          <div class="crumb">EasyNetTopo <span>/</span> 工作台 <span>/</span> {{ isNetworkPage ? '网络节点' : isDevicesPage ? '设备列表' : isMapPage ? '网络地图' : '主机商店' }}</div>
          <div class="page-title"><span class="title-square"></span><h1>{{ isNetworkPage ? '网络节点' : isDevicesPage ? '设备列表' : isMapPage ? '网络地图' : '主机商店' }}</h1></div>
        </div>
        <div class="topbar-actions">
          <el-select class="theme-switcher" aria-label="页面主题" :model-value="ui.themeMode"
            size="small" @update:model-value="changeThemeMode">
            <el-option label="跟随系统" value="system" />
            <el-option label="浅色模式" value="light" />
            <el-option label="深色模式" value="dark" />
          </el-select>
          <span v-if="controllerId" class="controller-id">控制端 {{ controllerId.slice(0, 8) }}</span>
          <el-tooltip content="删除 EasyNetTopo 本地节点及设备数据" placement="bottom">
            <el-button text aria-label="删除数据" :disabled="!!dataClearBusy" @click="dataManagementDialogOpen = true">
              <el-icon><Delete /></el-icon>删除数据
            </el-button>
          </el-tooltip>
          <el-tooltip content="退出登录" placement="bottom">
            <el-button text circle aria-label="退出登录" @click="logout">
              <el-icon><SwitchButton /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </header>

      <NetworkNodesPane v-if="isNetworkPage" :hosts="hosts" :hosts-loading="data.hostsQuery.isPending.value" :hosts-error="data.hostsQuery.error.value ? errorMessage(data.hostsQuery.error.value) : ''" />
      <DevicesPane v-else-if="isDevicesPage" :hosts="hosts" />
      <NetworkMapPane v-else-if="isMapPage" :hosts="hosts" :hosts-loading="data.hostsQuery.isPending.value" :hosts-error="data.hostsQuery.error.value ? errorMessage(data.hostsQuery.error.value) : ''" />
      <div v-else class="main-layout">
        <HostListPane
          :hosts="hosts"
          :tasks="executionTasks"
          :selected-host-id="selectedHostId"
          :selected-health-status="healthStatus"
          :refreshing="refreshing"
          @select="ui.selectHost"
          @add="addHostOpen = true"
          @refresh="refreshAll"
          @settings="sourceSettingsOpen = true"
          @discover="showDiscovery"
          @edit="openEditor"
          @docker="openDocker"
          @delete="removeHost"
          @network="toggleNetworkEntry"
        />
        <StorePane
          :host="selectedHost"
          :tasks="executionTasks"
          :recovered-uninstall-cards="recoveredUninstallCards"
          :snapshot="snapshot"
          :docker-inventory="dockerInventory"
          :snapshot-loading="snapshotLoading"
          :snapshot-error="snapshotError"
          :apps="visibleApps"
          :sources="sources"
          :rows="rows"
          :loading="marketLoading"
          :status-error="statusError"
          :filter="ui.appFilter"
          :busy-key="actionBusyKey"
          :pending-key="pendingKey"
          :agent-maintenance="agentMaintenanceForHost"
          :agent-update-busy="agentUpdateBusy"
          @agent-update="requestAgentUpdate"
          @filter-change="ui.appFilter = $event"
          @configure="openInstallConfig"
          @action="handleAppAction"
          @info="showAppInfo"
          @resolve-task="resolveTask"
        />
      </div>
    </main>

    <el-dialog v-model="discoveryOpen" title="内网主机发现" width="min(720px, calc(100vw - 28px))" append-to-body>
      <div class="discovery-panel">
        <el-alert
          title="开启后，内网 EasyHostAgent 会主动向当前 EasyNetTopo 报到。发现窗口有效期 30 分钟；加入临时主机后另行固定计时 30 分钟，不因心跳续期。"
          type="info"
          :closable="false"
          show-icon
        />
        <div class="discovery-actions">
          <el-tag :type="discoveryState.enabled ? 'success' : 'info'">
            {{ discoveryState.enabled ? '发现已开启' : '发现已关闭' }}
          </el-tag>
          <el-button size="small" type="primary" :loading="discoverySaving" :disabled="discoverySaving" @click="startDiscovery">开启发现并生成凭据</el-button>
          <el-button size="small" plain :loading="discoverySaving" :disabled="discoverySaving || !discoveryState.enabled" @click="stopDiscovery">关闭发现</el-button>
          <el-button size="small" :loading="discoveryLoading" @click="refreshDiscovery">刷新</el-button>
        </div>
        <div v-if="discoveryToken" class="discovery-token">
          <div>
            <strong>Agent 接入凭据</strong>
            <p>请配置到内网主机的 EasyHostAgent；此凭据只在本次开启操作后展示，服务端仅保存摘要。</p>
          </div>
          <el-input :model-value="discoveryToken" readonly />
          <el-button @click="copyDiscoveryToken">复制凭据</el-button>
        </div>
        <el-divider />
        <div class="discovery-list-head">
          <strong>待确认主机</strong>
          <span>{{ discoveredAgents.length }} 台</span>
        </div>
        <el-table v-if="discoveredAgents.length" :data="discoveredAgents" size="small" row-key="agent_id">
          <el-table-column prop="hostname" label="主机名称" min-width="130" />
          <el-table-column label="系统 / 架构" min-width="130">
            <template #default="{ row }">{{ row.platform }} · {{ row.arch }}</template>
          </el-table-column>
          <el-table-column prop="version" label="Agent 版本" min-width="110" />
          <el-table-column label="最后报到" min-width="150">
            <template #default="{ row }">{{ new Date(row.last_seen_at).toLocaleString() }}</template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" :loading="discoverySaving" :disabled="discoverySaving" @click="acceptDiscovery(row)">加入临时主机</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="暂无待确认主机。请先开启发现，并在内网 Agent 配置接入地址与凭据。" :image-size="76" />
      </div>
      <template #footer>
        <el-button @click="discoveryOpen = false">关闭</el-button>
      </template>
    </el-dialog>
    <AddHostDialog v-model="addHostOpen" :saving="addHostSaving" @submit="submitNewHost" />
    <HostEditorDialog
      v-model="editorOpen"
      :host="editorHost"
      :saving="editorSaving"
      :pairing="pairing"
      @save="saveHost"
      @pair="pairHost"
    />
    <SourceSettingsDialog
      v-model="sourceSettingsOpen"
      :sources="sources"
      :saving="sourceSaving"
      :busy-source-id="busySourceId"
      :deleting-source-id="deletingSourceId"
      @create="addSource"
      @refresh="syncSource"
      @delete="removeSource"
    />
    <DockerResourceDrawer
      v-model="dockerOpen"
      :host="selectedHost"
      :inventory="dockerInventory"
      :loading="dockerLoading"
      :error="dockerError"
    />
    <AppConfigDialog
      v-model="configDialogOpen"
      :app="configApp"
      :revision="configRevision"
      :fields="configFields"
      :saving="configBusy || !!actionBusyKey"
      @submit="submitInstallConfig"
    />
    <el-dialog
      v-model="dataManagementDialogOpen"
      title="删除数据"
      width="560px"
      :close-on-click-modal="!dataClearBusy"
      :close-on-press-escape="!dataClearBusy"
    >
      <div class="data-management-dialog">
        <el-alert
          title="这里只清除 EasyNetTopo 本地保存的数据，不会操作远程 EasyTier、Agent 或主机资源。删除后不可恢复。"
          type="warning"
          :closable="false"
          show-icon
        />
        <section class="data-clear-item">
          <div class="data-clear-copy">
            <h3>清除节点数据</h3>
            <p>仅清除 EasyNetTopo 网络节点属性：节点别名、公网 IP 标记及地址、国家地区、经纬度位置。</p>
            <span>保留设备记录、主机登记、Agent 配对及实时网络节点。</span>
          </div>
          <el-button
            type="danger"
            plain
            :loading="dataClearBusy === 'nodes'"
            :disabled="!!dataClearBusy"
            @click="clearNetworkNodeData"
          >清除节点数据</el-button>
        </section>
        <el-divider />
        <section class="data-clear-item">
          <div class="data-clear-copy">
            <h3>清除设备数据</h3>
            <p>清除节点基础设备、普通及历史设备、设备分组、网关组、OpenWrt RPCD 配置和节点同步记录。</p>
            <span>同时关闭节点自动补充；网络节点属性、主机和 Agent 配对保持不变。</span>
          </div>
          <el-button
            type="danger"
            plain
            :loading="dataClearBusy === 'devices'"
            :disabled="!!dataClearBusy"
            @click="clearAllDeviceData"
          >清除设备数据</el-button>
        </section>
      </div>
      <template #footer>
        <el-button :disabled="!!dataClearBusy" @click="dataManagementDialogOpen = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.auth-screen {
  display: grid;
  width: 100%;
  min-height: 100%;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(ellipse at 75% 5%, #14325366, transparent 42%),
    linear-gradient(180deg, #0b1726, #07111f 75%);
}
.auth-card {
  width: min(430px, 100%);
  padding: 30px;
  border: 1px solid #29445c;
  border-radius: 18px;
  background: linear-gradient(145deg, #102136, #0d1c2e);
  box-shadow: 0 22px 60px #0005;
}
.auth-brand {
  background: linear-gradient(100deg, #f5faff 30%, #88dff4);
  background-clip: text;
  color: transparent;
  font-size: 25px;
  font-weight: 780;
  letter-spacing: -.8px;
}
.auth-card > p { color: var(--ent-muted); font-size: 12px; }
.auth-kicker { margin: 6px 0 24px; color: #718daa !important; font-size: 9px !important; letter-spacing: 1.8px; }
.login-card h1 { margin: 0; font-size: 22px; }
.auth-description { margin: 8px 0 22px; }
.login-button { width: 100%; margin-top: 8px; }
.ent-shell {
  display: grid;
  width: 100%;
  height: 100dvh;
  grid-template-columns: var(--ent-sidebar-width) minmax(0, 1fr);
  transition: grid-template-columns .25s ease;
}
.ent-shell.is-collapsed { grid-template-columns: var(--ent-sidebar-collapsed-width) minmax(0, 1fr); }
.workspace {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  background: var(--ent-workspace-gradient);
}
.workspace::before {
  position: absolute;
  pointer-events: none;
  inset: 0;
  opacity: .2;
  background-image: linear-gradient(var(--ent-grid-line) 1px, transparent 1px), linear-gradient(90deg, var(--ent-grid-line) 1px, transparent 1px);
  background-size: 36px 36px;
  mask-image: linear-gradient(to bottom, #000, transparent 68%);
  content: '';
}
.workspace > * { position: relative; z-index: 1; }
.topbar {
  display: flex;
  min-height: 79px;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 0 27px;
  border-bottom: 1px solid #24384c;
  background: var(--ent-topbar-bg);
  backdrop-filter: blur(14px);
}
.crumb { display: flex; align-items: center; gap: 10px; color: #7f9eba; font-size: 12px; letter-spacing: .2px; }
.crumb span { opacity: .45; }
.page-title { display: flex; align-items: center; gap: 12px; margin-top: 5px; }
.title-square { width: 3px; height: 24px; border-radius: 10px; background: var(--ent-cyan); box-shadow: 0 0 12px #49e6eacc; }
.page-title h1 { margin: 0; font-size: 22px; font-weight: 750; letter-spacing: .4px; }
.topbar-actions { display: flex; align-items: center; gap: 10px; }
.theme-switcher { width: 120px; flex: none; }
.controller-id { color: var(--ent-dim); font-size: 10px; }
.topbar-actions :deep(.el-button) { color: #9ab6cf; }
.data-management-dialog { display: flex; flex-direction: column; gap: 16px; }
.discovery-panel { display: flex; flex-direction: column; gap: 14px; }
.discovery-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.discovery-token { display: grid; grid-template-columns: 1fr auto; align-items: end; gap: 9px; padding: 12px; border: 1px solid var(--ent-line); border-radius: 10px; background: var(--ent-panel); }
.discovery-token > div { grid-column: 1 / -1; }
.discovery-token strong { font-size: 13px; }
.discovery-token p { margin: 5px 0 0; color: var(--ent-muted); font-size: 11px; line-height: 1.5; }
.discovery-list-head { display: flex; align-items: center; justify-content: space-between; font-size: 13px; }
.discovery-list-head span { color: var(--ent-muted); font-size: 11px; }
.data-clear-item { display: flex; align-items: center; justify-content: space-between; gap: 18px; }
.data-clear-copy { min-width: 0; }
.data-clear-copy h3 { margin: 0 0 7px; font-size: 14px; font-weight: 700; }
.data-clear-copy p { margin: 0 0 6px; color: var(--el-text-color-regular); font-size: 13px; line-height: 1.65; }
.data-clear-copy span { color: var(--ent-dim); font-size: 11px; line-height: 1.5; }
@media (max-width: 560px) {
  .data-clear-item { align-items: stretch; flex-direction: column; }
  .data-clear-item :deep(.el-button) { align-self: flex-start; }
}
.main-layout { display: grid; min-height: 0; flex: 1; grid-template-columns: var(--ent-host-pane-width) minmax(0, 1fr); }
@media (max-width: 1460px) { .main-layout { grid-template-columns: 286px minmax(0, 1fr); } }
@media (max-width: 1110px) { .main-layout { grid-template-columns: 250px minmax(0, 1fr); } }
@media (max-width: 830px) {
  .ent-shell, .ent-shell.is-collapsed { grid-template-columns: 70px minmax(0, 1fr); }
  .topbar { min-height: 70px; padding: 0 18px; }
  .page-title h1 { font-size: 19px; }
  .main-layout { grid-template-columns: 220px minmax(0, 1fr); }
}
@media (max-width: 630px) {
  .main-layout { grid-template-columns: 1fr; grid-template-rows: auto minmax(0, 1fr); }
  .topbar { min-height: 64px; padding: 0 14px; }
  .crumb { display: none; }
  .page-title h1 { font-size: 17px; }
  .controller-id { display: none; }
  .auth-screen { padding: 14px; }
  .auth-card { padding: 22px; }
}
</style>
