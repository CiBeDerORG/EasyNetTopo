<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowDown, Plus, Refresh, Search } from '@element-plus/icons-vue'
import HostCard from './HostCard.vue'
import type { ExecutionTask, Host, HostHealthStatus } from '../types'
import { currentHostTask } from '../task-status'

const props = defineProps<{
  hosts: Host[]
  tasks: ExecutionTask[]
  selectedHostId: string
  selectedHealthStatus: HostHealthStatus
  refreshing: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  add: []
  refresh: []
  edit: [host: Host]
  docker: [host: Host]
  delete: [host: Host]
  network: [host: Host]
}>()

const search = ref('')
const hostListOpen = ref(false)
const mobileSwitch = ref<HTMLButtonElement>()
const selectedHost = computed(() => props.hosts.find(host => host.id === props.selectedHostId))

function selectHost(id: string) {
  emit('select', id)
  if (hostListOpen.value && mobileSwitch.value?.offsetWidth) {
    hostListOpen.value = false
    mobileSwitch.value.focus()
  }
}

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
  <section class="hosts-panel" :class="{ expanded: hostListOpen }" aria-label="主机列表">
    <div class="pane-head">
      <h2 class="pane-heading">主机列表 <span class="count">{{ hosts.length }}</span></h2>
      <button ref="mobileSwitch" type="button" class="mobile-switch" aria-controls="host-picker"
        :aria-expanded="hostListOpen" @click="hostListOpen = !hostListOpen">
        <span>切换主机</span>
        <strong>{{ selectedHost?.name || '选择主机' }}</strong>
        <el-icon><ArrowDown /></el-icon>
      </button>
      <div class="pane-actions">
        <el-tooltip content="刷新主机与应用状态" placement="top">
          <el-button class="glyph-btn" text circle :loading="refreshing" aria-label="刷新主机与应用状态" @click="$emit('refresh')">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
        <el-button class="primary" :icon="Plus" aria-label="添加主机" @click="$emit('add')">添加</el-button>
      </div>
    </div>

    <div id="host-picker" class="host-picker">
      <el-input v-model="search" class="host-search" clearable placeholder="搜索名称 / IP / Agent ID" aria-label="搜索主机">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>

      <div class="host-scroll">
        <HostCard
          v-for="host in filteredHosts"
          :key="host.id"
          :host="host"
          :task="currentHostTask(tasks, host.id)"
          :selected="host.id === selectedHostId"
          :health-status="host.id === selectedHostId ? selectedHealthStatus : host.agent_id ? 'paired' : 'unpaired'"
          @select="selectHost($event.id)"
          @edit="$emit('edit', $event)"
          @docker="$emit('docker', $event)"
          @network="$emit('network', $event)"
          @delete="$emit('delete', $event)"
        />
        <el-empty v-if="filteredHosts.length === 0" :description="hosts.length ? '没有匹配的主机' : '尚未添加主机'" :image-size="76">
          <el-button v-if="hosts.length === 0" type="primary" @click="$emit('add')">添加第一台主机</el-button>
          <el-button v-else @click="search = ''">清除搜索</el-button>
        </el-empty>
      </div>
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
.pane-heading { margin: 0; font-size: 15px; font-weight: 720; }
.mobile-switch { display: none; }
.host-picker { display: flex; min-height: 0; flex: 1; flex-direction: column; }
.count { margin-left: 2px; padding: 2px 6px; border: 1px solid #285367; border-radius: 20px; background: #102e3e; color: var(--ent-cyan); font-size: 11px; }
.pane-actions { display: flex; flex: none; align-items: center; gap: 5px; }
.pane-actions :deep(.el-button) { min-width: 34px; height: 34px; margin: 0; border: 1px solid #2a4056; border-radius: 10px; background: #112237; color: #acc9e1; }
.pane-actions :deep(.el-button:hover) { border-color: #3d879b; background: #17304a; color: var(--ent-cyan); }
.pane-actions :deep(.el-button.primary) { border-color: #77cfe9; background: linear-gradient(135deg, #48e1ea, #64a5ff); color: #022330; }
/* ElInput 默认宽度为 100%，加上左右 margin 会越过主机列表边界。 */
.host-search { width: calc(100% - 32px); max-width: calc(100% - 32px); margin: 0 16px 15px; }
.host-search :deep(.el-input__wrapper) { min-height: 40px; border-radius: 10px; }
.host-search :deep(.el-input__inner) { font-size: 12px; }
.host-scroll { overflow: auto; min-height: 0; flex: 1; padding: 0 11px 15px; }
.host-scroll :deep(.el-empty) { padding: 30px 0; }
@media (max-width: 1110px) {
  .pane-head { flex-wrap: wrap; padding-inline: 12px; }
}
@media (max-width: 760px) {
  .hosts-panel { max-height: min(45dvh, 320px); border-right: 0; border-bottom: 1px solid var(--ent-line); }
  .pane-head { flex-wrap: nowrap; padding: 8px 14px; }
  .pane-heading, .host-picker { display: none; }
  .expanded .host-picker { display: flex; }
  .mobile-switch {
    display: flex;
    min-width: 0;
    min-height: 38px;
    align-items: center;
    gap: 7px;
    border: 0;
    padding: 0;
    background: transparent;
    color: var(--ent-text);
    text-align: left;
  }
  .mobile-switch span { flex: none; color: var(--ent-muted); font-size: 12px; }
  .mobile-switch strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .mobile-switch .el-icon { color: var(--ent-cyan); }
  .expanded .mobile-switch .el-icon { transform: rotate(180deg); }
  .host-search { margin-bottom: 8px; }
  .host-scroll { padding-inline: 14px; }
}
</style>
