<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search, Edit, Delete, Connection, Monitor, Iphone, Box, Link, Setting } from '@element-plus/icons-vue'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { mergedNetworkRows, type NetworkRow } from '../network-nodes/data'
import { networkNodeAttributeKey } from '../network-nodes/identity'
import { listNodeAttributes, type NodeAttributes } from '../network-nodes/attributes'
import {
  createDevice, createDeviceGroup, deleteDevice, deleteDeviceGroup, getDeviceGroups, getDevices,
  getDeviceOpenWrtConfig, saveDeviceOpenWrtConfig, clearDeviceOpenWrtConfig, updateDevice, updateDeviceGroup
} from './api'
import type { Device, DeviceGroup, DeviceInput, DeviceOpenWrtConfig, DeviceType } from './types'

const props = defineProps<{ hosts: Host[] }>()
const queryClient = useQueryClient()
const devicesQuery = useQuery({ queryKey: ['devices'], queryFn: getDevices, retry: 0 })
const groupsQuery = useQuery({ queryKey: ['device-groups'], queryFn: getDeviceGroups, retry: 0 })
const attributesQuery = useQuery({ queryKey: ['network-node-attributes'], queryFn: listNodeAttributes, retry: 0 })
const networkEntries = computed(() => props.hosts.filter(host => host.network_entry))
const nodesQuery = useQuery({
  queryKey: computed(() => ['device-network-nodes', networkEntries.value.map(host => host.id).join('|')]),
  queryFn: async ({ signal }) => {
    const settled = await Promise.allSettled(networkEntries.value.map(async host => {
      const response = await apiRequest<{ data: unknown }>(
        '/api/v1/hosts/' + encodeURIComponent(host.id) + '/network/route', { signal })
      return mergedNetworkRows(response.data).filter(row => row.id !== '—').map(row => ({
        row, hostId: host.id, networkKey: networkNodeAttributeKey(row, host.id)
      }))
    }))
    const nodes = settled.flatMap(result => result.status === 'fulfilled' ? result.value : [])
    const errors = settled.filter(result => result.status === 'rejected').length
    if (!nodes.length && errors === settled.length && errors > 0) {
      throw new Error('暂时无法读取现有网络节点，请检查组网入口状态')
    }
    const unique = new Map<string, { row: NetworkRow; hostId: string; networkKey: string }>()
    for (const node of nodes) {
      const key = node.networkKey + '|' + node.row.id
      if (!unique.has(key)) unique.set(key, node)
    }
    return { nodes: [...unique.values()], errors }
  },
  enabled: computed(() => networkEntries.value.length > 0),
  retry: 0,
  staleTime: 10_000,
  refetchInterval: 15_000
})
const devices = computed(() => devicesQuery.data.value || [])
const groups = computed(() => groupsQuery.data.value || [])
const attributesByKey = computed(() => new Map<string, NodeAttributes>(
  (attributesQuery.data.value?.attributes || []).map(item => [item.network_key + '|' + item.peer_id, item])
))
const nodeOptions = computed(() => (nodesQuery.data.value?.nodes || []).map(node => ({
  ...node,
  label: attributesByKey.value.get(node.networkKey + '|' + node.row.id)?.alias || node.row.hostname
})))
const nodeErrors = computed(() => nodesQuery.data.value?.errors || 0)
const search = ref('')
const selectedDeviceIds = ref<string[]>([])
watch(search, () => { selectedDeviceIds.value = [] })
const filteredDevices = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  if (!keyword) return devices.value
  return devices.value.filter(device => [
    device.name, device.group_name, device.device_type, device.manufacturer, device.model,
    device.ip_address, device.mac_address, device.note, device.peer_id
  ].some(value => (value || '').toLocaleLowerCase().includes(keyword)))
})
const allFilteredSelected = computed(() => filteredDevices.value.length > 0 &&
  filteredDevices.value.every(device => selectedDeviceIds.value.includes(device.id)))
function toggleDeviceSelection(id: string, checked: boolean) {
  selectedDeviceIds.value = checked
    ? [...new Set([...selectedDeviceIds.value, id])]
    : selectedDeviceIds.value.filter(value => value !== id)
}
function toggleSelectAll(checked: boolean) {
  const visible = filteredDevices.value.map(device => device.id)
  selectedDeviceIds.value = checked
    ? [...new Set([...selectedDeviceIds.value, ...visible])]
    : selectedDeviceIds.value.filter(id => !visible.includes(id))
}
const sections = computed(() => {
  const result: Array<{ id: string; name: string; group: DeviceGroup | null; items: Device[] }> = groups.value.map(group => ({
    id: group.id, name: group.name, group,
    items: filteredDevices.value.filter(device => device.group_id === group.id)
  }))
  const ungrouped = filteredDevices.value.filter(device => !device.group_id)
  if (ungrouped.length || !result.length) {
    result.push({ id: '__ungrouped__', name: '未分组', group: null, items: ungrouped })
  }
  return result
})
const collapsed = ref<string[]>([])
function toggleGroup(id: string) {
  collapsed.value = collapsed.value.includes(id)
    ? collapsed.value.filter(value => value !== id) : [...collapsed.value, id]
}
const typeLabels: Record<DeviceType, string> = {
  router: '路由器', computer: '电脑', phone: '手机', tablet: '平板', tv: '电视',
  nas: 'NAS', other: '其他设备'
}
const typeIcons = { router: Connection, computer: Monitor, phone: Iphone, tablet: Iphone, tv: Monitor, nas: Box, other: Monitor }
function typeLabel(type: DeviceType) { return typeLabels[type] || '其他设备' }
function nodeValue(node: { networkKey: string; row: NetworkRow }) {
  return JSON.stringify([node.networkKey, node.row.id])
}
function nodeForDevice(device: Device) {
  return nodeOptions.value.find(node => node.networkKey === device.network_key && node.row.id === device.peer_id)
}
function deviceNodeLabel(device: Device) {
  const node = nodeForDevice(device)
  if (node) return node.label + ' · ' + node.row.instance
  return device.peer_id ? '已关联节点 · ' + device.peer_id : '未关联网络节点'
}

const deviceDialogOpen = ref(false)
const savingDevice = ref(false)
const editingDeviceId = ref('')
const deviceDraft = ref<DeviceInput>(emptyDevice())
function emptyDevice(): DeviceInput {
  return { name: '', group_id: '', device_type: 'other', manufacturer: '', model: '',
    ip_address: '', mac_address: '', note: '', is_openwrt: false, network_key: '', peer_id: '' }
}
function openCreateDevice() {
  editingDeviceId.value = ''
  deviceDraft.value = emptyDevice()
  deviceDialogOpen.value = true
}
function openEditDevice(device: Device) {
  editingDeviceId.value = device.id
  deviceDraft.value = {
    name: device.name, group_id: device.group_id || '', device_type: device.device_type,
    manufacturer: device.manufacturer || '', model: device.model || '',
    ip_address: device.ip_address || '', mac_address: device.mac_address || '',
    note: device.note || '', is_openwrt: device.is_openwrt,
    network_key: device.network_key || '', peer_id: device.peer_id || ''
  }
  deviceDialogOpen.value = true
}
const selectedNodeValue = computed(() => deviceDraft.value.network_key && deviceDraft.value.peer_id
  ? JSON.stringify([deviceDraft.value.network_key, deviceDraft.value.peer_id]) : '')
function selectNetworkNode(value: string) {
  if (!value) {
    deviceDraft.value.network_key = ''
    deviceDraft.value.peer_id = ''
    return
  }
  try {
    const [networkKey, peerId] = JSON.parse(value) as [string, string]
    deviceDraft.value.network_key = networkKey
    deviceDraft.value.peer_id = peerId
  } catch {
    deviceDraft.value.network_key = ''
    deviceDraft.value.peer_id = ''
  }
}
async function saveDeviceRecord() {
  if (savingDevice.value) return
  const draft = { ...deviceDraft.value }
  if (draft.is_openwrt && draft.device_type !== 'router') draft.device_type = 'router'
  if (!draft.name.trim()) { ElMessage.warning('请填写设备名称'); return }
  if (draft.is_openwrt && !draft.ip_address.trim()) { ElMessage.warning('OpenWrt 设备需要填写一个管理 IP'); return }
  const previous = devices.value.find(item => item.id === editingDeviceId.value)
  if (previous?.is_openwrt && previous.openwrt_configured && !draft.is_openwrt) {
    try {
      await ElMessageBox.confirm('取消 OpenWrt 标识后，将同时清除该设备保存的 RPCD 发现服务配置。是否继续？',
        '确认取消 OpenWrt 标识', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' })
    } catch { return }
  }
  savingDevice.value = true
  try {
    if (editingDeviceId.value) await updateDevice(editingDeviceId.value, draft)
    else await createDevice(draft)
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-groups'] })
    ])
    deviceDialogOpen.value = false
    ElMessage.success(editingDeviceId.value ? '设备资料已更新' : '设备已添加')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { savingDevice.value = false }
}
async function deleteSelectedDevices() {
  const ids = [...selectedDeviceIds.value]
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(
      '确定删除选中的 ' + ids.length + ' 台设备？只删除设备登记和各自的 OpenWrt 配置，不会删除关联的网络节点。',
      '批量删除设备', { type: 'warning', confirmButtonText: '删除所选设备', cancelButtonText: '取消' })
    const results = await Promise.allSettled(ids.map(id => deleteDevice(id)))
    const succeeded = ids.filter((_, index) => results[index].status === 'fulfilled')
    const failed = ids.filter((_, index) => results[index].status === 'rejected')
    selectedDeviceIds.value = failed
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-groups'] })
    ])
    if (failed.length) ElMessage.warning('已删除 ' + succeeded.length + ' 台，失败 ' + failed.length + ' 台；失败项仍保留在选择中')
    else ElMessage.success('已删除 ' + succeeded.length + ' 台设备')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(formatApiError(error))
  }
}
const groupDialogOpen = ref(false)
const savingGroup = ref(false)
const editingGroupId = ref('')
const groupNameDraft = ref('')
function openCreateGroup() { editingGroupId.value = ''; groupNameDraft.value = ''; groupDialogOpen.value = true }
function openEditGroup(group: DeviceGroup) { editingGroupId.value = group.id; groupNameDraft.value = group.name; groupDialogOpen.value = true }
async function saveGroup() {
  if (savingGroup.value) return
  if (!groupNameDraft.value.trim()) { ElMessage.warning('请填写分组名称'); return }
  savingGroup.value = true
  try {
    if (editingGroupId.value) await updateDeviceGroup(editingGroupId.value, groupNameDraft.value)
    else await createDeviceGroup(groupNameDraft.value)
    await queryClient.invalidateQueries({ queryKey: ['device-groups'] })
    groupDialogOpen.value = false
    ElMessage.success(editingGroupId.value ? '分组已更新' : '分组已创建')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { savingGroup.value = false }
}
async function removeGroup(group: DeviceGroup) {
  try {
    await ElMessageBox.confirm('删除分组「' + group.name + '」后，组内设备会自动转入未分组，不会删除设备。',
      '确认删除分组', { type: 'warning', confirmButtonText: '删除分组', cancelButtonText: '取消' })
    await deleteDeviceGroup(group.id)
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['device-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['devices'] })
    ])
    ElMessage.success('分组已删除，设备已转入未分组')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(formatApiError(error))
  }
}

const openWrtDialogOpen = ref(false)
const openWrtTarget = ref<Device | null>(null)
const openWrtSaving = ref(false)
const openWrtForm = ref({ rpcd_port: 80, https: false, username: 'easy-device', password: '' })
const openWrtConfig = ref<DeviceOpenWrtConfig | null>(null)
async function openOpenWrtSettings(device: Device) {
  openWrtTarget.value = device
  openWrtForm.value = { rpcd_port: device.openwrt_rpcd_port || 80, https: !!device.openwrt_https,
    username: device.openwrt_username || 'easy-device', password: '' }
  openWrtConfig.value = null
  openWrtDialogOpen.value = true
  try {
    const config = await getDeviceOpenWrtConfig(device.id)
    if (openWrtTarget.value?.id !== device.id) return
    openWrtConfig.value = config
    openWrtForm.value = { rpcd_port: config.rpcd_port || 80, https: config.https,
      username: config.username || 'easy-device', password: '' }
  } catch (error) {
    ElMessage.error(formatApiError(error))
  }
}
async function saveOpenWrtSettings() {
  const target = openWrtTarget.value
  if (!target || openWrtSaving.value) return
  if (!openWrtForm.value.username.trim()) { ElMessage.warning('请填写 OpenWrt 专用用户名'); return }
  if (!openWrtConfig.value?.configured && !openWrtForm.value.password) {
    ElMessage.warning('首次配置需要填写 OpenWrt 专用密钥')
    return
  }
  openWrtSaving.value = true
  try {
    await saveDeviceOpenWrtConfig(target.id, { ...openWrtForm.value, username: openWrtForm.value.username.trim() })
    await queryClient.invalidateQueries({ queryKey: ['devices'] })
    openWrtDialogOpen.value = false
    ElMessage.success('该设备的 OpenWrt RPCD 配置已保存')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { openWrtSaving.value = false }
}
async function removeOpenWrtSettings() {
  const target = openWrtTarget.value
  if (!target || !openWrtConfig.value?.configured) return
  try {
    await ElMessageBox.confirm('只清除该设备保存的 RPCD 账号配置，不会修改 OpenWrt 路由器本身。',
      '清除 OpenWrt 服务配置', { type: 'warning', confirmButtonText: '清除配置', cancelButtonText: '取消' })
    await clearDeviceOpenWrtConfig(target.id)
    await queryClient.invalidateQueries({ queryKey: ['devices'] })
    openWrtDialogOpen.value = false
    ElMessage.success('OpenWrt RPCD 配置已清除')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(formatApiError(error))
  }
}
const isRefreshing = computed(() => devicesQuery.isFetching.value || groupsQuery.isFetching.value)
async function refreshAll() {
  await Promise.all([
    devicesQuery.refetch(), groupsQuery.refetch(), nodesQuery.refetch(), attributesQuery.refetch()
  ])
}
</script>

<template>
  <section class="devices-pane" aria-label="设备列表">
    <div class="devices-intro">
      <div>
        <h2>设备列表</h2>
        <p>统一登记和分组管理设备；网络节点仅建立关联，不重复管理网络信息。</p>
      </div>
      <div class="devices-actions">
        <el-button @click="refreshAll" :loading="isRefreshing"><el-icon><Refresh /></el-icon>刷新</el-button>
        <el-button @click="openCreateGroup"><el-icon><Plus /></el-icon>新建分组</el-button>
        <el-button type="primary" @click="openCreateDevice"><el-icon><Plus /></el-icon>添加设备</el-button>
      </div>
    </div>

    <div class="devices-toolbar">
      <el-input v-model="search" clearable placeholder="搜索设备名称 / IP / MAC / 型号" aria-label="搜索设备">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <div class="devices-summary">
        <span>{{ filteredDevices.length }} 台设备</span>
        <span>{{ groups.length }} 个分组</span>
        <el-checkbox :model-value="allFilteredSelected" @update:model-value="toggleSelectAll">全选当前结果</el-checkbox>
        <el-button v-if="selectedDeviceIds.length" type="danger" plain size="small" @click="deleteSelectedDevices">
          <el-icon><Delete /></el-icon>删除所选 {{ selectedDeviceIds.length }} 台
        </el-button>
      </div>
    </div>

    <el-alert v-if="devicesQuery.isError.value || groupsQuery.isError.value"
      :title="formatApiError(devicesQuery.error.value || groupsQuery.error.value)"
      type="error" :closable="false" show-icon />
    <el-alert v-if="networkEntries.length && nodesQuery.isError.value"
      title="网络节点关联列表暂时无法读取；设备资料仍可正常管理。" type="warning" :closable="false" show-icon />
    <el-alert v-else-if="nodeErrors" :title="nodeErrors + ' 个组网入口暂时不可用，已显示其他入口可读取的节点。'"
      type="warning" :closable="false" show-icon />
    <el-alert v-if="!networkEntries.length" title="尚未设置组网入口，设备仍可独立登记；设置入口后可关联现有网络节点。"
      type="info" :closable="false" show-icon />

    <div v-if="devicesQuery.isPending.value || groupsQuery.isPending.value" class="devices-loading">
      <el-skeleton :rows="5" animated />
    </div>
    <template v-else>
      <div v-for="section in sections" :key="section.id" class="device-group">
        <div class="group-heading">
          <button class="group-toggle" type="button" :aria-expanded="!collapsed.includes(section.id)"
            @click="toggleGroup(section.id)">
            <span class="group-chevron" :class="{ collapsed: collapsed.includes(section.id) }">⌄</span>
            <strong>{{ section.name }}</strong>
            <span class="group-count">{{ section.items.length }}</span>
          </button>
          <div v-if="section.group" class="group-actions">
            <el-button text size="small" @click="openEditGroup(section.group)"><el-icon><Edit /></el-icon>重命名</el-button>
            <el-button text size="small" type="danger" @click="removeGroup(section.group)"><el-icon><Delete /></el-icon>删除分组</el-button>
          </div>
        </div>
        <div v-if="!collapsed.includes(section.id)" class="device-grid">
          <article v-for="device in section.items" :key="device.id" class="device-card">
            <div class="device-card-head">
              <el-checkbox class="device-select" :model-value="selectedDeviceIds.includes(device.id)"
                :aria-label="'选择设备 ' + device.name"
                @update:model-value="toggleDeviceSelection(device.id, $event)" />
              <div class="device-avatar">
                <el-icon><component :is="typeIcons[device.device_type]" /></el-icon>
              </div>
              <div class="device-main">
                <div class="device-title">
                  <strong>{{ device.name }}</strong>
                  <el-tag size="small" effect="plain">{{ typeLabel(device.device_type) }}</el-tag>
                  <el-tag v-if="device.is_openwrt" size="small" type="success" effect="dark">OpenWrt</el-tag>
                </div>
                <div class="device-subtitle">
                  <span>{{ [device.manufacturer, device.model].filter(Boolean).join(' · ') || '未填写厂商与型号' }}</span>
                  <span v-if="device.ip_address">{{ device.ip_address }}</span>
                </div>
              </div>
              <div class="device-actions">
                <el-button text circle aria-label="编辑设备" title="编辑设备" @click="openEditDevice(device)"><el-icon><Edit /></el-icon></el-button>
                <el-button v-if="device.is_openwrt" text circle aria-label="OpenWrt RPCD 配置" title="OpenWrt RPCD 配置"
                  @click="openOpenWrtSettings(device)"><el-icon><Setting /></el-icon></el-button>
              </div>
            </div>
            <div class="device-meta">
              <span v-if="device.mac_address">MAC：{{ device.mac_address }}</span>
              <span v-if="device.network_key" class="node-association"><el-icon><Link /></el-icon>{{ deviceNodeLabel(device) }}</span>
              <span v-else class="muted">未关联网络节点</span>
            </div>
            <p v-if="device.note" class="device-note">{{ device.note }}</p>
            <div v-if="device.is_openwrt" class="openwrt-status">
              <el-icon><Connection /></el-icon>
              <span>{{ device.openwrt_configured ? 'RPCD 账号已配置' : 'RPCD 账号尚未配置' }}</span>
              <span v-if="device.openwrt_configured" class="openwrt-endpoint">
                {{ device.openwrt_https ? 'HTTPS' : 'HTTP' }} · {{ device.openwrt_rpcd_port || 80 }}
              </span>
            </div>
          </article>
          <el-empty v-if="!section.items.length" description="该分组还没有设备" :image-size="54" />
        </div>
      </div>
      <el-empty v-if="!sections.some(section => section.items.length) && search" description="没有匹配的设备" />
    </template>

    <el-dialog v-model="deviceDialogOpen" :title="editingDeviceId ? '编辑设备' : '添加设备'" width="min(620px, 94vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="saveDeviceRecord">
        <div class="form-grid">
          <el-form-item label="设备名称" required>
            <el-input v-model="deviceDraft.name" maxlength="100" placeholder="例如：客厅路由器" />
          </el-form-item>
          <el-form-item label="所属分组">
            <el-select v-model="deviceDraft.group_id" clearable placeholder="未分组" class="full">
              <el-option v-for="group in groups" :key="group.id" :label="group.name" :value="group.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="设备类型" required>
            <el-select v-model="deviceDraft.device_type" class="full"
              @change="deviceDraft.device_type !== 'router' && (deviceDraft.is_openwrt = false)">
              <el-option v-for="(label, value) in typeLabels" :key="value" :label="label" :value="value" />
            </el-select>
          </el-form-item>
          <el-form-item label="厂商">
            <el-input v-model="deviceDraft.manufacturer" maxlength="100" placeholder="例如：TP-Link" />
          </el-form-item>
          <el-form-item label="型号">
            <el-input v-model="deviceDraft.model" maxlength="120" placeholder="例如：XDR6088" />
          </el-form-item>
          <el-form-item label="设备管理 IP" :required="deviceDraft.is_openwrt">
            <el-input v-model="deviceDraft.ip_address" placeholder="例如：192.168.1.1" />
          </el-form-item>
          <el-form-item label="MAC 地址">
            <el-input v-model="deviceDraft.mac_address" placeholder="例如：AA:BB:CC:DD:EE:FF" />
          </el-form-item>
          <el-form-item label="关联现有网络节点" class="span-two">
            <el-select :model-value="selectedNodeValue" clearable filterable class="full"
              placeholder="不关联网络节点" @update:model-value="selectNetworkNode">
              <el-option v-for="node in nodeOptions" :key="node.networkKey + '|' + node.row.id"
                :label="node.label + ' · ' + node.row.ip + ' · ' + node.row.instance"
                :value="nodeValue(node)" />
            </el-select>
            <div class="field-hint">只关联现有节点身份，不复制节点 IP、Peer ID 或网络属性。</div>
          </el-form-item>
          <el-form-item label="设备备注" class="span-two">
            <el-input v-model="deviceDraft.note" type="textarea" :rows="2" maxlength="1000" show-word-limit />
          </el-form-item>
        </div>
        <el-divider />
        <el-form-item label="设备能力">
          <el-checkbox v-model="deviceDraft.is_openwrt" @change="deviceDraft.is_openwrt && (deviceDraft.device_type = 'router')">
            这是 OpenWrt 路由器
          </el-checkbox>
          <div class="field-hint">只有标记为 OpenWrt 的设备才会出现专属 RPCD 发现服务配置。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="deviceDialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="savingDevice" @click="saveDeviceRecord">保存设备</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="groupDialogOpen" :title="editingGroupId ? '重命名分组' : '新建分组'" width="min(420px, 92vw)">
      <el-form label-position="top" @submit.prevent="saveGroup">
        <el-form-item label="分组名称" required>
          <el-input v-model="groupNameDraft" maxlength="64" show-word-limit placeholder="例如：家庭网络" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="savingGroup" @click="saveGroup">保存分组</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="openWrtDialogOpen" title="OpenWrt RPCD 配置" width="min(520px, 92vw)" destroy-on-close>
      <template v-if="openWrtTarget">
        <el-alert type="info" :closable="false" show-icon
          title="此配置属于设备「{{ openWrtTarget.name }}」，不会挂在分组上。RPCD 密钥在服务端加密保存，不会回传。" />
        <div class="rpcd-preview">
          RPCD 地址：{{ openWrtForm.https ? 'https' : 'http' }}://{{ openWrtTarget.ip_address }}:{{ openWrtForm.rpcd_port }}/ubus
        </div>
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item label="RPCD 端口">
              <el-input-number v-model="openWrtForm.rpcd_port" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item label="连接协议">
              <el-select v-model="openWrtForm.https" class="full">
                <el-option label="HTTP" :value="false" />
                <el-option label="HTTPS" :value="true" />
              </el-select>
            </el-form-item>
            <el-form-item label="OpenWrt 专用用户名">
              <el-input v-model="openWrtForm.username" maxlength="64" />
            </el-form-item>
            <el-form-item label="OpenWrt 专用密钥">
              <el-input v-model="openWrtForm.password" type="password" show-password autocomplete="new-password"
                :placeholder="openWrtConfig?.configured ? '留空保留现有密钥' : '首次配置时必填'" />
            </el-form-item>
          </div>
        </el-form>
        <p class="field-hint">仅允许使用设备管理 IP 对应的内网 IPv4 地址访问固定 /ubus 接口；不支持域名、公共地址或重定向。</p>
      </template>
      <template #footer>
        <el-button v-if="openWrtConfig?.configured" type="danger" plain @click="removeOpenWrtSettings">清除配置</el-button>
        <el-button @click="openWrtDialogOpen = false">关闭</el-button>
        <el-button type="primary" :loading="openWrtSaving" @click="saveOpenWrtSettings">保存配置</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.devices-pane { display:flex; min-width:0; min-height:0; flex:1; flex-direction:column; gap:18px; overflow:auto; padding:24px 28px 36px; }
.devices-intro { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; flex-wrap:wrap; }
.devices-intro h2 { margin:0; color:var(--ent-text); font-size:20px; font-weight:760; }
.devices-intro p { margin:7px 0 0; color:var(--ent-muted); font-size:12px; }
.devices-actions { display:flex; gap:8px; flex-wrap:wrap; }
.devices-toolbar { display:flex; align-items:center; gap:14px; }
.devices-toolbar :deep(.el-input) { max-width:420px; }
.devices-summary { display:flex; gap:12px; color:var(--ent-muted); font-size:12px; white-space:nowrap; }
.device-group { overflow:hidden; border:1px solid var(--ent-line); border-radius:14px; background:var(--ent-panel); }
.group-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; min-height:54px; padding:8px 15px; border-bottom:1px solid var(--ent-line-soft); background:var(--ent-panel-2); }
.group-toggle { display:flex; min-width:0; align-items:center; gap:10px; border:0; background:transparent; color:var(--ent-text); text-align:left; }
.group-toggle strong { font-size:14px; }
.group-chevron { display:inline-block; color:var(--ent-cyan); font-size:18px; transition:transform .15s; }
.group-chevron.collapsed { transform:rotate(-90deg); }
.group-count { padding:2px 8px; border-radius:12px; background:var(--ent-elev); color:var(--ent-muted); font-size:11px; }
.group-actions { display:flex; gap:4px; }
.device-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(min(360px,100%),1fr)); gap:12px; padding:14px; }
.device-card { min-width:0; padding:15px; border:1px solid var(--ent-line); border-radius:12px; background:var(--ent-node-row); }
.device-card-head { display:flex; align-items:flex-start; gap:10px; }
.device-select { flex:none; margin-top:10px; }
.device-avatar { display:grid; width:40px; height:40px; flex:none; place-items:center; border:1px solid var(--ent-line); border-radius:12px; background:var(--ent-elev); color:var(--ent-cyan); font-size:19px; }
.device-main { min-width:0; flex:1; }
.device-title { display:flex; align-items:center; gap:7px; flex-wrap:wrap; }
.device-title strong { overflow-wrap:anywhere; color:var(--ent-text); font-size:14px; }
.device-subtitle { display:flex; flex-wrap:wrap; gap:8px; margin-top:7px; color:var(--ent-muted); font-size:11px; }
.device-actions { display:flex; flex:none; gap:0; margin-left:auto; }
.device-actions :deep(.el-button) { color:var(--ent-muted); }
.device-meta { display:flex; align-items:center; flex-wrap:wrap; gap:12px; margin-top:13px; color:var(--ent-muted); font-size:11px; }
.node-association { display:inline-flex; align-items:center; gap:4px; color:var(--ent-data-highlight); }
.muted { color:var(--ent-dim); }
.device-note { margin:10px 0 0; color:var(--ent-muted); font-size:12px; white-space:pre-wrap; overflow-wrap:anywhere; }
.openwrt-status { display:flex; align-items:center; gap:7px; margin-top:12px; padding:8px 10px; border:1px solid var(--ent-detail-line); border-radius:8px; background:var(--ent-detail-bg); color:var(--ent-detail-accent); font-size:11px; }
.openwrt-endpoint { margin-left:auto; color:var(--ent-muted); }
.devices-loading { padding:18px; }
.form-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:0 14px; }
.full { width:100%; }
.span-two { grid-column:1 / -1; }
.field-hint { margin-top:5px; color:var(--ent-muted); font-size:11px; line-height:1.5; }
.rpcd-preview { margin:14px 0; padding:10px 12px; border:1px solid var(--ent-line); border-radius:8px; background:var(--ent-panel-2); color:var(--ent-data-highlight); font-size:12px; overflow-wrap:anywhere; }
@media(max-width:760px) {
  .devices-pane { padding:17px 14px 28px; gap:14px; }
  .devices-toolbar { align-items:stretch; flex-direction:column; }
  .devices-toolbar :deep(.el-input) { max-width:none; }
  .device-grid { grid-template-columns:1fr; padding:10px; }
  .group-heading { align-items:flex-start; }
  .form-grid { grid-template-columns:1fr; }
  .span-two { grid-column:auto; }
  .device-actions { gap:0; }
}
</style>
