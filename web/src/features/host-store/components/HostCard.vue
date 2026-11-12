<script setup lang="ts">
import { computed } from 'vue'
import { Box, Delete, Edit, Connection } from '@element-plus/icons-vue'
import type { ExecutionTask, Host, HostHealthStatus } from '../types'
import { formatTaskTime, taskSourceLabel, taskStateLabel } from '../task-status'

const props = defineProps<{
  host: Host
  now?: number
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

function countdown(target?: string) {
  if (!target) return '—'
  const diff = Math.max(0, Math.ceil((new Date(target).getTime() - (props.now || Date.now())) / 1000))
  return String(Math.floor(diff / 60)).padStart(2, '0') + ':' + String(diff % 60).padStart(2, '0')
}
const reverseOnline = computed(() => props.host.host_type === 'temporary' &&
  !!props.host.last_seen_at && (props.now || Date.now()) - new Date(props.host.last_seen_at).getTime() <= 30000)
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
  <article
    class="host-card"
    :class="{ active: selected }"
    role="button"
    tabindex="0"
    :aria-label="'选择主机 ' + host.name"
    :aria-pressed="selected"
    @click="$emit('select', host)"
    @keydown.enter.prevent="$emit('select', host)"
    @keydown.space.prevent="$emit('select', host)"
  >
    <div class="host-head">
      <div class="host-emblem"><el-icon><Box /></el-icon></div>
      <div class="host-info">
        <div class="host-name">{{ host.name }}</div>
        <div class="host-address">{{ host.endpoint }}</div>
      </div>
      <span class="status-dot" :class="'status-' + healthStatus"></span>
    </div>

    <div class="host-bottom">
      <div v-if="host.host_type === 'temporary'" class="temporary-countdowns">
        <span>有效期 <strong>{{ countdown(host.expires_at) }}</strong></span>
        <span>下次领取 <strong>{{ host.next_poll_at && new Date(host.next_poll_at).getTime() < (now || Date.now()) - 15000 ? '等待报到' : countdown(host.next_poll_at) }}</strong></span>
      </div>
      <div class="host-tags">
        <el-tag size="small" effect="dark" :type="host.host_type === 'temporary' ? reverseOnline ? 'success' : 'warning' : statusType">
          {{ host.host_type === 'temporary' ? reverseOnline ? '临时 · 在线' : '临时 · 等待报到' : statusLabel }}
        </el-tag>
        <el-tag v-if="host.network_entry" size="small" type="success">组网入口</el-tag>
        <el-tag v-if="host.agent_id" size="small" effect="plain" type="info">Agent</el-tag>
      </div>
      <div class="host-ops" @click.stop @keydown.stop>
        <el-tooltip v-if="host.host_type !== 'temporary'" :content="host.network_entry ? '取消组网入口' : '设为组网入口'" placement="top">
          <el-button text circle size="small" :aria-label="host.network_entry ? '取消组网入口' : '设为组网入口'" :disabled="!host.agent_id" @click="$emit('network', host)"><el-icon><Connection /></el-icon></el-button>
        </el-tooltip>
        <el-tooltip v-if="host.host_type !== 'temporary'" content="编辑主机" placement="top">
          <el-button text circle size="small" aria-label="编辑主机" @click="$emit('edit', host)">
            <el-icon><Edit /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip v-if="host.host_type !== 'temporary'" content="Docker 资源" placement="top">
          <el-button text circle size="small" aria-label="Docker 资源" :disabled="!host.agent_id" @click="$emit('docker', host)">
            <el-icon><Box /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip :content="host.host_type === 'temporary' ? '主动剔除临时主机' : '删除主机'" placement="top">
          <el-button text circle size="small" class="delete-action" aria-label="删除主机" @click="$emit('delete', host)">
            <el-icon><Delete /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
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
.host-info { min-width: 0; flex: 1; }
.host-name { overflow: hidden; font-size: 12px; font-weight: 730; text-overflow: ellipsis; white-space: nowrap; }
.host-address { overflow: hidden; margin-top: 5px; color: #8bacca; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.status-dot { display: block; width: 7px; height: 7px; flex: none; margin-top: 6px; border-radius: 50%; background: #8594a5; }
.status-online { background: var(--ent-green); box-shadow: 0 0 10px #46ddb766; }
.status-checking, .status-paired { background: var(--ent-amber); box-shadow: 0 0 10px #ffbf735c; }
.status-error { background: var(--ent-red); box-shadow: 0 0 10px #ff788b66; }
.status-unpaired { background: #8594a5; }
.host-task { margin-top: 9px; color: #93b4ca; font-size: 10px; line-height: 1.6; overflow-wrap: anywhere; }
.host-task-title { display: block; color: #e1e9f1; }
.host-task-source, .host-task-message { display: block; }
.host-task-message { color: #f8b2a8; }
.host-bottom { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 6px; margin-top: 10px; }
.host-tags { display: flex; min-width: 0; align-items: center; gap: 5px; flex-wrap: wrap; }
.host-ops { display: flex; flex: none; align-items: center; gap: 0; }
.host-ops :deep(.el-button) { width: 27px; height: 27px; margin: 0; color: #82acc4; }
.host-ops :deep(.el-button:hover) { background: #25516c77; color: var(--ent-cyan); }
.host-ops :deep(.delete-action:hover) { background: #75293a55; color: var(--ent-red); }
.temporary-countdowns { display: flex; gap: 12px; margin-top: 10px; font-size: 10px; color: #8daec6; }
.temporary-countdowns strong { color: #f0c67b; font-variant-numeric: tabular-nums; }
</style>
