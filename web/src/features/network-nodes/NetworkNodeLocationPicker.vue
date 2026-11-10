<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { mercatorWorldSize, projectMercator, unprojectMercator } from './map-mercator'

const props = defineProps<{ latitude: number | null; longitude: number | null }>()
const emit = defineEmits<{ select: [point: { latitude: number; longitude: number }] }>()
const panel = ref<HTMLElement | null>(null)
const size = ref({ width: 600, height: 300 })
const center = ref({ latitude: 30, longitude: 110 })
const zoom = ref(4)
const tileSize = 256
let observer: ResizeObserver | null = null
let drag: { x: number; y: number; cx: number; cy: number; moved: boolean } | null = null

function world(): number { return mercatorWorldSize(zoom.value) }
function project(latitude: number, longitude: number) {
  return projectMercator(latitude, longitude, zoom.value)
}
function unproject(x: number, y: number) {
  return unprojectMercator(x, y, zoom.value)
}
const tiles = computed(() => {
  const { x: cx, y: cy } = project(center.value.latitude, center.value.longitude)
  const width = size.value.width
  const height = size.value.height
  const startX = Math.floor((cx - width / 2) / tileSize)
  const endX = Math.floor((cx + width / 2) / tileSize)
  const startY = Math.floor((cy - height / 2) / tileSize)
  const endY = Math.floor((cy + height / 2) / tileSize)
  const count = Math.pow(2, zoom.value)
  const result: { key: string; url: string; left: number; top: number }[] = []
  for (let y = startY; y <= endY; y++) {
    if (y < 0 || y >= count) continue
    for (let x = startX; x <= endX; x++) {
      const wrappedX = ((x % count) + count) % count
      result.push({
        key: x + ':' + y,
        url: 'https://tile.openstreetmap.org/' + zoom.value + '/' + wrappedX + '/' + y + '.png',
        left: x * tileSize - cx + width / 2,
        top: y * tileSize - cy + height / 2
      })
    }
  }
  return result
})
const marker = computed(() => {
  if (props.latitude == null || props.longitude == null) return null
  const c = project(center.value.latitude, center.value.longitude)
  const p = project(props.latitude, props.longitude)
  const dx = ((p.x - c.x + world() / 2) % world() + world()) % world() - world() / 2
  return { left: size.value.width / 2 + dx, top: size.value.height / 2 + p.y - c.y }
})
function zoomBy(amount: number) { zoom.value = Math.min(18, Math.max(2, zoom.value + amount)) }
function down(event: PointerEvent) {
  if (event.button !== 0 || !panel.value) return
  const c = project(center.value.latitude, center.value.longitude)
  drag = { x: event.clientX, y: event.clientY, cx: c.x, cy: c.y, moved: false }
  panel.value.setPointerCapture(event.pointerId)
}
function move(event: PointerEvent) {
  if (!drag) return
  const dx = event.clientX - drag.x
  const dy = event.clientY - drag.y
  if (Math.abs(dx) + Math.abs(dy) > 4) drag.moved = true
  if (drag.moved) center.value = unproject(drag.cx - dx, drag.cy - dy)
}
function up(event: PointerEvent) {
  if (!drag || !panel.value) return
  const moved = drag.moved
  drag = null
  if (panel.value.hasPointerCapture(event.pointerId)) panel.value.releasePointerCapture(event.pointerId)
  if (moved) return
  const rect = panel.value.getBoundingClientRect()
  const c = project(center.value.latitude, center.value.longitude)
  const point = unproject(c.x + event.clientX - rect.left - rect.width / 2,
    c.y + event.clientY - rect.top - rect.height / 2)
  center.value = point
  emit('select', point)
}
watch(() => [props.latitude, props.longitude], ([lat, lng]) => {
  if (typeof lat === 'number' && typeof lng === 'number' &&
      Number.isFinite(lat) && Number.isFinite(lng)) center.value = { latitude: lat, longitude: lng }
}, { immediate: true })
onMounted(() => {
  if (!panel.value) return
  observer = new ResizeObserver(entries => {
    const rect = entries[0]?.contentRect
    if (rect) size.value = { width: rect.width, height: rect.height }
  })
  observer.observe(panel.value)
})
onUnmounted(() => { observer?.disconnect() })
</script>

<template>
  <div class="location-picker">
    <div ref="panel" class="map-canvas" role="application" aria-label="地图位置选点，点击选择坐标，拖动平移"
      @pointerdown="down" @pointermove="move" @pointerup="up" @pointercancel="drag = null">
      <img v-for="tile in tiles" :key="tile.key" :src="tile.url" alt="" draggable="false"
        :style="{ left: tile.left + 'px', top: tile.top + 'px' }" class="map-tile" />
      <span v-if="marker" class="map-marker" :style="{ left: marker.left + 'px', top: marker.top + 'px' }" />
      <div class="map-controls" @pointerdown.stop>
        <el-button size="small" @click.stop="zoomBy(1)">+</el-button>
        <el-button size="small" @click.stop="zoomBy(-1)">−</el-button>
      </div>
      <div class="map-attribution" @pointerdown.stop>
        © <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">OpenStreetMap</a> contributors
      </div>
    </div>
    <p class="map-hint">点击地图选取坐标，按住拖动平移，使用 + / − 缩放。地图瓦片需要外网；也可直接输入经纬度。</p>
  </div>
</template>

<style scoped>
.location-picker { width: 100%; }
.map-canvas { height: 300px; position: relative; overflow: hidden; border: 1px solid #35536d; border-radius: 10px; background: #dce6ee; cursor: crosshair; touch-action: none; user-select: none; }
.map-tile { position: absolute; width: 256px; height: 256px; pointer-events: none; user-select: none; }
.map-marker { position: absolute; width: 17px; height: 17px; margin: -15px 0 0 -9px; border: 3px solid white; border-radius: 50% 50% 50% 0; background: #f04438; transform: rotate(-45deg); box-shadow: 0 1px 6px #283e50; pointer-events: none; }
.map-controls { position: absolute; top: 12px; left: 12px; display: flex; flex-direction: column; gap: 4px; }
.map-controls :deep(.el-button) { margin: 0; }
.map-attribution { position: absolute; bottom: 0; right: 0; padding: 3px 6px; background: #fffd; color: #1a2a36; font-size: 10px; }
.map-attribution a { color: #164b8b; }
.map-hint { font-size: 12px; color: #8bacca; line-height: 1.6; }
</style>
