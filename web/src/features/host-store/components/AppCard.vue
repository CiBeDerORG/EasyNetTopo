<script setup lang="ts">
import { computed } from 'vue'
import { Delete, Download, InfoFilled, Lightning, Refresh, WarningFilled } from '@element-plus/icons-vue'
import type { AppAction, AppPermissions, AppStatus, CatalogApp } from '../types'

const props = defineProps<{
  app: CatalogApp
  status?: AppStatus
  sourceName: string
  permissions: AppPermissions
  busy: boolean
}>()

defineEmits<{
  configure: []
  action: [action: AppAction]
  info: []
}>()

const logoText = computed(() => {
  if (props.app.app_id === 'sub2api') return 'S2'
  if (props.app.app_id === 'portainer') return 'P'
  return props.app.name.slice(0, 2).toUpperCase()
})
const logoTone = computed(() => {
  if (props.app.app_id === 'sub2api') return 'cyan'
  if (props.app.app_id === 'portainer') return 'violet'
  if (props.app.app_id === 'openhands') return 'amber'
  return 'green'
})
const statusLabel = computed(() => {
  if (!props.status) return '状态待获取'
  return ({
    running: '运行中',
    stopped: '已停止',
    partial: '部分运行',
    not_installed: '未安装',
    unknown: '状态未知',
    missing: '容器缺失',
    external: '外部项目'
  } as Record<string, string>)[props.status.state] || props.status.state
})
const statusType = computed(() => {
  if (props.status?.state === 'running') return 'success'
  if (props.status?.state === 'stopped' || props.status?.state === 'partial') return 'warning'
  if (props.status?.state === 'unknown' || props.status?.state === 'missing') return 'danger'
  return 'info'
})
const resultLabel = computed(() => {
  if (!props.status?.result) return ''
  const action = ({ install: '安装', update: '更新', uninstall: '卸载', adopt: '接管' } as Record<string, string>)[props.status.action || ''] || '操作'
  const result = ({ running: '进行中', succeeded: '成功', failed: '失败', unknown: '结果未知' } as Record<string, string>)[props.status.result] || props.status.result
  return '最近' + action + ' · ' + result + (props.status.message ? ' · ' + props.status.message : '')
})
</script>

<template>
  <article class="product-card">
    <div class="product-top">
      <div class="product-logo" :class="logoTone">{{ logoText }}</div>
      <div class="product-title">
        <h4>{{ app.name }}</h4>
        <span>{{ app.category || '应用' }} · {{ sourceName }}</span>
      </div>
      <el-tooltip content="应用信息" placement="top">
        <el-button text circle size="small" aria-label="应用信息" @click="$emit('info')">
          <el-icon><InfoFilled /></el-icon>
        </el-button>
      </el-tooltip>
    </div>

    <p class="product-desc">{{ app.summary || '暂无应用简介' }}</p>

    <div class="product-meta">
      <el-tag size="small" effect="dark" :type="statusType">{{ statusLabel }}</el-tag>
      <span v-if="app.revision" class="revision">版本 {{ app.revision.slice(0, 12) }}</span>
      <span v-else class="revision">本机保留记录</span>
      <el-tooltip
        v-if="permissions.blockReason"
        :content="permissions.blockReason"
        :popper-style="{ maxWidth: '300px', whiteSpace: 'normal', overflowWrap: 'anywhere' }"
        placement="top"
      >
        <span class="restriction-icon" role="img" tabindex="0" :aria-label="permissions.blockReason">
          <el-icon><WarningFilled /></el-icon>
        </span>
      </el-tooltip>
    </div>
    <p v-if="resultLabel" class="result-label" role="status">{{ resultLabel }}</p>
    <div class="product-bottom">
      <div class="app-actions">
        <el-button v-if="permissions.showInstall" size="small" type="primary" :disabled="busy || !permissions.canInstall" @click="$emit('configure')">
          <el-icon><Download /></el-icon>{{ status?.state === 'missing' ? '重新安装' : '安装' }}
        </el-button>
        <el-button v-if="permissions.showForceUpdate" size="small" type="warning" :disabled="busy || !permissions.canForceUpdate" @click="$emit('action', 'force_update')">
          <el-icon><Lightning /></el-icon>强制更新
        </el-button>
        <el-button v-if="permissions.showUpdate" size="small" :disabled="busy || !permissions.canUpdate" @click="$emit('action', 'update')">
          <el-icon><Refresh /></el-icon>更新
        </el-button>
        <el-button v-if="permissions.showUninstall" size="small" type="danger" plain :disabled="busy || !permissions.canUninstall" @click="$emit('action', 'uninstall')">
          <el-icon><Delete /></el-icon>卸载
        </el-button>
      </div>
    </div>
  </article>
</template>

<style scoped>
.product-card {
  display: flex;
  min-width: 0;
  min-height: 220px;
  flex-direction: column;
  padding: 16px;
  border: 1px solid #263e54;
  border-radius: 13px;
  background: linear-gradient(145deg, #102136, #0d1c2e);
  transition: border-color .18s, transform .18s, box-shadow .18s;
}
.product-card:hover { transform: translateY(-1px); border-color: #3a7393; box-shadow: 0 10px 22px #0003; }
.product-top { display: flex; min-width: 0; align-items: flex-start; gap: 11px; }
.product-logo {
  display: grid;
  width: 43px;
  height: 43px;
  flex: none;
  place-items: center;
  border: 1px solid #3f617e;
  border-radius: 12px;
  background: #20314b;
  color: #a6c8fa;
  font-size: 16px;
  font-weight: 800;
  letter-spacing: -1px;
}
.product-logo.cyan { border-color: #216a76; background: #143a44; color: #65e3e6; }
.product-logo.violet { border-color: #555184; background: #29284c; color: #b3a5ff; }
.product-logo.amber { border-color: #7f5738; background: #45301d; color: #ffd19d; }
.product-logo.green { border-color: #366c53; background: #16382f; color: #9cecc8; }
.product-title { min-width: 0; flex: 1; padding-top: 2px; }
.product-title h4 { overflow: hidden; margin: 0; font-size: 14px; font-weight: 740; text-overflow: ellipsis; white-space: nowrap; }
.product-title span { display: block; overflow: hidden; margin-top: 5px; color: #789bb6; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.product-desc { display: -webkit-box; overflow: hidden; min-height: 40px; max-height: 44px; margin: 12px 0; color: #87a5bf; font-size: 11px; line-height: 1.7; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.product-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.revision { color: #617f97; font-size: 10px; }
.restriction-icon {
  display: inline-flex;
  width: 22px;
  height: 22px;
  flex: none;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--ent-amber);
  cursor: help;
}
button.restriction-icon { cursor: pointer; }
.restriction-icon:hover,
.restriction-icon:focus-visible { background: #80603430; color: #ffd496; }
.result-label { margin: 8px 0 0; color: var(--ent-muted); font-size: 10px; line-height: 1.5; }
.product-bottom { margin-top: auto; border-top: 1px solid #243b50; padding-top: 12px; }
.app-actions { display: flex; align-items: center; gap: 5px; flex-wrap: wrap; }
.app-actions :deep(.el-button) { margin: 0; }
@media (max-width: 630px) {
  .product-card { min-height: 0; }
}
</style>
