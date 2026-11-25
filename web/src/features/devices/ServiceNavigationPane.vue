<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { Compass, Connection, House, Link, Search, Setting, TopRight } from '@element-plus/icons-vue'
import { formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { getDevices, getNavigationServices, navigationServicesKey } from './api'
import DeviceServicesDialog from './DeviceServicesDialog.vue'
import { activePublicURL, serviceIcon, serviceURL } from './service-access'
import { useDeviceNetwork } from './useDeviceNetwork'

const props = defineProps<{ hosts: Host[] }>()
const { serviceDeviceNetworkIP, serviceEntryPublicIPs } = useDeviceNetwork(() => props.hosts)
const servicesQuery = useQuery({ queryKey: navigationServicesKey, queryFn: ({ signal }) => getNavigationServices(signal), retry: 0 })
const devicesQuery = useQuery({ queryKey: ['devices', '__all__'], queryFn: () => getDevices(), retry: 0 })
const devices = computed(() => devicesQuery.data.value ?? [])
const filter = ref('')
watch(devicesQuery.data, items => {
  if (items && filter.value && !items.some(device => device.id === filter.value)) filter.value = ''
})
const search = ref('')
const managing = ref(false)
const servicesOpen = ref(false)
const serviceDeviceId = ref('')
const serviceDevice = computed(() => devices.value.find(device => device.id === serviceDeviceId.value))
const cards = computed(() => {
  const byID = new Map(devices.value.map(device => [device.id, device]))
  return (servicesQuery.data.value ?? []).flatMap(service => {
    const device = byID.get(service.device_id)
    if (!device) return []
    const links = [
      { label: '内网访问', url: serviceURL(device.ip_address || '', service.port, service.access_url), icon: House, unavailable: '暂无内网地址' },
      { label: '组网访问', url: serviceURL(serviceDeviceNetworkIP(device), service.port, service.access_url), icon: Connection, unavailable: '暂无组网地址' },
      { label: '公网访问', url: activePublicURL(service, serviceEntryPublicIPs.value), icon: Link,
        unavailable: service.forward?.enabled ? '暂无公网地址' : '未开启转发' }
    ]
    return [{ service, device, links }]
  })
})
const filters = computed(() => {
  const counts = new Map<string, number>()
  for (const { device } of cards.value) counts.set(device.id, (counts.get(device.id) ?? 0) + 1)
  const groups = devices.value
    .filter(device => device.id === filter.value || counts.has(device.id))
    .map(device => ({ id: device.id, name: device.name, count: counts.get(device.id) ?? 0 }))
  return [{ id: '', name: '全部设备', count: cards.value.length }, ...groups]
})
const emptyDevice = computed(() => filters.value.find(item => item.id && item.id === filter.value && item.count === 0))
const emptyDescription = computed(() => {
  if (emptyDevice.value) return emptyDevice.value.name + ' · 0 个导航服务'
  return cards.value.length ? '没有符合条件的服务' : '在设备的服务列表中开启导航后，服务会出现在这里。'
})
const visibleCards = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return cards.value.filter(({ service, device }) =>
    (!filter.value || filter.value === device.id) &&
    [service.name, service.note, device.name].some(value => value.toLocaleLowerCase().includes(query)))
})
const loading = computed(() => servicesQuery.isPending.value || devicesQuery.isPending.value)
const error = computed(() => servicesQuery.error.value ?? devicesQuery.error.value)

function openServices(id: string) {
  serviceDeviceId.value = id
  managing.value = false
  servicesOpen.value = true
}

function retry() {
  void servicesQuery.refetch()
  void devicesQuery.refetch()
}
</script>

<template>
  <div class="navigation-workspace">
    <section class="navigation-panel" aria-label="服务导航">
      <header class="navigation-heading">
        <div>
          <h2>服务导航</h2>
          <p>快速访问已开启导航的设备服务。</p>
        </div>
        <el-button v-if="filter && !emptyDevice" type="primary" plain :icon="Setting" @click="openServices(filter)">管理该设备服务</el-button>
        <el-popover v-else-if="!filter" v-model:visible="managing" placement="bottom-end" trigger="click" :width="252">
          <template #reference>
            <el-button type="primary" plain :icon="Setting" :disabled="!devices.length">管理导航</el-button>
          </template>
          <el-select filterable placeholder="选择要管理的设备" aria-label="选择要管理的设备"
            :loading="devicesQuery.isPending.value" @change="openServices">
            <el-option v-for="device in devices" :key="device.id" :label="device.name" :value="device.id" />
          </el-select>
        </el-popover>
      </header>
      <el-alert v-if="error" class="navigation-feedback" type="error" :title="formatApiError(error)" :closable="false" show-icon>
        <el-button text @click="retry">重试</el-button>
      </el-alert>
      <el-skeleton v-else-if="loading" class="navigation-feedback" :rows="4" animated />
      <template v-else>
        <div class="navigation-toolbar">
          <el-input v-model="search" class="navigation-search" :prefix-icon="Search" clearable placeholder="搜索服务或设备" aria-label="搜索服务或设备" />
          <el-select v-model="filter" filterable :empty-values="[null, undefined]" class="navigation-filter" aria-label="按设备筛选"
            placeholder="全部设备">
            <el-option v-for="item in filters" :key="item.id" :value="item.id" :label="item.name + '（' + item.count + '）'" />
          </el-select>
        </div>
        <div v-if="visibleCards.length" class="navigation-grid">
          <article v-for="{ service, device, links } in visibleCards" :key="service.id" class="navigation-card">
            <div class="card-heading">
              <el-icon class="service-icon" :data-icon="service.icon"><component :is="serviceIcon(service.icon)" /></el-icon>
              <div>
                <h3>{{ service.name }}</h3>
                <p>{{ device.name }}</p>
              </div>
            </div>
            <p v-if="service.note" class="service-note">{{ service.note }}</p>
            <div class="service-addresses">
              <component :is="entry.url ? 'a' : 'div'" v-for="entry in links" :key="entry.label"
                class="service-address" :href="entry.url || undefined"
                :target="entry.url ? '_blank' : undefined" :rel="entry.url ? 'noopener noreferrer' : undefined">
                <el-icon><component :is="entry.icon" /></el-icon>
                <span>{{ entry.label }}</span>
                <span class="address-value" :class="{ 'has-link': entry.url }">{{ entry.url ? entry.url.split('://')[1] : entry.unavailable }}</span>
                <el-icon v-if="entry.url"><TopRight /></el-icon>
              </component>
            </div>
          </article>
        </div>
        <el-empty v-else :description="emptyDescription" :image-size="65">
          <template #image><el-icon class="empty-icon"><Compass /></el-icon></template>
          <el-button v-if="emptyDevice" type="primary" :icon="Setting" @click="openServices(filter)">管理该设备服务</el-button>
          <el-button v-else-if="cards.length" @click="search = ''">清除搜索</el-button>
          <el-button v-else type="primary" @click="$router.push('/devices')">前往设备列表</el-button>
        </el-empty>
      </template>
    </section>
    <DeviceServicesDialog v-if="serviceDevice" :key="serviceDevice.id" v-model="servicesOpen"
      :device="serviceDevice" :hosts="hosts" :network-ip="serviceDeviceNetworkIP(serviceDevice)" :entry-public-ips="serviceEntryPublicIPs" />
  </div>
</template>

<style scoped>
.navigation-workspace { flex:1; min-height:0; overflow:auto; padding:24px; }
.navigation-panel { max-width:1500px; margin:0 auto; padding:24px; background:var(--ent-panel); border:1px solid var(--ent-line); border-radius:var(--ent-radius); }
.navigation-heading { display:flex; align-items:flex-start; flex-wrap:wrap; justify-content:space-between; gap:12px; }
.navigation-heading h2 { margin:0; font-size:24px; }
.navigation-heading p { margin:6px 0 0; color:var(--ent-muted); }
.navigation-feedback { margin-top:18px; }
.navigation-toolbar { display:flex; align-items:center; flex-wrap:wrap; gap:10px; margin:20px 0 16px; padding-bottom:14px; border-bottom:1px solid var(--ent-line-soft); }
.navigation-search { width:min(100%,340px); }
.navigation-filter { width:min(100%,240px); }
.navigation-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(min(100%,270px),1fr)); gap:14px; }
@media (min-width:1280px) { .navigation-grid { grid-template-columns:repeat(3,minmax(0,1fr)); } }
.navigation-card { display:flex; flex-direction:column; min-width:0; padding:15px; border:1px solid var(--ent-line); background:var(--ent-panel-2); border-radius:12px; }
.card-heading { display:flex; align-items:center; gap:11px; overflow-wrap:anywhere; }
.service-icon { flex:none; width:42px; height:42px; font-size:22px; color:var(--ent-cyan); background:var(--ent-elev); border:1px solid var(--ent-line); border-radius:10px; }
.service-icon[data-icon=storage] { color:var(--ent-blue); }
.service-icon[data-icon=media] { color:var(--ent-green); }
.service-icon[data-icon=terminal] { color:var(--ent-violet); }
.card-heading h3 { margin:0; font-size:16px; }
.card-heading p, .service-note { margin:4px 0 0; color:var(--ent-muted); font-size:12px; }
.service-note { margin:10px 0 0; overflow-wrap:anywhere; white-space:pre-wrap; }
.service-addresses { margin-top:auto; padding-top:12px; }
.service-addresses > :first-child { border-top:1px solid var(--ent-line-soft); }
.service-address { display:flex; align-items:center; gap:7px; min-height:34px; padding:5px 4px; color:var(--ent-muted); font-size:12px; text-decoration:none; }
a.service-address:hover { background:var(--ent-elev); border-radius:5px; }
.service-address .el-icon { flex:none; }
a.service-address .el-icon { color:var(--ent-cyan); }
a.service-address:focus-visible { outline:2px solid var(--ent-cyan); outline-offset:1px; }
.service-address > span:first-of-type { white-space:nowrap; }
.address-value { min-width:0; margin-left:auto; overflow-wrap:anywhere; text-align:right; }
.address-value.has-link { color:var(--ent-data-highlight); font:11px/1.35 ui-monospace,monospace; }
.empty-icon { color:var(--ent-cyan); font-size:55px; }
@media (max-width:760px) {
  .navigation-workspace { padding:12px; }
  .navigation-panel { padding:14px; }
  .navigation-heading h2 { font-size:20px; }
  .navigation-toolbar { margin-top:14px; }
  .navigation-search, .navigation-filter { width:100%; }
  .service-address { min-height:44px; }
}
</style>
