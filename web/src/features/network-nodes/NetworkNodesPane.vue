<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { mergedNetworkRows, networkRows } from './data'

const props = defineProps<{ hosts: Host[]; hostsLoading: boolean; hostsError: string }>()
const entries = computed(() => props.hosts.filter(host => host.network_entry))
const hostId = ref('')
const view = ref<'merged' | 'node' | 'route'>('merged')
const search = ref('')
watch(entries, items => {
  if (!items.some(host => host.id === hostId.value)) hostId.value = items[0]?.id || ''
}, { immediate: true })

// 总览用单次 route 查询：含 node_info 与 peer_routes。
// 旧入口实例、网络路由两个视图保持独立查询，便于核对来源。
const query = useQuery({
  queryKey: computed(() => ['network-nodes', hostId.value, view.value]),
  queryFn: async ({ queryKey, signal }) => {
    const id = encodeURIComponent(String(queryKey[1]))
    const mode = String(queryKey[2])
    const capability = mode === 'node' ? 'node' : 'route'
    const response = await apiRequest<{ data: unknown }>(
      '/api/v1/hosts/' + id + '/network/' + capability, { signal })
    return mode === 'merged' ? mergedNetworkRows(response.data) : networkRows(response.data, capability)
  },
  enabled: computed(() => !!hostId.value),
  retry: 0,
  refetchInterval: 15000
})
const loading = computed(() => query.isFetching.value)
const error = computed(() => query.error.value ? formatApiError(query.error.value) : '')
const rows = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  return (query.data.value || []).filter(row =>
    !keyword || Object.entries(row).some(([key, value]) =>
      key !== 'key' && String(value).toLocaleLowerCase().includes(keyword)))
})
const updatedAt = computed(() =>
  query.dataUpdatedAt.value ? new Date(query.dataUpdatedAt.value).toLocaleTimeString() : '尚未采集')
</script>

<template>
  <section class="network-pane" aria-label="EasyTier 网络节点">
    <div class="network-intro">
      <h2>EasyTier 网络节点</h2>
      <p>节点总览合并入口实例和网络路由；独立查询视图保留用于核对，每 15 秒更新。</p>
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

      <el-tabs v-model="view" class="network-tabs" aria-label="网络数据视图">
        <el-tab-pane label="节点总览" name="merged" />
        <el-tab-pane label="入口实例" name="node" />
        <el-tab-pane label="网络路由" name="route" />
      </el-tabs>

      <el-alert v-if="error" :title="error"
        :description="query.data.value ? '以下保留上次成功采集的数据，不代表当前状态。' : '请检查组网入口、EasyTier Driver、容器与 RPC 状态。'"
        type="warning" :closable="false" show-icon class="network-alert" />
      <div class="network-summary">
        <span>{{ rows.length }} {{ view === 'merged' ? '个网络节点' : '条记录' }}</span>
        <span>最近成功采集：{{ updatedAt }}</span>
      </div>

      <template v-if="view === 'merged'">
        <el-table v-loading="loading" :data="rows" row-key="key"
          empty-text="当前没有匹配的网络节点" stripe class="network-desktop-table">
          <el-table-column label="节点" min-width="155">
            <template #default="{ row }">
              <div class="node-name">{{ row.hostname }}</div>
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
          <el-table-column prop="subnets" label="代理网段" min-width="165" show-overflow-tooltip />
          <el-table-column prop="version" label="版本" min-width="130" show-overflow-tooltip />
          <el-table-column type="expand" width="42">
            <template #default="{ row }">
              <dl class="node-detail">
                <div><dt>节点 ID</dt><dd>{{ row.id }}</dd></div>
                <div><dt>实例 ID</dt><dd>{{ row.instanceID }}</dd></div>
                <div><dt>连接报告</dt><dd>{{ row.connection }}</dd></div>
                <div><dt>下一跳节点</dt><dd>{{ row.nextHop }}</dd></div>
                <div><dt>路由开销</dt><dd>{{ row.cost }}</dd></div>
                <div><dt>直连延迟</dt><dd>{{ row.directLatency }}</dd></div>
                <div><dt>累计接收</dt><dd>{{ row.rx }}</dd></div>
                <div><dt>累计发送</dt><dd>{{ row.tx }}</dd></div>
                <div v-if="row.kind === '入口实例'"><dt>公网地址</dt><dd>{{ row.publicIP }}</dd></div>
                <div v-if="row.kind === '入口实例'"><dt>监听地址</dt><dd>{{ row.listeners }}</dd></div>
              </dl>
            </template>
          </el-table-column>
        </el-table>
        <div v-if="!rows.length && !loading" class="network-mobile-empty">当前没有匹配的网络节点</div>
        <div class="network-mobile-list">
          <article v-for="row in rows" :key="row.key" class="network-node-card">
            <div class="network-node-title">
              <strong>{{ row.hostname }}</strong>
              <el-tag size="small" :type="row.kind === '入口实例' ? 'success' : 'info'">{{ row.kind }}</el-tag>
            </div>
            <div class="node-meta" :title="row.instanceID">{{ row.instance }} · {{ row.id }}</div>
            <div class="network-node-ip">{{ row.cidr }}</div>
            <div class="network-node-details">
              <span>{{ row.routeLabel }}</span><span>{{ row.protocol }}</span><span>{{ row.latency }}</span>
            </div>
            <div class="node-meta">代理网段：{{ row.subnets }} · 版本：{{ row.version }}</div>
          </article>
        </div>
      </template>

      <el-table v-else v-loading="loading" :data="rows"
        empty-text="当前没有匹配的网络记录" stripe class="network-legacy-table">
        <el-table-column prop="instance" label="网络实例" min-width="145" />
        <el-table-column prop="hostname" label="主机名" min-width="130" />
        <el-table-column prop="id" label="节点 ID" min-width="125" />
        <el-table-column prop="cidr" label="虚拟 IPv4" min-width="145" />
        <el-table-column prop="subnets" label="代理网段" min-width="170" />
        <template v-if="view === 'node'">
          <el-table-column prop="publicIP" label="公网地址" min-width="150" />
          <el-table-column prop="listeners" label="监听地址" min-width="220" />
        </template>
        <template v-else>
          <el-table-column prop="routeLabel" label="路径" min-width="130" />
          <el-table-column prop="nextHop" label="下一跳节点" min-width="135" />
          <el-table-column prop="cost" label="路由开销" min-width="90" />
          <el-table-column prop="protocol" label="协议" min-width="95" />
          <el-table-column prop="latency" label="路径延迟" min-width="110" />
        </template>
        <el-table-column prop="version" label="版本" min-width="120" />
      </el-table>
      <p class="network-note">
        本页只展示所选入口当前可见的数据；路由可见不等于设备持续在线。节点总览来自 route CLI 的同一次采集，独立视图用于交叉核对。
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
.network-tabs { margin-top: 12px; }
.network-alert { margin-top: 12px; }
.network-summary { display: flex; justify-content: space-between; gap: 12px; margin: 12px 0; color: #8bacca; font-size: 12px; }
.node-name { font-weight: 650; }
.node-meta { margin-top: 5px; color: #8bacca; font-size: 12px; overflow-wrap: anywhere; }
.route-cell { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.node-detail { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 10px 20px; margin: 0; padding: 10px 22px; }
.node-detail div { min-width: 0; }
.node-detail dt { color: #8bacca; font-size: 12px; }
.node-detail dd { margin: 4px 0 0; overflow-wrap: anywhere; }
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
