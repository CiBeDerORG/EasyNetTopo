<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useQuery } from '@tanstack/vue-query'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { mergedNetworkRows, type NetworkRow } from './data'
import NetworkNodeDetails from './NetworkNodeDetails.vue'
import NetworkNodeLocationPicker from './NetworkNodeLocationPicker.vue'
import { listNodeAttributes, saveNodeAttributes, type NodeAttributes } from './attributes'

const props = defineProps<{ hosts: Host[]; hostsLoading: boolean; hostsError: string }>()
const entries = computed(() => props.hosts.filter(host => host.network_entry))
const hostId = ref('')
const expandedKeys = ref<string[]>([])
const search = ref('')
watch(entries, items => {
  if (!items.some(host => host.id === hostId.value)) hostId.value = items[0]?.id || ''
}, { immediate: true })

// 单次 route 查询同时包含本机和路由节点；保留原列表数据，不用加载遮罩。
const query = useQuery({
  queryKey: computed(() => ['network-nodes', hostId.value]),
  queryFn: async ({ queryKey, signal }) => {
    const id = encodeURIComponent(String(queryKey[1]))
    const response = await apiRequest<{ data: unknown }>(
      '/api/v1/hosts/' + id + '/network/route', { signal })
    return mergedNetworkRows(response.data)
  },
  enabled: computed(() => !!hostId.value),
  retry: 0,
  refetchInterval: 15000
})
// 人工属性只按需查询，网络路由每 15 秒刷新不触发属性覆盖或加载遮罩。
const attributesQuery = useQuery({
  queryKey: ['network-node-attributes'], queryFn: listNodeAttributes,
  enabled: computed(() => entries.value.length > 0), retry: 0
})
const attributesByKey = computed(() => new Map<string, NodeAttributes>((attributesQuery.data.value?.attributes || [])
  .map(attr => [attr.network_key + '|' + attr.peer_id, attr] as const)))
function networkScope(row: NetworkRow): string {
  return row.networkKey.startsWith('instance:') ? 'entry:' + hostId.value + ':' + row.networkKey : row.networkKey
}
function getAttributes(row: NetworkRow): NodeAttributes | undefined {
  return attributesByKey.value.get(networkScope(row) + '|' + row.id)
}
const editorOpen = ref(false)
const editorSaving = ref(false)
const locationLookingUp = ref(false)
const editingRow = ref<NetworkRow | null>(null)
const draft = ref<NodeAttributes>({ network_key: '', peer_id: '', alias: '', has_public_ip: false, public_ip: '',
  location: { country: '', province: '', city: '', county: '', latitude: null, longitude: null } })
const latitudeText = ref('')
const longitudeText = ref('')
function editAttributes(row: NetworkRow) {
  if (row.id === '—') { ElMessage.warning('该节点未报告有效节点 ID，暂无法保存属性'); return }
  editingRow.value = row
  const saved = getAttributes(row)
  draft.value = saved ? { ...saved, location: { ...saved.location } } : {
    network_key: networkScope(row), peer_id: row.id, alias: '',
    has_public_ip: false, public_ip: '',
    location: { country: '', province: '', city: '', county: '', latitude: null, longitude: null }
  }
  latitudeText.value = draft.value.location.latitude == null ? '' : String(draft.value.location.latitude)
  longitudeText.value = draft.value.location.longitude == null ? '' : String(draft.value.location.longitude)
  editorOpen.value = true
}
function chooseCandidate() {
  if (!editingRow.value?.candidatePublicIP) return
  draft.value.has_public_ip = true
  draft.value.public_ip = editingRow.value.candidatePublicIP
}
function coordinatesFromInputs(): boolean {
  const latRaw = latitudeText.value.trim(), lngRaw = longitudeText.value.trim()
  if (!latRaw && !lngRaw) {
    draft.value.location.latitude = null; draft.value.location.longitude = null; return true
  }
  const lat = Number(latRaw), lng = Number(lngRaw)
  if (!latRaw || !lngRaw || !Number.isFinite(lat) || !Number.isFinite(lng) ||
      lat < -85.05112878 || lat > 85.05112878 || lng < -180 || lng > 180) {
    ElMessage.warning('请同时填写有效经纬度（纬度 ±85.05°，经度 ±180°）')
    return false
  }
  draft.value.location.latitude = lat; draft.value.location.longitude = lng
  return true
}
function onMapSelect(point: { latitude: number; longitude: number }) {
  draft.value.location.latitude = point.latitude
  draft.value.location.longitude = point.longitude
  latitudeText.value = String(point.latitude)
  longitudeText.value = String(point.longitude)
}
async function lookupLocation() {
  if (!coordinatesFromInputs() || draft.value.location.latitude == null || draft.value.location.longitude == null) {
    ElMessage.warning('请先在地图选点或输入经纬度')
    return
  }
  locationLookingUp.value = true
  try {
    const { latitude, longitude } = draft.value.location
    const params = new URLSearchParams({ format: 'jsonv2', addressdetails: '1', zoom: '16',
      'accept-language': 'zh-CN', lat: String(latitude), lon: String(longitude) })
    const response = await fetch('https://nominatim.openstreetmap.org/reverse?' + params.toString())
    if (!response.ok) throw new Error('逆地理编码不可用')
    const result = await response.json() as { address?: Record<string, string> }
    const address = result.address || {}
    draft.value.location.country = address.country || ''
    draft.value.location.province = address.state || address.province || ''
    draft.value.location.city = address.city || address.town || address.municipality || address.village || ''
    draft.value.location.county = address.county || address.city_district || address.district || ''
    ElMessage.success('已识别行政区划，可继续手动修改')
  } catch { ElMessage.warning('地址解析失败，请手动填写国家、省、市、县') }
  finally { locationLookingUp.value = false }
}
async function saveAttributes() {
  if (!coordinatesFromInputs()) return
  editorSaving.value = true
  try {
    await saveNodeAttributes(draft.value)
    await attributesQuery.refetch()
    editorOpen.value = false
    ElMessage.success('节点属性已保存')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { editorSaving.value = false }
}
const loading = computed(() => query.isFetching.value)
const refreshing = computed(() => query.isFetching.value && !!query.data.value)
const error = computed(() => query.error.value ? formatApiError(query.error.value) : '')
const rows = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  return (query.data.value || []).filter(row =>
    !keyword || [getAttributes(row)?.alias, getAttributes(row)?.public_ip,
      ...Object.values(getAttributes(row)?.location || {}),
      ...Object.entries(row).filter(([key]) => key !== 'key' && key !== 'connections').map(([, value]) => value)]
      .some(value => String(value ?? '').toLocaleLowerCase().includes(keyword)))
})
const updatedAt = computed(() =>
  query.dataUpdatedAt.value ? new Date(query.dataUpdatedAt.value).toLocaleTimeString() : '尚未采集')

// 点击整行/任意单元格切换详情；点内置展开箭头时由 expand-change 同步，避免双重切换。
watch(hostId, () => { expandedKeys.value = [] })
function toggleKey(key: string) {
  expandedKeys.value = expandedKeys.value.includes(key)
    ? expandedKeys.value.filter(item => item !== key) : [...expandedKeys.value, key]
}
function toggleRow(row: NetworkRow, _column: unknown, event: MouseEvent) {
  if (event.target instanceof Element && event.target.closest('.el-table__expand-icon')) return
  toggleKey(row.key)
}
function syncExpandedRows(_row: NetworkRow, expandedRows: NetworkRow[]) {
  expandedKeys.value = expandedRows.map(item => item.key)
}
</script>

<template>
  <section class="network-pane" aria-label="EasyTier 网络节点">
    <div class="network-intro">
      <h2>节点列表</h2>
      <p>查看所选入口可见的 EasyTier 节点，点击任意一行展开或收起详情；每 15 秒后台刷新。</p>
    </div>
    <el-alert v-if="hostsError" :title="hostsError" type="error" :closable="false" />
    <p v-else-if="hostsLoading">正在读取组网入口…</p>
    <el-empty v-else-if="!entries.length" description="尚未设置组网入口">
      <p>请在主机商店中，为已配对且安装 EasyTier 的主机点击“设为组网入口”。</p>
      <el-button type="primary" @click="$router.push('/host-store')">前往主机列表</el-button>
    </el-empty>
    <template v-else>
      <div class="network-toolbar">
        <el-select v-model="hostId" aria-label="组网入口" placeholder="选择组网入口" class="entry-select">
          <el-option v-for="host in entries" :key="host.id" :value="host.id" :label="host.name" />
        </el-select>
        <el-input v-model="search" clearable placeholder="搜索主机名 / 虚拟 IP / 网段 / 节点 ID" aria-label="搜索网络节点" />
        <el-button :loading="loading" @click="query.refetch()">刷新</el-button>
      </div>

      <el-alert v-if="error" :title="error"
        :description="query.data.value ? '以下保留上次成功采集的数据，不代表当前状态。' : '请检查组网入口、EasyTier Driver、容器与 RPC 状态。'"
        type="warning" :closable="false" show-icon class="network-alert" />
      <div class="network-summary">
        <span>{{ rows.length }} 个网络节点</span>
        <span>{{ refreshing ? '后台刷新中 · ' : '' }}最近成功采集：{{ updatedAt }}</span>
      </div>

      <el-table :data="rows" row-key="key" :expand-row-keys="expandedKeys"
        @row-click="toggleRow" @expand-change="syncExpandedRows"
        empty-text="当前没有匹配的网络节点" stripe class="network-desktop-table">
          <el-table-column label="节点" min-width="155">
            <template #default="{ row }">
              <div class="node-name">{{ getAttributes(row)?.alias || row.hostname }}</div>
              <div v-if="getAttributes(row)?.alias" class="node-meta">原主机名：{{ row.hostname }}</div>
              <div v-if="getAttributes(row)?.has_public_ip" class="node-meta">公网：{{ getAttributes(row)?.public_ip || '已标记，IP 待补充' }}</div>
              <div v-if="row.flags.length" class="node-flags">
                <el-tag v-for="flag in row.flags" :key="flag" size="small" type="info">{{ flag }}</el-tag>
              </div>
              <div class="node-meta" :title="row.id">ID：{{ row.id }}</div>
            </template>
          </el-table-column>
          <el-table-column label="网络实例" min-width="130">
            <template #default="{ row }">
              <span :title="row.instanceID">{{ row.instance }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="cidr" label="虚拟 IPv4" min-width="150" />
          <el-table-column label="入口实例 / 网络路由" min-width="185">
            <template #default="{ row }">
              <div class="route-cell">
                <el-tag size="small" :type="row.kind === '入口实例' ? 'success' : 'info'">{{ row.kind }}</el-tag>
                <span>{{ row.routeLabel }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="protocol" label="连接协议" min-width="110" />
          <el-table-column prop="latency" label="路径延迟" min-width="105" />
          <el-table-column prop="loss" label="连接丢包率" min-width="110" />
          <el-table-column prop="natType" label="UDP NAT 类型" min-width="140" show-overflow-tooltip />
          <el-table-column prop="subnets" label="代理网段" min-width="165" show-overflow-tooltip />
          <el-table-column prop="version" label="版本" min-width="130" show-overflow-tooltip />
          <el-table-column type="expand" width="42">
            <template #default="{ row }">
              <NetworkNodeDetails :row="row" :attributes="getAttributes(row)" @edit="editAttributes(row)" />
            </template>
          </el-table-column>
        </el-table>
        <div v-if="!rows.length && !loading" class="network-mobile-empty">当前没有匹配的网络节点</div>
        <div class="network-mobile-list">
          <article v-for="row in rows" :key="row.key" class="network-node-card"
            role="button" tabindex="0" :aria-expanded="expandedKeys.includes(row.key)"
            :aria-label="row.hostname + '，点击展开或收起详情'"
            @click="toggleKey(row.key)" @keydown.enter.prevent="toggleKey(row.key)"
            @keydown.space.prevent="toggleKey(row.key)">
            <div class="network-node-title">
              <strong>{{ getAttributes(row)?.alias || row.hostname }}</strong>
              <el-tag v-if="getAttributes(row)?.has_public_ip" size="small" type="success">公网 IP</el-tag>
              <el-tag size="small" :type="row.kind === '入口实例' ? 'success' : 'info'">{{ row.kind }}</el-tag>
            </div>
            <div class="node-meta" :title="row.instanceID">{{ row.instance }} · {{ row.id }}</div>
            <div v-if="row.flags.length" class="node-flags">
              <el-tag v-for="flag in row.flags" :key="flag" size="small" type="info">{{ flag }}</el-tag>
            </div>
            <div class="network-node-ip">{{ row.cidr }}</div>
            <div class="network-node-details">
              <span>{{ row.routeLabel }}</span><span>{{ row.protocol }}</span><span>{{ row.latency }}</span>
            </div>
            <div class="node-meta">代理网段：{{ row.subnets }} · 版本：{{ row.version }}</div>
            <div class="node-meta">UDP NAT：{{ row.natType }} · 连接丢包率：{{ row.loss }}</div>
            <NetworkNodeDetails v-if="expandedKeys.includes(row.key)" :row="row"
              :attributes="getAttributes(row)" @edit="editAttributes(row)" />
          </article>
        </div>
      <p class="network-note">
        本页只展示所选入口当前可见的节点；路由可见不等于设备持续在线。刷新时保持列表可见，可随时展开节点详情。
      </p>
    </template>
    <el-dialog v-model="editorOpen" title="编辑节点属性" width="min(760px, 96vw)" :close-on-click-modal="false">
      <el-form label-position="top" class="node-editor">
        <el-form-item label="节点别名">
          <el-input v-model="draft.alias" maxlength="100" show-word-limit placeholder="自定义节点显示名称" />
        </el-form-item>
        <el-form-item label="是否有公网 IP">
          <el-switch v-model="draft.has_public_ip" active-text="是" inactive-text="否" />
        </el-form-item>
        <el-form-item v-if="draft.has_public_ip" label="公网 IP">
          <el-input v-model="draft.public_ip" placeholder="IPv4 或 IPv6 地址，不含端口" />
        </el-form-item>
        <div v-if="editingRow?.candidatePublicIP" class="candidate-ip">
          对端地址识别候选：<code>{{ editingRow.candidatePublicIP }}</code>
          <el-button size="small" @click="chooseCandidate">使用此 IP</el-button>
        </div>
        <p class="editor-note">对端地址可能指向中继服务器，不能据此保证节点拥有公网 IP；请确认后再保存。</p>
        <h3>节点位置</h3>
        <NetworkNodeLocationPicker :latitude="draft.location.latitude" :longitude="draft.location.longitude" @select="onMapSelect" />
        <div class="location-coordinate-grid">
          <el-form-item label="纬度"><el-input v-model="latitudeText" placeholder="-85.0511 至 85.0511" /></el-form-item>
          <el-form-item label="经度"><el-input v-model="longitudeText" placeholder="-180 至 180" /></el-form-item>
        </div>
        <div class="location-actions">
          <el-button @click="coordinatesFromInputs">定位到输入坐标</el-button>
          <el-button :loading="locationLookingUp" @click="lookupLocation">根据坐标识别行政区划</el-button>
        </div>
        <div class="location-address-grid">
          <el-form-item label="国家"><el-input v-model="draft.location.country" maxlength="120" /></el-form-item>
          <el-form-item label="省 / 州"><el-input v-model="draft.location.province" maxlength="120" /></el-form-item>
          <el-form-item label="市"><el-input v-model="draft.location.city" maxlength="120" /></el-form-item>
          <el-form-item label="县 / 区"><el-input v-model="draft.location.county" maxlength="120" /></el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="editorOpen = false">取消</el-button>
        <el-button type="primary" :loading="editorSaving" @click="saveAttributes">保存属性</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.network-pane { margin: 24px; padding: 24px; min-width: 0; border: 1px solid var(--ent-line-soft); border-radius: 16px; background: #0e1c2e; }
.candidate-ip { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; font-size: 13px; }
.candidate-ip code { color: #8ccffc; }
.editor-note { font-size: 12px; color: #8bacca; line-height: 1.5; }
.node-editor h3 { font-size: 15px; margin: 20px 0 12px; }
.location-coordinate-grid, .location-address-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 14px; margin-top: 16px; }
.location-actions { display: flex; gap: 8px; flex-wrap: wrap; margin: 0 0 14px; }
@media (max-width: 540px) { .location-coordinate-grid, .location-address-grid { grid-template-columns: 1fr; } }
.network-intro h2 { margin: 0; font-size: 19px; }
.network-intro p, .network-note { color: #8bacca; font-size: 12px; line-height: 1.7; }
.network-toolbar { display: flex; gap: 12px; margin-top: 22px; }
.entry-select { min-width: 180px; }
.network-alert { margin-top: 12px; }
.network-summary { display: flex; justify-content: space-between; gap: 12px; margin: 12px 0; color: #8bacca; font-size: 12px; }
.node-name { font-weight: 650; }
.node-meta { margin-top: 5px; color: #8bacca; font-size: 12px; overflow-wrap: anywhere; }
.node-flags { display: flex; gap: 5px; flex-wrap: wrap; margin-top: 5px; }
.network-desktop-table { container-type: inline-size; }
:deep(.el-table__expanded-cell) { padding: 0; }
/* 列表可横向滚动，展开详情始终适配可见宽度。 */
:deep(.el-table__expanded-cell .node-details) { width: 100cqw; box-sizing: border-box; position: sticky; left: 0; }
:deep(.network-desktop-table .el-table__row) { cursor: pointer; }
:deep(.network-desktop-table .el-table__row:hover > td) { background: var(--el-fill-color-light); }
.route-cell { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.network-node-card { cursor: pointer; }
.network-node-card:focus-visible { outline: 2px solid var(--el-color-primary); outline-offset: 2px; }
.network-mobile-list, .network-mobile-empty { display: none; }
@media (max-width: 760px) {
  .network-pane { margin: 12px; padding: 14px; }
  .network-toolbar { flex-wrap: wrap; }
  .entry-select { width: 100%; }
  .network-summary { flex-wrap: wrap; }
  .network-desktop-table { display: none; }
  .network-mobile-empty { display: block; padding: 24px; text-align: center; color: #8bacca; }
  .network-mobile-list { display: grid; gap: 10px; }
  .network-node-card { min-width: 0; padding: 13px; border: 1px solid #28435c; border-radius: 12px; background: #11263c; }
  .network-node-title, .network-node-details { display: flex; align-items: center; gap: 9px; flex-wrap: wrap; }
  .network-node-title strong { overflow-wrap: anywhere; }
  .network-node-ip { margin: 10px 0; font-family: ui-monospace, monospace; }
  .network-node-details { color: #bad3e8; font-size: 12px; }
}
</style>
