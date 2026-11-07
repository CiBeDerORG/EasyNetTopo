<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  cancelDockerTask as cancelDockerTaskApi,
  getDockerResource,
  getDockerTask as getDockerTaskApi,
  startDockerTask as startDockerTaskApi
} from '../api'
import type {
  DockerInventory, DockerResource, DockerResourceLogs, DockerResourceStats, DockerTask, Host
} from '../types'

const props = defineProps<{
  modelValue: boolean
  host: Host | null
  inventory: DockerInventory | undefined
  loading: boolean
  error: string
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  refresh: []
}>()

const selectedResourceId = ref('')
const resourceStats = ref<DockerResourceStats | null>(null)
const resourceLogs = ref<DockerResourceLogs | null>(null)
const dockerTask = ref<DockerTask | null>(null)
const busyResourceId = ref('')
const taskBusy = ref(false)
const actionError = ref('')
const taskActive = computed(() => !!dockerTask.value && ['accepted', 'running'].includes(dockerTask.value.status))

function clearDetails() {
  selectedResourceId.value = ''
  resourceStats.value = null
  resourceLogs.value = null
  dockerTask.value = null
  busyResourceId.value = ''
  taskBusy.value = false
  actionError.value = ''
}
watch(() => props.host?.id, clearDetails)
watch(() => props.modelValue, open => { if (!open) clearDetails() })

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value < 0) return '未知'
  return (value / 1024 / 1024).toFixed(1) + ' MiB'
}

async function queryResource(resource: DockerResource, detail: 'logs' | 'stats') {
  const host = props.host
  if (!host?.agent_id || !props.inventory?.complete || busyResourceId.value || taskBusy.value) return
  busyResourceId.value = resource.resource_id
  selectedResourceId.value = resource.resource_id
  resourceStats.value = null
  resourceLogs.value = null
  actionError.value = ''
  try {
    const result = await getDockerResource(host.id, resource.resource_id, detail, 100)
    if (detail === 'logs') resourceLogs.value = result as DockerResourceLogs
    else resourceStats.value = result as DockerResourceStats
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : 'Docker 资源详情查询失败'
  } finally {
    busyResourceId.value = ''
  }
}

async function startTask(resource: DockerResource, action: 'start' | 'stop' | 'restart') {
  const host = props.host
  if (!host?.agent_id || !props.inventory?.complete || !props.inventory.actions_supported ||
      !resource.managed || taskBusy.value || busyResourceId.value) return
  if (action !== 'start') {
    try {
      await ElMessageBox.confirm(
        '此操作会影响「' + resource.name + '」项目中的现有容器，确定继续吗？',
        '确认 Docker 操作',
        { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
  }
  taskBusy.value = true
  actionError.value = ''
  try {
    dockerTask.value = await startDockerTaskApi(host.id, {
      task_id: crypto.randomUUID(),
      kind: 'docker_resource',
      resource_id: resource.resource_id,
      action
    })
    ElMessage.success('Docker 操作任务已提交。请刷新任务状态确认执行结果。')
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : 'Docker 任务提交失败'
  } finally {
    taskBusy.value = false
  }
}

async function refreshTask() {
  const host = props.host
  const current = dockerTask.value
  if (!host?.agent_id || !current || taskBusy.value) return
  taskBusy.value = true
  actionError.value = ''
  try {
    dockerTask.value = await getDockerTaskApi(host.id, current.task_id)
    if (['succeeded', 'failed', 'cancelled'].includes(dockerTask.value.status)) emit('refresh')
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : '任务状态查询失败，请勿直接重复提交'
  } finally {
    taskBusy.value = false
  }
}

async function cancelTask() {
  const host = props.host
  const current = dockerTask.value
  if (!host?.agent_id || !current || !taskActive.value || taskBusy.value) return
  try {
    await ElMessageBox.confirm(
      '取消请求不能撤销已经完成的容器操作，确定请求取消吗？',
      '请求取消任务',
      { type: 'warning', confirmButtonText: '请求取消', cancelButtonText: '返回' }
    )
  } catch {
    return
  }
  taskBusy.value = true
  actionError.value = ''
  try {
    dockerTask.value = await cancelDockerTaskApi(host.id, current.task_id)
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : '取消请求失败，请刷新任务状态'
  } finally {
    taskBusy.value = false
  }
}
</script>

<template>
  <el-drawer :model-value="modelValue" :title="(host?.name || '主机') + ' · Docker 资源'"
    direction="rtl" size="min(560px, 100vw)" append-to-body
    @update:model-value="$emit('update:modelValue', $event)">
    <template v-if="host">
      <div class="drawer-summary">
        <span>{{ host.endpoint }}</span>
        <el-tag v-if="inventory" size="small" :type="inventory.complete ? 'success' : 'warning'">
          {{ inventory.complete ? '查询完整' : '查询不完整' }}
        </el-tag>
      </div>
      <el-alert v-if="error" :title="error" type="warning" :closable="false" show-icon />
      <el-alert v-if="actionError" :title="actionError" type="error" :closable="false" show-icon class="action-error" />
      <el-skeleton v-if="loading && !inventory" :rows="5" animated />
      <template v-else-if="inventory">
        <p class="observed-at">采集时间：{{ new Date(inventory.observed_at).toLocaleString() }}</p>
        <el-alert v-if="!inventory.complete"
          title="资源清单不完整。连接或权限异常不能作为删除资源的依据。"
          type="warning" :closable="false" show-icon />
        <el-alert v-else-if="!inventory.actions_supported"
          title="当前 Agent 版本不支持 Docker 持久操作。"
          type="info" :closable="false" show-icon />
        <div class="resource-list">
          <article v-for="resource in inventory.resources || []" :key="resource.resource_id" class="resource-row">
            <div class="resource-icon">◇</div>
            <div class="resource-info">
              <strong>{{ resource.name }}</strong>
              <span>{{ resource.kind }} · {{ resource.state }}</span>
              <small>{{ resource.managed ? 'EasyNetTopo 受管资源' : '外部资源 · 只读发现' }}</small>
            </div>
            <el-tag size="small" :type="resource.managed ? 'success' : 'info'">
              {{ resource.managed ? '受管' : '外部' }}
            </el-tag>
            <div class="resource-actions">
              <el-button size="small" :disabled="!inventory.complete || !!busyResourceId || taskBusy"
                :loading="busyResourceId === resource.resource_id" @click="queryResource(resource, 'stats')">
                资源占用
              </el-button>
              <el-button size="small" :disabled="!inventory.complete || !!busyResourceId || taskBusy"
                @click="queryResource(resource, 'logs')">
                查看日志
              </el-button>
              <template v-if="inventory.actions_supported && resource.managed">
                <el-button size="small" type="success" plain
                  :disabled="!inventory.complete || !!busyResourceId || taskBusy"
                  @click="startTask(resource, 'start')">启动</el-button>
                <el-button size="small" type="warning" plain
                  :disabled="!inventory.complete || !!busyResourceId || taskBusy"
                  @click="startTask(resource, 'stop')">停止</el-button>
                <el-button size="small" type="primary" plain
                  :disabled="!inventory.complete || !!busyResourceId || taskBusy"
                  @click="startTask(resource, 'restart')">重启</el-button>
              </template>
            </div>
            <div v-if="selectedResourceId === resource.resource_id && resourceStats" class="detail">
              <p v-for="item in resourceStats.containers" :key="item.container_id">
                <code>{{ item.container_id.slice(0, 12) }}</code> · {{ item.state }}
                <span v-if="item.available">
                  · CPU {{ item.cpu_percent.toFixed(1) }}%
                  · 内存 {{ formatBytes(item.memory_bytes) }} / {{ formatBytes(item.memory_limit_bytes) }}
                </span>
                <span v-else> · 无实时统计</span>
              </p>
              <p class="observed-at">统计采集时间：{{ new Date(resourceStats.observed_at).toLocaleString() }}</p>
              <el-alert v-if="!resourceStats.complete" title="部分容器统计不可用。" type="warning" :closable="false" />
            </div>
            <div v-if="selectedResourceId === resource.resource_id && resourceLogs" class="detail">
              <el-alert v-if="resourceLogs.truncated" title="日志已截断，仅显示最近部分内容。" type="warning" :closable="false" />
              <pre class="log">{{ resourceLogs.text }}</pre>
            </div>
          </article>
        </div>
        <el-empty v-if="!inventory.resources?.length" description="没有发现 Docker 资源" :image-size="80" />
        <section v-if="dockerTask" class="task-card" role="status">
          <div class="task-heading">
            <strong>Docker 持久操作任务</strong>
            <el-tag size="small" :type="dockerTask.status === 'succeeded' ? 'success' : taskActive ? 'warning' : 'info'">
              {{ dockerTask.action }} · {{ dockerTask.status }}
            </el-tag>
          </div>
          <p>资源：<code>{{ dockerTask.resource_id }}</code></p>
          <p>任务：<code>{{ dockerTask.task_id }}</code></p>
          <p v-if="dockerTask.error" class="task-error">{{ dockerTask.error }}</p>
          <p v-if="dockerTask.result?.verified" class="task-success">
            已核验最终状态：{{ dockerTask.result.final_status || '完成' }}
          </p>
          <div class="task-actions">
            <el-button size="small" :loading="taskBusy" :disabled="taskBusy" @click="refreshTask">刷新任务状态</el-button>
            <el-button v-if="taskActive" size="small" type="warning" plain
              :loading="taskBusy" :disabled="taskBusy" @click="cancelTask">请求取消</el-button>
          </div>
        </section>
      </template>
      <el-empty v-else-if="!loading && !error" description="尚未获取 Docker 资源" :image-size="80" />
    </template>
  </el-drawer>
</template>

<style scoped>
.drawer-summary { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 12px; color: var(--ent-muted); font-size: 11px; overflow-wrap: anywhere; }
.observed-at { color: var(--ent-dim); font-size: 10px; }
.resource-list { display: grid; gap: 9px; margin-top: 14px; }
.resource-row { display: flex; align-items: center; gap: 11px; flex-wrap: wrap; padding: 12px; border: 1px solid #254059; border-radius: 11px; background: #112338; }
.resource-icon { display: grid; width: 34px; height: 34px; flex: none; place-items: center; border-radius: 9px; background: #203d58; color: var(--ent-cyan); font-size: 21px; }
.resource-info { display: grid; min-width: 0; flex: 1; gap: 4px; }
.resource-info strong { overflow-wrap: anywhere; color: var(--ent-text); font-size: 12px; }
.resource-info span { color: #87a5bf; font-size: 11px; }
.resource-info small { color: var(--ent-dim); font-size: 10px; }
.resource-actions { display: flex; width: 100%; flex-wrap: wrap; gap: 6px; }
.detail { width: 100%; min-width: 0; padding: 9px 11px; border-radius: 8px; background: #0a1928; overflow-wrap: anywhere; }
.detail p { margin: 6px 0; color: #a8c0d5; font-size: 11px; }
.log { max-height: 300px; overflow: auto; margin: 8px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; color: #c5d7e6; font-size: 11px; }
.action-error { margin-top: 10px; }
.task-card { display: grid; gap: 7px; margin-top: 16px; padding: 13px; border: 1px solid #31506a; border-radius: 10px; background: #0e2032; }
.task-heading, .task-actions { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
.task-card p { margin: 0; color: #9bb6cd; font-size: 11px; overflow-wrap: anywhere; }
.task-error { color: #f3a18b !important; }
.task-success { color: #82d5a5 !important; }
</style>
