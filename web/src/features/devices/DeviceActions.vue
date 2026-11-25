<script setup lang="ts">
import { ArrowDown, Delete, Edit, Grid, Iphone, Plus, Search, Setting } from '@element-plus/icons-vue'
import type { Device, OpenWrtDiscoveryKind } from './types'

defineProps<{ device: Device; openWrt: boolean }>()
const emit = defineEmits<{
  services: [device: Device]
  edit: [device: Device]
  add: [device: Device]
  settings: [device: Device]
  discover: [device: Device, kind: OpenWrtDiscoveryKind]
  remove: [device: Device]
}>()
</script>

<template>
  <div class="device-actions" role="group" :aria-label="device.name + '的操作'">
    <el-button text type="primary" :icon="Grid" @click="emit('services', device)">服务列表</el-button>
    <el-button text :icon="Edit" @click="emit('edit', device)">编辑</el-button>
    <el-button v-if="device.is_gateway && device.owned_gateway_group_id" text :icon="Plus"
      @click="emit('add', device)">新增下挂设备</el-button>
    <el-dropdown v-if="openWrt || device.is_node_device" trigger="click" placement="bottom-end">
      <el-button text :aria-label="device.name + '的更多操作'">更多<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item v-if="openWrt" :icon="Setting" @click="emit('settings', device)">OpenWrt RPCD 配置</el-dropdown-item>
          <template v-if="openWrt && device.is_gateway && device.openwrt_configured">
            <el-dropdown-item :icon="Search" @click="emit('discover', device, 'devices')">服务发现</el-dropdown-item>
            <el-dropdown-item :icon="Iphone" @click="emit('discover', device, 'adb')">ADB 发现</el-dropdown-item>
          </template>
          <el-dropdown-item v-if="device.is_node_device" :divided="openWrt" :icon="Delete" class="remove-device"
            @click="emit('remove', device)">删除节点基础设备</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<style scoped>
.device-actions { display:flex; flex-wrap:wrap; align-items:center; justify-content:flex-end; gap:2px; }
.device-actions :deep(.el-button) { margin:0; padding-inline:8px; }
.remove-device { color:var(--el-color-danger); }
@media (pointer:coarse) {
  .device-actions :deep(.el-button) { min-height:44px; }
}
</style>
