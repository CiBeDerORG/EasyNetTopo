<script setup lang="ts">
import { ArrowLeft, HomeFilled, Connection, Monitor, Location } from '@element-plus/icons-vue'

defineProps<{ collapsed: boolean }>()
import { useRoute, useRouter } from 'vue-router'
const route = useRoute()
const router = useRouter()
defineEmits<{ toggle: [] }>()
</script>

<template>
  <aside class="sidebar" :class="{ 'is-collapsed': collapsed }" aria-label="主导航">
    <button class="collapse-toggle" type="button"
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

    <nav class="sidebar-menu">
      <el-tooltip content="主机商店" placement="right" :disabled="!collapsed">
        <button class="nav-item" type="button" :aria-current="route.path === '/host-store' ? 'page' : undefined" @click="router.push('/host-store')">
          <el-icon><HomeFilled /></el-icon>
          <span>主机商店</span>
        </button>
      </el-tooltip>
      <el-tooltip content="网络节点" placement="right" :disabled="!collapsed">
        <button class="nav-item" type="button" :aria-current="route.path === '/network-nodes' ? 'page' : undefined" @click="router.push('/network-nodes')">
          <el-icon><Connection /></el-icon><span>网络节点</span>
        </button>
      </el-tooltip>
      <el-tooltip content="网络地图" placement="right" :disabled="!collapsed">
        <button class="nav-item" type="button" :aria-current="route.path === '/network-map' ? 'page' : undefined" @click="router.push('/network-map')">
          <el-icon><Location /></el-icon><span>网络地图</span>
        </button>
      </el-tooltip>
      <el-tooltip content="设备列表" placement="right" :disabled="!collapsed">
        <button class="nav-item" type="button" :aria-current="route.path === '/devices' ? 'page' : undefined" @click="router.push('/devices')">
          <el-icon><Monitor /></el-icon><span>设备列表</span>
        </button>
      </el-tooltip>
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
.sidebar-menu { display: flex; flex-direction: column; gap: 8px; margin-top: 24px; }
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
}
.nav-item:not([aria-current="page"]) { background: transparent; box-shadow: none; border-color: transparent; color: #8bacca; }
.nav-item .el-icon { color: #62e1f1; font-size: 21px; }
.nav-item span { overflow: hidden; font-size: 13px; font-weight: 680; white-space: nowrap; }
.is-collapsed .brand { justify-content: center; padding-inline: 0; }
.is-collapsed .brand-names, .is-collapsed .nav-item span { display: none; }
.is-collapsed .nav-item { justify-content: center; padding: 0; }
@media (max-width: 830px) {
  .sidebar { padding: 18px 10px; }
  .collapse-toggle { display: none; }
  .brand { justify-content: center; padding-inline: 0; }
  .brand-names, .nav-item span { display: none; }
  .nav-item { justify-content: center; padding: 0; }
}
</style>
