<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Box, Search, Setting } from '@element-plus/icons-vue'
import AppCard from './AppCard.vue'
import NativeAgentCard from './NativeAgentCard.vue'
import { appKey, canRecoverFailedUninstall, getAppPermissions, isInstalledStatus, recoveredUninstallStatus } from '../rules'
import type { AppFilter } from '../../../stores/ui'
import type { AgentMaintenanceState, AgentUpdateAction, AppAction } from '../types'
import type { AppStatus, CatalogApp, CatalogSource, DockerInventory, DockerPort, ExecutionTask, Host, HostSnapshot } from '../types'
import { currentAppTask, currentHostTask, formatTaskTime, formatTaskTriggerTime, taskActive, taskSourceLabel, taskStateLabel } from '../task-status'

const props = defineProps<{
  host: Host | null
  tasks: ExecutionTask[]
  recoveredUninstallCards: string[]
  snapshot: HostSnapshot | null
  dockerInventory?: DockerInventory
  snapshotLoading: boolean
  snapshotError: string
  apps: CatalogApp[]
  sources: CatalogSource[]
  rows: AppStatus[]
  loading: boolean
  statusError: string
  filter: AppFilter
  busyKeys: ReadonlySet<string>
  loadingConfigKey: string
  pendingKey: string
  agentMaintenance: AgentMaintenanceState | null
  agentUpdateBusy: boolean
  agentCanUpdate: boolean
}>()

defineEmits<{
  'filter-change': [filter: AppFilter]
  settings: []
  configure: [app: CatalogApp]
  action: [app: CatalogApp, action: AppAction]
  info: [app: CatalogApp]
  'resolve-task': [task: ExecutionTask]
  'agent-update': [action: AgentUpdateAction]
}>()

const search = ref('')
const query = computed(() => search.value.trim().toLocaleLowerCase())
const emptyDescriptions: Record<AppFilter, string> = {
  all: '应用目录为空',
  installed: '当前主机没有已安装应用',
  available: '当前没有未安装的应用',
  update: '当前没有支持更新的应用'
}
const hostTask = computed(() => props.host ? currentHostTask(props.tasks, props.host.id) : undefined)
const storeScroll = ref<HTMLElement>()
watch(() => props.host?.id, () => storeScroll.value?.scrollTo({ top: 0 }), { flush: 'post' })

function recovered(row: AppStatus): boolean {
  return !!props.host &&
    props.recoveredUninstallCards.includes(props.host.id + '|' + appKey(row)) &&
    canRecoverFailedUninstall(row, props.dockerInventory, row.app_id)
}
const statusByKey = computed<Record<string, AppStatus>>(() => Object.fromEntries(props.rows.map(row =>
  [appKey(row), recovered(row) ? recoveredUninstallStatus(row) : row]
)))
const installedCount = computed(() => Object.values(statusByKey.value)
  .filter(isInstalledStatus).length + (props.snapshot ? 1 : 0))
const runningCount = computed(() => Object.values(statusByKey.value)
  .filter(row => row.state === 'running').length + (props.snapshot ? 1 : 0))
const availableCount = computed(() => props.apps.filter(app => !!app.revision && !app.catalog_only).length + 1)
const agentTask = computed(() => props.host ? currentAppTask(props.tasks, props.host.id, 'native', 'easyhostagent') : undefined)
const agentState = computed(() => {
  if (!props.host?.agent_id) return { label: '未配对', type: 'info' as const }
  if (taskActive(agentTask.value) || props.agentMaintenance?.state === 'waiting') return { label: '更新中', type: 'warning' as const }
  if (props.agentMaintenance?.state === 'timeout') return {
    label: props.snapshotError ? '重连超时' : '更新未完成', type: 'danger' as const
  }
  if (props.snapshotLoading) return { label: '检测中', type: 'warning' as const }
  if (props.snapshotError) return { label: '连接异常', type: 'danger' as const }
  if (props.snapshot) return { label: '已连接', type: 'success' as const }
  return { label: '已配对', type: 'info' as const }
})
const showNativeAgent = computed(() =>
  'easyhostagent'.includes(query.value) && (
    props.filter === 'all' || props.filter === 'installed' && !!props.snapshot ||
    props.filter === 'available' && !props.snapshot ||
    props.filter === 'update' && props.agentCanUpdate
  )
)
const visibleApps = computed(() => props.apps.filter(app => {
  if (query.value && ![app.name, app.app_id].some(value => value.toLocaleLowerCase().includes(query.value))) return false
  const row = statusByKey.value[appKey(app)]
  const permissions = getAppPermissions(app, row, props.snapshot?.platform, !!props.host?.agent_id)
  if (props.filter === 'installed') return !!row && isInstalledStatus(row)
  if (props.filter === 'available' && app.native_install) return props.snapshot?.platform === 'macos' && row?.state === 'not_installed'
  if (props.filter === 'available') return !app.catalog_only && (!row || (!row.managed && !row.external))
  if (props.filter === 'update') return permissions.canUpdate
  return true
}))
function cardTask(app: CatalogApp): ExecutionTask | undefined {
  if (!props.host) return undefined
  const task = currentAppTask(props.tasks, props.host.id, app.source_id, app.app_id)
  // 只隐藏已核实资源不存在的历史记录，不隐藏活跃或中断任务。
  if (task && !taskActive(task) && task.state !== 'interrupted') {
    const original = props.rows.find(row => appKey(row) === appKey(app))
    if (original && recovered(original)) return undefined
  }
  return task
}
function sourceName(id: string) {
  return props.sources.find(source => source.id === id)?.name || id
}
function appPorts(app: CatalogApp): DockerPort[] {
  if (app.native_install) return []
  const resources = props.dockerInventory?.resources || []
  const resource = resources.find(item =>
    item.kind === 'compose_project' &&
    (item.compose_project === app.app_id || item.name === app.app_id)
  )
  return resource?.ports || []
}

</script>

<template>
  <section class="store-panel" aria-label="主机应用商店">
    <div v-if="!host" class="store-empty">
      <el-empty description="请先添加并选择一台主机" :image-size="110">
        <el-button :icon="Setting" @click="$emit('settings')">应用源设置</el-button>
      </el-empty>
    </div>
    <div v-else ref="storeScroll" class="store-scroll">
      <div class="selected-host">
        <div class="selected-heading">
          <div class="selected-icon"><el-icon><Box /></el-icon></div>
          <div class="selected-info">
            <h2>{{ host.name }}</h2>
            <p>
              <span class="endpoint">{{ host.endpoint }}</span>
              <span v-if="snapshot"> · {{ snapshot.platform }} · {{ snapshot.arch }}</span>
              <el-tag size="small" effect="dark" :type="agentState.type">{{ agentState.label }}</el-tag>
            </p>
          </div>
        </div>
        <div v-if="hostTask" class="host-observed" role="status">
          <template v-if="hostTask.state === 'succeeded'">
            <span>最近任务</span>
            <strong>{{ formatTaskTime(hostTask) }}</strong>
          </template>
          <template v-else>
            <span>{{ taskStateLabel(hostTask.state) }}</span>
            <strong>{{ hostTask.id }}</strong>
            <span>{{ taskSourceLabel(hostTask.trigger_source) }}</span>
          </template>
          <span>触发时间：{{ formatTaskTriggerTime(hostTask) }}</span>
        </div>
        <div v-if="snapshot" class="host-observed">
          <span>最近采集</span>
          <strong>{{ new Date(snapshot.observed_at).toLocaleString() }}</strong>
        </div>
      </div>

      <dl class="metrics">
        <div><dt>已安装</dt><dd>{{ installedCount }}</dd></div>
        <div><dt>运行中</dt><dd>{{ runningCount }}</dd></div>
        <div><dt>可用应用</dt><dd>{{ availableCount }}</dd></div>
      </dl>

      <div v-if="snapshotError && !agentMaintenance" class="notice-line" role="status">
        <el-alert title="暂时无法获取主机快照。连接失败不代表主机或 Docker 资源已删除。" type="warning" :closable="false" show-icon />
      </div>
      <div v-if="statusError && !agentMaintenance" class="notice-line" role="alert">
        <el-alert :title="'应用状态未知。' + statusError" type="warning" :closable="false" show-icon />
      </div>
      <div v-if="!host.agent_id" class="notice-line">
        <el-alert title="该主机尚未配对 EasyHostAgent。请先编辑主机并完成身份核验与配对。" type="info" :closable="false" show-icon />
      </div>

      <div class="store-heading">
        <h3>应用市场</h3>
        <el-button text :icon="Setting" @click="$emit('settings')">应用源设置</el-button>
      </div>
      <div class="store-filters">
        <el-input v-model="search" class="app-search" clearable :prefix-icon="Search"
          aria-label="搜索应用" placeholder="搜索应用名称或 ID" />
        <el-radio-group :model-value="filter" aria-label="安装状态" @update:model-value="$emit('filter-change', $event)">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="installed">已安装</el-radio-button>
          <el-radio-button label="available">未安装</el-radio-button>
          <el-radio-button label="update">支持更新</el-radio-button>
        </el-radio-group>
      </div>

      <div v-if="loading && rows.length === 0" class="market-loading">
        <el-skeleton :rows="4" animated />
      </div>
      <div v-else-if="visibleApps.length || showNativeAgent" class="market-grid">
        <NativeAgentCard
          v-if="showNativeAgent"
          :key="host.id"
          :host-id="host.id"
          :software-root="snapshot?.software_root"
          :platform="snapshot?.platform"
          :task="agentTask"
          :connected="!!snapshot && !snapshotError"
          :paired="!!host.agent_id"
          :checking="snapshotLoading"
          :connection-error="!!snapshotError"
          :maintenance="agentMaintenance"
          :busy="agentUpdateBusy"
          :can-update="agentCanUpdate"
          @update="$emit('agent-update', $event)"
          @resolve="agentTask && $emit('resolve-task', agentTask)"
        />
        <AppCard
          v-for="app in visibleApps"
          :key="appKey(app)"
          :app="app"
          :status="statusByKey[appKey(app)]"
          :task="cardTask(app)"
          :ports="appPorts(app)"
          :source-name="sourceName(app.source_id)"
          :permissions="getAppPermissions(app, statusByKey[appKey(app)], snapshot?.platform, !!host.agent_id)"
          :loading-config="loadingConfigKey === appKey(app)"
          :busy="busyKeys.has(host.id + '|' + appKey(app)) || pendingKey === appKey(app) || agentMaintenance?.state === 'waiting' || taskActive(agentTask) || taskActive(cardTask(app))"
          @configure="$emit('configure', app)"
          @action="$emit('action', app, $event)"
          @info="$emit('info', app)"
          @resolve="cardTask(app) && $emit('resolve-task', cardTask(app)!)"
        />
      </div>
      <el-empty v-else :description="query ? '没有匹配的应用' : emptyDescriptions[filter]" :image-size="80">
        <el-button v-if="query" @click="search = ''">清除搜索</el-button>
        <el-button v-else @click="$emit('filter-change', 'all')">查看全部应用</el-button>
      </el-empty>
    </div>
  </section>
</template>

<style scoped>
.store-panel { display: flex; min-width: 0; min-height: 0; flex-direction: column; }
.store-scroll { overflow: auto; min-height: 0; flex: 1; padding: 23px 27px 35px; }
.store-empty { display: grid; overflow: auto; flex: 1; place-items: center; }
.selected-host { display: flex; align-items: center; justify-content: space-between; gap: 18px; flex-wrap: wrap; }
.selected-heading { display: flex; min-width: 0; align-items: center; gap: 12px; }
.selected-icon {
  display: grid;
  width: 49px;
  height: 49px;
  flex: none;
  place-items: center;
  border: 1px solid #33657d;
  border-radius: 14px;
  background: linear-gradient(145deg, #1c4159, #20314c);
  color: #a4e6f5;
  font-size: 24px;
}
.selected-info { min-width: 0; }
.selected-info h2 { overflow: hidden; margin: 0; font-size: 19px; letter-spacing: .2px; text-overflow: ellipsis; white-space: nowrap; }
.selected-info p { display: flex; align-items: center; gap: 8px; margin: 6px 0 0; color: #88abc6; font-size: 11px; line-height: 1.6; flex-wrap: wrap; }
.endpoint, .host-observed { overflow-wrap: anywhere; }
.host-observed { min-width: 0; max-width: 100%; display: grid; gap: 4px; color: var(--ent-dim); font-size: 10px; text-align: right; }
.host-observed strong { color: var(--ent-muted); font-weight: 500; }
.metrics { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 18px; margin: 14px 0 0; }
.metrics div { display: inline-flex; align-items: baseline; gap: 6px; }
.metrics dt { color: var(--ent-muted); font-size: 12px; }
.metrics dd { margin: 0; color: var(--ent-text); font-weight: 700; font-variant-numeric: tabular-nums; }
.notice-line { margin-top: 12px; }
.store-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 20px 0 10px; flex-wrap: wrap; }
.store-heading h3 { margin: 0; font-size: 16px; font-weight: 720; }
.store-filters { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 12px; margin-bottom: 14px; }
.app-search { width: min(280px, 100%); }
.market-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 260px), 1fr)); align-items: stretch; gap: 13px; }
.market-loading { padding: 18px; border: 1px solid var(--ent-line); border-radius: 12px; background: var(--ent-panel); }
@media (max-width: 1110px) { .store-scroll { padding: 18px; } }
@media (max-width: 760px) {
  .store-scroll { padding: 14px; }
  .selected-host { justify-content: flex-start; gap: 8px 16px; }
  .host-observed { display: flex; align-items: baseline; flex-wrap: wrap; text-align: left; }
  .app-search { flex: 1 1 200px; }
}
</style>
