<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { Box, Connection, Link, Plus, VideoPlay } from '@element-plus/icons-vue'
import { formatApiError } from '../../lib/api-client'
import { createDeviceService, getDeviceServices } from './api'
import type { Device, DeviceServiceInput } from './types'

const props = defineProps<{ device: Device }>()
const open = defineModel<boolean>({ required: true })
const client = useQueryClient()
const queryKey = computed(() => ['device-services', props.device.id])
const { data: services, isPending: loading, isError: loadFailed, error: loadError, refetch } = useQuery({
  queryKey, enabled: open, retry: 0,
  queryFn: ({ signal }) => getDeviceServices(props.device.id, signal)
})
const icons = [
  { value: 'web', label: '网页', component: Link },
  { value: 'storage', label: '存储', component: Box },
  { value: 'terminal', label: '远程连接', component: Connection },
  { value: 'media', label: '影音', component: VideoPlay }
]
function serviceIcon(value: string) {
  return icons.find(icon => icon.value === value)?.component || Connection
}

const adding = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const form = reactive<Omit<DeviceServiceInput, 'port'> & { port: number | undefined }>({
  name: '', protocols: ['tcp'], port: undefined, access_url: '', icon: '', note: ''
})

function startForm() {
  Object.assign(form, { name: '', protocols: ['tcp'], port: undefined, access_url: '', icon: '', note: '' })
  errorMessage.value = ''
  adding.value = true
}

watch(open, value => {
  if (value) adding.value = false
})

async function saveService() {
  if (saving.value) return
  const { port, name, protocols } = form
  if (!name.trim() || !protocols.length || port == null || !Number.isInteger(port) || port < 1 || port > 65535) {
    errorMessage.value = '请填写服务名称，至少选择一种协议，并填写 1–65535 的端口。'
    return
  }
  saving.value = true
  errorMessage.value = ''
  try {
    await createDeviceService(props.device.id, { ...form, port })
    adding.value = false
    await client.invalidateQueries({ queryKey: queryKey.value })
  } catch (error) {
    errorMessage.value = formatApiError(error)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="open" :title="'服务列表 · ' + device.name" width="min(660px, 94vw)" destroy-on-close
    :close-on-click-modal="!saving" :close-on-press-escape="!saving" :show-close="!saving"
  >
    <el-form v-if="adding" class="service-form" label-position="top" @submit.prevent="saveService">
      <el-alert v-if="errorMessage" type="error" :title="errorMessage" show-icon :closable="false" />
      <el-form-item label="服务名称" required>
        <el-input v-model="form.name" maxlength="128" placeholder="例如：NAS 管理后台" :disabled="saving" />
      </el-form-item>
      <div class="service-form-grid">
        <el-form-item label="协议" required>
          <el-checkbox-group v-model="form.protocols" :disabled="saving">
            <el-checkbox value="tcp">TCP</el-checkbox>
            <el-checkbox value="udp">UDP</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="端口" required>
          <el-input-number v-model="form.port" :min="1" :max="65535" :precision="0" :controls="false" :disabled="saving" />
        </el-form-item>
      </div>
      <el-form-item label="访问 URL（可选）">
        <el-input v-model="form.access_url" maxlength="2048" placeholder="https://nas.example.com/" :disabled="saving" />
      </el-form-item>
      <el-form-item label="图标（可选）">
        <el-select v-model="form.icon" clearable placeholder="默认服务图标" :disabled="saving">
          <el-option v-for="icon in icons" :key="icon.value" :label="icon.label" :value="icon.value">
            <el-icon><component :is="icon.component" /></el-icon> {{ icon.label }}
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="备注（可选）">
        <el-input v-model="form.note" type="textarea" :rows="2" maxlength="2000" :disabled="saving" />
      </el-form-item>
      <div class="service-form-actions">
        <el-button :disabled="saving" @click="adding = false">返回列表</el-button>
        <el-button type="primary" native-type="submit" :loading="saving">保存服务</el-button>
      </div>
    </el-form>
    <template v-else>
      <div class="service-list-actions">
        <el-button type="primary" @click="startForm"><el-icon><Plus /></el-icon>新增服务</el-button>
      </div>
      <el-skeleton v-if="loading" :rows="3" animated />
      <el-alert v-else-if="loadFailed" type="error" :title="formatApiError(loadError)" :closable="false" show-icon>
        <el-button text @click="refetch()">重试</el-button>
      </el-alert>
      <ul v-else-if="services?.length" class="service-list">
        <li v-for="service in services" :key="service.id">
          <el-icon class="service-icon"><component :is="serviceIcon(service.icon)" /></el-icon>
          <div class="service-details">
            <strong>{{ service.name }}</strong>
            <el-tag size="small" effect="plain">{{ service.protocols.join(' / ').toUpperCase() }} · {{ service.port }}</el-tag>
            <a v-if="service.access_url" :href="service.access_url" target="_blank" rel="noopener noreferrer">{{ service.access_url }}</a>
            <p v-if="service.note">{{ service.note }}</p>
          </div>
        </li>
      </ul>
      <el-empty v-else description="该设备还没有登记服务" :image-size="84" />
    </template>
  </el-dialog>
</template>

<style scoped>
.service-list-actions, .service-form-actions { display:flex; justify-content:flex-end; margin-bottom:12px; }
.service-form-actions { margin-bottom:0; }
.service-form-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; }
.service-form :deep(.el-input-number), .service-form :deep(.el-select) { width:100%; }
.service-form > .el-alert { margin-bottom:14px; }
.service-list { display:grid; gap:10px; padding:0; margin:0; list-style:none; }
.service-list li { display:flex; gap:12px; padding:12px; border:1px solid var(--ent-line); border-radius:10px; }
.service-icon { flex:none; margin-top:2px; color:var(--ent-data-highlight); font-size:19px; }
.service-details { display:flex; min-width:0; align-items:flex-start; flex-wrap:wrap; gap:6px 10px; overflow-wrap:anywhere; }
.service-details a, .service-details p { flex-basis:100%; }
.service-details a { color:var(--ent-data-highlight); }
.service-details p { margin:0; white-space:pre-wrap; }
@media (max-width:440px) { .service-form-grid { grid-template-columns:1fr; gap:0; } }
</style>
