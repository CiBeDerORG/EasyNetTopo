<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { formatApiError } from '../../lib/api-client'
import { getOpenWrtDiscoverySnapshot, scanOpenWrtDiscovery } from './api'
import type { Device, OpenWrtDiscoveryKind, OpenWrtDiscoverySnapshot } from './types'

const props = defineProps<{ modelValue: boolean; device: Device; kind: OpenWrtDiscoveryKind }>()
const queryClient = useQueryClient()
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
const diagnosticsExpanded = ref<string[]>([])
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
    if (current === requestNumber) {
      snapshot.value = result
      if (result.diagnostics) {
        diagnosticsExpanded.value = ['scan']
        const diagnostics = result.diagnostics
        console.groupCollapsed('[EasyNetTopo][OpenWrtDiscovery] 扫描诊断 · ' + props.device.name)
        console.info('网关身份', diagnostics.gateway)
        console.info('OpenWrt 发现快照', diagnostics.snapshot)
        console.info('EasyTier 路由查询与代理网段', diagnostics.route)
        console.info('设备入库及组网 IP 同步汇总', diagnostics.import)
        console.table(diagnostics.import.devices)
        console.groupEnd()
      }
      if (result.import) {
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: ['devices'] }),
          queryClient.invalidateQueries({ queryKey: ['gateway-groups'] })
        ])
      }
    }
  } catch (error) {
    if (current === requestNumber) {
      requestError.value = formatApiError(error)
      console.error('[EasyNetTopo][OpenWrtDiscovery] 扫描请求失败', {
        device_id: props.device.id,
        device_name: props.device.name,
        kind: props.kind,
        message: requestError.value
      })
    }
  } finally {
    if (current === requestNumber) scanning.value = false
  }
}

watch(() => props.modelValue, open => {
  if (open) {
    snapshot.value = null
    diagnosticsExpanded.value = []
    void loadSnapshot()
  } else {
    requestNumber++
    loading.value = false
    scanning.value = false
    diagnosticsExpanded.value = []
  }
}, { immediate: true })

function snapshotTime(timestamp?: number | null) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString('zh-CN') : '暂无'
}
function addresses(value?: string[]) {
  return Array.isArray(value) && value.length ? value.join('、') : '—'
}
function observationLabel(value?: string) {
  if (value === 'cached') return '离线（历史缓存）'
  if (value === 'lease') return '在线（DHCP 租约）'
  return '在线（本次观测）'
}
function diagnosticTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : '—'
}
function routeStatusLabel(value?: string) {
  const labels: Record<string, string> = {
    matched: '匹配成功',
    no_match: '未匹配',
    failed: '查询失败',
    cancelled: '已取消',
    ambiguous: '结果冲突'
  }
  return value ? (labels[value] || value) : '—'
}
function importResultLabel(value?: string) {
  const labels: Record<string, string> = { created: '新增', existing: '已存在', skipped: '跳过' }
  return value ? (labels[value] || value) : '—'
}
function networkIPActionLabel(value?: string) {
  const labels: Record<string, string> = {
    written: '写入 / 更新',
    cleared: '清空旧组网 IP',
    unavailable_cleared: '映射不可用，清空旧值',
    unchanged: '映射一致，保持不变',
    unavailable: '映射不可用',
    unavailable_preserved: '映射不可用，保留历史值',
    unmapped: '命中规则但无法映射',
    no_ip: '没有可映射的 IPv4'
  }
  return value ? (labels[value] || value) : '—'
}
function networkIPMappingStatusLabel(value?: string) {
  const labels: Record<string, string> = {
    mapped: '命中规则并成功映射',
    mapper_unavailable: '未获取到路由映射器',
    no_matching_rule: 'IP 不在代理源网段内',
    matched_unmapped: '命中网段但地址不可映射',
    ipv4_only: '当前仅支持 IPv4 映射',
    invalid_address: 'IP 地址无效',
    no_ip: '没有可映射的 IP'
  }
  return value ? (labels[value] || value) : '—'
}
function matchLocationLabel(value?: string) {
  if (value === 'node_info') return 'node_info'
  if (value === 'peer_routes') return 'peer_routes'
  return value || '—'
}
</script>

<template>
  <el-dialog v-model="visible" :title="title + ' · ' + device.name" width="min(980px, 94vw)" destroy-on-close>
    <el-alert type="info" :closable="false" show-icon
      :title="kind === 'adb'
        ? '扫描会将发现的无线 ADB 设备自动添加到该网关下的设备列表；不执行配对、连接或 ADB 指令。'
        : '点击开始扫描后，发现的设备会自动添加到当前 OpenWrt 网关下；已登记设备不覆盖、不重复添加。'" />
    <div class="discovery-toolbar">
      <span>上次快照：{{ snapshotTime(snapshot?.scanned_at) }}</span>
      <span v-if="snapshot">记录 {{ snapshot.count ?? snapshot.devices?.length ?? 0 }} 条</span>
      <el-tag v-if="snapshot?.truncated" size="small" type="warning">结果已截断</el-tag>
      <el-tag v-if="snapshot?.status === 'partial'" size="small" type="warning">部分成功</el-tag>
    </div>
    <el-alert v-if="snapshot?.import" type="success" :closable="false" show-icon
      :title="'自动入库完成：新增 ' + snapshot.import.created + ' 台，已存在 ' + snapshot.import.existing + ' 台，跳过 ' + snapshot.import.skipped + ' 条。'" />
    <el-collapse v-if="snapshot?.diagnostics" v-model="diagnosticsExpanded" class="discovery-diagnostics">
      <el-collapse-item name="scan">
        <template #title>
          <span class="diagnostics-title">扫描诊断日志</span>
          <el-tag size="small" :type="snapshot.diagnostics.route.available ? 'success' : 'warning'">
            {{ snapshot.diagnostics.route.available ? '组网 IP 映射已就绪' : '未获取到映射规则' }}
          </el-tag>
          <span class="diagnostics-title-meta">
            {{ snapshot.diagnostics.import.devices.length }} 条设备处理记录
          </span>
        </template>
        <div class="diagnostics-content">
          <div class="diagnostics-generated">生成时间：{{ diagnosticTime(snapshot.diagnostics.generated_at) }}</div>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="扫描网关">
              {{ snapshot.diagnostics.gateway.name || device.name }}（{{ snapshot.diagnostics.gateway.ip_address || device.ip_address || '—' }}）
            </el-descriptions-item>
            <el-descriptions-item label="OpenWrt 快照">
              {{ snapshot.diagnostics.snapshot.status }} · {{ snapshot.diagnostics.snapshot.device_count }} 条
              <span v-if="snapshot.diagnostics.snapshot.truncated">· 已截断</span>
            </el-descriptions-item>
            <el-descriptions-item label="network_key">
              {{ snapshot.diagnostics.gateway.network_key || snapshot.diagnostics.route.target_network_key || '—' }}
            </el-descriptions-item>
            <el-descriptions-item label="peer_id">
              {{ snapshot.diagnostics.gateway.peer_id || snapshot.diagnostics.route.target_peer_id || '—' }}
            </el-descriptions-item>
            <el-descriptions-item label="组网入口查询">
              {{ snapshot.diagnostics.route.hosts_queried }} 个入口 · 成功 {{ snapshot.diagnostics.route.hosts_succeeded }} · 失败 {{ snapshot.diagnostics.route.hosts_failed }}
            </el-descriptions-item>
            <el-descriptions-item label="目标节点匹配">
              {{ snapshot.diagnostics.route.hosts_matched }} 个入口 · {{ matchLocationLabel(snapshot.diagnostics.route.match_location) }}
            </el-descriptions-item>
          </el-descriptions>

          <section class="diagnostics-section">
            <div class="diagnostics-section-title">EasyTier 路由与代理网段</div>
            <div class="diagnostics-note">{{ snapshot.diagnostics.route.reason || '—' }}</div>
            <div v-if="snapshot.diagnostics.route.mapping_rules?.length" class="diagnostics-rules">
              <el-tag v-for="rule in snapshot.diagnostics.route.mapping_rules" :key="rule" size="small" type="success">{{ rule }}</el-tag>
            </div>
            <el-table v-if="snapshot.diagnostics.route.hosts.length" :data="snapshot.diagnostics.route.hosts"
              border size="small" max-height="190" class="diagnostics-table">
              <el-table-column prop="host_name" label="组网入口" min-width="115" show-overflow-tooltip />
              <el-table-column label="查询结果" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.status === 'matched' ? 'success' : row.status === 'failed' || row.status === 'ambiguous' ? 'danger' : 'warning'">
                    {{ routeStatusLabel(row.status) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="匹配位置" width="105">
                <template #default="{ row }">{{ matchLocationLabel(row.match_location) }}</template>
              </el-table-column>
              <el-table-column label="返回 network_key" min-width="150" show-overflow-tooltip>
                <template #default="{ row }">{{ row.observed_network_keys?.join('；') || '—' }}</template>
              </el-table-column>
              <el-table-column label="返回 peer_id" min-width="150" show-overflow-tooltip>
                <template #default="{ row }">{{ row.observed_peer_ids?.join('；') || '—' }}</template>
              </el-table-column>
              <el-table-column label="代理网段规则" min-width="210" show-overflow-tooltip>
                <template #default="{ row }">{{ row.mapping_rules?.join('；') || '—' }}</template>
              </el-table-column>
              <el-table-column prop="message" label="诊断信息" min-width="220" show-overflow-tooltip />
            </el-table>
            <el-empty v-else description="没有可查询的组网入口" :image-size="55" />
          </section>

          <section class="diagnostics-section">
            <div class="diagnostics-section-title">设备入库与组网 IP 写入</div>
            <el-descriptions :column="3" border size="small">
              <el-descriptions-item label="新增设备">{{ snapshot.diagnostics.import.created }}</el-descriptions-item>
              <el-descriptions-item label="匹配历史设备">{{ snapshot.diagnostics.import.existing }}</el-descriptions-item>
              <el-descriptions-item label="跳过记录">{{ snapshot.diagnostics.import.skipped }}</el-descriptions-item>
              <el-descriptions-item label="组网 IP 写入 / 更新">{{ snapshot.diagnostics.import.network_ip_written }}</el-descriptions-item>
              <el-descriptions-item label="组网 IP 清空">{{ snapshot.diagnostics.import.network_ip_cleared }}</el-descriptions-item>
              <el-descriptions-item label="组网 IP 保持不变">{{ snapshot.diagnostics.import.network_ip_unchanged }}</el-descriptions-item>
              <el-descriptions-item label="未命中 / 映射不可用">{{ snapshot.diagnostics.import.network_ip_unavailable }}</el-descriptions-item>
              <el-descriptions-item label="命中规则但无法映射">{{ snapshot.diagnostics.import.network_ip_unmapped }}</el-descriptions-item>
            </el-descriptions>
            <div class="diagnostics-note">说明：未命中 / 映射不可用包含无路由映射器、IP 不在代理源网段或仅支持 IPv4 等情况；可能同时伴随清空旧组网 IP 或保留历史值。</div>
            <el-table v-if="snapshot.diagnostics.import.devices.length" :data="snapshot.diagnostics.import.devices"
              border size="small" max-height="280" class="diagnostics-table">
              <el-table-column prop="name" label="设备" min-width="115" show-overflow-tooltip />
              <el-table-column prop="ip_address" label="内网 IP" min-width="125" show-overflow-tooltip />
              <el-table-column prop="mac_address" label="MAC" min-width="135" show-overflow-tooltip />
              <el-table-column label="匹配方式" width="90">
                <template #default="{ row }">{{ row.matched_by || '—' }}</template>
              </el-table-column>
              <el-table-column label="处理结果" width="85">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.result === 'created' ? 'success' : row.result === 'skipped' ? 'warning' : 'info'">
                    {{ importResultLabel(row.result) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="previous_network_ip" label="原组网 IP" min-width="120" show-overflow-tooltip />
              <el-table-column prop="current_network_ip" label="当前组网 IP" min-width="120" show-overflow-tooltip />
              <el-table-column label="映射判定" min-width="165">
                <template #default="{ row }">{{ networkIPMappingStatusLabel(row.network_ip_mapping_status) }}</template>
              </el-table-column>
              <el-table-column label="组网 IP 处理" min-width="145">
                <template #default="{ row }">{{ networkIPActionLabel(row.network_ip_action) }}</template>
              </el-table-column>
              <el-table-column label="IP 字段写入" width="95">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.address_fields_updated ? 'success' : 'info'">{{ row.address_fields_updated ? '已写入' : '无变更' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="在线状态" width="95">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.discovery_status_updated ? 'success' : 'info'">{{ row.discovery_status_updated ? '已同步' : '未更新' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="reason" label="处理说明" min-width="220" show-overflow-tooltip />
            </el-table>
          </section>
        </div>
      </el-collapse-item>
    </el-collapse>
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
        <el-table-column label="发现状态" width="145">
          <template #default="{ row }">
            <el-tag :type="row.observation === 'cached' ? 'info' : 'success'" size="small">
              {{ observationLabel(row.observation) }}
            </el-tag>
          </template>
        </el-table-column>
      </template>
      <template v-else>
        <el-table-column prop="instance" label="ADB 服务" min-width="145" show-overflow-tooltip />
        <el-table-column prop="host" label="主机" min-width="120" show-overflow-tooltip />
        <el-table-column prop="address" label="地址" min-width="165" show-overflow-tooltip />
        <el-table-column prop="port" label="端口" width="90" />
        <el-table-column prop="protocol" label="协议" width="90" />
        <el-table-column prop="network" label="网络" min-width="110" show-overflow-tooltip />
        <el-table-column prop="iface" label="接口" min-width="105" show-overflow-tooltip />
        <el-table-column label="发现状态" width="120">
          <template #default>
            <el-tag type="success" size="small">在线（本次观测）</el-tag>
          </template>
        </el-table-column>
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
.discovery-diagnostics { margin:12px 0; border-top:1px solid var(--el-border-color); border-bottom:1px solid var(--el-border-color); }
.diagnostics-title { font-weight:600; margin-right:10px; }
.diagnostics-title-meta { margin-left:10px; color:var(--ent-muted); font-size:12px; }
.diagnostics-content { display:flex; flex-direction:column; gap:12px; padding:0 2px 8px; }
.diagnostics-generated, .diagnostics-note { color:var(--ent-muted); font-size:12px; line-height:1.6; overflow-wrap:anywhere; }
.diagnostics-section { display:flex; flex-direction:column; gap:8px; min-width:0; }
.diagnostics-section-title { font-size:13px; font-weight:600; color:var(--el-text-color-primary); }
.diagnostics-rules { display:flex; flex-wrap:wrap; gap:6px; }
.diagnostics-table { width:100%; }
</style>
