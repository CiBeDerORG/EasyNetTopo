<script setup lang="ts">
import { ref } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { formatApiError } from '../../lib/api-client'
import type { NetworkRow } from './data'
import { saveNodeAttributes, updateNodeAttributesCache, type NodeAttributes } from './attributes'
import { networkNodeAttributeKey } from './identity'
import NetworkNodeLocationPicker from './NetworkNodeLocationPicker.vue'

const props = defineProps<{
  entryId: string
  row: NetworkRow
  attributes?: NodeAttributes
  locationOnly?: boolean
  unavailable?: string
}>()
const client = useQueryClient()
const emit = defineEmits<{ saved: []; close: [] }>()
const editorSaving = ref(false)
const locationLookingUp = ref(false)
const saved = props.attributes
const draft = ref<NodeAttributes>(saved ? { ...saved, location: { ...saved.location } } : {
  network_key: networkNodeAttributeKey(props.row, props.entryId), peer_id: props.row.id, alias: '',
  has_public_ip: false, public_ip: '',
  location: { country: '', province: '', city: '', county: '', latitude: null, longitude: null }
})
const latitudeText = ref(draft.value.location.latitude == null ? '' : String(draft.value.location.latitude))
const longitudeText = ref(draft.value.location.longitude == null ? '' : String(draft.value.location.longitude))
function chooseCandidate() {
  if (!props.row.candidatePublicIP) return
  draft.value.has_public_ip = true
  draft.value.public_ip = props.row.candidatePublicIP
}
function coordinatesFromInputs(): boolean {
  const latRaw = latitudeText.value.trim(), lngRaw = longitudeText.value.trim()
  if (!latRaw && !lngRaw) {
    draft.value.location.latitude = null; draft.value.location.longitude = null; return true
  }
  const lat = Number(latRaw), lng = Number(lngRaw)
  if (!latRaw || !lngRaw || !Number.isFinite(lat) || !Number.isFinite(lng) ||
      lat < -85.05112878 || lat > 85.05112878 || lng < -180 || lng > 180) {
    ElMessage.warning('请同时填写有效经纬度（纬度 ±85.05°，经度 ±180°）')
    return false
  }
  draft.value.location.latitude = lat; draft.value.location.longitude = lng
  return true
}
function onMapSelect(point: { latitude: number; longitude: number }) {
  draft.value.location.latitude = point.latitude
  draft.value.location.longitude = point.longitude
  latitudeText.value = String(point.latitude)
  longitudeText.value = String(point.longitude)
}
async function lookupLocation() {
  if (!coordinatesFromInputs() || draft.value.location.latitude == null || draft.value.location.longitude == null) {
    ElMessage.warning('请先在地图选点或输入经纬度')
    return
  }
  locationLookingUp.value = true
  try {
    const { latitude, longitude } = draft.value.location
    const params = new URLSearchParams({ format: 'jsonv2', addressdetails: '1', zoom: '16',
      'accept-language': 'zh-CN', lat: String(latitude), lon: String(longitude) })
    const response = await fetch('https://nominatim.openstreetmap.org/reverse?' + params.toString())
    if (!response.ok) throw new Error('逆地理编码不可用')
    const result = await response.json() as { address?: Record<string, string> }
    const address = result.address || {}
    draft.value.location.country = address.country || ''
    draft.value.location.province = address.state || address.province || ''
    draft.value.location.city = address.city || address.town || address.municipality || address.village || ''
    draft.value.location.county = address.county || address.city_district || address.district || ''
    ElMessage.success('已识别行政区划，可继续手动修改')
  } catch { ElMessage.warning('地址解析失败，请手动填写国家、省、市、县') }
  finally { locationLookingUp.value = false }
}
async function saveAttributes() {
  if (editorSaving.value || props.unavailable || !coordinatesFromInputs()) return
  editorSaving.value = true
  try {
    // 仅编辑位置时保留最新读取的其他属性。
    const value = props.locationOnly && props.attributes
      ? { ...props.attributes, location: draft.value.location }
      : draft.value
    const saved = await saveNodeAttributes({ ...value, network_key: networkNodeAttributeKey(props.row, props.entryId),
      peer_id: props.row.id, hostname: props.row.rawHostname || '' })
    updateNodeAttributesCache(client, saved)
    emit('saved')
    ElMessage.success(props.locationOnly ? '节点位置已保存' : '节点属性已保存')
  } catch (error) { ElMessage.error(formatApiError(error)) }
  finally { editorSaving.value = false }
}
</script>

<template>
  <el-dialog append-to-body :model-value="true"
    :title="(locationOnly ? '编辑位置 · ' : '编辑节点属性 · ') + (attributes?.alias || row.hostname)"
    :close-on-press-escape="!editorSaving" :show-close="!editorSaving" @update:model-value="emit('close')" width="min(760px, 96vw)" :close-on-click-modal="false">
    <el-alert v-if="unavailable" :title="unavailable" type="warning" :closable="false" />
    <el-form label-position="top" class="node-editor">
      <template v-if="!locationOnly">
        <el-form-item label="节点别名">
          <el-input v-model="draft.alias" maxlength="100" show-word-limit placeholder="自定义节点显示名称" />
        </el-form-item>
        <el-form-item label="是否有公网 IP">
          <el-switch v-model="draft.has_public_ip" active-text="是" inactive-text="否" />
        </el-form-item>
        <el-form-item v-if="draft.has_public_ip" label="公网 IP">
          <el-input v-model="draft.public_ip" placeholder="IPv4 或 IPv6 地址，不含端口" />
        </el-form-item>
        <div v-if="row.candidatePublicIP" class="candidate-ip">
          对端地址识别候选：<code>{{ row.candidatePublicIP }}</code>
          <el-button size="small" @click="chooseCandidate">使用此 IP</el-button>
        </div>
        <p class="editor-note">对端地址可能指向中继服务器，不能据此保证节点拥有公网 IP；请确认后再保存。</p>
        <h3>节点位置</h3>
      </template>
      <NetworkNodeLocationPicker :latitude="draft.location.latitude" :longitude="draft.location.longitude" @select="onMapSelect" />
      <div class="location-coordinate-grid">
        <el-form-item label="纬度"><el-input v-model="latitudeText" placeholder="-85.0511 至 85.0511" /></el-form-item>
        <el-form-item label="经度"><el-input v-model="longitudeText" placeholder="-180 至 180" /></el-form-item>
      </div>
      <div class="location-actions">
        <el-button @click="coordinatesFromInputs">定位到输入坐标</el-button>
        <el-button :loading="locationLookingUp" @click="lookupLocation">根据坐标识别行政区划</el-button>
      </div>
      <div class="location-address-grid">
        <el-form-item label="国家"><el-input v-model="draft.location.country" maxlength="120" /></el-form-item>
        <el-form-item label="省 / 州"><el-input v-model="draft.location.province" maxlength="120" /></el-form-item>
        <el-form-item label="市"><el-input v-model="draft.location.city" maxlength="120" /></el-form-item>
        <el-form-item label="县 / 区"><el-input v-model="draft.location.county" maxlength="120" /></el-form-item>
      </div>
    </el-form>
    <template #footer>
      <el-button :disabled="editorSaving" @click="emit('close')">取消</el-button>
      <el-button type="primary" :loading="editorSaving" :disabled="!!unavailable" @click="saveAttributes">{{ locationOnly ? '保存位置' : '保存属性' }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.candidate-ip { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; font-size: 13px; }
.candidate-ip code { color: #8ccffc; }
.editor-note { font-size: 12px; color: #8bacca; line-height: 1.5; }
.node-editor h3 { font-size: 15px; margin: 20px 0 12px; }
.location-coordinate-grid, .location-address-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 14px; margin-top: 16px; }
.location-actions { display: flex; gap: 8px; flex-wrap: wrap; margin: 0 0 14px; }
@media (max-width: 540px) { .location-coordinate-grid, .location-address-grid { grid-template-columns: 1fr; } }
</style>
