<script setup lang="ts">
import { computed } from 'vue'
import { Box, CircleCheck, Download, TrendCharts } from '@element-plus/icons-vue'
import AppCard from './AppCard.vue'
import NativeAgentCard from './NativeAgentCard.vue'
import { appKey, canRecoverFailedUninstall, getAppPermissions, recoveredUninstallStatus } from '../rules'
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
  busyKey: string
  pendingKey: string
  agentMaintenance: AgentMaintenanceState | null
  agentUpdateBusy: boolean
}>()

defineEmits<{
  'filter-change': [filter: AppFilter]
  configure: [app: CatalogApp]
  action: [app: CatalogApp, action: AppAction]
  info: [app: CatalogApp]
  'resolve-task': [task: ExecutionTask]
  'agent-update': [action: AgentUpdateAction]
  'easytier-config': []
}>()

function recovered(row: AppStatus): boolean {
  return !!props.host &&
    props.recoveredUninstallCards.includes(props.host.id + '|' + appKey(row)) &&
    canRecoverFailedUninstall(row, props.dockerInventory, row.app_id)
}
const statusByKey = computed<Record<string, AppStatus>>(() => Object.fromEntries(props.rows.map(row =>
  [appKey(row), recovered(row) ? recoveredUninstallStatus(row) : row]
)))
const installedCount = computed(() => Object.values(statusByKey.value)
  .filter(row => row.managed || row.external).length + (props.snapshot ? 1 : 0))
const runningCount = computed(() => Object.values(statusByKey.value)
  .filter(row => row.state === 'running').length + (props.snapshot ? 1 : 0))
const availableCount = computed(() => props.apps.filter(app => !!app.revision).length + (props.host?.host_type === 'temporary' ? 0 : 1))
const agentTask = computed(() => props.host ? currentAppTask(props.tasks, props.host.id, 'native', 'easyhostagent') : undefined)
const agentState = computed(() => {
  if (props.host?.host_type === 'permanent' && props.host.connection_mode === 'reverse') {
    return { label: '已转正 · 等待组网直连', type: 'warning' as const }
  }
  if (props.host?.host_type === 'temporary') {
    const seen = props.host.last_seen_at ? new Date(props.host.last_seen_at).getTime() : 0
    return seen && Date.now() - seen < 60_000
      ? { label: '反向在线', type: 'success' as const }
      : { label: '等待 Agent 报到', type: 'warning' as const }
  }
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
  props.host?.connection_mode !== 'reverse' && (props.filter === 'all' || props.filter === 'installed' && !!props.snapshot ||
  props.filter === 'available' && !props.snapshot ||
  props.filter === 'update' && !!props.snapshot)
)
function appPermissions(app: CatalogApp) {
  const row = statusByKey.value[appKey(app)]
  const platform = props.snapshot?.platform || props.host?.platform || ''
  const base = getAppPermissions(app, row, platform)
  if (props.host?.host_type === 'permanent' && props.host.connection_mode === 'reverse') {
    return {
      ...base,
      canInstall: false, canUpdate: false, canUninstall: false,
      canAdopt: false, canForceUpdate: false,
      blockReason: '等待 EasyTier 组网 IP 的直连身份与授权验证'
    }
  }
  if (props.host?.host_type !== 'temporary') return base
  return {
    ...base,
    showInstall: !!app.revision,
    canInstall: !!app.revision,
    showUpdate: false,
    canUpdate: false,
    showUninstall: false,
    canUninstall: false,
    showAdopt: false,
    canAdopt: false,
    showForceUpdate: false,
    canForceUpdate: false,
    grantRequired: false,
    blockReason: ''
  }
}
const visibleApps = computed(() => props.apps.filter(app => {
  const row = statusByKey.value[appKey(app)]
  const permissions = appPermissions(app)
  if (props.host?.host_type === 'temporary') {
    if (props.filter === 'installed' || props.filter === 'update') return false
    return !!app.revision
  }
  if (props.filter === 'installed') return !!row && (row.managed || row.external)
  if (props.filter === 'available') return !row || (!row.managed && !row.external)
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
      <el-empty description="请先添加并选择一台主机" :image-size="110" />
    </div>
    <div v-else class="store-scroll">
      <div class="selected-host">
        <div class="selected-heading">
          <div class="selected-icon"><el-icon><Box /></el-icon></div>
          <div class="selected-info">
            <h2>{{ host.name }}</h2>
            <p>
              {{ host.connection_mode === 'reverse' ? '反向接入' : host.endpoint }}
              <span v-if="snapshot"> · {{ snapshot.platform }} · {{ snapshot.arch }}</span>
              <span v-else-if="host.host_type === 'temporary'"> · {{ host.platform }} · {{ host.arch }}</span>
              <el-tag size="small" effect="dark" :type="agentState.type">{{ agentState.label }}</el-tag>
            </p>
          </div>
        </div>
        <div v-if="currentHostTask(tasks, host.id)" class="host-observed" role="status">
          <template v-if="currentHostTask(tasks, host.id)?.state === 'succeeded'">
            <span>最近任务</span>
            <strong>{{ formatTaskTime(currentHostTask(tasks, host.id)!) }}</strong>
          </template>
          <template v-else>
            <span>{{ taskStateLabel(currentHostTask(tasks, host.id)!.state) }}</span>
            <strong>{{ currentHostTask(tasks, host.id)?.id }}</strong>
            <span>{{ taskSourceLabel(currentHostTask(tasks, host.id)!.trigger_source) }}</span>
          </template>
          <span>触发时间：{{ formatTaskTriggerTime(currentHostTask(tasks, host.id)!) }}</span>
        </div>
        <div v-if="snapshot" class="host-observed">
          <span>最近采集</span>
          <strong>{{ new Date(snapshot.observed_at).toLocaleString() }}</strong>
        </div>
      </div>

      <div class="metrics">
        <div class="metric">
          <div class="metric-caption"><el-icon><Download /></el-icon>已安装应用</div>
          <div class="metric-num">{{ String(installedCount).padStart(2, '0') }}</div>
          <div class="metric-foot">目标主机当前部署</div>
        </div>
        <div class="metric">
          <div class="metric-caption"><el-icon><TrendCharts /></el-icon>运行中</div>
          <div class="metric-num green">{{ String(runningCount).padStart(2, '0') }}</div>
          <div class="metric-foot">Docker 与原生应用运行状态</div>
        </div>
        <div class="metric">
          <div class="metric-caption"><el-icon><Box /></el-icon>可用应用</div>
          <div class="metric-num cyan">{{ String(availableCount).padStart(2, '0') }}</div>
          <div class="metric-foot">来自已同步的应用源</div>
        </div>
        <div class="metric">
          <div class="metric-caption"><el-icon><CircleCheck /></el-icon>Agent 状态</div>
          <div class="metric-num agent" :class="{ green: agentState.type === 'success' }">{{ agentState.label }}</div>
          <div class="metric-foot">{{ snapshot ? snapshot.platform + ' · ' + snapshot.arch : host.host_type === 'temporary' ? '固定 30 分钟临时接入' : '通过配对身份安全连接' }}</div>
        </div>
      </div>

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
        <div>
          <h3>应用市场</h3>
          <p>{{ host.host_type === 'temporary' ? '临时主机仅支持预安装；配置保存后由 Agent 主动领取并执行' : 'Docker 应用按可信来源执行；EasyHostAgent 支持在线更新与强制更新' }}</p>
        </div>
        <el-radio-group :model-value="filter" size="small" @update:model-value="$emit('filter-change', $event)">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="installed">已安装</el-radio-button>
          <el-radio-button label="available">未安装</el-radio-button>
          <el-radio-button label="update">可更新</el-radio-button>
        </el-radio-group>
      </div>

      <div v-if="loading && rows.length === 0" class="market-loading">
        <el-skeleton :rows="4" animated />
      </div>
      <div v-else-if="visibleApps.length || showNativeAgent" class="market-grid">
        <NativeAgentCard
          v-if="showNativeAgent"
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
          :permissions="appPermissions(app)"
          :temporary="host.host_type === 'temporary'"
          :install-label="host.host_type === 'temporary' ? '预安装' : undefined"
          :config-url="app.app_id === 'easytier' ? host.easytier_config_url || '' : ''"
          :busy="busyKey === appKey(app) || pendingKey === appKey(app) || agentMaintenance?.state === 'waiting' || taskActive(agentTask) || taskActive(cardTask(app))"
          @configure="$emit('configure', app)"
          @action="$emit('action', app, $event)"
          @info="$emit('info', app)"
          @config="$emit('easytier-config')"
          @resolve="cardTask(app) && $emit('resolve-task', cardTask(app)!)"
        />
      </div>
      <el-empty v-else :description="filter === 'update' ? '当前没有可更新的应用' : '暂无应用，请先在主机商店设置中添加并同步 Git 应用源'" :image-size="100" />

      <p class="side-note">
        <strong>安全提示：</strong>Docker 应用操作需要主机已配对及可信应用源，无需逐个授权。原生 Agent 的更新由系统独立维护任务执行，短暂断联正常；状态未知或身份不匹配时禁止远程操作。
      </p>
    </div>
  </section>
</template>

<style scoped>
.store-panel { display: flex; min-width: 0; min-height: 0; flex-direction: column; }
.store-scroll { overflow: auto; min-height: 0; flex: 1; padding: 23px 27px 35px; }
.store-empty { display: grid; min-height: 100%; place-items: center; }
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
.host-observed { display: grid; gap: 4px; color: var(--ent-dim); font-size: 10px; text-align: right; }
.host-observed strong { color: var(--ent-muted); font-weight: 500; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 11px; margin-top: 21px; }
.metric { min-height: 84px; padding: 13px 15px; border: 1px solid #20364a; border-radius: 12px; background: linear-gradient(135deg, #101f31, #0e1b2b); }
.metric-caption { display: flex; align-items: center; gap: 8px; color: #88a8c3; font-size: 11px; }
.metric-caption .el-icon { font-size: 14px; }
.metric-num { margin-top: 7px; color: var(--ent-text); font-size: 25px; font-weight: 750; letter-spacing: -.7px; line-height: 1; }
.metric-num.green { color: var(--ent-green); }
.metric-num.cyan { color: var(--ent-cyan); }
.metric-num.agent { margin-top: 11px; font-size: 18px; }
.metric-foot { overflow: hidden; margin-top: 6px; color: #5e7e9c; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.notice-line { margin-top: 12px; }
.store-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 27px 0 14px; flex-wrap: wrap; }
.store-heading h3 { margin: 0; font-size: 16px; font-weight: 720; }
.store-heading p { margin: 6px 0 0; color: var(--ent-muted); font-size: 11px; }
.market-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: stretch; gap: 13px; }
.market-loading { padding: 18px; border: 1px solid var(--ent-line); border-radius: 12px; background: var(--ent-panel); }
.side-note { margin: 19px 0; color: #617f97; font-size: 10px; line-height: 1.7; }
.side-note strong { color: #98cce1; font-weight: 650; }
@media (max-width: 1460px) { .market-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 1110px) { .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } .store-scroll { padding: 19px; } }
@media (max-width: 830px) { .market-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 630px) {
  .store-scroll { padding: 15px; }
  .metrics { gap: 7px; }
  .metric { padding: 11px; }
  .market-grid { grid-template-columns: 1fr; }
  .store-heading { align-items: flex-start; }
}
</style>
