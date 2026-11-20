<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Link, Plus } from '@element-plus/icons-vue'
import { apiRequest, formatApiError } from '../../lib/api-client'
import { instances, list, publicIPv4Candidate, record } from '../network-nodes/data'
import type { Host } from '../host-store/types'
import { checkDeviceServiceForward, createDeviceService, deleteDeviceService, getDeviceServices, navigationServicesKey, setDeviceServiceForward, setDeviceServiceNavigation, updateDeviceService } from './api'
import type { Device, DeviceService, DeviceServiceInput } from './types'
import DeviceServiceImport from './DeviceServiceImport.vue'
import { activePublicURL, publicAccessURL, serviceIcon, serviceIcons, serviceURL } from './service-access'

const props = defineProps<{
  device: Device
  hosts: Host[]
  networkIp: string
  entryPublicIps: Record<string, Record<string, string>>
}>()
const open = defineModel<boolean>({ required: true })
const client = useQueryClient()
const queryKey = computed(() => ['device-services', props.device.id])
const { data: services, isPending: loading, isError: loadFailed, error: loadError, refetch } = useQuery({
  queryKey, enabled: open, retry: 0,
  queryFn: ({ signal }) => getDeviceServices(props.device.id, signal)
})
const adding = ref(false)
const importing = ref(false)
const savingImport = ref(false)
const editing = ref<DeviceService | null>(null)
const saving = ref(false)
const checking = ref(false)
const switching = ref('')
const deleting = ref('')
const savingNavigation = ref('')
const busy = computed(() => savingImport.value || saving.value || checking.value || !!switching.value || !!deleting.value || !!savingNavigation.value)
const errorMessage = ref('')
const checkMessage = ref('')
const form = reactive<Omit<DeviceServiceInput, 'port'> & { port: number | undefined }>({
  name: '', protocols: ['tcp'], port: undefined, access_url: '', icon: '', note: ''
})
const enableForward = ref(false)
const forward = reactive({ host_id: '', instance_id: '', bindIP: '0.0.0.0', port: undefined as number | undefined, confirmed: false })
const entryHosts = computed(() => props.hosts.filter(host => host.network_entry && host.agent_id))
const { data: routes, isFetching: loadingInstances, isError: instancesFailed, refetch: reloadInstances } = useQuery({
  queryKey: computed(() => ['service-forward-instances', forward.host_id]),
  enabled: computed(() => open.value && adding.value && enableForward.value && !!forward.host_id),
  retry: 0,
  queryFn: async ({ signal }) => {
    const value = await apiRequest<{ data: unknown }>('/api/v1/hosts/' + encodeURIComponent(forward.host_id) + '/network/route', { signal })
    return instances(value.data).filter(item => /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(item.id))
  }
})
const draftNetworkURL = computed(() => serviceURL(props.networkIp, form.port, form.access_url))
const draftPublicURL = computed(() => {
  if (editing.value?.forward) return activePublicURL(editing.value, props.entryPublicIps, form.access_url)
  if (!enableForward.value || !forward.host_id || !forward.instance_id) return ''
  const selected = routes.value?.find(item => item.id === forward.instance_id)
  const candidates = new Set(list(record(record(selected?.result.node_info).stun_info).public_ip).map(publicIPv4Candidate).filter(Boolean))
  const ip = props.entryPublicIps[forward.host_id]?.[forward.instance_id] || (candidates.size === 1 ? [...candidates][0] : '')
  const bind = forward.bindIP.trim()
  return publicAccessURL(ip, bind + ':' + (forward.port ?? form.port), form.access_url)
})
watch(() => forward.host_id, () => { forward.instance_id = '' }, { flush: 'sync' })
watch([() => forward.host_id, () => forward.instance_id, () => forward.bindIP, () => forward.port], () => { forward.confirmed = false })

function startForm(service?: DeviceService) {
  editing.value = service ?? null
  Object.assign(form, {
    name: service?.name ?? '', protocols: service ? [...service.protocols] : ['tcp'],
    port: service?.port, access_url: service?.access_url ?? '', icon: service?.icon ?? '', note: service?.note ?? ''
  })
  Object.assign(forward, { host_id: '', instance_id: '', bindIP: '0.0.0.0', port: service?.port, confirmed: false })
  enableForward.value = false
  errorMessage.value = ''
  checkMessage.value = ''
  adding.value = true
}

function updateList(service: DeviceService) {
  client.setQueryData<DeviceService[]>(queryKey.value, previous => previous?.some(item => item.id === service.id)
    ? previous.map(item => item.id === service.id ? service : item)
    : [...(previous ?? []), service])
  void client.invalidateQueries({ queryKey: navigationServicesKey })
}

async function checkForward() {
  const service = editing.value
  if (!service?.forward?.enabled || checking.value) return
  checking.value = true
  checkMessage.value = ''
  try {
    const updated = await checkDeviceServiceForward(props.device.id, service.id)
    editing.value = updated
    updateList(updated)
    if (!updated.forward?.enabled) checkMessage.value = '当前规则缺失或目标地址已变化，转发已标记为关闭。'
  } catch (error) {
    checkMessage.value = '未能校验当前转发：' + formatApiError(error)
  } finally {
    checking.value = false
  }
}

async function editService(service: DeviceService) {
  startForm(service)
  await checkForward()
}

async function toggleListForward(service: DeviceService, enabled: boolean) {
  if (busy.value) return
  const cfg = service.forward
  if (!cfg) {
    if (enabled) {
      startForm(service)
      enableForward.value = true
    }
    return
  }

  switching.value = service.id
  try {
    const updated = await setDeviceServiceForward(props.device.id, service.id, enabled ? {
      enabled: true, host_id: cfg.host_id, instance_id: cfg.instance_id,
      listen: cfg.listen, allow_public_bind: true
    } : { enabled: false, retain_config: true })
    updateList(updated)
  } catch (error) {
    ElMessage.error(formatApiError(error))
  } finally {
    switching.value = ''
  }
}

async function toggleNavigation(service: DeviceService, enabled: boolean) {
  if (busy.value) return
  savingNavigation.value = service.id
  try {
    updateList(await setDeviceServiceNavigation(props.device.id, service.id, enabled))
  } catch (error) {
    ElMessage.error(formatApiError(error))
  } finally {
    savingNavigation.value = ''
  }
}

async function removeService(service: DeviceService) {
  if (busy.value) return
  deleting.value = service.id
  try {
    try {
      await ElMessageBox.confirm(
        '确定删除「' + service.name + '」？' + (service.forward?.enabled ? '系统会先关闭它的公网转发。' : ''),
        '删除服务', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
    await deleteDeviceService(props.device.id, service.id)
    client.setQueryData<DeviceService[]>(queryKey.value, items => items?.filter(item => item.id !== service.id))
    await client.invalidateQueries({ queryKey: navigationServicesKey })
    await client.invalidateQueries({ queryKey: queryKey.value })
    ElMessage.success('服务已删除')
  } catch (error) {
    ElMessage.error(formatApiError(error))
  } finally {
    deleting.value = ''
  }
}

async function removeForward() {
  if (!editing.value || busy.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    editing.value = await setDeviceServiceForward(props.device.id, editing.value.id, { enabled: false })
    updateList(editing.value)
    checkMessage.value = ''
  } catch (error) {
    errorMessage.value = formatApiError(error)
  } finally {
    saving.value = false
  }
}

watch(open, value => {
  if (value) {
    adding.value = false
    importing.value = false
  }
})

async function afterImport() {
  importing.value = false
  await client.invalidateQueries({ queryKey: queryKey.value })
}

async function saveService() {
  if (busy.value) return
  const { port, name, protocols } = form
  if (!name.trim() || !protocols.length || port == null || !Number.isInteger(port) || port < 1 || port > 65535) {
    errorMessage.value = '请填写服务名称，至少选择一种协议，并填写 1–65535 的端口。'
    return
  }
  const bindIP = forward.bindIP.trim().replace(/^\[|\]$/g, '')
  const bindPort = forward.port ?? port
  const listen = (bindIP.includes(':') ? '[' + bindIP + ']' : bindIP) + ':' + bindPort
  const publish = enableForward.value && !editing.value?.forward
  if (publish && (!forward.host_id || !routes.value?.some(item => item.id === forward.instance_id) ||
    !bindIP || !Number.isInteger(bindPort) || bindPort < 1 || bindPort > 65535 || !forward.confirmed)) {
    errorMessage.value = '请选择入口和实例，填写有效的监听地址及端口，并确认公网转发。'
    return
  }
  saving.value = true
  errorMessage.value = ''
  try {
    const payload = { ...form, port }
    editing.value = editing.value
      ? await updateDeviceService(props.device.id, editing.value.id, payload)
      : await createDeviceService(props.device.id, payload)
    updateList(editing.value)
    if (publish) {
      editing.value = await setDeviceServiceForward(props.device.id, editing.value.id, {
        enabled: true, host_id: forward.host_id, instance_id: forward.instance_id,
        listen, allow_public_bind: forward.confirmed
      })
      updateList(editing.value)
    }
    adding.value = false
    await client.invalidateQueries({ queryKey: queryKey.value })
  } catch (error) {
    errorMessage.value = (publish && editing.value ? '转发未开启：' : '') + formatApiError(error)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="open" :title="(importing ? '关联主机' : adding ? (editing ? '编辑服务' : '新增服务') : '服务列表') + ' · ' + device.name" width="min(660px, 94vw)" destroy-on-close
    :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy"
  >
    <DeviceServiceImport v-if="importing" v-model:saving="savingImport" :device="device" :hosts="hosts" :network-ip="networkIp"
      @close="importing = false" @imported="afterImport" />
    <el-form v-else-if="adding" class="service-form" label-position="top" @submit.prevent="saveService">
      <el-alert v-if="errorMessage" type="error" :title="errorMessage" show-icon :closable="false" />
      <el-form-item label="服务名称" required>
        <el-input v-model="form.name" maxlength="128" placeholder="例如：NAS 管理后台" :disabled="saving" />
      </el-form-item>
      <div class="service-form-grid">
        <el-form-item label="协议" required>
          <el-checkbox-group v-model="form.protocols" :disabled="busy || editing?.forward?.enabled">
            <el-checkbox value="tcp">TCP</el-checkbox>
            <el-checkbox value="udp">UDP</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="端口" required>
          <el-input-number v-model="form.port" :min="1" :max="65535" :precision="0" :controls="false" :disabled="busy || editing?.forward?.enabled" />
        </el-form-item>
      </div>
      <div class="service-urls" aria-label="自动生成的访问 URL">
        <el-form-item label="组网访问 URL">
          <el-input :model-value="draftNetworkURL" readonly placeholder="需要设备组网 IP 和服务端口" />
        </el-form-item>
        <el-form-item label="公网访问 URL">
          <el-input :model-value="draftPublicURL" readonly placeholder="需要公网入口 IP 和转发端口" />
        </el-form-item>
      </div>
      <el-form-item v-if="form.access_url" label="原有自定义访问 URL">
        <el-input v-model="form.access_url" maxlength="2048" :disabled="saving" />
      </el-form-item>
      <el-form-item label="图标（可选）">
        <el-select v-model="form.icon" clearable placeholder="默认服务图标" :disabled="saving">
          <el-option v-for="icon in serviceIcons" :key="icon.value" :label="icon.label" :value="icon.value">
            <el-icon><component :is="icon.component" /></el-icon> {{ icon.label }}
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="备注（可选）">
        <el-input v-model="form.note" type="textarea" :rows="2" maxlength="2000" :disabled="saving" />
      </el-form-item>
      <section class="service-forward">
        <h3>公网转发</h3>
        <el-alert v-if="checkMessage" :title="checkMessage" type="warning" :closable="false" show-icon />
        <p v-if="checking">正在校验实际转发规则…</p>
        <template v-if="editing?.forward">
          <p>{{ editing.forward.enabled ? '已开启' : '已关闭' }} · {{ editing.forward.listen }} → {{ editing.forward.destination }}</p>
          <el-button v-if="editing.forward.enabled" :disabled="busy" @click="checkForward">重新校验</el-button>
          <el-button :disabled="busy" @click="removeForward">{{ editing.forward.enabled ? '移除转发配置' : '移除原规则和配置' }}</el-button>
        </template>
        <template v-else>
          <el-checkbox v-model="enableForward" :disabled="busy">保存并开启公网转发</el-checkbox>
          <template v-if="enableForward">
            <el-form-item label="公网入口" required>
              <el-select v-model="forward.host_id" placeholder="选择已配对的组网入口" :disabled="busy">
                <el-option v-for="host in entryHosts" :key="host.id" :label="host.name" :value="host.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="EasyTier 网络实例" required>
              <el-select v-model="forward.instance_id" placeholder="选择网络实例" :loading="loadingInstances" :disabled="busy || !forward.host_id">
                <el-option v-for="instance in routes" :key="instance.id" :label="instance.name" :value="instance.id" />
              </el-select>
              <el-button v-if="instancesFailed" text @click="reloadInstances()">网络实例读取失败，重试</el-button>
            </el-form-item>
            <div class="service-form-grid">
              <el-form-item label="监听 IP" required>
                <el-input v-model="forward.bindIP" :disabled="busy" placeholder="0.0.0.0" />
              </el-form-item>
              <el-form-item label="外部端口">
                <el-input-number v-model="forward.port" :min="1" :max="65535" :controls="false" :disabled="busy" :placeholder="String(form.port ?? '')" />
              </el-form-item>
            </div>
            <p>目标取设备的组网 IP 和服务端口。监听 0.0.0.0 或 :: 会绑定所有对应网卡。</p>
            <el-checkbox v-model="forward.confirmed" :disabled="busy">确认在所选入口开启转发</el-checkbox>
          </template>
        </template>
      </section>
      <div class="service-form-actions">
        <el-button :disabled="busy" @click="adding = false">返回列表</el-button>
        <el-button type="primary" native-type="submit" :disabled="checking" :loading="saving">保存服务</el-button>
      </div>
    </el-form>
    <template v-else>
      <div class="service-list-actions">
        <el-button :disabled="busy" :icon="Link" @click="importing = true">关联主机</el-button>
        <el-button type="primary" :disabled="busy" @click="startForm()"><el-icon><Plus /></el-icon>新增服务</el-button>
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
            <el-tag v-if="service.forward?.enabled" size="small" type="success" effect="plain">转发已开启</el-tag>
            <div v-for="link in [
              { label: '组网访问', url: serviceURL(networkIp, service.port, service.access_url) },
              { label: '公网访问', url: activePublicURL(service, entryPublicIps) },
              { label: '自定义访问', url: service.access_url }
            ].filter(item => item.url)" :key="link.label" class="service-link">
              <span>{{ link.label }}：</span>
              <a :href="link.url" target="_blank" rel="noopener noreferrer">{{ link.url }}</a>
            </div>
            <p v-if="service.source" class="service-source">来源主机 {{ service.source.host_name }} · {{ service.source.app_id }}</p>
            <p v-if="service.note">{{ service.note }}</p>
          </div>
          <div class="service-actions">
            <div class="navigation-switch">
              <el-switch
                :model-value="service.navigation_enabled" :loading="savingNavigation === service.id"
                :disabled="busy" :aria-label="'开启导航：' + service.name"
                @update:model-value="toggleNavigation(service, $event === true)"
              />
              <span aria-hidden="true">开启导航</span>
            </div>
            <el-switch
              :model-value="service.forward?.enabled === true" :loading="switching === service.id"
              :disabled="busy" :aria-label="'公网转发：' + service.name" active-text="公网转发"
              @update:model-value="toggleListForward(service, $event === true)"
            />
            <el-button text circle aria-label="编辑服务" title="编辑服务" :disabled="busy" @click="editService(service)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button text circle type="danger" aria-label="删除服务" title="删除服务"
              :disabled="busy" :loading="deleting === service.id" @click="removeService(service)">
              <el-icon><Delete /></el-icon>
            </el-button>
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
.service-forward { border-block-start:1px solid var(--ent-line); margin-block:12px; padding-block:12px; }
.service-forward h3 { margin:0 0 8px; font-size:14px; }
.service-forward p { color:var(--ent-muted); overflow-wrap:anywhere; }
.navigation-switch { display:flex; align-items:center; gap:6px; margin-right:8px; color:var(--ent-muted); white-space:nowrap; }
.service-actions { display:flex; flex-wrap:wrap; align-items:center; gap:4px; margin-left:auto; flex:none; }
.service-details { display:flex; min-width:0; align-items:flex-start; flex-wrap:wrap; gap:6px 10px; overflow-wrap:anywhere; }
.service-link, .service-details p { flex-basis:100%; }
.service-details a { color:var(--ent-data-highlight); }
.service-source { color:var(--ent-muted); font-size:12px; }
.service-details p { margin:0; white-space:pre-wrap; }
@media (max-width:560px) {
  .service-form-grid { grid-template-columns:1fr; gap:0; }
  .service-list li { flex-wrap:wrap; }
  .service-details { flex:1; }
  .service-actions { flex-basis:100%; justify-content:flex-end; }
}
</style>
