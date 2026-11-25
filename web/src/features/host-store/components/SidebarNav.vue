<script setup lang="ts">
import { ArrowLeft, HomeFilled, Connection, Monitor, Location, Compass } from '@element-plus/icons-vue'

defineProps<{ collapsed: boolean }>()
const pages = [
  { path: '/navigation', label: '服务导航', icon: Compass },
  { path: '/host-store', label: '主机商店', icon: HomeFilled },
  { path: '/network-nodes', label: '网络节点', icon: Connection },
  { path: '/network-map', label: '网络地图', icon: Location },
  { path: '/devices', label: '设备列表', icon: Monitor }
]
defineEmits<{ toggle: [] }>()
</script>

<template>
  <aside class="sidebar" :class="{ 'is-collapsed': collapsed }" aria-label="主导航">
    <button class="collapse-toggle" type="button" :aria-expanded="!collapsed" aria-controls="workbench-pages"
      :title="collapsed ? '展开左侧菜单' : '收起左侧菜单'"
      :aria-label="collapsed ? '展开左侧菜单' : '收起左侧菜单'"
      @click="$emit('toggle')">
      <el-icon><ArrowLeft /></el-icon>
    </button>

    <div class="brand" title="EasyNetTopo · 网络拓扑与主机管理">
      <svg class="brand-mark" viewBox="0 0 128 128" role="img" aria-label="EasyNetTopo 网络拓扑商标">
        <defs>
          <linearGradient id="brandLine" x1="0" y1="0" x2="1" y2="1">
            <stop stop-color="#40e9e0" />
            <stop offset=".52" stop-color="#56b9ff" />
            <stop offset="1" stop-color="#8879ff" />
          </linearGradient>
          <linearGradient id="brandPlate" x1="0" y1="0" x2="1" y2="1">
            <stop stop-color="#152e46" />
            <stop offset="1" stop-color="#0a1429" />
          </linearGradient>
        </defs>
        <rect x="5" y="5" width="118" height="118" rx="33" fill="url(#brandPlate)" stroke="#355474" stroke-width="2" />
        <path d="M24 37 64 15 104 37v54l-40 22-40-22Z" fill="none" stroke="#24516e" stroke-width="2" opacity=".7" />
        <g fill="none" stroke="url(#brandLine)" stroke-linecap="round" stroke-linejoin="round">
          <path d="M36 91V37l56 54V37" stroke-width="10" />
          <path d="M36 37h24M68 91h24" stroke-width="4" opacity=".82" />
        </g>
        <circle cx="36" cy="37" r="9" fill="#42e1e1" stroke="#0e2842" stroke-width="4" />
        <circle cx="92" cy="37" r="9" fill="#8885ff" stroke="#0e2842" stroke-width="4" />
        <circle cx="36" cy="91" r="9" fill="#48cff5" stroke="#0e2842" stroke-width="4" />
        <circle cx="92" cy="91" r="9" fill="#7a99ff" stroke="#0e2842" stroke-width="4" />
        <circle cx="64" cy="64" r="5" fill="#e4faff" />
      </svg>
      <div class="brand-names">
        <div class="brand-name">EasyNetTopo</div>
        <div class="brand-tagline">易网拓扑</div>
      </div>
    </div>

    <nav id="workbench-pages" class="sidebar-menu" aria-label="工作台页面">
      <router-link v-for="page in pages" :key="page.path" :to="page.path" class="nav-item"
        :aria-label="page.label" :title="page.label">
        <el-icon><component :is="page.icon" /></el-icon>
        <span>{{ page.label }}</span>
      </router-link>
    </nav>

  </aside>
</template>

<style scoped>
.sidebar {
  position: relative;
  z-index: 3;
  display: flex;
  min-width: 0;
  flex-direction: column;
  padding: 24px 15px;
  border-right: 1px solid var(--ent-line-soft);
  background: linear-gradient(180deg, #081625 0%, #070f1b 90%);
  box-shadow: 6px 0 25px rgb(0 0 0 / 8%);
}
.sidebar::after {
  position: absolute;
  z-index: 0;
  pointer-events: none;
  inset: 0;
  background: radial-gradient(ellipse at 0% 10%, #2888cc13, transparent 46%);
  content: '';
}
.sidebar > * { position: relative; z-index: 1; }
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 5px 21px;
  border-bottom: 1px solid #21344a;
}
.brand-mark { width: 46px; height: 46px; flex: none; border-radius: 15px; filter: drop-shadow(0 0 14px #2a90e02b); }
.brand-names { overflow: hidden; white-space: nowrap; }
.brand-name {
  background: linear-gradient(100deg, #f5faff 30%, #88dff4);
  background-clip: text;
  color: transparent;
  font-size: 19px;
  font-weight: 780;
  letter-spacing: -.7px;
  line-height: 1.25;
}
.brand-tagline { margin-top: 5px; color: #718daa; font-size: 9px; font-weight: 650; letter-spacing: 1.65px; }
.collapse-toggle {
  position: absolute;
  z-index: 8;
  top: 37px;
  right: -14px;
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: 1px solid #29546e;
  border-radius: 11px;
  background: #0d2033;
  box-shadow: 0 5px 14px #0004;
  color: #a3eafa;
}
.collapse-toggle:hover { background: #143450; }
.collapse-toggle .el-icon { transition: transform .25s; }
.is-collapsed .collapse-toggle .el-icon { transform: rotate(180deg); }
.sidebar-menu { display: flex; min-height: 0; overflow-y: auto; flex-direction: column; gap: 8px; margin-top: 24px; }
.nav-item {
  display: flex;
  width: 100%;
  min-height: 48px;
  align-items: center;
  gap: 12px;
  padding: 0 13px;
  border: 1px solid #1e5269;
  border-radius: 12px;
  background: linear-gradient(125deg, #12354890, #11233d 85%);
  box-shadow: inset 3px 0 0 #4bd9ec, 0 0 24px #33b6e70b;
  color: #d7f7ff;
  text-align: left;
  text-decoration: none;
}
.nav-item:not([aria-current="page"]) { background: transparent; box-shadow: none; border-color: transparent; color: #8bacca; }
.nav-item:not([aria-current="page"]):hover { background: var(--ent-elev); }
.nav-item .el-icon { color: #62e1f1; font-size: 21px; }
.nav-item span { overflow: hidden; font-size: 13px; font-weight: 680; white-space: nowrap; }
.is-collapsed .brand { justify-content: center; padding-inline: 0; }
.is-collapsed .brand-names, .is-collapsed .nav-item span { display: none; }
.is-collapsed .nav-item { justify-content: center; padding: 0; }
@media (min-width: 761px) and (max-width: 1000px) {
  .sidebar { padding: 18px 10px; }
  .collapse-toggle { display: none; }
  .brand { justify-content: center; padding-inline: 0; }
  .brand-names { display: none; }
  .nav-item, .is-collapsed .nav-item { flex-direction: column; justify-content: center; gap: 4px; padding: 6px 0; }
  .nav-item span, .is-collapsed .nav-item span { display: inline; font-size: 11px; }
}
@media (max-width: 760px) {
  .sidebar { grid-row: 2; padding: 4px 4px calc(4px + env(safe-area-inset-bottom)); border: 0; border-top: 1px solid var(--ent-line); }
  .brand, .collapse-toggle { display: none; }
  .sidebar-menu { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 2px; margin: 0; }
  .nav-item, .is-collapsed .nav-item { min-height: 50px; flex-direction: column; justify-content: center; gap: 3px; padding: 3px 0; border-radius: 8px; }
  .nav-item .el-icon { font-size: 18px; }
  .nav-item span, .is-collapsed .nav-item span { display: inline; font-size: 11px; }
}
</style>
