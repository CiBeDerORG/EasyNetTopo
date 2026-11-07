<script setup lang="ts">
import type { DockerInventory, DockerPort, Host } from '../types'

defineProps<{
  modelValue: boolean
  host: Host | null
  inventory: DockerInventory | undefined
  loading: boolean
  error: string
}>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()
function formatPort(port: DockerPort) {
  const host = !port.host_ip || port.host_ip === '0.0.0.0' || port.host_ip === '::'
    ? '*'
    : port.host_ip
  return host + ':' + port.host_port + ' → ' + port.container_port + '/' + port.protocol
}
</script>

<template>
  <el-drawer :model-value="modelValue" :title="(host?.name || '主机') + ' · Docker 资源'"
    direction="rtl" size="min(500px, 100vw)" append-to-body
    @update:model-value="$emit('update:modelValue', $event)">
    <template v-if="host">
      <div class="drawer-summary">
        <span>{{ host.endpoint }}</span>
        <el-tag v-if="inventory" size="small" :type="inventory.complete ? 'success' : 'warning'">
          {{ inventory.complete ? '查询完整' : '查询不完整' }}
        </el-tag>
      </div>
      <el-alert v-if="error" :title="error" type="warning" :closable="false" show-icon />
      <el-skeleton v-if="loading && !inventory" :rows="5" animated />
      <template v-else-if="inventory">
        <p class="observed-at">采集时间：{{ new Date(inventory.observed_at).toLocaleString() }}</p>
        <el-alert v-if="!inventory.complete" title="资源清单不完整。连接或权限异常不能作为删除资源的依据。" type="warning" :closable="false" show-icon />
        <div class="resource-list">
          <article v-for="resource in inventory.resources || []" :key="resource.resource_id" class="resource-row">
            <div class="resource-icon">◇</div>
            <div class="resource-info">
              <strong>{{ resource.name }}</strong>
              <span>{{ resource.kind }} · {{ resource.state }}</span>
              <small>{{ resource.managed ? 'EasyNetTopo 受管资源' : '外部资源 · 只读发现' }}</small>
              <div v-if="resource.ports?.length" class="resource-ports">
                <span>对外端口</span>
                <el-tag v-for="(port, index) in resource.ports" :key="index" size="small" effect="plain" type="success">
                  {{ formatPort(port) }}
                </el-tag>
              </div>
              <div v-if="resource.containers?.length" class="container-list">
                <div v-for="container in resource.containers" :key="container.container_id" class="container-item">
                  <span>{{ container.name }} · {{ container.state }}</span>
                  <small v-if="container.ports?.length">{{ container.ports.map(formatPort).join('；') }}</small>
                </div>
              </div>
            </div>
            <el-tag size="small" :type="resource.managed ? 'success' : 'info'">{{ resource.managed ? '受管' : '外部' }}</el-tag>
          </article>
        </div>
        <el-empty v-if="!inventory.resources?.length" description="没有发现 Docker 资源" :image-size="80" />
      </template>
      <el-empty v-else-if="!loading && !error" description="尚未获取 Docker 资源" :image-size="80" />
    </template>
  </el-drawer>
</template>

<style scoped>
.drawer-summary { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 12px; color: var(--ent-muted); font-size: 11px; overflow-wrap: anywhere; }
.observed-at { color: var(--ent-dim); font-size: 10px; }
.resource-list { display: grid; gap: 9px; margin-top: 14px; }
.resource-row { display: flex; align-items: center; gap: 11px; padding: 12px; border: 1px solid #254059; border-radius: 11px; background: #112338; }
.resource-icon { display: grid; width: 34px; height: 34px; flex: none; place-items: center; border-radius: 9px; background: #203d58; color: var(--ent-cyan); font-size: 21px; }
.resource-info { display: grid; min-width: 0; flex: 1; gap: 4px; }
.resource-info strong { overflow-wrap: anywhere; color: var(--ent-text); font-size: 12px; }
.resource-info span { color: #87a5bf; font-size: 11px; }
.resource-info small { color: var(--ent-dim); font-size: 10px; }
.resource-ports, .container-list { display: grid; gap: 5px; margin-top: 4px; }
.resource-ports { display: flex; align-items: center; flex-wrap: wrap; }
.resource-ports > span, .container-item > span { color: #8eb5cc; font-size: 10px; }
.resource-ports :deep(.el-tag) { height: auto; white-space: normal; overflow-wrap: anywhere; font-size: 10px; }
.container-list { padding-top: 5px; border-top: 1px solid #254059; }
.container-item { display: grid; gap: 3px; }
.container-item small { overflow-wrap: anywhere; color: var(--ent-dim); font-size: 10px; }
</style>
