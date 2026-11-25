<script setup lang="ts">
import { computed, defineAsyncComponent, h, nextTick, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useRoute, useRouter } from 'vue-router'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { listNodeAttributes } from '../network-nodes/attributes'
import NetworkNodeEditor from '../network-nodes/NetworkNodeEditor.vue'
import { nodeEditingIssue } from '../network-nodes/identity'
import { mergedNetworkRows } from '../network-nodes/data'
import { buildMapTopology, trafficRates, type MapLink, type MapNode, type TrafficSample } from './topology'

 // 将 Cesium 地球引擎拆成第二层异步模块。路由数据、统计和侧栏先渲染，
 // 大体积 WebGL 引擎随后下载，不再拖住地图页面的可见内容。
const GlobeMap = defineAsyncComponent({
  loader: () => import('./GlobeMap.vue'),
  loadingComponent: {
    setup: () => () => h('div', { class: 'globe-loading', role: 'status' }, [
      h('span', { class: 'globe-loading-spinner', 'aria-hidden': 'true' }),
      h('span', '正在后台加载 3D 地图引擎…')
    ])
  },
  errorComponent: {
    setup: () => () => h('div', { class: 'globe-load-error', role: 'alert' },
      '3D 地图引擎加载失败，请检查网络后刷新页面重试。')
  },
  delay: 0,
  timeout: 60_000,
  onError: (_error, retry, fail, attempts) => {
    if (attempts <= 2) retry()
    else fail()
  }
})

const props = defineProps<{ hosts: Host[]; hostsLoading: boolean; hostsError: string }>()
const router = useRouter()
const route = useRoute()
const entries = computed(() => props.hosts.filter(host => host.network_entry))
const requestedEntry = computed(() => typeof route.query.entry === 'string' ? route.query.entry : '')
const editingLocation = computed(() => route.query.edit === 'location')
const hostId = computed(() => {
  if (requestedEntry.value) return entries.value.find(host => host.id === requestedEntry.value)?.id || ''
  return editingLocation.value ? '' : entries.value[0]?.id || ''
})
function changeEntry(id: string) {
  void router.replace({ path: route.path, query: { ...route.query, entry: id, node: undefined, edit: undefined } })
}

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
watch(hostId, () => { samples.clear(); currentRates.value = {}; selectedLinkKey.value = '' })
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
const selectedNodeKey = computed(() => typeof route.query.node === 'string' ? route.query.node : '')
const selectedLinkKey = ref('')
const selectedNode = computed(() => topology.value.nodes.find(node => node.key === selectedNodeKey.value))
const selectedLink = computed(() => topology.value.links.find(link => link.key === selectedLinkKey.value))
watch(visibleLinks, links => {
  if (selectedLinkKey.value && !links.some(link => link.key === selectedLinkKey.value)) selectedLinkKey.value = ''
})
const detailHeading = ref<HTMLElement | null>(null)
async function showDetails() {
  await nextTick()
  detailHeading.value?.focus({ preventScroll: true })
  detailHeading.value?.scrollIntoView({ block: 'nearest' })
}
const byKey = computed(() => new Map(topology.value.nodes.map(node => [node.key, node])))
const selectionTitle = computed(() => {
  if (selectedNode.value) return selectedNode.value.label
  const link = selectedLink.value
  if (!link) return ''
  return (byKey.value.get(link.from)?.label || '未知节点') + ' → ' + (byKey.value.get(link.to)?.label || '未知节点')
})
const lastUpdated = computed(() => routeQuery.dataUpdatedAt.value
  ? new Date(routeQuery.dataUpdatedAt.value).toLocaleTimeString() : '尚未采集')
const routeError = computed(() => routeQuery.error.value ? formatApiError(routeQuery.error.value) : '')
const locationReady = computed(() => !!routeQuery.data.value && !!attrsQuery.data.value)
const peerStatus = computed(() => {
  if (peerQuery.isPending.value) return '正在读取 Peer Center 拓扑'
  return peerQuery.isSuccess.value ? '已接入 Peer Center 全网直连拓扑' : '显示入口路由拓扑 · Peer Center 暂不可用'
})
const globeRef = ref<{ focusNode?: (key: string) => void } | null>(null)
async function selectNode(key: string, fromList = false) {
  selectedLinkKey.value = ''
  await router.replace({ path: route.path, query: { ...route.query, entry: hostId.value, node: key } })
  globeRef.value?.focusNode?.(key)
  if (fromList) await showDetails()
}
function selectLink(key: string) {
  selectedLinkKey.value = key
  if (selectedNodeKey.value) void router.replace({ path: route.path, query: { ...route.query, node: undefined } })
}
const editIssue = computed(() => {
  if (props.hostsLoading) return ''
  if (props.hostsError) return '组网入口读取失败，请重试。'
  if (!hostId.value) return '原组网入口已不可用，请重新选择入口。'
  if (routeQuery.isError.value || attrsQuery.isError.value) return '路由或节点属性读取失败，请刷新后重试。'
  if (!locationReady.value) return ''
  if (!selectedNode.value) return '当前入口已找不到所选节点，请重新选择。'
  return nodeEditingIssue(selectedNode.value.row, hostId.value)
})
function openEditor() {
  if (!selectedNode.value || editIssue.value || !locationReady.value) return
  void router.push({ path: route.path, query: { ...route.query, entry: hostId.value, node: selectedNodeKey.value, edit: 'location' } })
}
function closeEditor() {
  const location = { path: route.path, query: { ...route.query, edit: undefined } }
  // 从地图进入时弹出这一条编辑历史；直接访问编辑链接时只清除编辑参数。
  if (router.options.history.state.back === router.resolve(location).fullPath) router.back()
  else void router.replace(location)
}
async function onSaved() {
  closeEditor()
  await nextTick()
  globeRef.value?.focusNode?.(selectedNodeKey.value)
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
    <el-alert v-if="editIssue && (editingLocation || (requestedEntry && !hostId) || (selectedNodeKey && !selectedNode))"
      type="warning" :closable="false" :title="editIssue">
      <el-button v-if="editingLocation" link @click="closeEditor">返回地图</el-button>
    </el-alert>
    <NetworkNodeEditor v-if="editingLocation && locationReady && selectedNode && !nodeEditingIssue(selectedNode.row, hostId)"
      :key="selectedNode.key" :entry-id="hostId" :row="selectedNode.row" :attributes="selectedNode.attributes"
      location-only :unavailable="editIssue" @saved="onSaved" @close="closeEditor" />
    <el-alert v-if="hostsError" type="error" :title="hostsError" :closable="false" />
    <div v-if="hostsLoading" class="map-empty">正在读取组网入口…</div>
    <el-empty v-else-if="!hostsError && !entries.length" description="尚未设置组网入口">
      <el-button type="primary" @click="router.push('/host-store')">前往主机商店设置入口</el-button>
    </el-empty>
    <template v-else-if="entries.length">
      <div class="map-toolbar">
        <div class="entry-picker">
          <label for="map-entry-host">组网入口</label>
          <el-select id="map-entry-host" :model-value="hostId" @change="changeEntry" aria-label="组网入口" class="map-entry">
            <el-option v-for="host in entries" :key="host.id" :label="host.name" :value="host.id" />
          </el-select>
        </div>
        <div class="refresh-controls">
          <span class="last-updated">最近采集 {{ lastUpdated }}</span>
          <el-button :disabled="!hostId" :loading="routeQuery.isFetching.value || peerQuery.isFetching.value || attrsQuery.isFetching.value"
            @click="routeQuery.refetch(); peerQuery.refetch(); attrsQuery.refetch()">刷新</el-button>
        </div>
      </div>
      <div v-if="routeError" class="status-alert" role="alert">
        {{ routeError }}。{{ routeQuery.data.value ? '以下保留上一次成功采集的节点，不代表当前实时状态。' : '请检查 Agent、EasyTier Driver 及 RPC 连接。' }}
      </div>
      <div class="map-stats">
        <div><span>当前路由可见节点</span><strong>{{ routeQuery.data.value ? topology.nodes.length : '—' }}</strong></div>
        <div><span>已定位</span><strong>{{ locationReady ? withLocation.length : '—' }}</strong></div>
        <div><span>P2P 直连关系</span><strong class="cyan">{{ routeQuery.data.value ? topology.links.filter(link => link.kind === 'p2p').length : '—' }}</strong></div>
        <div><span>中继路由</span><strong class="orange">{{ routeQuery.data.value ? topology.links.filter(link => link.kind === 'relay').length : '—' }}</strong></div>
      </div>
      <div class="map-layout">
        <div class="globe-area">
          <div class="map-controls" role="group" aria-label="地图显示设置">
            <el-select v-model="basemap" class="basemap-select" aria-label="地图底图">
              <el-option value="auto" label="跟随页面主题" />
              <el-option value="standard" label="标准地图" />
              <el-option value="satellite" label="卫星影像" />
              <el-option value="dark" label="深色地图" />
            </el-select>
            <el-select v-model="filter" aria-label="连接筛选" class="link-filter">
              <el-option value="all" label="全部连接" />
              <el-option value="p2p" label="仅直连" />
              <el-option value="relay" label="仅中继" />
              <el-option value="unknown" label="未知路由" />
            </el-select>
            <el-switch v-model="showTraffic" active-text="显示流量粒子" />
          </div>
          <div class="map-viewport">
            <GlobeMap :entry-id="hostId" :data-ready="routeQuery.isSuccess.value && attrsQuery.isSuccess.value" ref="globeRef" :nodes="topology.nodes" :links="visibleLinks"
              :basemap="basemap" :show-traffic="showTraffic"
              :selected-node-key="selectedNodeKey" :selected-link-key="selectedLinkKey"
              @node-click="selectNode" @link-click="selectLink" />
          </div>
          <button v-if="selectedNode || selectedLink" type="button" class="selection-shortcut" @click="showDetails">
            <span>{{ selectionTitle }}</span>
            <strong>查看详情 ↓</strong>
          </button>
          <div class="map-legend">
            <span><i class="line p2p" /> P2P 直连</span>
            <span><i class="line relay" /> 中继路由</span>
            <span><i class="line unknown" /> 未知</span>
            <span><i class="point" /> 地图节点</span>
          </div>
          <div class="map-source-info">
            {{ peerStatus }} · 路由每 15 秒刷新
          </div>
        </div>

        <aside class="map-side">
          <section class="side-section" aria-labelledby="map-detail-title">
            <h3 id="map-detail-title" ref="detailHeading" tabindex="-1">{{ selectedNode ? '节点详情' : selectedLink ? '连接详情' : '选中详情' }}</h3>
            <div v-if="selectedNode || selectedLink" class="detail-title">{{ selectionTitle }}</div>
            <template v-if="selectedNode">
              <el-button type="primary" size="small" :disabled="!!editIssue || !locationReady" @click="openEditor">编辑位置</el-button>
              <p v-if="editIssue" class="side-caption">{{ editIssue }}</p>
              <dl class="node-properties">
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
            <div class="side-heading"><h3>待定位节点</h3><span>{{ locationReady ? withoutLocation.length : '—' }}</span></div>
            <p v-if="attrsQuery.isError.value && !attrsQuery.data.value" class="side-caption" role="alert">
              读取节点位置失败，可点击刷新重试。
            </p>
            <p v-else-if="!locationReady" class="side-caption">{{ routeError ? '暂无节点位置数据。' : '正在读取节点位置…' }}</p>
            <div v-else-if="withoutLocation.length" class="missing-nodes">
              <button v-for="node in withoutLocation" :key="node.key" type="button"
                class="missing-node" :aria-pressed="node.key === selectedNodeKey"
                @click="selectNode(node.key, true)">
                <span>{{ node.label }}</span><small>{{ node.ip }}</small>
              </button>
            </div>
            <p v-else class="side-caption">{{ topology.nodes.length ? '当前路由中的节点均已设置位置。' : '当前路由中没有节点。' }}</p>
          </section>
          <p class="side-footnote">节点位置来自人工确认的 WGS84 经纬度，路由可见仅表示最近采集到了该节点。</p>
        </aside>
      </div>
    </template>
  </section>
</template>

<style scoped>
.map-page { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0;
  container-type: inline-size; overflow: auto; padding: 16px 20px 20px; gap: 12px; color: var(--ent-text); }
.map-toolbar, .entry-picker, .refresh-controls { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.map-toolbar { justify-content: space-between; }
.entry-picker label, .last-updated { font-size: 12px; color: var(--ent-muted); }
.map-entry { width: 176px; }
.last-updated { font-variant-numeric: tabular-nums; }
.map-stats { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: 8px; }
.map-stats > div { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between;
  gap: 4px 8px; padding: 8px 12px; border: 1px solid var(--ent-line); border-radius: 10px; background: var(--ent-panel); }
.map-stats span { font-size: 12px; color: var(--ent-muted); }
.map-stats strong { font-size: 19px; font-variant-numeric: tabular-nums; }
.map-stats strong.cyan { color: var(--ent-cyan); } .map-stats strong.orange { color: var(--ent-amber); }
.status-alert { border-radius: 8px; background: #5c401c37; color: var(--ent-text);
  font-size: 12px; padding: 8px 12px; }
.map-empty { margin: 30px; color: var(--ent-muted); }
.map-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(228px, 26%); min-height: 440px; flex: 1; gap: 12px; }
.globe-area { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden;
  border: 1px solid var(--ent-line); border-radius: 14px; background: var(--ent-panel); }
.map-viewport { position: relative; flex: 1; min-height: 280px; }
.globe-loading, .globe-load-error {
  position: absolute; inset: 0; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 13px;
  color: var(--ent-muted); font-size: 12px;
  background: radial-gradient(circle at 48% 46%, #18364d, #071423 76%);
}
:root[data-theme='light'] .globe-loading,
:root[data-theme='light'] .globe-load-error {
  background: radial-gradient(circle at 48% 46%, #f7fbff, #d7e6f0 76%);
}
.globe-loading-spinner {
  width: 26px; height: 26px; border: 2px solid var(--ent-line);
  border-top-color: var(--ent-cyan); border-radius: 50%;
  animation: globe-loading-spin .8s linear infinite;
}
@keyframes globe-loading-spin { to { transform: rotate(360deg); } }
.map-legend { display: flex; flex-wrap: wrap; gap: 6px 14px; padding: 9px 12px 4px;
  font-size: 12px; color: var(--ent-text); border-top: 1px solid var(--ent-line); }
.map-legend span { display: inline-flex; align-items: center; gap: 7px; }
.line { display: inline-block; width: 24px; border-top: 2px solid; }
.line.p2p { border-color: #38d8ff; border-top-style: solid; border-top-width: 3px; box-shadow: 0 0 8px #3ae6ff; }
.line.relay { border-color: #ffae55; border-top-style: dashed; border-top-width: 3px; }
.line.unknown { border-color: #b1bfd0; border-top-style: dotted; border-top-width: 2px; }
:root[data-theme='light'] .line.p2p { border-color: #087f9a; box-shadow: 0 0 6px #087f9a55; }
:root[data-theme='light'] .line.relay { border-color: #a85b0a; }
:root[data-theme='light'] .line.unknown { border-color: #74899a; }
.point { width: 9px; height: 9px; border: 1px solid white; border-radius: 50%; background: #36dcec; }
.map-controls { display: flex; flex-wrap: wrap; gap: 6px 10px; align-items: center;
  padding: 8px 12px; border-bottom: 1px solid var(--ent-line); }
.basemap-select { width: 142px; }
.link-filter { width: 114px; }
.map-source-info { padding: 3px 12px 9px; color: var(--ent-muted); font-size: 11px; }
.selection-shortcut { display: none; }
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
.missing-node:hover, .missing-node[aria-pressed="true"] { border-color: var(--ent-cyan); }
.missing-node span { overflow-wrap: anywhere; }
.missing-node small { color: var(--ent-muted); white-space: nowrap; }
.node-properties { margin-top: 12px; }
.side-footnote { margin: 0 4px; }
@container (max-width: 740px) {
  .map-layout { display: flex; flex-direction: column; min-height: auto; flex: none; }
  .map-viewport { flex: none; height: clamp(280px, 47dvh, 450px); }
  .map-side { overflow: visible; }
  .selection-shortcut { display: flex; align-items: center; justify-content: space-between; gap: 8px;
    border: 0; border-top: 1px solid var(--ent-line); background: var(--ent-panel-2); color: var(--ent-text);
    padding: 10px 12px; text-align: left; }
  .selection-shortcut span { overflow-wrap: anywhere; }
  .selection-shortcut strong { flex-shrink: 0; color: var(--ent-cyan); }
}
@container (max-width: 480px) {
  .map-stats { grid-template-columns: repeat(2,minmax(0,1fr)); }
  .entry-picker, .refresh-controls { width: 100%; }
  .map-entry { flex: 1; min-width: 0; }
  .refresh-controls { justify-content: space-between; }
  .map-controls { padding: 8px; }
  .basemap-select, .link-filter { flex: 1 1 112px; }
}
@media (max-width: 760px) {
  .map-page { padding: 10px; }
}
</style>
