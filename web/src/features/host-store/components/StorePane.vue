<script setup lang="ts">
import { computed } from 'vue'
import { Box, CircleCheck, Download, TrendCharts } from '@element-plus/icons-vue'
import AppCard from './AppCard.vue'
import { appKey, getAppPermissions } from '../rules'
import type { AppFilter } from '../../../stores/ui'
import type { AppAction } from '../types'
import type { AppStatus, CatalogApp, CatalogSource, Host, HostSnapshot } from '../types'

const props = defineProps<{
  host: Host | null
  snapshot: HostSnapshot | null
  snapshotLoading: boolean
  snapshotError: string
  apps: CatalogApp[]
  sources: CatalogSource[]
  rows: AppStatus[]
  loading: boolean
  statusError: string
  filter: AppFilter
  controllerId: string
  busyKey: string
  pendingKey: string
}>()

defineEmits<{
  'filter-change': [filter: AppFilter]
  configure: [app: CatalogApp]
  action: [app: CatalogApp, action: AppAction]
  info: [app: CatalogApp]
  'copy-grant': [value: string]
  'copy-command': [value: string]
}>()

const statusByKey = computed(() => Object.fromEntries(props.rows.map(row => [appKey(row), row])))
const installedCount = computed(() => props.rows.filter(row => row.managed || row.external).length)
const runningCount = computed(() => props.rows.filter(row => row.state === 'running').length)
const availableCount = computed(() => props.apps.filter(app => !!app.revision).length)
const agentState = computed(() => {
  if (!props.host?.agent_id) return { label: '未配对', type: 'info' as const }
  if (props.snapshotLoading) return { label: '检测中', type: 'warning' as const }
  if (props.snapshotError) return { label: '连接异常', type: 'danger' as const }
  if (props.snapshot) return { label: '已连接', type: 'success' as const }
  return { label: '已配对', type: 'info' as const }
})
const visibleApps = computed(() => props.apps.filter(app => {
  const row = statusByKey.value[appKey(app)]
  const permissions = getAppPermissions(app, row)
  if (props.filter === 'installed') return !!row && (row.managed || row.external)
  if (props.filter === 'available') return !row || (!row.managed && !row.external)
  if (props.filter === 'update') return permissions.canUpdate
  return true
}))
function sourceName(id: string) {
  return props.sources.find(source => source.id === id)?.name || id
}
function authorizationValue(app: CatalogApp) {
  return props.controllerId + '/' + app.source_id + '/' + app.app_id
}
function grantCommand(app: CatalogApp) {
  if (app.app_id !== 'sub2api' && app.app_id !== 'portainer') return ''
  const ids = props.apps
    .filter(item => item.source_id === app.source_id &&
      (item.app_id === 'sub2api' || item.app_id === 'portainer'))
    .map(item => item.app_id)
  const selected = Array.from(new Set(ids)).sort()
  return 'cd /vol3/1000/software/deploy && sudo bash deploy.sh easyhostagent grant ' +
    props.controllerId + ' ' + app.source_id + ' ' + (selected.length ? selected.join(' ') : app.app_id)
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
              {{ host.endpoint }}
              <span v-if="snapshot"> · {{ snapshot.platform }} · {{ snapshot.arch }}</span>
              <el-tag size="small" effect="dark" :type="agentState.type">{{ agentState.label }}</el-tag>
            </p>
          </div>
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
          <div class="metric-foot">Docker 应用运行状态</div>
        </div>
        <div class="metric">
          <div class="metric-caption"><el-icon><Box /></el-icon>可用应用</div>
          <div class="metric-num cyan">{{ String(availableCount).padStart(2, '0') }}</div>
          <div class="metric-foot">来自已同步的应用源</div>
        </div>
        <div class="metric">
          <div class="metric-caption"><el-icon><CircleCheck /></el-icon>Agent 状态</div>
          <div class="metric-num agent" :class="{ green: agentState.type === 'success' }">{{ agentState.label }}</div>
          <div class="metric-foot">{{ snapshot ? snapshot.platform + ' · ' + snapshot.arch : '通过配对身份安全连接' }}</div>
        </div>
      </div>

      <div v-if="snapshotError" class="notice-line" role="status">
        <el-alert title="暂时无法获取主机快照。连接失败不代表主机或 Docker 资源已删除。" type="warning" :closable="false" show-icon />
      </div>
      <div v-if="statusError" class="notice-line" role="alert">
        <el-alert :title="'应用状态未知。' + statusError" type="warning" :closable="false" show-icon />
      </div>
      <div v-if="!host.agent_id" class="notice-line">
        <el-alert title="该主机尚未配对 EasyHostAgent。请先编辑主机并完成身份核验与配对。" type="info" :closable="false" show-icon />
      </div>

      <div class="store-heading">
        <div>
          <h3>应用市场</h3>
          <p>安装、更新、卸载仅作用于当前选中的主机</p>
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
      <div v-else-if="visibleApps.length" class="market-grid">
        <AppCard
          v-for="app in visibleApps"
          :key="appKey(app)"
          :app="app"
          :status="statusByKey[appKey(app)]"
          :source-name="sourceName(app.source_id)"
          :permissions="getAppPermissions(app, statusByKey[appKey(app)])"
          :busy="busyKey === appKey(app) || pendingKey === appKey(app)"
          :authorization-value="authorizationValue(app)"
          :grant-command="grantCommand(app)"
          @configure="$emit('configure', app)"
          @action="$emit('action', app, $event)"
          @info="$emit('info', app)"
          @copy-grant="$emit('copy-grant', $event)"
          @copy-command="$emit('copy-command', $event)"
        />
      </div>
      <el-empty v-else :description="filter === 'update' ? '当前没有可更新的应用' : '暂无应用，请先在主机商店设置中添加并同步 Git 应用源'" :image-size="100" />

      <p class="side-note">
        <strong>安全提示：</strong>所有安装与管理操作均由目标主机 EasyHostAgent 按已授权的应用清单执行；状态未知、身份不匹配或未授权时不会开放操作。
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
