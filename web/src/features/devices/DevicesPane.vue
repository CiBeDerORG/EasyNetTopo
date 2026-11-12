<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search, Edit, Delete, Connection, Monitor, Iphone, Box, Link, Setting } from '@element-plus/icons-vue'
import { apiRequest, formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { mergedNetworkRows, type NetworkRow } from '../network-nodes/data'
import { networkNodeAttributeKey } from '../network-nodes/identity'
import OpenWrtDiscoveryDialog from './OpenWrtDiscoveryDialog.vue'
import { suggestedDirectGatewayIP } from './gateway-ip'
import { listNodeAttributes, type NodeAttributes } from '../network-nodes/attributes'
import {
  createDevice, deleteDevice, getDevices, getNodeDevices, getGatewayGroups,
  getDeviceOpenWrtConfig, getNetworkNodeSyncStatus,
  syncNetworkNodeDevices, saveDeviceOpenWrtConfig, clearDeviceOpenWrtConfig, updateDevice
} from './api'
import type {
  Device, DeviceInput, DeviceOpenWrtConfig, GatewayGroup, NetworkNodeDeviceInput, OpenWrtDiscoveryKind
} from './types'

const props = defineProps<{ hosts: Host[] }>()
const queryClient = useQueryClient()
const route = useRoute()
const router = useRouter()
const UNASSIGNED_SCOPE = '__unassigned__'

type LiveNodeOption = {
  networkKey: string
  peerId: string
  label: string
  instance: string
  ip: string
  isOnline: true
  row: NetworkRow
  hostId: string
}
type NodeOption = {
  networkKey: string
  peerId: string
  label: string
  instance: string
  ip: string
  isOnline: boolean
  row?: NetworkRow
  hostId?: string
}
type DeviceSection = {
  id: string
  name: string
  kind: 'direct' | 'gateway'
  items: Device[]
  gatewayGroup?: GatewayGroup
  gatewayDevice?: Device
}

const networkNodeSyncQuery = useQuery({
  queryKey: ['device-network-node-sync'], queryFn: getNetworkNodeSyncStatus, retry: 0
})
const attributesQuery = useQuery({
  queryKey: ['network-node-attributes'], queryFn: listNodeAttributes, retry: 0
})
const nodeDevicesQuery = useQuery({
  queryKey: ['device-node-devices'], queryFn: getNodeDevices, retry: 0, staleTime: 10_000
})
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
const attributesByKey = computed(() => new Map<string, NodeAttributes>(
  (attributesQuery.data.value?.attributes || []).map(item => [item.network_key + '|' + item.peer_id, item])
))
const liveNodeOptions = computed<LiveNodeOption[]>(() => (nodesQuery.data.value?.nodes || []).map(node => ({
  networkKey: node.networkKey,
  peerId: node.row.id,
  label: attributesByKey.value.get(node.networkKey + '|' + node.row.id)?.alias || node.row.hostname,
  instance: node.row.instance,
  ip: node.row.ip,
  isOnline: true,
  row: node.row,
  hostId: node.hostId
})))
// 以持久化的节点设备补齐当前离线节点，保证历史设备仍可按原节点管理。
const nodeOptions = computed<NodeOption[]>(() => {
  const result: NodeOption[] = [...liveNodeOptions.value]
  const seen = new Set(result.map(node => node.networkKey + '|' + node.peerId))
  for (const device of nodeDevicesQuery.data.value || []) {
    const key = (device.network_key || '') + '|' + (device.peer_id || '')
    if (!device.network_key || !device.peer_id || seen.has(key)) continue
    seen.add(key)
    result.push({
      networkKey: device.network_key, peerId: device.peer_id, label: device.name,
      instance: '', ip: '', isOnline: false
    })
  }
  return result
})
function nodeValue(node: Pick<NodeOption, 'networkKey' | 'peerId'>) {
  return JSON.stringify([node.networkKey, node.peerId])
}
const selectedNodeKey = ref(typeof route.query.node === 'string' ? route.query.node : '')
const selectedNode = computed(() => nodeOptions.value.find(node => nodeValue(node) === selectedNodeKey.value))
const isUnassignedScope = computed(() => selectedNodeKey.value === UNASSIGNED_SCOPE)
watch(
  () => [nodeOptions.value, nodesQuery.isFetched.value, nodeDevicesQuery.isFetched.value] as const,
  ([items, liveFetched, storedFetched]) => {
    if ((networkEntries.value.length > 0 && !liveFetched) || !storedFetched) return
    if (selectedNodeKey.value === UNASSIGNED_SCOPE || items.some(node => nodeValue(node) === selectedNodeKey.value)) return
    selectedNodeKey.value = items.length ? nodeValue(items[0]) : UNASSIGNED_SCOPE
  },
  { immediate: true }
)
watch(selectedNodeKey, value => {
  const next = { ...route.query }
  if (value) next.node = value
  else delete next.node
  if (route.query.node !== value) void router.replace({ query: next })
})
watch(() => route.query.node, value => {
  if (typeof value === 'string' && value !== selectedNodeKey.value) selectedNodeKey.value = value
})

const devicesQuery = useQuery({
  queryKey: computed(() => ['devices', selectedNodeKey.value]),
  queryFn: () => isUnassignedScope.value
    ? getDevices({ unassigned: true })
    : selectedNode.value
      ? getDevices({ networkKey: selectedNode.value.networkKey, peerId: selectedNode.value.peerId })
      : Promise.resolve([] as Device[]),
  enabled: computed(() => isUnassignedScope.value || !!selectedNode.value),
  retry: 0
})
const gatewayGroupsQuery = useQuery({
  queryKey: computed(() => ['gateway-groups', selectedNode.value?.networkKey || '', selectedNode.value?.peerId || '']),
  queryFn: () => selectedNode.value
    ? getGatewayGroups(selectedNode.value.networkKey, selectedNode.value.peerId)
    : Promise.resolve([] as GatewayGroup[]),
  enabled: computed(() => !!selectedNode.value),
  retry: 0
})
const devices = computed(() => devicesQuery.data.value || [])
const gatewayGroups = computed(() => gatewayGroupsQuery.data.value || [])
const nodeErrors = computed(() => nodesQuery.data.value?.errors || 0)
const search = ref('')
const selectedDeviceIds = ref<string[]>([])
watch([search, selectedNodeKey], () => { selectedDeviceIds.value = [] })
const filteredDevices = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  if (!keyword) return devices.value
  return devices.value.filter(device => [
    device.name, device.group_name, device.gateway_group_name,
    device.manufacturer, device.model, device.ip_address, device.mac_address,
    device.note, device.peer_id, device.network_key, ...(device.semantic_kinds || [])
  ].some(value => (value || '').toLocaleLowerCase().includes(keyword)))
})
const nodeDevices = computed(() => filteredDevices.value.filter(device => device.is_node_device))
const gatewayGroupSections = computed(() => gatewayGroups.value
  .filter(group => !!group.gateway_device_id)
  .map(group => {
    const items = filteredDevices.value.filter(device => device.gateway_group_id === group.id)
    const gatewayDevice = devices.value.find(device => device.id === group.gateway_device_id)
    const gatewayMatchesSearch = !!gatewayDevice && filteredDevices.value.some(device => device.id === gatewayDevice.id)
    return { ...group, items, gatewayDevice, gatewayMatchesSearch }
  })
  .filter(group => !!group.gatewayDevice &&
    (!search.value.trim() || group.items.length > 0 || group.gatewayMatchesSearch)))
// 拥有下挂设备分组的直属网关信息直接显示在对应分组标题，不再重复占用一张直属设备卡片。
const directDevices = computed(() => {
  const gatewayDeviceIds = new Set(gatewayGroupSections.value.map(group => group.gateway_device_id))
  return filteredDevices.value.filter(device => !device.gateway_group_id && !gatewayDeviceIds.has(device.id))
})
const deviceSections = computed<DeviceSection[]>(() => {
  const result: DeviceSection[] = []
  if (directDevices.value.length) {
    result.push({ id: 'direct-devices', name: '直属设备', kind: 'direct', items: directDevices.value })
  }
  for (const group of gatewayGroupSections.value) {
    result.push({
      id: group.id,
      name: (group.gatewayDevice?.name || group.name) + ' 下挂设备',
      kind: 'gateway',
      items: group.items,
      gatewayGroup: group,
      gatewayDevice: group.gatewayDevice
    })
  }
  return result
})


const selectableDevices = computed(() => filteredDevices.value.filter(device => !device.is_node_device))
const allFilteredSelected = computed(() => selectableDevices.value.length > 0 &&
  selectableDevices.value.every(device => selectedDeviceIds.value.includes(device.id)))
function toggleDeviceSelection(id: string, checked: boolean) {
  selectedDeviceIds.value = checked
    ? [...new Set([...selectedDeviceIds.value, id])]
    : selectedDeviceIds.value.filter(value => value !== id)
}
function toggleSelectAll(checked: boolean) {
  const visible = selectableDevices.value.map(device => device.id)
  selectedDeviceIds.value = checked
    ? [...new Set([...selectedDeviceIds.value, ...visible])]
    : selectedDeviceIds.value.filter(id => !visible.includes(id))
}
const collapsed = ref<string[]>([])
watch(selectedNodeKey, () => { collapsed.value = [] })
function toggleGroup(id: string) {
  collapsed.value = collapsed.value.includes(id)
    ? collapsed.value.filter(value => value !== id) : [...collapsed.value, id]
}
const semanticKindLabels: Record<string, string> = {
  router: '路由器', server: '服务器', openwrt: 'OpenWrt', phone: '手机', computer: '电脑',
  tablet: '平板', tv: '电视', nas: 'NAS', linux: 'Linux', android: 'Android', other: '其他'
}
const semanticKindOptions = Object.entries(semanticKindLabels).map(([value, label]) => ({ value, label }))
const typeIcons: Record<string, typeof Connection> = {
  router: Connection, server: Monitor, openwrt: Connection, phone: Iphone, computer: Monitor,
  tablet: Iphone, tv: Monitor, nas: Box, linux: Monitor, android: Iphone, other: Monitor
}
function deviceIcon(device: Device) {
  const kind = (device.semantic_kinds || []).find(value => value in typeIcons)
  return kind ? typeIcons[kind] : Monitor
}
function isOpenWrtDevice(device: Pick<Device, 'semantic_kinds'>) {
  return (device.semantic_kinds || []).includes('openwrt')
}
const gatewayCount = computed(() => gatewayGroups.value.filter(group => !!group.gateway_device_id).length)
const deviceAssetCount = computed(() => filteredDevices.value.filter(device => !device.is_node_device).length)
const networkNodeSyncEnabled = computed(() => networkNodeSyncQuery.data.value?.enabled === true)

const deviceDialogOpen = ref(false)
const savingDevice = ref(false)
const editingDeviceId = ref('')
const deviceDraft = ref<DeviceInput>(emptyDevice())
const isOpenWrtDraft = computed(() => deviceDraft.value.semantic_kinds.includes('openwrt'))
// 设备编辑期间若代理网段尚未加载，可在节点数据返回后补填一次；用户手动输入会取消待补填状态。
const gatewayIPAutofillPending = ref(false)
function fillGatewayIPIfNeeded() {
  if (!gatewayIPAutofillPending.value || !deviceDialogOpen.value || !editingDeviceId.value) return
  const suggested = suggestedDirectGatewayIP(deviceDraft.value, nodeOptions.value)
  if (!suggested) return
  deviceDraft.value.ip_address = suggested
  gatewayIPAutofillPending.value = false
}
watch(
  [() => deviceDraft.value.is_gateway, () => deviceDraft.value.gateway_group_id,
    () => deviceDraft.value.network_key, () => deviceDraft.value.peer_id, nodeOptions],
  fillGatewayIPIfNeeded
)
watch(deviceDialogOpen, open => { if (!open) gatewayIPAutofillPending.value = false })
function emptyDevice(): DeviceInput {
  return {
    name: '', group_id: '', gateway_group_id: '', semantic_kinds: [],
    is_gateway: false, manufacturer: '', model: '', ip_address: '', mac_address: '',
    note: '', network_key: '', peer_id: ''
  }
}
const editingDevice = computed(() => devices.value.find(item => item.id === editingDeviceId.value))
const isNodeDeviceEditing = computed(() => editingDevice.value?.is_node_device === true)
const deviceDialogGatewayGroupsQuery = useQuery({
  queryKey: computed(() => ['device-dialog-gateway-groups', deviceDraft.value.network_key, deviceDraft.value.peer_id]),
  queryFn: () => deviceDraft.value.network_key && deviceDraft.value.peer_id
    ? getGatewayGroups(deviceDraft.value.network_key, deviceDraft.value.peer_id)
    : Promise.resolve([] as GatewayGroup[]),
  enabled: computed(() => deviceDialogOpen.value && !!deviceDraft.value.network_key && !!deviceDraft.value.peer_id),
  retry: 0
})
const deviceDialogGatewayGroupOptions = computed(() =>
  (deviceDialogGatewayGroupsQuery.data.value || []).filter(group => group.gateway_device_id !== editingDeviceId.value))
const deviceDraftNodeKey = computed({
  get: () => deviceDraft.value.network_key && deviceDraft.value.peer_id
    ? JSON.stringify([deviceDraft.value.network_key, deviceDraft.value.peer_id]) : '',
  set: (value: string) => {
    const node = nodeOptions.value.find(item => nodeValue(item) === value)
    const networkKey = node?.networkKey || ''
    const peerId = node?.peerId || ''
    if (networkKey !== deviceDraft.value.network_key || peerId !== deviceDraft.value.peer_id) {
      deviceDraft.value.gateway_group_id = ''
    }
    deviceDraft.value.network_key = networkKey
    deviceDraft.value.peer_id = peerId
  }
})
function openCreateDevice(gatewayDevice: Device) {
  gatewayIPAutofillPending.value = false
  if (!gatewayDevice.owned_gateway_group_id) {
    ElMessage.warning('该设备尚未配置网关层级')
    return
  }
  editingDeviceId.value = ''
  deviceDraft.value = {
    ...emptyDevice(),
    gateway_group_id: gatewayDevice.owned_gateway_group_id,
    network_key: gatewayDevice.network_key || '',
    peer_id: gatewayDevice.peer_id || ''
  }
  deviceDialogOpen.value = true
}
function openEditDevice(device: Device) {
  gatewayIPAutofillPending.value = !device.ip_address?.trim()
  editingDeviceId.value = device.id
  deviceDraft.value = {
    name: device.name, group_id: '', gateway_group_id: device.gateway_group_id || '',
    semantic_kinds: [...(device.semantic_kinds || [])], is_gateway: device.is_gateway,
    manufacturer: device.manufacturer || '', model: device.model || '',
    ip_address: device.ip_address || '', mac_address: device.mac_address || '',
    note: device.note || '',
    network_key: device.network_key || '', peer_id: device.peer_id || ''
  }
  deviceDialogOpen.value = true
  fillGatewayIPIfNeeded()
}
async function saveDeviceRecord() {
  if (savingDevice.value) return
  const previous = editingDevice.value
  const draft = { ...deviceDraft.value, group_id: '', semantic_kinds: [...deviceDraft.value.semantic_kinds] }
  if (previous?.is_node_device) {
    draft.network_key = previous.network_key || ''
    draft.peer_id = previous.peer_id || ''
    draft.gateway_group_id = ''
  }
  if (draft.is_gateway && (!draft.network_key || !draft.peer_id)) {
    ElMessage.warning('网关设备必须归属于一个网络节点')
    return
  }
  if (!draft.name.trim()) { ElMessage.warning('请填写设备名称'); return }
  const isOpenWrt = draft.semantic_kinds.includes('openwrt')
  if (isOpenWrt && !draft.ip_address.trim()) { ElMessage.warning('OpenWrt 设备需要填写一个管理 IP'); return }
  if (previous?.is_gateway && !draft.is_gateway) {
    const ownedGroup = gatewayGroups.value.find(group => group.gateway_device_id === previous.id)
    if (ownedGroup?.device_count) {
      try {
        await ElMessageBox.confirm(
          '取消网关属性后，该设备的下挂设备会提升为当前设备的同级设备；如果下挂设备本身也是网关，其下级关系会保留。是否继续？',
          '确认取消网关属性', { type: 'warning', confirmButtonText: '确认取消', cancelButtonText: '保留网关属性' })
      } catch { return }
    }
  }
  if (previous && isOpenWrtDevice(previous) && previous.openwrt_configured && !isOpenWrt) {
    try {
      await ElMessageBox.confirm('取消 OpenWrt 标识后，将同时清除该设备保存的 RPCD 发现服务配置。',
        '确认取消 OpenWrt 标识', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' })
    } catch { return }
  }
  savingDevice.value = true
  try {
    if (editingDeviceId.value) await updateDevice(editingDeviceId.value, draft)
    else await createDevice(draft)
    await queryClient.invalidateQueries({ queryKey: ['devices'] })
    await queryClient.invalidateQueries({ queryKey: ['gateway-groups'] })
    await queryClient.invalidateQueries({ queryKey: ['device-node-devices'] })
    const targetScope = draft.network_key && draft.peer_id
      ? JSON.stringify([draft.network_key, draft.peer_id]) : UNASSIGNED_SCOPE
    deviceDialogOpen.value = false
    if (targetScope !== selectedNodeKey.value) selectedNodeKey.value = targetScope
    ElMessage.success(editingDeviceId.value ? '设备资料已更新' : '设备已添加')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { savingDevice.value = false }
}

async function deleteSelectedDevices() {
  const ids = [...selectedDeviceIds.value]
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(
      '确定删除当前节点选中的 ' + ids.length + ' 台设备？只删除设备登记和各自的 OpenWrt 配置，不会删除 EasyTier 网络节点。',
      '批量删除设备', { type: 'warning', confirmButtonText: '删除所选设备', cancelButtonText: '取消' })
    const results = await Promise.allSettled(ids.map(id => deleteDevice(id)))
    const succeeded = ids.filter((_, index) => results[index].status === 'fulfilled')
    const failed = ids.filter((_, index) => results[index].status === 'rejected')
    selectedDeviceIds.value = failed
    await queryClient.invalidateQueries({ queryKey: ['devices'] })
    await queryClient.invalidateQueries({ queryKey: ['device-node-devices'] })
    await queryClient.invalidateQueries({ queryKey: ['gateway-groups'] })
    if (failed.length) ElMessage.warning('已删除 ' + succeeded.length + ' 台，失败 ' + failed.length + ' 台；失败项仍保留在选择中')
    else ElMessage.success('已删除 ' + succeeded.length + ' 台设备')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(formatApiError(error))
  }
}



const discoveryDialogOpen = ref(false)
const discoveryTarget = ref<Device | null>(null)
const discoveryKind = ref<OpenWrtDiscoveryKind>('devices')
function openDiscovery(device: Device, kind: OpenWrtDiscoveryKind) {
  if (!isOpenWrtDevice(device) || !device.is_gateway || !device.openwrt_configured) return
  discoveryTarget.value = device
  discoveryKind.value = kind
  discoveryDialogOpen.value = true
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

const initializingDevices = ref(false)
const canInitializeNetworkNodes = computed(() =>
  !initializingDevices.value && networkEntries.value.length > 0 &&
  nodesQuery.isSuccess.value && !nodesQuery.isError.value &&
  !attributesQuery.isPending.value && liveNodeOptions.value.length > 0
)
function truncateDeviceName(value: string, length: number): string {
  return Array.from(value).slice(0, length).join('')
}
function uniqueNodeDeviceName(node: LiveNodeOption, used: Set<string>): string {
  const raw = node.label && node.label.trim() && node.label !== '—' ? node.label.trim()
    : node.row.hostname && node.row.hostname !== '—' ? node.row.hostname
      : 'EasyTier 节点 ' + node.peerId.slice(0, 8)
  let name = truncateDeviceName(raw, 100)
  if (used.has(name.toLocaleLowerCase())) {
    const suffix = ' · ' + truncateDeviceName(node.instance || '网络节点', 24)
    name = truncateDeviceName(raw, 100 - Array.from(suffix).length) + suffix
  }
  let sequence = 2
  while (used.has(name.toLocaleLowerCase())) {
    const suffix = ' (' + sequence + ')'
    name = truncateDeviceName(raw, 100 - Array.from(suffix).length) + suffix
    sequence++
  }
  used.add(name.toLocaleLowerCase())
  return name
}
function networkNodeDeviceInputs(): NetworkNodeDeviceInput[] {
  const used = new Set((nodeDevicesQuery.data.value || []).map(device => device.name.toLocaleLowerCase()))
  return liveNodeOptions.value.map(node => ({
    name: uniqueNodeDeviceName(node, used),
    network_key: node.networkKey,
    peer_id: node.peerId
  }))
}
async function runNetworkNodeSync(showMessage: boolean) {
  const inputs = networkNodeDeviceInputs()
  if (!inputs.length) {
    if (showMessage) ElMessage.warning('没有可同步的有效网络节点')
    return
  }
  initializingDevices.value = true
  try {
    const result = await syncNetworkNodeDevices(inputs, showMessage)
    queryClient.setQueryData(['device-network-node-sync'], { enabled: result.enabled })
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-node-devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-network-node-sync'] })
    ])
    if (showMessage) {
      ElMessage.success('网络节点同步完成：新增 ' + result.created + ' 台，已存在 ' + result.existing + ' 台，跳过已删除 ' + result.skipped + ' 台；新节点自动补充已开启')
    }
  } catch (error) {
    if (showMessage) ElMessage.error(formatApiError(error))
  } finally {
    initializingDevices.value = false
  }
}
async function initializeNetworkNodes() {
  if (!canInitializeNetworkNodes.value) return
  const inputs = networkNodeDeviceInputs()
  if (!inputs.length) { ElMessage.warning('没有可初始化的有效网络节点'); return }
  try {
    await ElMessageBox.confirm(
      '将为当前 ' + inputs.length + ' 个网络节点创建基础设备记录，并标记为「节点设备」。后续发现的新节点会自动补充；不会覆盖已有设备资料，也不会删除离线节点或复制节点实时 IP、路由和位置。是否继续？',
      '初始化网络节点', { type: 'info', confirmButtonText: '开始初始化', cancelButtonText: '取消' })
  } catch { return }
  await runNetworkNodeSync(true)
}
const isRefreshing = computed(() =>
  devicesQuery.isFetching.value || nodesQuery.isFetching.value || gatewayGroupsQuery.isFetching.value)
async function refreshAll() {
  await Promise.all([
    devicesQuery.refetch(), nodesQuery.refetch(), nodeDevicesQuery.refetch(),
    attributesQuery.refetch(), networkNodeSyncQuery.refetch(), gatewayGroupsQuery.refetch()
  ])
}
watch(
  () => [networkNodeSyncEnabled.value, nodesQuery.dataUpdatedAt.value] as const,
  async ([enabled]) => {
    if (!enabled || initializingDevices.value || !nodesQuery.isSuccess.value || nodesQuery.isError.value) return
    if (!liveNodeOptions.value.length) return
    await runNetworkNodeSync(false)
  },
  { immediate: true }
)
</script>

<template>
  <section class="devices-pane" aria-label="设备列表">
    <div class="devices-intro">
      <div>
        <h2>设备列表</h2>
        <p>先选择 EasyTier 网络节点，再管理节点直属设备及各网关设备的下挂设备。</p>
      </div>
      <div class="devices-actions">
        <el-button @click="refreshAll" :loading="isRefreshing"><el-icon><Refresh /></el-icon>刷新</el-button>
        <el-button v-if="devices.length && networkEntries.length && !networkNodeSyncEnabled"
          :disabled="!canInitializeNetworkNodes" :loading="initializingDevices" @click="initializeNetworkNodes">
          <el-icon><Connection /></el-icon>同步网络节点
        </el-button>
        <el-tag v-if="networkNodeSyncEnabled" type="success" effect="plain">新节点自动补充已开启</el-tag>
      </div>
    </div>

    <div class="node-scope-toolbar">
      <div class="node-scope-label">所处节点</div>
      <el-select v-model="selectedNodeKey" filterable class="node-scope-select" placeholder="选择网络节点">
        <el-option label="未关联网络节点（旧设备）" :value="UNASSIGNED_SCOPE" />
        <el-option v-for="node in nodeOptions" :key="node.networkKey + '|' + node.peerId"
          :label="node.label + ' · ' + (node.isOnline ? '在线' : '离线')" :value="nodeValue(node)">
          <div class="node-option">
            <span>{{ node.label }}</span>
            <el-tag size="small" :type="node.isOnline ? 'success' : 'info'">{{ node.isOnline ? '在线' : '离线' }}</el-tag>
            <span class="node-option-meta">{{ node.ip || node.peerId }}</span>
          </div>
        </el-option>
      </el-select>
      <div v-if="selectedNode" class="node-scope-meta">
        <el-tag :type="selectedNode.isOnline ? 'success' : 'info'" effect="plain">
          {{ selectedNode.isOnline ? '当前在线' : '历史节点' }}
        </el-tag>
        <span>{{ selectedNode.instance || '历史节点信息' }}</span>
        <span>{{ selectedNode.ip || selectedNode.peerId }}</span>
      </div>
      <div v-else class="node-scope-meta">用于查看未关联网络节点的旧设备记录</div>
    </div>

    <el-alert v-if="devicesQuery.isError.value"
      :title="formatApiError(devicesQuery.error.value)" type="error" :closable="false" show-icon />
    <el-alert v-if="networkEntries.length && nodesQuery.isError.value"
      title="实时网络节点暂时无法读取；已登记的历史节点和设备仍可管理。" type="warning" :closable="false" show-icon />
    <el-alert v-else-if="nodeErrors" :title="nodeErrors + ' 个组网入口暂时不可用，已显示其他入口可读取的节点。'"
      type="warning" :closable="false" show-icon />
    <el-alert v-if="!networkEntries.length && !nodeOptions.length"
      title="尚未设置组网入口；可以先管理未关联节点的旧设备，设置入口后即可选择网络节点。" type="info" :closable="false" />

    <div class="devices-toolbar">
      <el-input v-model="search" clearable placeholder="搜索设备名称 / IP / MAC / 型号" aria-label="搜索设备">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <div class="devices-summary">
        <span>{{ deviceAssetCount }} 台设备</span>
        <span>{{ gatewayCount }} 个网关设备</span>
        <el-checkbox :model-value="allFilteredSelected" @update:model-value="toggleSelectAll">全选当前结果</el-checkbox>
        <el-button v-if="selectedDeviceIds.length" type="danger" plain size="small" @click="deleteSelectedDevices">
          <el-icon><Delete /></el-icon>删除所选 {{ selectedDeviceIds.length }} 台
        </el-button>
      </div>
    </div>

    <el-alert v-if="gatewayGroupsQuery.isError.value"
      :title="formatApiError(gatewayGroupsQuery.error.value)" type="error" :closable="false" show-icon />

    <div v-if="devicesQuery.isPending.value" class="devices-loading">
      <el-skeleton :rows="5" animated />
    </div>
    <template v-else>
      <el-empty v-if="!devices.length && !gatewayGroups.length" description="当前节点还没有登记设备" :image-size="120">
        <div class="empty-initialize">
          <p v-if="selectedNode && !nodeDevices.length">该节点尚未生成基础节点设备记录，可以同步网络节点后继续。</p>
          <p v-else-if="isUnassignedScope">这里保留尚未关联网络节点的旧设备记录。</p>
          <p v-else>先编辑当前节点的基础设备，将其设置为网关设备，再从该设备卡片新增下挂设备。</p>
          <el-button v-if="selectedNode && !nodeDevices.length && networkEntries.length"
            :loading="initializingDevices" :disabled="!canInitializeNetworkNodes" @click="initializeNetworkNodes">
            <el-icon><Connection /></el-icon>初始化 / 同步网络节点
          </el-button>
        </div>
      </el-empty>

      <div v-for="section in deviceSections" :key="section.id" class="device-group">
        <div class="group-heading">
          <div v-if="section.kind === 'gateway' && section.gatewayDevice" class="gateway-heading-main">
            <el-checkbox class="gateway-heading-select"
              :model-value="selectedDeviceIds.includes(section.gatewayDevice.id)"
              :disabled="section.gatewayDevice.is_node_device"
              :aria-label="'选择设备 ' + section.gatewayDevice.name"
              @update:model-value="toggleDeviceSelection(section.gatewayDevice.id, $event)" />
            <button class="group-toggle gateway-group-toggle" type="button"
              :aria-expanded="!collapsed.includes(section.id)" @click="toggleGroup(section.id)">
              <span class="group-chevron" :class="{ collapsed: collapsed.includes(section.id) }">⌄</span>
              <span class="device-avatar gateway-heading-avatar">
                <el-icon><component :is="deviceIcon(section.gatewayDevice)" /></el-icon>
              </span>
              <span class="gateway-heading-info">
                <span class="gateway-heading-title">
                  <strong>{{ section.gatewayDevice.name }} 下挂设备</strong>
                  <span v-if="[section.gatewayDevice.manufacturer, section.gatewayDevice.model].filter(Boolean).length">
                    {{ [section.gatewayDevice.manufacturer, section.gatewayDevice.model].filter(Boolean).join(' · ') }}
                  </span>
                  <span v-if="section.gatewayDevice.ip_address">{{ section.gatewayDevice.ip_address }}</span>
                  <span v-if="section.gatewayDevice.mac_address">MAC · {{ section.gatewayDevice.mac_address }}</span>
                  <span v-if="section.gatewayDevice.discovery_status === 'online'" class="gateway-discovery-state online">在线</span>
                  <span v-else-if="section.gatewayDevice.discovery_status === 'offline'" class="gateway-discovery-state offline">离线</span>
                </span>
                <span v-if="isOpenWrtDevice(section.gatewayDevice)" class="gateway-heading-rpcd">
                  <el-icon><Connection /></el-icon>
                  {{ section.gatewayDevice.openwrt_configured ? 'RPCD 账号已配置' : 'RPCD 账号尚未配置' }}
                  <span v-if="section.gatewayDevice.openwrt_configured">
                    {{ section.gatewayDevice.openwrt_https ? 'HTTPS' : 'HTTP' }} · {{ section.gatewayDevice.openwrt_rpcd_port || 80 }}
                  </span>
                  <span v-if="section.gatewayDevice.discovery_last_seen_at">
                    最近观测 {{ new Date(section.gatewayDevice.discovery_last_seen_at).toLocaleString('zh-CN') }}
                  </span>
                </span>
              </span>
              <span class="group-count">{{ section.items.length }}</span>
            </button>
          </div>
          <button v-else class="group-toggle" type="button" :aria-expanded="!collapsed.includes(section.id)"
            @click="toggleGroup(section.id)">
            <span class="group-chevron" :class="{ collapsed: collapsed.includes(section.id) }">⌄</span>
            <strong>{{ section.name }}</strong>
            <span class="group-count">{{ section.items.length }}</span>
          </button>
          <div v-if="section.kind === 'gateway' && section.gatewayDevice" class="gateway-heading-actions">
            <el-button text circle aria-label="新增下挂设备" title="新增下挂设备"
              @click="openCreateDevice(section.gatewayDevice)"><el-icon><Plus /></el-icon></el-button>
            <el-button text circle aria-label="编辑网关设备" title="编辑网关设备"
              @click="openEditDevice(section.gatewayDevice)"><el-icon><Edit /></el-icon></el-button>
            <el-button v-if="isOpenWrtDevice(section.gatewayDevice)" text circle
              aria-label="OpenWrt RPCD 配置" title="OpenWrt RPCD 配置"
              @click="openOpenWrtSettings(section.gatewayDevice)"><el-icon><Setting /></el-icon></el-button>
            <el-button v-if="isOpenWrtDevice(section.gatewayDevice) && section.gatewayDevice.openwrt_configured"
              text circle aria-label="OpenWrt 服务发现" title="OpenWrt 服务发现"
              @click="openDiscovery(section.gatewayDevice, 'devices')"><el-icon><Search /></el-icon></el-button>
            <el-button v-if="isOpenWrtDevice(section.gatewayDevice) && section.gatewayDevice.openwrt_configured"
              text circle aria-label="ADB 发现" title="ADB 发现"
              @click="openDiscovery(section.gatewayDevice, 'adb')"><el-icon><Iphone /></el-icon></el-button>
          </div>
        </div>
        <div v-if="!collapsed.includes(section.id)" class="device-grid">
          <article v-for="device in section.items" :key="device.id" class="device-card">
            <div class="device-card-head">
              <el-checkbox class="device-select" :model-value="selectedDeviceIds.includes(device.id)"
                :disabled="device.is_node_device"
                :aria-label="'选择设备 ' + device.name"
                @update:model-value="toggleDeviceSelection(device.id, $event)" />
              <div class="device-avatar">
                <el-icon><component :is="deviceIcon(device)" /></el-icon>
              </div>
              <div class="device-main">
                <div class="device-title">
                  <strong>{{ device.name }}</strong>
                  <el-tag v-for="kind in device.semantic_kinds" :key="kind" size="small" effect="plain">
                    {{ semanticKindLabels[kind] || kind }}
                  </el-tag>
                  <el-tag v-if="device.is_node_device" size="small" type="info" effect="dark">节点设备</el-tag>
                  <el-tag v-if="device.is_gateway" size="small" type="success" effect="dark">网关设备</el-tag>
                  <el-tag v-if="device.gateway_group_id" size="small" type="info" effect="plain">下挂设备</el-tag>
                  <el-tag v-if="device.discovery_status === 'online'" size="small" type="success" effect="dark">在线</el-tag>
                  <el-tag v-else-if="device.discovery_status === 'offline'" size="small" type="info" effect="plain">离线</el-tag>
                </div>
                <div class="device-subtitle">
                  <span>{{ [device.manufacturer, device.model].filter(Boolean).join(' · ') || '未填写厂商与型号' }}</span>
                  <span v-if="device.ip_address">{{ device.ip_address }}</span>
                </div>
              </div>
              <div class="device-actions">
                <el-button v-if="device.is_gateway && device.owned_gateway_group_id" size="small" type="primary" plain
                  @click="openCreateDevice(device)"><el-icon><Plus /></el-icon>新增设备</el-button>
                <el-button text circle aria-label="编辑设备" title="编辑设备" @click="openEditDevice(device)"><el-icon><Edit /></el-icon></el-button>
                <el-button v-if="isOpenWrtDevice(device)" text circle aria-label="OpenWrt RPCD 配置" title="OpenWrt RPCD 配置"
                  @click="openOpenWrtSettings(device)"><el-icon><Setting /></el-icon></el-button>
              </div>
            </div>
            <div class="device-meta">
              <span v-if="device.mac_address">MAC：{{ device.mac_address }}</span>
              <span v-if="device.gateway_group_name" class="node-association"><el-icon><Link /></el-icon>上级网关：{{ device.gateway_group_name }}</span>
              <span v-if="device.is_node_device" class="node-association"><el-icon><Link /></el-icon>当前网络节点基础设备</span>
              <span v-if="device.discovery_last_seen_at" class="muted">
                最近观测：{{ new Date(device.discovery_last_seen_at).toLocaleString('zh-CN') }}
              </span>
            </div>
            <p v-if="device.note" class="device-note">{{ device.note }}</p>
            <div v-if="isOpenWrtDevice(device)" class="openwrt-status">
              <el-icon><Connection /></el-icon>
              <span>{{ device.openwrt_configured ? 'RPCD 账号已配置' : 'RPCD 账号尚未配置' }}</span>
              <span v-if="device.openwrt_configured" class="openwrt-endpoint">
                {{ device.openwrt_https ? 'HTTPS' : 'HTTP' }} · {{ device.openwrt_rpcd_port || 80 }}
              </span>
            </div>
            <div v-if="device.is_gateway && isOpenWrtDevice(device) && device.openwrt_configured"
              class="discovery-actions">
              <el-button size="small" plain @click="openDiscovery(device, 'devices')">服务发现</el-button>
              <el-button size="small" plain @click="openDiscovery(device, 'adb')">ADB 发现</el-button>
            </div>
          </article>
          <el-empty v-if="!section.items.length"
            :description="section.kind === 'gateway'
              ? (search ? '没有匹配的下挂设备' : '该网关设备还没有下挂设备')
              : '暂无设备'" :image-size="54" />
        </div>
      </div>
      <el-empty v-if="devices.length && !deviceSections.length && search" description="没有匹配的设备" />
    </template>

    <el-dialog v-model="deviceDialogOpen" :title="editingDeviceId ? '编辑设备' : '新增设备'" width="min(620px, 94vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="saveDeviceRecord">
        <div class="form-grid">
          <el-form-item label="设备名称" required>
            <el-input v-model="deviceDraft.name" maxlength="100" placeholder="例如：客厅电视" />
          </el-form-item>
          <el-form-item label="所属网络节点" class="span-two">
            <el-select v-model="deviceDraftNodeKey" clearable filterable class="full"
              :disabled="isNodeDeviceEditing || (!editingDeviceId && !!deviceDraft.gateway_group_id)" placeholder="选择设备所属的网络节点">
              <el-option v-for="node in nodeOptions" :key="node.networkKey + '|' + node.peerId"
                :label="node.label + ' · ' + (node.isOnline ? '在线' : '离线')" :value="nodeValue(node)">
                <div class="node-option">
                  <span>{{ node.label }}</span>
                  <el-tag size="small" :type="node.isOnline ? 'success' : 'info'">{{ node.isOnline ? '在线' : '离线' }}</el-tag>
                  <span class="node-option-meta">{{ node.ip || node.peerId }}</span>
                </div>
              </el-option>
            </el-select>
            <div class="field-hint">普通设备可以在编辑时迁移到其他网络节点；节点基础设备固定归属于其自身节点。通过网关设备卡片新增的设备会自动归属该网关。</div>
          </el-form-item>
          <el-form-item label="所属上级网关" class="span-two">
            <el-select v-model="deviceDraft.gateway_group_id" clearable filterable class="full"
              :disabled="isNodeDeviceEditing" placeholder="直属于当前网络节点">
              <el-option v-for="group in deviceDialogGatewayGroupOptions" :key="group.id"
                :label="(group.gateway_device_name || group.name) + '（网关设备）'" :value="group.id" />
            </el-select>
            <div class="field-hint">不选择上级网关时，设备直接归属于网络节点；也可以将普通设备或另一台网关设备挂到现有网关下面。</div>
          </el-form-item>
          <el-form-item label="设备角色" class="span-two">
            <el-checkbox v-model="deviceDraft.is_gateway" :disabled="!deviceDraft.network_key || !deviceDraft.peer_id">
              这是网关设备
            </el-checkbox>
            <div class="field-hint">任何已归属网络节点的设备都可以独立设置网关属性。网关设备卡片会提供「新增设备」入口；关闭网关属性后，其下挂设备会提升为同级设备。</div>
          </el-form-item>
          <el-form-item label="设备语义标签（可多选）" class="span-two">
            <el-select v-model="deviceDraft.semantic_kinds" multiple collapse-tags collapse-tags-tooltip
              clearable filterable class="full" placeholder="选择设备具备的类型/能力">
              <el-option v-for="item in semanticKindOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
            <div class="field-hint">语义标签直接描述设备类型与能力，可多选；选择「OpenWrt」后启用该设备的 RPCD 配置入口，后续发现功能还要求该设备同时标记为网关设备。</div>
          </el-form-item>
          <el-form-item label="厂商">
            <el-input v-model="deviceDraft.manufacturer" maxlength="100" placeholder="例如：TP-Link" />
          </el-form-item>
          <el-form-item label="型号">
            <el-input v-model="deviceDraft.model" maxlength="120" placeholder="例如：XDR6088" />
          </el-form-item>
          <el-form-item label="设备管理 IP" :required="isOpenWrtDraft">
            <el-input v-model="deviceDraft.ip_address" placeholder="例如：192.168.1.1"
              @input="gatewayIPAutofillPending = false" />
            <div v-if="editingDeviceId && deviceDraft.is_gateway && !deviceDraft.gateway_group_id" class="field-hint">
              直属网关设备的管理 IP 为空时，将从所属网络节点代理网段自动建议首个可用 IP；不覆盖已有值，可手动修改。
            </div>
          </el-form-item>
          <el-form-item label="MAC 地址">
            <el-input v-model="deviceDraft.mac_address" placeholder="例如：AA:BB:CC:DD:EE:FF" />
          </el-form-item>
          <el-form-item label="设备备注" class="span-two">
            <el-input v-model="deviceDraft.note" type="textarea" :rows="2" maxlength="1000" show-word-limit />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="deviceDialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="savingDevice" @click="saveDeviceRecord">保存设备</el-button>
      </template>
    </el-dialog>

    <OpenWrtDiscoveryDialog v-if="discoveryTarget"
      :key="discoveryTarget.id + ':' + discoveryKind"
      v-model="discoveryDialogOpen" :device="discoveryTarget" :kind="discoveryKind" />

    <el-dialog v-model="openWrtDialogOpen" title="OpenWrt RPCD 配置" width="min(520px, 92vw)" destroy-on-close>
      <template v-if="openWrtTarget">
        <el-alert type="info" :closable="false" show-icon
          title="此配置属于设备「{{ openWrtTarget.name }}」，不会挂在网关组上。RPCD 密钥在服务端加密保存，不会回传。" />
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
.group-toggle { display:flex; min-width:0; align-items:center; gap:10px; padding:0; border:0; background:transparent; color:var(--ent-text); text-align:left; cursor:pointer; }
.group-toggle strong { font-size:14px; }
.gateway-heading-main { display:flex; min-width:0; flex:1; align-items:center; gap:9px; }
.gateway-heading-select { flex:none; }
.gateway-group-toggle { min-width:0; flex:1; }
.gateway-heading-avatar { width:34px; height:34px; border-radius:10px; font-size:16px; }
.gateway-heading-info { display:flex; min-width:0; flex:1; flex-direction:column; gap:4px; }
.gateway-heading-title { display:flex; min-width:0; align-items:center; flex-wrap:wrap; gap:4px 11px; color:var(--ent-muted); font-size:11px; }
.gateway-heading-title strong { color:var(--ent-text); font-size:14px; }
.gateway-discovery-state { display:inline-flex; align-items:center; gap:4px; }
.gateway-discovery-state::before { width:6px; height:6px; border-radius:50%; background:currentColor; content:""; }
.gateway-discovery-state.online { color:var(--el-color-success); }
.gateway-discovery-state.offline { color:var(--ent-dim); }
.gateway-heading-rpcd { display:flex; align-items:center; flex-wrap:wrap; gap:5px; color:var(--ent-data-highlight); font-size:10px; }
.gateway-heading-rpcd > span { color:var(--ent-muted); }
.gateway-heading-actions { display:flex; flex:none; align-items:center; gap:1px; }
.gateway-heading-actions :deep(.el-button) { margin-left:0; color:var(--ent-muted); }
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
.discovery-actions { display:flex; flex-wrap:wrap; gap:6px; margin-top:9px; }
.discovery-actions :deep(.el-button + .el-button) { margin-left:0; }
.devices-loading { padding:18px; }
.empty-initialize { display:flex; align-items:center; flex-direction:column; gap:10px; }
.empty-initialize p { margin:0; color:var(--ent-muted); font-size:12px; text-align:center; }
.initialization-failures { display:flex; flex-direction:column; align-items:flex-start; gap:6px; }
.initialization-failures span { overflow-wrap:anywhere; }
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
  .group-heading { align-items:flex-start; flex-wrap:wrap; }
  .gateway-heading-main { width:100%; }
  .gateway-heading-actions { margin-left:auto; }
  .form-grid { grid-template-columns:1fr; }
  .span-two { grid-column:auto; }
  .device-actions { gap:0; }
}

.node-scope-toolbar { display:flex; align-items:center; flex-wrap:wrap; gap:12px; padding:13px 15px; border:1px solid var(--ent-line); border-radius:12px; background:var(--ent-panel); }
.node-scope-label { flex:none; color:var(--ent-muted); font-size:12px; font-weight:700; }
.node-scope-select { width:min(420px,100%); }
.node-scope-meta { display:flex; align-items:center; flex-wrap:wrap; gap:9px; color:var(--ent-muted); font-size:11px; }
.node-option { display:flex; align-items:center; gap:9px; min-width:0; }
.node-option-meta { overflow:hidden; margin-left:auto; color:var(--ent-dim); font-size:11px; text-overflow:ellipsis; }
.gateway-group-summary { display:flex; align-items:center; flex-wrap:wrap; gap:12px; padding:10px 15px; border-bottom:1px solid var(--ent-line-soft); color:var(--ent-muted); font-size:11px; background:var(--ent-panel); }
@media(max-width:760px) {
  .node-scope-toolbar { align-items:stretch; flex-direction:column; }
  .node-scope-select { width:100%; }
  .node-scope-meta { align-items:flex-start; }
  .devices-summary { align-items:flex-start; flex-wrap:wrap; white-space:normal; }
  .group-actions { flex-wrap:wrap; justify-content:flex-end; }
}
</style>
