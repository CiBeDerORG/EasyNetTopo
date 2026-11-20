<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { getDeviceServiceImport, importDeviceServices } from './api'
import type { Device, ServiceImportApp, ServiceImportSelection } from './types'

const props = defineProps<{ device: Device; hosts: Host[]; networkIp: string }>()
const saving = defineModel<boolean>('saving', { required: true })
const emit = defineEmits<{ close: []; imported: [] }>()
const hostId = ref('')
const selected = ref<Record<string, number>>({})
const confirmed = ref(false)
const saveError = ref('')
const host = computed(() => props.hosts.find(item => item.id === hostId.value))
const paired = computed(() => !!host.value?.agent_id)
const query = useQuery({
  queryKey: computed(() => ['device-service-import', props.device.id, hostId.value] as const),
  queryFn: ({ queryKey: [, deviceId, sourceHostId], signal }) => getDeviceServiceImport(deviceId, sourceHostId, signal),
  enabled: paired,
  retry: false
})
const apps = computed(() => query.data.value?.apps ?? [])
const addressCheck = computed(() => query.data.value?.address_check)
const appKey = (app: ServiceImportApp) => JSON.stringify([app.source_id, app.app_id])
const stateLabels: Record<string, string> = { running: '运行中', stopped: '已停止', partial: '部分运行', missing: '容器缺失' }
const importLimit = 100
const availablePorts = (app: ServiceImportApp) => app.ports.filter(port => !port.registered)
const selectedCount = computed(() => Object.keys(selected.value).length)
const items = computed(() => apps.value.flatMap<ServiceImportSelection>(app => {
  const port = availablePorts(app).find(entry => entry.port === selected.value[appKey(app)])
  return port ? [{ source_id: app.source_id, app_id: app.app_id, port: port.port, protocols: port.protocols }] : []
}))
const canSave = computed(() => paired.value && !query.isFetching.value && !query.isError.value &&
  items.value.length > 0 && items.value.length === selectedCount.value && items.value.length <= importLimit &&
  (addressCheck.value === 'matched' || confirmed.value))

function resetSelection() {
  selected.value = {}
  confirmed.value = false
  saveError.value = ''
}

function toggleApp(app: ServiceImportApp, checked: boolean) {
  const key = appKey(app)
  if (!checked) {
    delete selected.value[key]
    return
  }
  const ports = availablePorts(app)
  selected.value[key] = ports.length === 1 ? ports[0].port : 0
}

async function save() {
  if (!canSave.value || saving.value) return
  saving.value = true
  saveError.value = ''
  try {
    const result = await importDeviceServices(props.device.id, hostId.value, items.value, confirmed.value)
    ElMessage.success('已导入 ' + result.imported + ' 项，跳过 ' + result.skipped + ' 项重复服务')
    resetSelection()
    emit('imported')
  } catch (error) {
    saveError.value = formatApiError(error)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="service-import">
    <el-form-item label="来源主机">
      <el-select v-model="hostId" filterable placeholder="请选择一台主机" aria-label="来源主机"
        :disabled="saving" @change="resetSelection">
        <el-option v-for="item in hosts" :key="item.id" :label="item.name + (item.agent_id ? '' : '（未配对）')"
          :value="item.id" :disabled="!item.agent_id" />
      </el-select>
    </el-form-item>
    <div class="import-endpoints">
      <div><span>来源主机</span><strong>{{ host?.endpoint || '未选择' }}</strong></div>
      <div><span>目标设备</span><strong>{{ device.name }}<template v-if="networkIp || device.ip_address"> · {{ networkIp || device.ip_address }}</template></strong></div>
    </div>
    <el-empty v-if="!hostId" :description="hosts.some(item => item.agent_id) ? '选择主机后，查看可导入的已安装应用' : '暂无已配对的主机'" :image-size="65" />
    <el-skeleton v-else-if="paired && query.isPending.value" :rows="3" animated />
    <el-alert v-else-if="query.isError.value" type="error" :title="formatApiError(query.error.value)" :closable="false" show-icon>
      <el-button text @click="query.refetch()">重试</el-button>
    </el-alert>
    <template v-else>
      <el-empty v-if="!apps.length" description="没有可识别的已安装应用" :image-size="65" />
      <div v-else class="import-apps" aria-label="已安装应用">
        <div v-for="app in apps" :key="appKey(app)" class="import-app">
          <div class="import-app-title">
            <el-checkbox :model-value="appKey(app) in selected" :disabled="saving || !availablePorts(app).length || (selectedCount >= importLimit && !(appKey(app) in selected))"
              @update:model-value="toggleApp(app, $event === true)">{{ app.name }}</el-checkbox>
            <el-tag size="small" effect="plain" :type="app.state === 'running' ? 'success' : 'info'">{{ stateLabels[app.state] }}</el-tag>
          </div>
          <div v-if="!app.ports.length" class="import-tip">无可用的主机发布端口</div>
          <div v-else-if="!availablePorts(app).length" class="import-tip">端口已登记，将跳过</div>
          <div v-else-if="appKey(app) in selected && app.ports.length > 1">
            <el-select v-model="selected[appKey(app)]" :disabled="saving" :aria-label="app.name + ' 的主机端口'"
              :placeholder="'选择 ' + app.name + ' 的主机端口'" :empty-values="[0, null, undefined]">
              <el-option v-for="port in app.ports" :key="port.port" :value="port.port" :disabled="port.registered"
                :label="port.port + ' · ' + port.protocols.join(' / ').toUpperCase() + (port.registered ? '（已登记）' : '')" />
            </el-select>
          </div>
          <div v-else class="import-tip">
            {{ availablePorts(app).map(port => port.port + ' · ' + port.protocols.join(' / ').toUpperCase()).join('、') }}
          </div>
        </div>
      </div>
      <div v-if="apps.length && addressCheck !== 'matched'" class="import-confirm">
        <p>{{ addressCheck === 'different' ? '主机 Agent 地址与设备地址不同。' : '无法通过地址自动核对主机与设备。' }}</p>
        <el-checkbox v-model="confirmed" :disabled="saving">我确认所选主机对应当前设备</el-checkbox>
      </div>
    </template>
    <el-alert v-if="saveError" :title="saveError" type="error" :closable="false" show-icon>
      <el-button text :disabled="saving" @click="resetSelection(); query.refetch()">刷新应用列表</el-button>
    </el-alert>
    <div class="import-actions">
      <el-button :disabled="saving" @click="emit('close')">返回列表</el-button>
      <el-button type="primary" :disabled="!canSave" :loading="saving" @click="save">导入 {{ selectedCount }} 项</el-button>
    </div>
  </div>
</template>

<style scoped>
.service-import { display:grid; gap:14px; }
.service-import :deep(.el-form-item) { margin:0; }
.service-import :deep(.el-select) { width:100%; }
.import-endpoints { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,210px),1fr)); gap:10px; }
.import-endpoints div, .import-app { display:grid; gap:4px; padding:10px 12px; border:1px solid var(--ent-line); border-radius:8px; overflow-wrap:anywhere; }
.import-endpoints span,.import-tip { color:var(--ent-muted); font-size:12px; }
.import-apps { display:grid; gap:8px; max-height:36vh; overflow:auto; }
.import-app-title { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.import-app-title :deep(.el-checkbox) { height:auto; min-width:0; white-space:normal; }
.import-confirm p { color:var(--ent-amber); margin:0 0 4px; }
.import-confirm :deep(.el-checkbox) { white-space:normal; }
.import-actions { display:flex; justify-content:flex-end; }
</style>
