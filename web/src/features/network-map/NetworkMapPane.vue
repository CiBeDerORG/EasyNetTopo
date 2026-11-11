<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useRouter } from 'vue-router'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { listNodeAttributes } from '../network-nodes/attributes'
import { mergedNetworkRows } from '../network-nodes/data'
import GlobeMap from './GlobeMap.vue'
import { buildMapTopology, trafficRates, type MapLink, type MapNode, type TrafficSample } from './topology'

const props = defineProps<{ hosts: Host[]; hostsLoading: boolean; hostsError: string }>()
const router = useRouter()
const entries = computed(() => props.hosts.filter(host => host.network_entry))
const hostId = ref('')
watch(entries, items => {
  if (!items.some(host => host.id === hostId.value)) hostId.value = items[0]?.id || ''
}, { immediate: true })

const routeQuery = useQuery({
  queryKey: computed(() => ['network-map-route', hostId.value]),
  enabled: computed(() => !!hostId.value),
  retry: 0, refetchInterval: 15000,
  queryFn: async ({ queryKey, signal }) => {
    const response = await apiRequest<{ data: unknown }>(
      '/api/v1/hosts/' + encodeURIComponent(String(queryKey[1])) + '/network/route', { signal })
    mergedNetworkRows(response.data)
    return response.data
  }
})
const peerQuery = useQuery({
  queryKey: computed(() => ['network-map-peer-center', hostId.value]),
  enabled: computed(() => !!hostId.value),
  retry: 0, refetchInterval: 30000,
  queryFn: async ({ queryKey, signal }) => {
    const response = await apiRequest<{ data: unknown }>(
      '/api/v1/hosts/' + encodeURIComponent(String(queryKey[1])) + '/network/peer-center', { signal })
    return response.data
  }
})
const attrsQuery = useQuery({
  queryKey: ['network-node-attributes'], queryFn: listNodeAttributes,
  enabled: computed(() => entries.value.length > 0), retry: 0
})
const rawTopology = computed(() => {
  if (!routeQuery.data.value) return { nodes: [] as MapNode[], links: [] as MapLink[] }
  try {
    return buildMapTopology(routeQuery.data.value, attrsQuery.data.value?.attributes || [],
      hostId.value, peerQuery.data.value)
  } catch {
    return { nodes: [] as MapNode[], links: [] as MapLink[] }
  }
})
const samples = new Map<string, TrafficSample>()
const currentRates = ref<Record<string, { rxBps: number | null; txBps: number | null }>>({})
watch(hostId, () => { samples.clear(); currentRates.value = {}; selectedNodeKey.value = ''; selectedLinkKey.value = '' })
watch(() => routeQuery.dataUpdatedAt.value, () => {
  if (!routeQuery.data.value) return
  const current = rawTopology.value
  const now = routeQuery.dataUpdatedAt.value
  const rates: Record<string, { rxBps: number | null; txBps: number | null }> = {}
  const remaining = new Set<string>()
  for (const link of current.links.filter(item => item.source === 'route')) {
    const sample = { rx: link.rxBytes, tx: link.txBytes, time: now }
    rates[link.key] = trafficRates(samples.get(link.key), sample)
    samples.set(link.key, sample)
    remaining.add(link.key)
  }
  for (const key of samples.keys()) if (!remaining.has(key)) samples.delete(key)
  currentRates.value = rates
})
const topology = computed(() => ({
  nodes: rawTopology.value.nodes,
  links: rawTopology.value.links.map(link => ({
    ...link, ...currentRates.value[link.key]
  }))
}))
const basemap = ref<'auto' | 'standard' | 'satellite' | 'dark'>('auto')
const filter = ref<'all' | 'p2p' | 'relay' | 'unknown'>('all')
const showTraffic = ref(true)
const visibleLinks = computed(() =>
  topology.value.links.filter(link => filter.value === 'all' || link.kind === filter.value))
const withLocation = computed(() => topology.value.nodes.filter(node =>
  node.latitude !== null && node.longitude !== null))
const withoutLocation = computed(() => topology.value.nodes.filter(node =>
  node.latitude === null || node.longitude === null))
const selectedNodeKey = ref('')
const selectedLinkKey = ref('')
const selectedNode = computed(() => topology.value.nodes.find(node => node.key === selectedNodeKey.value))
const selectedLink = computed(() => topology.value.links.find(link => link.key === selectedLinkKey.value))
const byKey = computed(() => new Map(topology.value.nodes.map(node => [node.key, node])))
const lastUpdated = computed(() => routeQuery.dataUpdatedAt.value
  ? new Date(routeQuery.dataUpdatedAt.value).toLocaleTimeString() : '尚未采集')
const routeError = computed(() => routeQuery.error.value ? formatApiError(routeQuery.error.value) : '')
const peerAvailable = computed(() => peerQuery.isSuccess.value)
const globeRef = ref<{ focusNode: (key: string) => void; resetCamera: () => void } | null>(null)
function selectNode(key: string) {
  selectedNodeKey.value = key
  selectedLinkKey.value = ''
  globeRef.value?.focusNode(key)
}
function selectLink(key: string) {
  selectedNodeKey.value = ''
  selectedLinkKey.value = key
}
function formatSpeed(bytes: number | null): string {
  if (bytes === null) return '暂无连续采样'
  if (bytes < 1024) return bytes.toFixed(0) + ' B/s'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KiB/s'
  return (bytes / 1048576).toFixed(2) + ' MiB/s'
}
function formatMs(value: number | null) {
  return value === null ? '—' : value.toFixed(1) + ' ms'
}
</script>

<template>
  <section class="map-page" aria-label="EasyTier 网络地图">
    <div class="map-header">
      <div class="map-header-copy">
        <div class="map-eyebrow">EASYTIER · GEO NETWORK</div>
        <p>基于已迁移的网络节点与真实路由信息，展示节点位置及直连拓扑。</p>
      </div>
      <div class="map-toolbar">
        <el-select v-model="hostId" placeholder="组网入口" aria-label="组网入口" class="map-entry">
          <el-option v-for="host in entries" :key="host.id" :label="host.name" :value="host.id" />
        </el-select>
        <el-select v-model="basemap" class="basemap-select" aria-label="地图底图">
          <el-option value="auto" label="跟随页面主题" />
          <el-option value="standard" label="标准地图" />
          <el-option value="satellite" label="卫星影像" />
          <el-option value="dark" label="深色地图" />
        </el-select>
        <el-button :loading="routeQuery.isFetching.value" @click="routeQuery.refetch(); peerQuery.refetch()">
          刷新
        </el-button>
      </div>
    </div>

    <el-alert v-if="hostsError" type="error" :title="hostsError" :closable="false" />
    <div v-if="hostsLoading" class="map-empty">正在读取组网入口…</div>
    <el-empty v-else-if="!entries.length" description="尚未设置组网入口">
      <el-button type="primary" @click="router.push('/host-store')">前往主机商店设置入口</el-button>
    </el-empty>
    <template v-else>
      <div v-if="routeError" class="status-alert" role="alert">
        {{ routeError }}。{{ routeQuery.data.value ? '以下保留上一次成功采集的节点，不代表当前实时状态。' : '请检查 Agent、EasyTier Driver 及 RPC 连接。' }}
      </div>
      <div class="map-stats">
        <div><span>当前路由可见节点</span><strong>{{ topology.nodes.length }}</strong></div>
        <div><span>已定位</span><strong>{{ withLocation.length }}</strong></div>
        <div><span>P2P 直连关系</span><strong class="cyan">{{ topology.links.filter(link => link.kind === 'p2p').length }}</strong></div>
        <div><span>中继路由</span><strong class="orange">{{ topology.links.filter(link => link.kind === 'relay').length }}</strong></div>
        <div><span>最近采集</span><strong class="stamp">{{ lastUpdated }}</strong></div>
      </div>
      <div class="map-layout">
        <div class="globe-area">
          <GlobeMap ref="globeRef" :nodes="topology.nodes" :links="visibleLinks"
            :basemap="basemap" :show-traffic="showTraffic"
            :selected-node-key="selectedNodeKey" :selected-link-key="selectedLinkKey"
            @node-click="selectNode" @link-click="selectLink" />
          <div class="map-legend">
            <span><i class="line p2p" /> P2P 直连</span>
            <span><i class="line relay" /> 中继路由</span>
            <span><i class="line unknown" /> 未知</span>
            <span><i class="point" /> 地图节点</span>
          </div>
          <div class="map-controls">
            <el-select v-model="filter" aria-label="连接筛选" size="small" class="link-filter">
              <el-option value="all" label="全部连接" />
              <el-option value="p2p" label="仅直连" />
              <el-option value="relay" label="仅中继" />
              <el-option value="unknown" label="未知路由" />
            </el-select>
            <el-switch v-model="showTraffic" size="small" active-text="真实流量粒子" />
          </div>
          <div class="map-source-info">
            {{ peerAvailable ? '全网直连拓扑已接入 Peer Center' : '显示入口路由拓扑 · Peer Center 暂不可用' }}
            · 每 15 秒后台刷新
          </div>
        </div>

        <aside class="map-side">
          <section class="side-section">
            <h3>选中详情</h3>
            <template v-if="selectedNode">
              <div class="detail-title">{{ selectedNode.label }}</div>
              <dl>
                <div><dt>主机名</dt><dd>{{ selectedNode.hostname }}</dd></div>
                <div><dt>虚拟地址</dt><dd>{{ selectedNode.ip }}</dd></div>
                <div><dt>节点 ID</dt><dd>{{ selectedNode.id }}</dd></div>
                <div><dt>网络实例</dt><dd>{{ selectedNode.instance }}</dd></div>
                <div><dt>地理位置</dt><dd>{{ selectedNode.location || '尚未设置' }}</dd></div>
                <div><dt>公网 IP</dt><dd>{{ selectedNode.publicIP || '未确认' }}</dd></div>
                <div><dt>NAT 类型</dt><dd>{{ selectedNode.natType }}</dd></div>
                <div><dt>版本</dt><dd>{{ selectedNode.version }}</dd></div>
                <div><dt>节点角色</dt><dd>{{ selectedNode.flags.join('、') || '普通节点' }}</dd></div>
              </dl>
            </template>
            <template v-else-if="selectedLink">
              <div class="detail-title">
                {{ byKey.get(selectedLink.from)?.label || '未知节点' }} →
                {{ byKey.get(selectedLink.to)?.label || '未知节点' }}
              </div>
              <dl>
                <div><dt>连接类型</dt><dd>{{ selectedLink.kind === 'p2p' ? 'P2P 直连' : selectedLink.kind === 'relay' ? '中继转发' : '路由未知' }}</dd></div>
                <div><dt>来源</dt><dd>{{ selectedLink.source === 'peer-center' ? 'Peer Center 全局直连关系' : '入口 Route 路由' }}</dd></div>
                <div><dt>路径延迟</dt><dd>{{ formatMs(selectedLink.latencyMs) }}</dd></div>
                <div><dt>下一跳 ID</dt><dd>{{ selectedLink.nextHop }}</dd></div>
                <div><dt>路由开销</dt><dd>{{ selectedLink.cost ?? '—' }}</dd></div>
                <div><dt>传输协议</dt><dd>{{ selectedLink.protocol }}</dd></div>
                <div><dt>丢包率</dt><dd>{{ selectedLink.lossPercent === null ? '—' : selectedLink.lossPercent + '%' }}</dd></div>
                <div><dt>发送速度</dt><dd>{{ formatSpeed(selectedLink.txBps) }}</dd></div>
                <div><dt>接收速度</dt><dd>{{ formatSpeed(selectedLink.rxBps) }}</dd></div>
              </dl>
              <p v-if="selectedLink.kind === 'relay'" class="side-caption">
                弧线表达已报告的下一跳路由；没有完整中继路径数据时，不代表实际物理传输轨迹。
              </p>
            </template>
            <p v-else class="side-caption">点击地球上的节点或连接线查看详情；拖动旋转，滚轮缩放到城市级地图。</p>
          </section>
          <section class="side-section">
            <div class="side-heading"><h3>待定位节点</h3><span>{{ withoutLocation.length }}</span></div>
            <div v-if="withoutLocation.length" class="missing-nodes">
              <button v-for="node in withoutLocation" :key="node.key" type="button"
                class="missing-node" @click="selectNode(node.key)">
                <span>{{ node.label }}</span><small>{{ node.ip }}</small>
              </button>
            </div>
            <p v-else class="side-caption">当前路由中的节点均已设置位置。</p>
            <el-button class="edit-location" text type="primary" @click="router.push('/network-nodes')">
              前往网络节点编辑位置
            </el-button>
          </section>
          <p class="side-footnote">地图使用人工确认的 WGS84 经纬度。路由可见不等于持续在线；不展示虚构地理位置、离线状态或未采集的流量。</p>
        </aside>
      </div>
    </template>
  </section>
</template>

<style scoped>
.map-page { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0;
  padding: 16px 20px 20px; gap: 12px; color: var(--ent-text); overflow: hidden; }
.map-header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; }
.map-header-copy p { margin: 5px 0 0; font-size: 12px; color: var(--ent-muted); }
.map-eyebrow { font-size: 10px; color: var(--ent-cyan); font-weight: 750; letter-spacing: 1.5px; }
.map-toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.map-entry { width: 172px; } .basemap-select { width: 118px; }
.map-stats { display: grid; grid-template-columns: repeat(5,minmax(0,1fr)); gap: 9px; }
.map-stats > div { display: flex; flex-direction: column; padding: 10px 13px; border: 1px solid var(--ent-line);
  border-radius: 11px; background: var(--ent-panel); }
.map-stats span { font-size: 11px; color: var(--ent-muted); }
.map-stats strong { font-size: 23px; margin-top: 5px; font-variant-numeric: tabular-nums; }
.map-stats strong.stamp { font-size: 16px; margin-top: 9px; }
.map-stats strong.cyan { color: #25bfcf; } .map-stats strong.orange { color: #e39a48; }
.status-alert { border-radius: 8px; background: #5c401c37; color: var(--ent-text);
  font-size: 12px; padding: 8px 12px; }
.map-empty { margin: 30px; color: var(--ent-muted); }
.map-layout { display: grid; grid-template-columns: minmax(0, 1fr) 286px; min-height: 0; flex: 1; gap: 12px; }
.globe-area { position: relative; min-height: 0; overflow: hidden; border: 1px solid var(--ent-line);
  border-radius: 14px; background: #061323; }
:global(:root[data-theme='light']) .globe-area { background: #dceaf4; }
.map-legend { position: absolute; left: 16px; bottom: 16px; background: #0b1f32e8; color: #dbeaf8;
  border: 1px solid #28465b; padding: 10px 12px; border-radius: 10px; display: flex; flex-wrap: wrap; gap: 12px; font-size: 11px; box-shadow: 0 6px 20px #0003; backdrop-filter: blur(10px); }
:global(:root[data-theme='light']) .map-legend { background: #ffffffed; color: #29445d; border-color: #bfd0df; box-shadow: 0 5px 18px #365b7620; }
.map-legend span { display: inline-flex; align-items: center; gap: 7px; }
.line { display: inline-block; width: 24px; border-top: 2px solid; }
.line.p2p { border-color: #38d8ff; border-top-style: solid; border-top-width: 3px; box-shadow: 0 0 8px #3ae6ff; }
.line.relay { border-color: #ffae55; border-top-style: dashed; border-top-width: 3px; }
.line.unknown { border-color: #b1bfd0; border-top-style: dotted; border-top-width: 2px; }
:global(:root[data-theme='light']) .line.p2p { border-color: #087f9a; box-shadow: 0 0 6px #087f9a55; }
:global(:root[data-theme='light']) .line.relay { border-color: #a85b0a; }
:global(:root[data-theme='light']) .line.unknown { border-color: #74899a; }
.point { width: 9px; height: 9px; border: 1px solid white; border-radius: 50%; background: #36dcec; }
.map-controls { position: absolute; top: 15px; right: 16px; display: flex; gap: 12px; align-items: center;
  padding: 7px 9px; border-radius: 10px; background: #0c2138e9; color: #eafaff; border: 1px solid #31516a; box-shadow: 0 5px 18px #0002; backdrop-filter: blur(10px); }
:global(:root[data-theme='light']) .map-controls { background: #ffffffed; color: #29445d; border-color: #bfd0df; box-shadow: 0 5px 18px #365b7620; }
:global(:root[data-theme='light']) .map-controls .el-switch { --el-switch-on-color: #087f9a; }
:global(:root[data-theme='light']) .map-controls .el-select__wrapper { background: #f6fafd; box-shadow: 0 0 0 1px #c2d3e1 inset; }
:global(:root[data-theme='light']) .map-controls .el-select__selected-item { color: #29445d; }
.link-filter { width: 116px; }
.map-source-info { position: absolute; bottom: 62px; left: 16px; color: #b7d8ec;
  padding: 5px 8px; border-radius: 5px; background: #081b2be8; border: 1px solid #29485f; font-size: 10px; backdrop-filter: blur(8px); }
:global(:root[data-theme='light']) .map-source-info { color: #4c657b; background: #ffffffed; border-color: #c2d3e1; }
.map-side { min-width: 0; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: 11px; }
.side-section { padding: 15px; border: 1px solid var(--ent-line); border-radius: 13px; background: var(--ent-panel); }
.side-section h3 { margin: 0 0 11px; font-size: 13px; }
.detail-title { font-size: 15px; font-weight: 750; color: var(--ent-cyan); overflow-wrap: anywhere; margin-bottom: 12px; }
dl { display: grid; gap: 9px; margin: 0; } dl div { display: grid; gap: 3px; }
dt { font-size: 11px; color: var(--ent-muted); }
dd { margin: 0; font-size: 12px; font-weight: 600; color: var(--ent-text); overflow-wrap: anywhere; }
.side-caption, .side-footnote { color: var(--ent-muted); font-size: 12px; line-height: 1.7; }
.side-heading { display: flex; justify-content: space-between; align-items: center; }
.side-heading span { color: var(--ent-cyan); font-weight: 700; }
.missing-nodes { display: grid; max-height: 220px; overflow: auto; gap: 7px; }
.missing-node { display: flex; align-items: center; justify-content: space-between; gap: 8px; text-align: left;
  border: 1px solid var(--ent-line); border-radius: 8px; padding: 9px 8px;
  background: var(--ent-panel-2); color: var(--ent-text); cursor: pointer; }
.missing-node:hover { border-color: var(--ent-cyan); }
.missing-node small { color: var(--ent-muted); white-space: nowrap; }
.edit-location { margin-top: 8px; }
.side-footnote { margin: 0 4px; }
@media (max-width: 1080px) {
  .map-layout { grid-template-columns: minmax(0,1fr) 248px; }
  .map-page { padding: 12px; }
  .map-stats { grid-template-columns: repeat(3,minmax(0,1fr)); }
}
@media (max-width: 760px) {
  .map-page { overflow: auto; }
  .map-layout { grid-template-columns: 1fr; display: flex; flex-direction: column; overflow: visible; }
  .globe-area { height: 480px; flex: none; }
  .map-side { overflow: visible; }
  .map-stats { grid-template-columns: repeat(2,minmax(0,1fr)); }
  .map-legend { max-width: 80%; }
}
</style>
