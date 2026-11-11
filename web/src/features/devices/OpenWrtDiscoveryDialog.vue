<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { formatApiError } from '../../lib/api-client'
import { getOpenWrtDiscoverySnapshot, scanOpenWrtDiscovery } from './api'
import type { Device, OpenWrtDiscoveryKind, OpenWrtDiscoverySnapshot } from './types'

const props = defineProps<{ modelValue: boolean; device: Device; kind: OpenWrtDiscoveryKind }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const visible = computed({
  get: () => props.modelValue,
  set: value => emit('update:modelValue', value)
})
const title = computed(() => props.kind === 'adb' ? 'ADB 发现' : '服务发现')
const loading = ref(false)
const scanning = ref(false)
const requestError = ref('')
const snapshot = ref<OpenWrtDiscoverySnapshot | null>(null)
let requestNumber = 0

async function loadSnapshot() {
  if (loading.value || scanning.value) return
  const current = ++requestNumber
  loading.value = true
  requestError.value = ''
  try {
    const result = await getOpenWrtDiscoverySnapshot(props.device.id, props.kind)
    if (current === requestNumber) snapshot.value = result
  } catch (error) {
    if (current === requestNumber) requestError.value = formatApiError(error)
  } finally {
    if (current === requestNumber) loading.value = false
  }
}

async function scan() {
  if (scanning.value || loading.value) return
  const current = ++requestNumber
  scanning.value = true
  requestError.value = ''
  try {
    const result = await scanOpenWrtDiscovery(props.device.id, props.kind)
    if (current === requestNumber) snapshot.value = result
  } catch (error) {
    if (current === requestNumber) requestError.value = formatApiError(error)
  } finally {
    if (current === requestNumber) scanning.value = false
  }
}

watch(() => props.modelValue, open => {
  if (open) {
    snapshot.value = null
    void loadSnapshot()
  } else {
    requestNumber++
    loading.value = false
    scanning.value = false
  }
}, { immediate: true })

function snapshotTime(timestamp?: number | null) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString('zh-CN') : '暂无'
}
function addresses(value?: string[]) {
  return Array.isArray(value) && value.length ? value.join('、') : '—'
}
</script>

<template>
  <el-dialog v-model="visible" :title="title + ' · ' + device.name" width="min(980px, 94vw)" destroy-on-close>
    <el-alert type="info" :closable="false" show-icon
      :title="kind === 'adb'
        ? '读取该 OpenWrt 网关的无线 ADB 服务端点。这里只发现地址，不会自动配对、连接或执行 ADB 指令。'
        : '通过该 OpenWrt 网关的 Easy Discovery 服务采集局域网设备。扫描结果仅供预览，不会自动添加到设备列表。'" />
    <div class="discovery-toolbar">
      <span>上次快照：{{ snapshotTime(snapshot?.scanned_at) }}</span>
      <span v-if="snapshot">记录 {{ snapshot.count ?? snapshot.devices?.length ?? 0 }} 条</span>
      <el-tag v-if="snapshot?.truncated" size="small" type="warning">结果已截断</el-tag>
      <el-tag v-if="snapshot?.status === 'partial'" size="small" type="warning">部分成功</el-tag>
    </div>
    <el-alert v-if="requestError" :title="requestError" type="error" :closable="false" show-icon />
    <el-alert v-if="snapshot?.status === 'error'" type="error" :closable="false" show-icon
      :title="snapshot.message || '本次发现未成功；以下可能是之前的历史快照，请检查错误详情。'" />
    <el-alert v-if="snapshot?.status === 'partial'" type="warning" :closable="false" show-icon
      title="部分来源采集失败，结果可能不完整。" />
    <div v-if="snapshot?.errors?.length" class="discovery-errors">
      <div v-for="(error, index) in snapshot.errors" :key="index">
        {{ error.source || '发现服务' }}：{{ error.message || error.code || '未知错误' }}
      </div>
    </div>
    <el-skeleton v-if="loading" :rows="4" animated />
    <el-table v-else-if="snapshot?.devices?.length" :data="snapshot.devices" border stripe
      max-height="440" size="small" class="discovery-table">
      <template v-if="kind === 'devices'">
        <el-table-column prop="name" label="设备名称" min-width="130" show-overflow-tooltip />
        <el-table-column prop="hostname" label="主机名" min-width="130" show-overflow-tooltip />
        <el-table-column label="IPv4" min-width="155" show-overflow-tooltip>
          <template #default="{ row }">{{ addresses(row.ipv4) }}</template>
        </el-table-column>
        <el-table-column prop="mac" label="MAC" min-width="145" show-overflow-tooltip />
        <el-table-column prop="vendor" label="厂商" min-width="120" show-overflow-tooltip />
        <el-table-column prop="model" label="型号" min-width="110" show-overflow-tooltip />
        <el-table-column prop="observation" label="观测状态" width="100" />
      </template>
      <template v-else>
        <el-table-column prop="instance" label="ADB 服务" min-width="145" show-overflow-tooltip />
        <el-table-column prop="host" label="主机" min-width="120" show-overflow-tooltip />
        <el-table-column prop="address" label="地址" min-width="165" show-overflow-tooltip />
        <el-table-column prop="port" label="端口" width="90" />
        <el-table-column prop="protocol" label="协议" width="90" />
        <el-table-column prop="network" label="网络" min-width="110" show-overflow-tooltip />
        <el-table-column prop="iface" label="接口" min-width="105" show-overflow-tooltip />
      </template>
    </el-table>
    <el-empty v-else-if="!loading" :description="snapshot?.has_snapshot ? '快照中没有发现记录' : '尚无快照，请点击开始扫描'" :image-size="85" />
    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
      <el-button :loading="loading" :disabled="scanning" @click="loadSnapshot">读取快照</el-button>
      <el-button type="primary" :loading="scanning" :disabled="loading" @click="scan">开始扫描</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.discovery-toolbar { display:flex; align-items:center; gap:12px; flex-wrap:wrap; padding:14px 2px 12px; color:var(--ent-muted); font-size:12px; }
.discovery-errors { margin:10px 0; font-size:12px; color:var(--el-color-danger); line-height:1.6; overflow-wrap:anywhere; }
.discovery-table { width:100%; }
</style>
