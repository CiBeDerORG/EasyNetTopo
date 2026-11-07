<script setup lang="ts">
import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Close } from '@element-plus/icons-vue'
import { formatApiError } from '../../../lib/api-client'
import { useHostStoreUiStore } from '../../../stores/ui'
import { queryClient as appQueryClient } from '../../../app/query-client'
import {
  createHost,
  createSource,
  deleteHost as deleteHostApi,
  deleteSource,
  executeAppAction,
  getAppConfig,
  getAuthStatus,
  getControllerInfo,
  login,
  logout as logoutApi,
  pairHost as pairHostApi,
  queueNativeAgentUpdate,
  refreshSource,
  updateHost
} from '../api'
import { appKey, getAppPermissions } from '../rules'
import { useHostStoreData } from '../composables/useHostStoreData'
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
  Host,
  HostHealthStatus
} from '../types'

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
const data = useHostStoreData(selectedHostId, authenticated, shouldPoll, dockerOpen, agentUpdating)

const hosts = data.hosts
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

watch(() => data.statusQuery.data.value, result => {
  const job = pendingAction.value
  if (!job || job.hostId !== selectedHostId.value || !result) return
  const row = result.find(item => appKey(item) === job.appKey)
  const expectedAction = job.action === 'force_update' ? 'update' : job.action
  if (!row || row.action !== expectedAction || !row.result || row.result === 'running') return

  pendingAction.value = null
  if (row.result === 'failed') {
    ElMessage.error(job.appName + ' ' + actionName(job.action) + '失败：' + (row.message || '请检查 Agent 执行状态'))
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

async function refreshAll() {
  autoLinkAttempts.clear()
  const tasks: Promise<unknown>[] = [data.hostsQuery.refetch()]
  if (selectedHost.value?.agent_id) {
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
  const warning = host.agent_id
    ? '该操作只删除 EasyNetTopo 本地登记和已保存凭据，不会撤销 Agent 端授权、停止 Agent 或删除 Docker 应用。'
    : '该操作只删除 EasyNetTopo 的主机登记，不影响 Agent 或 Docker 应用。'
  try {
    await ElMessageBox.confirm(
      '确定从 EasyNetTopo 主机列表删除「' + host.name + '」？\n\n' + warning,
      '删除主机',
      { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' }
    )
    await deleteHostApi(host.id)
    if (ui.selectedHostId === host.id) ui.selectHost('')
    await queryClient.invalidateQueries({ queryKey: ['hosts'] })
    ElMessage.success('主机已从 EasyNetTopo 删除。远端 Agent 和 Docker 应用未受影响。')
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
  const host = selectedHost.value
  if (!host?.agent_id) {
    ElMessage.warning('请先选择并配对一台主机。')
    return
  }
  const key = appKey(app)
  actionBusyKey.value = key
  try {
    const payload = action === 'install' && install
      ? { source_id: app.source_id, app_id: app.app_id, revision: install.revision, config: install.config }
      : action === 'update' || action === 'adopt' || action === 'force_update'
        ? { source_id: app.source_id, app_id: app.app_id, revision: app.revision }
        : { source_id: app.source_id, app_id: app.app_id }
    const accepted = await executeAppAction(host.id, action, payload)
    configDialogOpen.value = false
    if (accepted.result === 'running') {
      pendingAction.value = { hostId: host.id, appKey: key, action, appName: app.name, errors: 0 }
      ElMessage.info(app.name + ' ' + actionName(action) + '请求已受理，正在等待 Agent 执行结果。')
    } else if (accepted.result === 'succeeded') {
      ElMessage.success(app.name + ' ' + actionName(action) + '已完成。')
    } else {
      ElMessage.warning(app.name + ' 已受理，但最终结果尚未确认，请刷新状态核对。')
    }
    await data.statusQuery.refetch()
  } catch (error) {
    ElMessage.error(app.name + ' ' + actionName(action) + '失败：' + errorMessage(error))
    await data.statusQuery.refetch()
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
          <div class="crumb">EasyNetTopo <span>/</span> 工作台 <span>/</span> 主机商店</div>
          <div class="page-title"><span class="title-square"></span><h1>主机商店</h1></div>
        </div>
        <div class="topbar-actions">
          <span v-if="controllerId" class="controller-id">控制端 {{ controllerId.slice(0, 8) }}</span>
          <el-tooltip content="退出登录" placement="bottom">
            <el-button text circle aria-label="退出登录" @click="logout">
              <el-icon><Close /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </header>

      <div class="main-layout">
        <HostListPane
          :hosts="hosts"
          :selected-host-id="selectedHostId"
          :selected-health-status="healthStatus"
          :refreshing="refreshing"
          @select="ui.selectHost"
          @add="addHostOpen = true"
          @refresh="refreshAll"
          @settings="sourceSettingsOpen = true"
          @edit="openEditor"
          @docker="openDocker"
          @delete="removeHost"
        />
        <StorePane
          :host="selectedHost"
          :snapshot="snapshot"
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
        />
      </div>
    </main>

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
  background:
    radial-gradient(ellipse at 88% 0%, #14325356, transparent 37%),
    linear-gradient(180deg, #0b1726, #07111f 75%);
}
.workspace::before {
  position: absolute;
  pointer-events: none;
  inset: 0;
  opacity: .2;
  background-image: linear-gradient(#1e4967 1px, transparent 1px), linear-gradient(90deg, #1e4967 1px, transparent 1px);
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
  background: #0c1728af;
  backdrop-filter: blur(14px);
}
.crumb { display: flex; align-items: center; gap: 10px; color: #7f9eba; font-size: 12px; letter-spacing: .2px; }
.crumb span { opacity: .45; }
.page-title { display: flex; align-items: center; gap: 12px; margin-top: 5px; }
.title-square { width: 3px; height: 24px; border-radius: 10px; background: var(--ent-cyan); box-shadow: 0 0 12px #49e6eacc; }
.page-title h1 { margin: 0; font-size: 22px; font-weight: 750; letter-spacing: .4px; }
.topbar-actions { display: flex; align-items: center; gap: 10px; }
.controller-id { color: var(--ent-dim); font-size: 10px; }
.topbar-actions :deep(.el-button) { color: #9ab6cf; }
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
