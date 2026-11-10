<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { mergedNetworkRows, type NetworkRow } from './data'
import NetworkNodeDetails from './NetworkNodeDetails.vue'

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
const loading = computed(() => query.isFetching.value)
const refreshing = computed(() => query.isFetching.value && !!query.data.value)
const error = computed(() => query.error.value ? formatApiError(query.error.value) : '')
const rows = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  return (query.data.value || []).filter(row =>
    !keyword || Object.entries(row).some(([key, value]) =>
      key !== 'key' && key !== 'connections' && String(value).toLocaleLowerCase().includes(keyword)))
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
              <div class="node-name">{{ row.hostname }}</div>
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
              <NetworkNodeDetails :row="row" />
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
              <strong>{{ row.hostname }}</strong>
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
            <NetworkNodeDetails v-if="expandedKeys.includes(row.key)" :row="row" />
          </article>
        </div>
      <p class="network-note">
        本页只展示所选入口当前可见的节点；路由可见不等于设备持续在线。刷新时保持列表可见，可随时展开节点详情。
      </p>
    </template>
  </section>
</template>

<style scoped>
.network-pane { margin: 24px; padding: 24px; min-width: 0; border: 1px solid var(--ent-line-soft); border-radius: 16px; background: #0e1c2e; }
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
