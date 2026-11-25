<script setup lang="ts">
import { computed } from 'vue'
import { Box, Delete, Edit, Connection, MoreFilled } from '@element-plus/icons-vue'
import type { ExecutionTask, Host, HostHealthStatus } from '../types'
import { formatTaskTime, taskSourceLabel, taskStateLabel } from '../task-status'

const props = defineProps<{
  host: Host
  selected: boolean
  healthStatus: HostHealthStatus
  task?: ExecutionTask
}>()

defineEmits<{
  select: [host: Host]
  edit: [host: Host]
  docker: [host: Host]
  delete: [host: Host]
  network: [host: Host]
}>()

const statusLabel = computed(() => ({
  online: '已连接',
  checking: '检测中',
  error: '连接异常',
  paired: '已配对',
  unpaired: '未配对'
})[props.healthStatus])

const statusType = computed(() => ({
  online: 'success',
  checking: 'warning',
  error: 'danger',
  paired: 'info',
  unpaired: 'info'
})[props.healthStatus] as 'success' | 'warning' | 'danger' | 'info')
</script>

<template>
  <article class="host-card" :class="{ active: selected }">
    <div class="host-head">
      <div class="host-emblem"><el-icon><Box /></el-icon></div>
      <button class="host-select" type="button" :aria-label="'选择主机 ' + host.name" :aria-pressed="selected"
        @click="$emit('select', host)">
        <span class="host-name">{{ host.name }}</span>
        <span class="host-address" :title="host.endpoint">{{ host.endpoint }}</span>
      </button>
      <el-dropdown trigger="click" placement="bottom-end">
        <el-button class="host-menu" text :aria-label="host.name + '的主机操作'" title="主机操作">
          <el-icon><MoreFilled /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item :icon="Edit" @click="$emit('edit', host)">编辑主机</el-dropdown-item>
            <el-dropdown-item :icon="Box" :disabled="!host.agent_id" @click="$emit('docker', host)">Docker 资源</el-dropdown-item>
            <el-dropdown-item :icon="Connection" :disabled="!host.agent_id" @click="$emit('network', host)">
              {{ host.network_entry ? '取消组网入口' : '设为组网入口' }}
            </el-dropdown-item>
            <el-dropdown-item divided :icon="Delete" class="delete-action" @click="$emit('delete', host)">删除主机</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
    <div class="host-tags">
      <el-tag size="small" effect="dark" :type="statusType">{{ statusLabel }}</el-tag>
      <el-tag v-if="host.network_entry" size="small" type="success">组网入口</el-tag>
      <el-tag v-if="host.agent_id" size="small" effect="plain" type="info">Agent</el-tag>
    </div>
    <div v-if="task" class="host-task" role="status">
      <template v-if="task.state === 'succeeded'">
        最近任务：{{ formatTaskTime(task) }}
      </template>
      <template v-else>
        <span class="host-task-title">{{ taskStateLabel(task.state) }} · {{ task.id }}</span>
        <span class="host-task-source">{{ taskSourceLabel(task.trigger_source) }}</span>
        <span v-if="task.message" class="host-task-message">{{ task.message }}</span>
      </template>
    </div>
  </article>
</template>

<style scoped>
.host-card {
  position: relative;
  display: block;
  width: 100%;
  margin-bottom: 10px;
  padding: 13px 12px;
  border: 1px solid #21364a;
  border-radius: 13px;
  background: #0e1c2e;
  color: var(--ent-text);
  cursor: pointer;
  text-align: left;
  transition: border-color .18s, background .18s, box-shadow .18s;
}
.host-card:hover { border-color: #34738c; background: #11263c; }
.host-card.active {
  border-color: #3b8fad;
  background: linear-gradient(110deg, #133549, #10263c);
  box-shadow: inset 3px 0 0 var(--ent-cyan), 0 0 22px #17abc411;
}
.host-head { display: flex; align-items: flex-start; gap: 10px; }
.host-emblem {
  display: grid;
  width: 37px;
  height: 37px;
  flex: none;
  place-items: center;
  border: 1px solid #2a5a79;
  border-radius: 10px;
  background: #203d58;
  color: #79d7f2;
  font-size: 19px;
}
.host-select { flex: 1; min-width: 0; border: 0; padding: 0; background: transparent; color: inherit; text-align: left; }
/* 主机卡片整体可点，操作菜单单独接收点击和键盘焦点。 */
.host-select::after { position: absolute; inset: 0; border-radius: 13px; content: ''; }
.host-select:focus-visible::after { outline: 2px solid var(--ent-cyan); outline-offset: 1px; }
.host-head :deep(.el-dropdown) { position: relative; }
.host-menu { width: 34px; height: 34px; padding: 0; color: var(--ent-muted); }
.delete-action { color: var(--ent-red); }
.host-name { display: block; overflow: hidden; font-size: 14px; font-weight: 730; text-overflow: ellipsis; white-space: nowrap; }
.host-address { display: block; overflow: hidden; margin-top: 5px; color: #8bacca; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.host-task { margin-top: 9px; color: #93b4ca; font-size: 10px; line-height: 1.6; overflow-wrap: anywhere; }
.host-task-title { display: block; color: #e1e9f1; }
.host-task-source, .host-task-message { display: block; }
.host-task-message { color: #f8b2a8; }
.host-tags { display: flex; margin-top: 10px; align-items: center; gap: 5px; flex-wrap: wrap; }
</style>
