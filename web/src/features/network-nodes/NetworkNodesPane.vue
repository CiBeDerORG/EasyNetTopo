<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { networkRows } from './data'

const props = defineProps<{ hosts: Host[]; hostsLoading: boolean; hostsError: string }>()
const entries = computed(() => props.hosts.filter(host => host.network_entry))
const hostId = ref('')
const capability = ref('peer')
const search = ref('')
watch(entries, items => { if (!items.some(host => host.id === hostId.value)) hostId.value = items[0]?.id || '' }, { immediate: true })
const query = useQuery({
  queryKey: computed(() => ['network-data', hostId.value, capability.value]),
  queryFn: async ({ queryKey, signal }) => {
    const id = String(queryKey[1]), kind = String(queryKey[2])
    const response = await apiRequest<{ data: unknown }>('/api/v1/hosts/' + encodeURIComponent(id) + '/network/' + kind, { signal })
    return networkRows(response.data, kind)
  },
  enabled: computed(() => !!hostId.value), retry: 0, refetchInterval: 15000
})
const loading = computed(() => query.isFetching.value)
const error = computed(() => query.error.value ? formatApiError(query.error.value) : '')
const rows = computed(() => (query.data.value || []).filter(row => Object.values(row).some(value => String(value).toLowerCase().includes(search.value.toLowerCase()))))
const updatedAt = computed(() => query.dataUpdatedAt.value ? new Date(query.dataUpdatedAt.value).toLocaleTimeString() : '尚未采集')
</script>

<template>
  <section class="network-pane" aria-label="EasyTier 网络节点">
    <div class="network-intro"><h2>EasyTier 网络节点</h2><p>通过组网入口查看网络实例、对等节点与子网路由，每 15 秒更新。</p></div>
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
        <el-input v-model="search" clearable placeholder="搜索主机名 / 虚拟 IP / 子网" aria-label="搜索网络节点" />
        <el-button :loading="loading" @click="query.refetch()">刷新</el-button>
      </div>
      <el-tabs v-model="capability">
        <el-tab-pane label="对等节点" name="peer" />
        <el-tab-pane label="入口实例" name="node" />
        <el-tab-pane label="网络路由" name="route" />
      </el-tabs>
      <el-alert v-if="error" :title="error" :description="query.data.value ? '下方保留上次成功采集的数据，不能作为当前在线状态。' : '请在主机的 EasyHostAgent 设置中启用 EasyTier Driver，并确认 EasyTier 正在运行。'" type="warning" :closable="false" show-icon />
      <div class="network-summary"><span>{{ rows.length }} 条记录</span><span>最近成功采集：{{ updatedAt }}</span></div>
      <el-table v-loading="loading" :data="rows" empty-text="当前没有匹配的网络记录" stripe>
        <el-table-column prop="instance" label="网络实例" min-width="150" />
        <el-table-column prop="hostname" label="主机名" min-width="130" />
        <el-table-column prop="id" label="节点 ID" min-width="120" />
        <el-table-column prop="ip" label="虚拟 IPv4" min-width="145" />
        <el-table-column prop="subnets" label="代理网段" min-width="175" />
        <template v-if="capability === 'peer'">
          <el-table-column prop="connection" label="连接方式" min-width="110" />
          <el-table-column prop="protocol" label="协议" min-width="90" />
          <el-table-column prop="latency" label="直连延迟" min-width="110" />
          <el-table-column prop="rx" label="接收累计" min-width="125" />
          <el-table-column prop="tx" label="发送累计" min-width="125" />
        </template>
        <template v-if="capability === 'node'">
          <el-table-column prop="publicIP" label="公网地址" min-width="150" />
          <el-table-column prop="listeners" label="监听地址" min-width="220" />
        </template>
        <template v-if="capability === 'route'">
          <el-table-column prop="nextHop" label="下一跳节点" min-width="130" />
          <el-table-column prop="cost" label="路由开销" min-width="100" />
        </template>
        <el-table-column prop="version" label="版本" min-width="100" />
      </el-table>
      <p class="network-note">展示入口当前可见的数据，不代表所有设备或历史在线记录；累计流量来自入口的直连连接。</p>
    </template>
  </section>
</template>

<style scoped>
.network-pane { margin: 24px; padding: 24px; min-width: 0; border: 1px solid var(--ent-line-soft); border-radius: 16px; background: #0e1c2e; }
.network-intro h2 { margin: 0; font-size: 19px; }
.network-intro p, .network-note { color: #8bacca; font-size: 12px; line-height: 1.7; }
.network-toolbar { display: flex; gap: 12px; margin-top: 22px; }
.entry-select { min-width: 180px; }
.network-summary { display: flex; justify-content: space-between; margin: 16px 0; color: #8bacca; font-size: 12px; }
@media (max-width: 700px) { .network-pane { margin: 12px; padding: 14px; } .network-toolbar { flex-wrap: wrap; } .entry-select { width: 100%; } }
</style>
