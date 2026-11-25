<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox, type ButtonInstance } from 'element-plus'
import { Refresh, Search, Delete, Connection, Monitor, Iphone, Box, Link } from '@element-plus/icons-vue'
import { formatApiError } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import type { NetworkRow } from '../network-nodes/data'
import DeviceActions from './DeviceActions.vue'
import DeviceServicesDialog from './DeviceServicesDialog.vue'
import OpenWrtDiscoveryDialog from './OpenWrtDiscoveryDialog.vue'
import { suggestedDirectGatewayIP } from './gateway-ip'
import { nodeValue, useDeviceNetwork, type LiveNodeOption } from './useDeviceNetwork'
import {
  createDevice, deleteDevice, getDevices, getGatewayGroups,
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
const ALL_SCOPE = '__all__'

type NodeOption = {
  networkKey: string
  peerId: string
  hostname: string
  identityKey?: string
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
const {
  networkEntries, nodesQuery, nodeDevicesQuery, attributesQuery, attributesByKey, liveNodeOptions,
  conflictingNodeNames, deviceNetworkIP, serviceDeviceNetworkIP, serviceEntryPublicIPs
} = useDeviceNetwork(() => props.hosts)
// 已登记的历史节点可能在旧版本没有写入 node_hostname，优先显示可核对的历史属性。
function originalNodeHostname(node: NodeOption): string {
  return node.hostname || attributesByKey.value.get(node.networkKey + '|' + node.peerId)?.hostname || ''
}
const nodeOptions = computed<NodeOption[]>(() => {
  const options = liveNodeOptions.value.filter(node => nodeValue(node) && !conflictingNodeNames.value.has(nodeValue(node)))
  const result: NodeOption[] = [...options]
  const seen = new Set(result.map(nodeValue))
  for (const device of nodeDevicesQuery.data.value || []) {
    const option: NodeOption = {
      networkKey: device.network_key || '', peerId: device.peer_id || '',
      hostname: device.node_hostname || '', label: device.name, instance: '', ip: '', isOnline: false
    }
    const key = nodeValue(option)
    if (!key || seen.has(key) || conflictingNodeNames.value.has(key)) continue
    seen.add(key)
    result.push(option)
  }
  return result
})
const selectedNodeKey = ref(typeof route.query.node === 'string' && route.query.node !== '__unassigned__' ? route.query.node : ALL_SCOPE)
const selectedNode = computed(() => nodeOptions.value.find(node => nodeValue(node) === selectedNodeKey.value))
const isAllScope = computed(() => selectedNodeKey.value === ALL_SCOPE)
watch(
  () => [nodeOptions.value, nodesQuery.isFetched.value, nodeDevicesQuery.isFetched.value] as const,
  ([items, liveFetched, storedFetched]) => {
    if ((networkEntries.value.length > 0 && !liveFetched) || !storedFetched) return
    if (selectedNodeKey.value === ALL_SCOPE || items.some(node => nodeValue(node) === selectedNodeKey.value)) return
    // 旧 URL 使用 [network_key,peer_id]；打开后升级为基于主机名的稳定选择键。
    try {
      const legacy = JSON.parse(selectedNodeKey.value)
      if (Array.isArray(legacy) && (legacy.length === 2 || legacy.length === 3)) {
        const matched = items.find(node => node.networkKey === legacy[0] &&
          (legacy[1] === 'hostname' ? node.hostname.toLowerCase() === legacy[2] : node.peerId === legacy.at(-1)))
        if (matched) { selectedNodeKey.value = nodeValue(matched); return }
      }
    } catch { /* 非旧格式，使用默认筛选 */ }
    selectedNodeKey.value = ALL_SCOPE
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
  selectedNodeKey.value = typeof value === 'string' && value !== '__unassigned__' ? value : ALL_SCOPE
})

const devicesQuery = useQuery({
  queryKey: computed(() => ['devices', selectedNodeKey.value]),
  queryFn: () => isAllScope.value
    ? getDevices()
    : selectedNode.value
      ? getDevices({ hostname: selectedNode.value.hostname })
      : Promise.resolve([] as Device[]),
  enabled: computed(() => isAllScope.value || !!selectedNode.value),
  retry: 0
})
const devices = computed(() => devicesQuery.data.value || [])
const gatewayGroupsQuery = useQuery({
  queryKey: computed(() => ['gateway-groups', selectedNodeKey.value]),
  queryFn: () => getGatewayGroups(selectedNode.value?.hostname),
  enabled: computed(() => isAllScope.value || !!selectedNode.value),
  retry: 0
})
const gatewayGroups = computed(() => gatewayGroupsQuery.data.value || [])
const nodeErrors = computed(() => nodesQuery.data.value?.errors || 0)
const nodeConflicts = computed(() => conflictingNodeNames.value.size)
const search = ref('')
const selectedDeviceIds = ref<string[]>([])
watch([search, selectedNodeKey], () => { selectedDeviceIds.value = [] })
const filteredDevices = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  if (!keyword) return devices.value
  return devices.value.filter(device => [
    device.name, device.group_name, device.gateway_group_name,
    device.manufacturer, device.model, device.ip_address, device.network_ip, device.mac_address,
    device.note, device.peer_id, device.network_key, ...(device.semantic_kinds || [])
  ].some(value => (value || '').toLocaleLowerCase().includes(keyword)))
})
const matchedIds = computed(() => new Set(filteredDevices.value.map(device => device.id)))
const nodeDevices = computed(() => filteredDevices.value.filter(device => device.is_node_device))
const discoveryStatusOrder: Record<Device['discovery_status'], number> = { online: 0, offline: 1, unknown: 2 }
const gatewayGroupSections = computed(() => gatewayGroups.value
  .filter(group => !!group.gateway_device_id)
  .map(group => {
    const items = filteredDevices.value
      .filter(device => device.gateway_group_id === group.id)
      .sort((a, b) => (discoveryStatusOrder[a.discovery_status] ?? discoveryStatusOrder.unknown) -
        (discoveryStatusOrder[b.discovery_status] ?? discoveryStatusOrder.unknown))
    const gatewayDevice = devices.value.find(device => device.id === group.gateway_device_id)
    const gatewayMatchesSearch = !!gatewayDevice && matchedIds.value.has(gatewayDevice.id)
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
  for (const device of directDevices.value) {
    result.push({ id: 'direct-' + device.id, name: device.name, kind: 'direct', items: [], gatewayDevice: device })
  }
  for (const group of gatewayGroupSections.value) {
    result.push({
      id: group.id,
      name: group.gatewayDevice?.name || group.name,
      kind: 'gateway',
      items: group.items,
      gatewayGroup: group,
      gatewayDevice: group.gatewayDevice
    })
  }
  return result
})

const collapsedGroups = ref<string[]>([])
const collapsedSearchGroups = ref<string[]>([])
const collapsed = computed({
  get: () => search.value.trim() ? collapsedSearchGroups.value : collapsedGroups.value,
  set: (ids: string[]) => {
    if (search.value.trim()) collapsedSearchGroups.value = ids
    else collapsedGroups.value = ids
  }
})
watch(search, () => { collapsedSearchGroups.value = [] })
watch(selectedNodeKey, () => {
  collapsedGroups.value = []
  collapsedSearchGroups.value = []
})
function toggleGroup(id: string) {
  collapsed.value = collapsed.value.includes(id)
    ? collapsed.value.filter(value => value !== id) : [...collapsed.value, id]
}

// 上级网关可用于说明归属，但只有当前显示且命中筛选的设备才能参与批量操作。
const selectableIds = computed(() => {
  const ids = new Set<string>()
  for (const section of deviceSections.value) {
    if (section.gatewayDevice && matchedIds.value.has(section.gatewayDevice.id)) ids.add(section.gatewayDevice.id)
    if (!collapsed.value.includes(section.id)) {
      for (const device of section.items) ids.add(device.id)
    }
  }
  return ids
})
watch(selectableIds, ids => {
  selectedDeviceIds.value = selectedDeviceIds.value.filter(id => ids.has(id))
})
const selectedDevices = computed(() => {
  const selectedIds = new Set(selectedDeviceIds.value)
  return filteredDevices.value.filter(device => selectableIds.value.has(device.id) && selectedIds.has(device.id))
})
const allFilteredSelected = computed(() => selectableIds.value.size > 0 &&
  selectedDevices.value.length === selectableIds.value.size)
function toggleDeviceSelection(id: string, checked: boolean) {
  if (!selectableIds.value.has(id)) return
  selectedDeviceIds.value = checked
    ? [...new Set([...selectedDeviceIds.value, id])]
    : selectedDeviceIds.value.filter(value => value !== id)
}
function toggleSelectAll(checked: boolean) {
  selectedDeviceIds.value = checked ? [...selectableIds.value] : []
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
const gatewayCount = computed(() => filteredDevices.value.filter(device => device.is_gateway && selectableIds.value.has(device.id)).length)
const networkNodeSyncEnabled = computed(() => networkNodeSyncQuery.data.value?.enabled === true)

const serviceDevice = ref<Device | null>(null)
const servicesOpen = ref(false)
function openDeviceServices(device: Device) {
  serviceDevice.value = device
  servicesOpen.value = true
}

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
    () => deviceDraft.value.node_hostname, nodeOptions],
  fillGatewayIPIfNeeded
)
watch(deviceDialogOpen, open => { if (!open) gatewayIPAutofillPending.value = false })
function emptyDevice(): DeviceInput {
  return {
    name: '', group_id: '', gateway_group_id: '', semantic_kinds: [],
    is_gateway: false, manufacturer: '', model: '', ip_address: '', mac_address: '',
    note: '', node_hostname: ''
  }
}
const editingDevice = computed(() => devices.value.find(item => item.id === editingDeviceId.value))
const isNodeDeviceEditing = computed(() => editingDevice.value?.is_node_device === true)
const deviceDialogGatewayGroupsQuery = useQuery({
  queryKey: computed(() => ['device-dialog-gateway-groups', deviceDraft.value.node_hostname]),
  queryFn: () => deviceDraft.value.node_hostname
    ? getGatewayGroups(deviceDraft.value.node_hostname)
    : Promise.resolve([] as GatewayGroup[]),
  enabled: computed(() => deviceDialogOpen.value && !!deviceDraft.value.node_hostname),
  retry: 0
})
const deviceDialogGatewayGroupOptions = computed(() =>
  (deviceDialogGatewayGroupsQuery.data.value || []).filter(group => group.gateway_device_id !== editingDeviceId.value))
const deviceDraftNodeKey = computed({
  get: () => nodeValue({ hostname: deviceDraft.value.node_hostname || '' }),
  set: (value: string) => {
    const node = nodeOptions.value.find(item => nodeValue(item) === value)
    if (value !== deviceDraftNodeKey.value) deviceDraft.value.gateway_group_id = ''
    deviceDraft.value.node_hostname = node?.hostname || ''
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
    node_hostname: gatewayDevice.node_hostname || ''
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
    node_hostname: device.node_hostname || ''
  }
  deviceDialogOpen.value = true
  fillGatewayIPIfNeeded()
}
async function saveDeviceRecord() {
  if (savingDevice.value) return
  const previous = editingDevice.value
  const draft = { ...deviceDraft.value, group_id: '', semantic_kinds: [...deviceDraft.value.semantic_kinds] }
  if (previous?.is_node_device) {
    // 基础设备可以改绑到新的所属网络节点，但必须保持节点直属层级。
    draft.gateway_group_id = ''
    if (!draft.node_hostname) {
      ElMessage.warning('请为节点基础设备选择包含原始主机名的所属网络节点')
      return
    }
  }
  if (draft.is_gateway && !draft.node_hostname) {
    ElMessage.warning('网关设备必须归属于一个网络节点')
    return
  }
  if (draft.node_hostname && conflictingNodeNames.value.has(nodeValue({ hostname: draft.node_hostname }))) {
    ElMessage.warning('原始主机名有冲突，请为节点改名或选择其他节点')
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
    const targetScope = draft.node_hostname ? nodeValue({ hostname: draft.node_hostname }) : ALL_SCOPE
    deviceDialogOpen.value = false
    if (!isAllScope.value && targetScope !== selectedNodeKey.value) selectedNodeKey.value = targetScope
    ElMessage.success(editingDeviceId.value ? '设备资料已更新' : '设备已添加')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { savingDevice.value = false }
}

async function deleteNodeBaseDevice(device: Device) {
  if (!device.is_node_device) return
  try {
    await ElMessageBox.confirm(
      '确定删除节点基础设备「' + device.name + '」吗？该设备的服务登记也会删除。不会删除实际 EasyTier 节点或同节点其他设备。' +
      (device.is_gateway ? '该设备下的设备会保留，并提升为当前节点直属设备。' : '') +
      '删除后后台不会自动补回这个原主机名的基础设备。',
      '删除节点基础设备', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
    await deleteDevice(device.id)
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['devices'] }),
      queryClient.invalidateQueries({ queryKey: ['device-node-devices'] }),
      queryClient.invalidateQueries({ queryKey: ['gateway-groups'] })
    ])
    ElMessage.success('节点基础设备已删除，其他设备及 EasyTier 节点保留')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(formatApiError(error))
  }
}
const deleteDialogOpen = ref(false)
const deleteTargets = ref<Device[]>([])
const deletingDevices = ref(false)
const cancelDeleteButton = ref<ButtonInstance>()
const deleteNodeCount = computed(() => deleteTargets.value.filter(device => device.is_node_device).length)
function reviewSelectedDevices() {
  if (!selectedDevices.value.length || deletingDevices.value) return
  deleteTargets.value = [...selectedDevices.value]
  deleteDialogOpen.value = true
}
async function deleteSelectedDevices() {
  if (!deleteDialogOpen.value || deletingDevices.value || !deleteTargets.value.length) return
  const ids = deleteTargets.value.map(device => device.id)
  const eligible = new Set(selectedDevices.value.map(device => device.id))
  if (ids.some(id => !eligible.has(id))) {
    deleteDialogOpen.value = false
    ElMessage.warning('设备选择已变化，请重新核对删除范围')
    return
  }
  deletingDevices.value = true
  try {
    const results = await Promise.allSettled(ids.map(id => deleteDevice(id)))
    const succeeded = ids.filter((_, index) => results[index].status === 'fulfilled')
    const failed = ids.filter((_, index) => results[index].status === 'rejected')
    selectedDeviceIds.value = failed
    await queryClient.invalidateQueries({ queryKey: ['devices'] })
    await queryClient.invalidateQueries({ queryKey: ['device-node-devices'] })
    await queryClient.invalidateQueries({ queryKey: ['gateway-groups'] })
    if (failed.length) ElMessage.warning('已删除 ' + succeeded.length + ' 台，失败 ' + failed.length + ' 台；请核对后重试失败项')
    else ElMessage.success('已删除 ' + succeeded.length + ' 台设备')
  } catch (error) {
    ElMessage.error(formatApiError(error))
  } finally {
    deletingDevices.value = false
    deleteDialogOpen.value = false
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
  !attributesQuery.isPending.value && !nodeErrors.value && networkNodeDeviceInputs().length > 0
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
  return liveNodeOptions.value.filter(node => nodeValue(node) && !conflictingNodeNames.value.has(nodeValue(node))).map(node => ({
    name: uniqueNodeDeviceName(node, used),
    network_key: node.networkKey,
    peer_id: node.peerId,
    hostname: node.row.rawHostname || ''
  }))
}
async function runNetworkNodeSync(showMessage: boolean) {
  if (nodeErrors.value > 0) {
    if (showMessage) ElMessage.warning('部分组网入口不可用，暂不能确认节点主机名唯一')
    return
  }
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
      queryClient.invalidateQueries({ queryKey: ['device-network-node-sync'] }),
      queryClient.invalidateQueries({ queryKey: ['network-node-attributes'] })
    ])
    if (showMessage) {
      ElMessage.success('网络节点同步完成：新增 ' + result.created + ' 台，已存在 ' + result.existing + ' 台，重新关联 ' + result.rebound + ' 台，跳过已删除 ' + result.skipped + ' 台；新节点自动补充已开启')
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
        <p>查看全部设备，或选择 EasyTier 网络节点管理直属设备及网关下挂设备。</p>
      </div>
      <div class="devices-actions">
        <el-button @click="refreshAll" :loading="isRefreshing"><el-icon><Refresh /></el-icon>刷新</el-button>
        <el-button v-if="networkEntries.length && !networkNodeSyncEnabled"
          :disabled="!canInitializeNetworkNodes" :loading="initializingDevices" @click="initializeNetworkNodes">
          <el-icon><Connection /></el-icon>同步网络节点
        </el-button>
        <el-tag v-if="networkNodeSyncEnabled" type="success" effect="plain">新节点自动补充已开启</el-tag>
      </div>
    </div>

    <div class="node-scope-toolbar">
      <div class="node-scope-label">所处节点</div>
      <el-select v-model="selectedNodeKey" filterable aria-label="所处节点" class="node-scope-select" popper-class="device-node-dropdown" placeholder="选择网络节点">
        <el-option label="全部" :value="ALL_SCOPE" />
        <el-option v-for="node in nodeOptions" :key="nodeValue(node)"
          :label="node.label + ' · ' + (node.isOnline ? '在线' : '离线')" :value="nodeValue(node)">
          <div class="node-option">
            <div class="node-option-info">
              <div class="node-option-heading">
                <span class="node-option-name" :title="node.label">{{ node.label }}</span>
                <el-tag size="small" :type="node.isOnline ? 'success' : 'info'">{{ node.isOnline ? '在线' : '离线' }}</el-tag>
              </div>
              <span v-if="originalNodeHostname(node)" class="node-option-hostname" :title="originalNodeHostname(node)">
                原主机名：{{ originalNodeHostname(node) }}
              </span>
            </div>
            <span class="node-option-meta" :title="node.ip || node.peerId">{{ node.ip || node.peerId }}</span>
          </div>
        </el-option>
      </el-select>
      <div v-if="selectedNode" class="node-scope-meta">
        <el-tag :type="selectedNode.isOnline ? 'success' : 'info'" effect="plain">
          {{ selectedNode.isOnline ? '当前在线' : '历史节点' }}
        </el-tag>
        <span v-if="originalNodeHostname(selectedNode)">原主机名：{{ originalNodeHostname(selectedNode) }}</span>
        <span>{{ selectedNode.instance || '历史节点信息' }}</span>
        <span>{{ selectedNode.ip || selectedNode.peerId }}</span>
      </div>
      <div v-else class="node-scope-meta">显示所有网络节点及未关联节点的设备</div>
    </div>

    <el-alert v-if="devicesQuery.isError.value"
      :title="formatApiError(devicesQuery.error.value)" type="error" :closable="false" show-icon />
    <el-alert v-if="networkEntries.length && nodesQuery.isError.value"
      title="实时网络节点暂时无法读取；已登记的历史节点和设备仍可管理。" type="warning" :closable="false" show-icon />
    <el-alert v-else-if="nodeErrors" :title="nodeErrors + ' 个组网入口暂时不可用，已显示其他入口可读取的节点。'"
      type="warning" :closable="false" show-icon />
    <el-alert v-if="nodeConflicts" :title="nodeConflicts + ' 组 EasyTier 原始主机名冲突，已暂停这些节点的设备关联。'"
      type="warning" :closable="false" show-icon />
    <el-alert v-if="!networkEntries.length && !nodeOptions.length"
      title="尚未设置组网入口；可以先查看全部设备，设置入口后即可选择网络节点。" type="info" :closable="false" />

    <div class="devices-toolbar">
      <el-input v-model="search" clearable placeholder="搜索设备名称 / IP / MAC / 型号" aria-label="搜索设备">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <div class="devices-summary">
        <span>显示 {{ selectableIds.size }} 台设备</span>
        <span v-if="gatewayCount">含 {{ gatewayCount }} 台网关</span>
        <el-checkbox :model-value="allFilteredSelected" :disabled="!selectableIds.size"
          :indeterminate="selectedDevices.length > 0 && !allFilteredSelected" @update:model-value="toggleSelectAll">全选显示的设备</el-checkbox>
        <el-button v-if="selectedDevices.length" type="danger" plain size="small" @click="reviewSelectedDevices">
          <el-icon><Delete /></el-icon>删除所选 {{ selectedDevices.length }} 台
        </el-button>
      </div>
    </div>

    <el-alert v-if="gatewayGroupsQuery.isError.value"
      :title="formatApiError(gatewayGroupsQuery.error.value)" type="error" :closable="false" show-icon />

    <div v-if="devicesQuery.isPending.value" class="devices-loading">
      <el-skeleton :rows="5" animated />
    </div>
    <template v-else>
      <el-empty v-if="!devicesQuery.isError.value && !gatewayGroupsQuery.isError.value && !devices.length && !gatewayGroups.length" :description="isAllScope ? '尚未登记设备' : '当前节点还没有登记设备'" :image-size="90">
        <div class="empty-initialize">
          <p v-if="selectedNode && !nodeDevices.length">该节点尚未生成基础节点设备记录，可以同步网络节点后继续。</p>
          <p v-else-if="!isAllScope">先编辑当前节点的基础设备，将其设置为网关设备，再从该设备卡片新增下挂设备。</p>
          <el-button v-if="networkNodeSyncEnabled && selectedNode && !nodeDevices.length && networkEntries.length"
            :loading="initializingDevices" :disabled="!canInitializeNetworkNodes" @click="initializeNetworkNodes">
            <el-icon><Connection /></el-icon>初始化 / 同步网络节点
          </el-button>
        </div>
      </el-empty>

      <div v-for="section in deviceSections" :key="section.id" class="device-group">
        <div class="group-heading">
          <div v-if="section.gatewayDevice" class="gateway-heading-main">
            <el-checkbox class="gateway-heading-select" :disabled="!selectableIds.has(section.gatewayDevice.id)"
              :model-value="selectedDeviceIds.includes(section.gatewayDevice.id)"
              :aria-label="'选择设备 ' + section.gatewayDevice.name"
              @update:model-value="toggleDeviceSelection(section.gatewayDevice.id, $event)" />
            <button class="group-toggle gateway-group-toggle" type="button"
              :aria-expanded="section.kind === 'gateway' ? !collapsed.includes(section.id) : undefined" @click="section.kind === 'gateway' ? toggleGroup(section.id) : openEditDevice(section.gatewayDevice)">
              <span v-if="section.kind === 'gateway'" class="group-chevron" :class="{ collapsed: collapsed.includes(section.id) }">⌄</span>
              <span class="device-avatar gateway-heading-avatar">
                <el-icon><component :is="deviceIcon(section.gatewayDevice)" /></el-icon>
              </span>
              <span class="gateway-heading-info">
                <span class="gateway-heading-title">
                  <strong>{{ section.gatewayDevice.name }}</strong>
                  <el-tag v-for="kind in section.gatewayDevice.semantic_kinds" :key="kind" size="small" effect="plain">{{ semanticKindLabels[kind] || kind }}</el-tag>
                  <span v-if="[section.gatewayDevice.manufacturer, section.gatewayDevice.model].filter(Boolean).length">
                    {{ [section.gatewayDevice.manufacturer, section.gatewayDevice.model].filter(Boolean).join(' · ') }}
                  </span>
                  <span v-if="section.gatewayDevice.node_hostname" class="device-network-ip">
                    组网 IP：{{ deviceNetworkIP(section.gatewayDevice) || '暂不可用' }}
                  </span>
                  <span v-if="section.gatewayDevice.ip_address">管理 IP：{{ section.gatewayDevice.ip_address }}</span>
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
              <span v-if="section.kind === 'gateway'" class="group-count">{{ section.items.length }}</span>
            </button>
          </div>
          <button v-else class="group-toggle" type="button" :aria-expanded="!collapsed.includes(section.id)"
            @click="toggleGroup(section.id)">
            <span v-if="section.kind === 'gateway'" class="group-chevron" :class="{ collapsed: collapsed.includes(section.id) }">⌄</span>
            <strong>{{ section.name }}</strong>
            <span v-if="section.kind === 'gateway'" class="group-count">{{ section.items.length }}</span>
          </button>
          <DeviceActions v-if="section.gatewayDevice" :device="section.gatewayDevice" :open-wrt="isOpenWrtDevice(section.gatewayDevice)"
            @services="openDeviceServices" @edit="openEditDevice" @add="openCreateDevice"
            @settings="openOpenWrtSettings" @discover="openDiscovery" @remove="deleteNodeBaseDevice" />
        </div>
        <div v-if="section.kind === 'gateway' && !collapsed.includes(section.id)" class="device-grid">
          <article v-for="device in section.items" :key="device.id" class="device-card">
            <div class="device-card-head">
              <el-checkbox class="device-select" :model-value="selectedDeviceIds.includes(device.id)"
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
                  <el-tag v-if="device.is_gateway" size="small" type="success" effect="dark">网关设备</el-tag>
                  <el-tag v-if="device.gateway_group_id" size="small" type="info" effect="plain">下挂设备</el-tag>
                  <el-tag v-if="device.discovery_status === 'online'" size="small" type="success" effect="dark">在线</el-tag>
                  <el-tag v-else-if="device.discovery_status === 'offline'" size="small" type="info" effect="plain">离线</el-tag>
                </div>
                <div class="device-subtitle">
                  <span>{{ [device.manufacturer, device.model].filter(Boolean).join(' · ') || '未填写厂商与型号' }}</span>
                  <div class="device-ip-addresses">
                    <span v-if="device.ip_address">内网 IP：{{ device.ip_address }}</span>
                    <span v-if="device.network_ip">组网 IP：{{ device.network_ip }}</span>
                  </div>
                </div>
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
            <DeviceActions :device="device" :open-wrt="isOpenWrtDevice(device)"
              @services="openDeviceServices" @edit="openEditDevice" @add="openCreateDevice"
              @settings="openOpenWrtSettings" @discover="openDiscovery" @remove="deleteNodeBaseDevice" />
          </article>
          <p v-if="!section.items.length" class="device-group-empty">{{ search ? '没有匹配的下挂设备' : '该网关还没有下挂设备' }}</p>
        </div>
      </div>
      <el-empty v-if="devices.length && !deviceSections.length && search" description="没有匹配的设备">
        <el-button @click="search = ''">清除搜索</el-button>
      </el-empty>
    </template>

    <el-dialog v-model="deleteDialogOpen" title="确认批量删除设备" width="min(500px, 94vw)" append-to-body destroy-on-close
      :show-close="!deletingDevices" :close-on-click-modal="!deletingDevices" :close-on-press-escape="!deletingDevices"
      @opened="cancelDeleteButton?.ref?.focus()">
      <p class="delete-impact">将删除以下 {{ deleteTargets.length }} 台设备的登记、服务记录及 OpenWrt 配置。</p>
      <p v-if="deleteNodeCount" class="delete-impact">其中 {{ deleteNodeCount }} 台为节点基础设备，删除后不会自动补回。实际 EasyTier 节点会保留。</p>
      <p v-if="deleteTargets.some(device => device.is_gateway)" class="delete-impact">所选网关中未勾选的下挂设备会保留，并提升到上一级。</p>
      <ul class="delete-target-list">
        <li v-for="device in deleteTargets" :key="device.id">
          <strong>{{ device.name }}</strong>
          <span>{{ device.ip_address || serviceDeviceNetworkIP(device) || device.mac_address || '暂无地址' }}</span>
        </li>
      </ul>
      <template #footer>
        <el-button ref="cancelDeleteButton" :disabled="deletingDevices" @click="deleteDialogOpen = false">取消</el-button>
        <el-button type="danger" :loading="deletingDevices" @click="deleteSelectedDevices">删除 {{ deleteTargets.length }} 台设备</el-button>
      </template>
    </el-dialog>

    <el-dialog append-to-body v-model="deviceDialogOpen" :title="editingDeviceId ? '编辑设备' : '新增设备'" width="min(620px, 94vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="saveDeviceRecord">
        <div class="form-grid">
          <el-form-item label="设备名称" required>
            <el-input v-model="deviceDraft.name" maxlength="100" placeholder="例如：客厅电视" />
          </el-form-item>
          <el-form-item label="所属网络节点" class="span-two">
            <el-select v-model="deviceDraftNodeKey" clearable filterable class="full" popper-class="device-node-dropdown"
              :disabled="!editingDeviceId && !!deviceDraft.gateway_group_id" placeholder="选择设备所属的网络节点">
              <el-option v-for="node in nodeOptions" :key="nodeValue(node)"
                :label="node.label + ' · ' + (node.isOnline ? '在线' : '离线')" :value="nodeValue(node)">
                <div class="node-option">
                  <div class="node-option-info">
                    <div class="node-option-heading">
                      <span class="node-option-name" :title="node.label">{{ node.label }}</span>
                      <el-tag size="small" :type="node.isOnline ? 'success' : 'info'">{{ node.isOnline ? '在线' : '离线' }}</el-tag>
                    </div>
                    <span v-if="originalNodeHostname(node)" class="node-option-hostname" :title="originalNodeHostname(node)">
                      原主机名：{{ originalNodeHostname(node) }}
                    </span>
                  </div>
                  <span class="node-option-meta" :title="node.ip || node.peerId">{{ node.ip || node.peerId }}</span>
                </div>
              </el-option>
            </el-select>
            <div class="field-hint">普通设备和节点基础设备均可更换所属网络节点。保存后会将选中节点的原始主机名绑定到当前设备；基础设备必须直属节点，目标节点已有基础设备时需先处理冲突。通过网关卡片新增的设备会自动归属该网关。</div>
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
            <el-checkbox v-model="deviceDraft.is_gateway" :disabled="!deviceDraft.node_hostname">
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
          <el-form-item v-if="editingDeviceId" label="组网 IP（自动映射）">
            <el-input :model-value="editingDevice?.network_ip || '尚未计算'" readonly />
            <div class="field-hint">根据所属网络节点的 EasyTier 代理网段映射，扫描发现后自动更新；此字段不可手动修改。</div>
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

    <DeviceServicesDialog v-if="serviceDevice" :key="serviceDevice.id"
      v-model="servicesOpen" :device="serviceDevice" :hosts="props.hosts"
      :network-ip="serviceDeviceNetworkIP(serviceDevice)" :entry-public-ips="serviceEntryPublicIPs" />

    <OpenWrtDiscoveryDialog v-if="discoveryTarget"
      :key="discoveryTarget.id + ':' + discoveryKind"
      v-model="discoveryDialogOpen" :device="discoveryTarget" :kind="discoveryKind" />

    <el-dialog append-to-body v-model="openWrtDialogOpen" title="OpenWrt RPCD 配置" width="min(520px, 92vw)" destroy-on-close>
      <template v-if="openWrtTarget">
        <el-alert type="info" :closable="false" show-icon
          :title="'设备「' + openWrtTarget.name + '」的 RPCD 密钥在服务端加密保存。'" />
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
.devices-pane { display:flex; min-width:0; min-height:0; flex:1; flex-direction:column; gap:18px; overflow:auto; padding:24px 28px 36px; container:devices / inline-size; }
/* 设备面板负责纵向滚动，列表项保留内容高度，避免被压缩后裁切。 */
.devices-pane > * { flex-shrink:0; }
.devices-intro { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; flex-wrap:wrap; }
.devices-intro h2 { margin:0; color:var(--ent-text); font-size:20px; font-weight:760; }
.devices-intro p { margin:7px 0 0; color:var(--ent-muted); font-size:12px; }
.devices-actions { display:flex; gap:8px; flex-wrap:wrap; }
.devices-toolbar { display:flex; align-items:center; gap:14px; }
.devices-toolbar :deep(.el-input) { max-width:420px; }
.devices-summary { display:flex; align-items:center; gap:12px; color:var(--ent-muted); font-size:12px; white-space:nowrap; }
.device-group { overflow:hidden; border:1px solid var(--ent-line); border-radius:14px; background:var(--ent-panel); }
.group-heading { display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; gap:12px; min-height:54px; padding:8px 15px; border-bottom:1px solid var(--ent-line-soft); background:var(--ent-panel-2); }
.group-toggle { display:flex; min-width:0; align-items:center; gap:10px; padding:0; border:0; background:transparent; color:var(--ent-text); text-align:left; cursor:pointer; }
.group-toggle strong { font-size:14px; }
.gateway-heading-main { display:flex; min-width:min(100%,260px); flex:1; align-items:center; gap:9px; }
.gateway-heading-select { flex:none; }
.gateway-group-toggle { min-width:0; flex:1; }
.gateway-heading-avatar { width:34px; height:34px; border-radius:10px; font-size:16px; }
.gateway-heading-info { display:flex; min-width:0; flex:1; flex-direction:column; gap:4px; }
.gateway-heading-title { display:flex; min-width:0; align-items:center; flex-wrap:wrap; gap:4px 11px; color:var(--ent-muted); font-size:11px; }
.device-network-ip { color:var(--ent-text); font-variant-numeric:tabular-nums; }
.gateway-heading-title strong { color:var(--ent-text); font-size:14px; }
.gateway-heading-title > :is(strong, span), .device-subtitle, .device-meta { min-width:0; overflow-wrap:anywhere; }
.gateway-discovery-state { display:inline-flex; align-items:center; gap:4px; }
.gateway-discovery-state::before { width:6px; height:6px; border-radius:50%; background:currentColor; content:""; }
.gateway-discovery-state.online { color:var(--el-color-success); }
.gateway-discovery-state.offline { color:var(--ent-dim); }
.gateway-heading-rpcd { display:flex; align-items:center; flex-wrap:wrap; gap:5px; color:var(--ent-data-highlight); font-size:10px; }
.gateway-heading-rpcd > span { color:var(--ent-muted); }
.group-heading > .device-actions { margin-left:auto; }
.group-chevron { display:inline-block; color:var(--ent-cyan); font-size:18px; transition:transform .15s; }
.group-chevron.collapsed { transform:rotate(-90deg); }
.group-count { padding:2px 8px; border-radius:12px; background:var(--ent-elev); color:var(--ent-muted); font-size:11px; }
.group-actions { display:flex; gap:4px; }
.device-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(min(360px,100%),1fr)); gap:12px; padding:14px; }
.device-group-empty { grid-column:1 / -1; margin:0; color:var(--ent-muted); font-size:12px; }
.device-card { min-width:0; padding:15px; border:1px solid var(--ent-line); border-radius:12px; background:var(--ent-node-row); }
.device-card-head { display:flex; align-items:flex-start; gap:10px; }
.device-select { flex:none; margin-top:10px; }
.device-avatar { display:grid; width:40px; height:40px; flex:none; place-items:center; border:1px solid var(--ent-line); border-radius:12px; background:var(--ent-elev); color:var(--ent-cyan); font-size:19px; }
.device-main { min-width:0; flex:1; }
.device-title { display:flex; align-items:center; gap:7px; flex-wrap:wrap; }
.device-title strong { overflow-wrap:anywhere; color:var(--ent-text); font-size:14px; }
.device-subtitle { display:flex; flex-wrap:wrap; gap:8px; margin-top:7px; color:var(--ent-muted); font-size:11px; }
.device-ip-addresses { display:flex; flex-direction:column; align-items:flex-start; gap:3px; }
.device-card > .device-actions { justify-content:flex-start; border-top:1px solid var(--ent-line-soft); margin-top:12px; padding-top:6px; }
.device-meta { display:flex; align-items:center; flex-wrap:wrap; gap:12px; margin-top:13px; color:var(--ent-muted); font-size:11px; }
.node-association { display:inline-flex; align-items:center; gap:4px; color:var(--ent-data-highlight); }
.muted { color:var(--ent-dim); }
.device-note { margin:10px 0 0; color:var(--ent-muted); font-size:12px; white-space:pre-wrap; overflow-wrap:anywhere; }
.openwrt-status { display:flex; align-items:center; gap:7px; margin-top:12px; padding:8px 10px; border:1px solid var(--ent-detail-line); border-radius:8px; background:var(--ent-detail-bg); color:var(--ent-detail-accent); font-size:11px; }
.openwrt-endpoint { margin-left:auto; color:var(--ent-muted); }
.delete-impact { margin:0 0 10px; }
.delete-target-list { display:grid; gap:6px; margin:0; padding:0; list-style:none; }
.delete-target-list li { display:flex; flex-wrap:wrap; align-items:baseline; justify-content:space-between; gap:4px 12px; padding:8px 10px; background:var(--ent-panel-2); border-radius:8px; overflow-wrap:anywhere; }
.delete-target-list span { color:var(--ent-muted); font-variant-numeric:tabular-nums; }
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
  .form-grid { grid-template-columns:1fr; }
  .span-two { grid-column:auto; }
}

.node-scope-toolbar { display:flex; align-items:center; flex-wrap:wrap; gap:12px; padding:13px 15px; border:1px solid var(--ent-line); border-radius:12px; background:var(--ent-panel); }
.node-scope-label { flex:none; color:var(--ent-muted); font-size:12px; font-weight:700; }
.node-scope-select { width:min(420px,100%); }
.node-scope-meta { display:flex; align-items:center; flex-wrap:wrap; gap:9px; color:var(--ent-muted); font-size:11px; }
.node-option { display:flex; align-items:center; width:100%; gap:10px; min-width:0; padding:6px 0; }
.node-option-info { display:flex; min-width:0; flex:1; flex-direction:column; gap:3px; }
.node-option-heading { display:flex; min-width:0; align-items:center; gap:8px; }
.node-option-name { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.node-option-hostname { overflow:hidden; color:var(--ent-muted); font-size:11px; line-height:1.4; text-overflow:ellipsis; white-space:nowrap; }
.node-option-meta { flex:none; max-width:40%; overflow:hidden; margin-left:auto; color:var(--ent-dim); font-size:11px; text-overflow:ellipsis; white-space:nowrap; }
:global(.device-node-dropdown .el-select-dropdown__item) { display:flex; align-items:center; min-height:42px; height:auto; line-height:normal; }
.gateway-group-summary { display:flex; align-items:center; flex-wrap:wrap; gap:12px; padding:10px 15px; border-bottom:1px solid var(--ent-line-soft); color:var(--ent-muted); font-size:11px; background:var(--ent-panel); }
@container devices (max-width: 640px) {
  .devices-toolbar { align-items:stretch; flex-direction:column; gap:8px; }
  .devices-toolbar :deep(.el-input) { max-width:none; }
  .devices-summary { align-items:center; flex-wrap:wrap; gap:4px 12px; white-space:normal; }
  .node-scope-toolbar { align-items:stretch; flex-direction:column; }
  .node-scope-select { width:100%; }
  .node-scope-meta { align-items:flex-start; }
  .device-grid { grid-template-columns:minmax(0,1fr); padding:10px; }
  .group-heading { flex-wrap:wrap; gap:4px 8px; padding-inline:12px; }
  .gateway-heading-main { flex-basis:100%; }
  .group-heading > .device-actions { width:100%; justify-content:flex-start; border-top:1px solid var(--ent-line-soft); padding-top:4px; }
  .group-actions { flex-wrap:wrap; justify-content:flex-end; }
}

@container devices (max-width: 400px) {
  .group-heading, .device-card { padding-inline:9px; }
  .gateway-heading-main, .group-toggle, .device-card-head { gap:6px; }
  .gateway-heading-avatar, .device-avatar { width:32px; height:32px; font-size:16px; }
  .openwrt-status { flex-wrap:wrap; overflow-wrap:anywhere; }
}
</style>
