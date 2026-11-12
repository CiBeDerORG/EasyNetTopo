<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { Plus, Refresh, Search, Setting, Radar } from '@element-plus/icons-vue'
import HostCard from './HostCard.vue'
import type { ExecutionTask, Host, HostHealthStatus } from '../types'
import { currentHostTask } from '../task-status'

const props = defineProps<{
  hosts: Host[]
  tasks: ExecutionTask[]
  selectedHostId: string
  selectedHealthStatus: HostHealthStatus
  refreshing: boolean
  discoveryEnabled?: boolean
  candidateCount?: number
}>()

defineEmits<{
  select: [id: string]
  add: []
  refresh: []
  settings: []
  discovery: []
  edit: [host: Host]
  docker: [host: Host]
  delete: [host: Host]
  network: [host: Host]
}>()

const search = ref('')
const clock = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { clock.value = Date.now() }, 1000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
const filteredHosts = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  if (!query) return props.hosts
  return props.hosts.filter(host =>
    [host.name, host.endpoint, host.agent_id].some(value =>
      (value || '').toLocaleLowerCase().includes(query)
    )
  )
})
</script>

<template>
  <section class="hosts-panel" aria-label="主机列表">
    <div class="pane-head">
      <div>
        <div class="pane-heading">主机列表 <span class="count">{{ String(hosts.length).padStart(2, '0') }}</span></div>
        <div class="pane-caption">选择主机以查看可安装应用</div>
      </div>
      <div class="pane-actions">
        <el-tooltip content="刷新主机与应用状态" placement="top">
          <el-button class="glyph-btn" text circle :loading="refreshing" aria-label="刷新主机与应用状态" @click="$emit('refresh')">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="主机商店设置 · 应用源" placement="top">
          <el-button class="glyph-btn" text circle aria-label="应用源设置" @click="$emit('settings')">
            <el-icon><Setting /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip :content="'主机发现' + (candidateCount ? ' · ' + candidateCount + ' 台待确认' : '')" placement="top">
          <el-button class="glyph-btn" text circle aria-label="主机发现" @click="$emit('discovery')">
            <el-icon><Radar /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="添加主机" placement="top">
          <el-button class="glyph-btn primary" circle aria-label="添加主机" @click="$emit('add')">
            <el-icon><Plus /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </div>

    <el-input v-model="search" class="host-search" clearable placeholder="搜索名称 / IP / Agent ID" aria-label="搜索主机">
      <template #prefix><el-icon><Search /></el-icon></template>
    </el-input>

    <div class="host-scroll">
      <HostCard
        v-for="host in filteredHosts"
        :key="host.id"
        :host="host"
        :now="clock"
        :task="currentHostTask(tasks, host.id)"
        :selected="host.id === selectedHostId"
        :health-status="host.id === selectedHostId ? selectedHealthStatus : host.agent_id ? 'paired' : 'unpaired'"
        @select="$emit('select', $event.id)"
        @edit="$emit('edit', $event)"
        @docker="$emit('docker', $event)"
        @network="$emit('network', $event)"
        @delete="$emit('delete', $event)"
      />
      <el-empty v-if="filteredHosts.length === 0" :description="hosts.length ? '没有匹配的主机' : '尚未添加主机'" :image-size="76">
        <el-button v-if="hosts.length === 0" type="primary" @click="$emit('add')">添加第一台主机</el-button>
      </el-empty>
    </div>
  </section>
</template>

<style scoped>
.hosts-panel {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  border-right: 1px solid var(--ent-line);
  background: #0a1727e8;
}
.pane-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 21px 17px 14px; }
.pane-heading { font-size: 15px; font-weight: 720; }
.pane-caption { margin-top: 5px; color: var(--ent-muted); font-size: 11px; }
.count { margin-left: 7px; padding: 3px 9px; border: 1px solid #285367; border-radius: 20px; background: #102e3e; color: var(--ent-cyan); font-size: 11px; }
.pane-actions { display: flex; flex: none; align-items: center; gap: 5px; }
.pane-actions :deep(.el-button) { width: 34px; height: 34px; border: 1px solid #2a4056; border-radius: 10px; background: #112237; color: #acc9e1; }
.pane-actions :deep(.el-button:hover) { border-color: #3d879b; background: #17304a; color: var(--ent-cyan); }
.pane-actions :deep(.el-button.primary) { border-color: #77cfe9; background: linear-gradient(135deg, #48e1ea, #64a5ff); color: #022330; }
/* ElInput 默认宽度为 100%，加上左右 margin 会越过主机列表边界。 */
.host-search { width: calc(100% - 32px); max-width: calc(100% - 32px); margin: 0 16px 15px; }
.host-search :deep(.el-input__wrapper) { min-height: 40px; border-radius: 10px; }
.host-search :deep(.el-input__inner) { font-size: 12px; }
.host-scroll { overflow: auto; min-height: 0; flex: 1; padding: 0 11px 15px; }
.host-scroll :deep(.el-empty) { padding: 30px 0; }
@media (max-width: 1110px) {
  /* 窄布局时工具按钮换行，避免顶栏挤出主机侧栏。 */
  .pane-head { flex-wrap: wrap; }
  .pane-actions { margin-left: auto; }
}
@media (max-width: 630px) {
  .hosts-panel { max-height: 220px; border-right: 0; border-bottom: 1px solid var(--ent-line); }
  .pane-head { padding: 12px 16px 8px; }
  .host-scroll { display: flex; overflow: auto; gap: 10px; padding: 0 12px 12px; }
  .host-scroll :deep(.host-card) { width: 230px; min-width: 230px; margin: 0; }
  .host-scroll :deep(.el-empty) { min-width: 100%; padding: 10px; }
  .host-search { margin-bottom: 9px; }
}
</style>
