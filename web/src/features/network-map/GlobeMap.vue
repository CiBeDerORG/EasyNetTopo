<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import * as Cesium from 'cesium'
import 'cesium/Build/Cesium/Widgets/widgets.css'
import type { MapLink, MapNode } from './topology'

const props = defineProps<{
  nodes: MapNode[]
  links: MapLink[]
  basemap: 'standard' | 'satellite' | 'dark'
  showTraffic: boolean
  selectedNodeKey: string
  selectedLinkKey: string
}>()
const emit = defineEmits<{ nodeClick: [key: string]; linkClick: [key: string] }>()
const globe = ref<HTMLElement | null>(null)
const loading = ref(true)
const error = ref('')
let viewer: Cesium.Viewer | null = null
const located = () => props.nodes.filter(node => node.latitude !== null && node.longitude !== null)
const nodeEntities = new Map<string, Cesium.Entity>()
let cancelled = false
let tileFailureCount = 0

function imageryProvider(key: typeof props.basemap): Cesium.ImageryProvider {
  if (key === 'satellite') return new Cesium.UrlTemplateImageryProvider({
    url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    credit: 'Esri'
  })
  if (key === 'dark') return new Cesium.UrlTemplateImageryProvider({
    url: 'https://basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png',
    credit: '© OpenStreetMap contributors · CARTO'
  })
  return new Cesium.OpenStreetMapImageryProvider({ url: 'https://tile.openstreetmap.org/' })
}
function setBaseLayer() {
  if (!viewer) return
  tileFailureCount = 0
  viewer.imageryLayers.removeAll(true)
  try {
    const provider = imageryProvider(props.basemap)
    provider.errorEvent.addEventListener(() => {
      tileFailureCount += 1
      if (tileFailureCount >= 8) {
        error.value = '地图瓦片加载异常，请检查网络或切换右上角底图；节点及连接数据不会丢失。'
      }
    })
    viewer.imageryLayers.addImageryProvider(provider)
    error.value = ''
  } catch {
    error.value = '地图底图初始化失败。地球仍可操作，请切换底图重试。'
  }
}
function arcPoints(start: MapNode, end: MapNode): Cesium.Cartesian3[] {
  const aLat = start.latitude!, aLon = start.longitude!
  const bLat = end.latitude!, bLon = end.longitude!
  const dLon = ((bLon - aLon + 540) % 360) - 180
  const km = Cesium.Cartesian3.distance(
    Cesium.Cartesian3.fromDegrees(aLon, aLat),
    Cesium.Cartesian3.fromDegrees(bLon, bLat)
  ) / 1000
  const height = Math.min(900000, Math.max(45000, km * 115))
  const steps = Math.max(24, Math.min(90, Math.ceil(km / 180)))
  const positions: Cesium.Cartesian3[] = []
  for (let i = 0; i <= steps; i++) {
    const t = i / steps
    positions.push(Cesium.Cartesian3.fromDegrees(
      aLon + dLon * t, aLat + (bLat - aLat) * t, height * Math.sin(t * Math.PI)
    ))
  }
  return positions
}
function linkPoints(link: MapLink, byKey: Map<string, MapNode>): Cesium.Cartesian3[] | null {
  const from = byKey.get(link.from), to = byKey.get(link.to)
  if (!from || !to || from.latitude === null || from.longitude === null ||
      to.latitude === null || to.longitude === null) return null
  const via = link.via ? byKey.get(link.via) : undefined
  if (via && via.latitude !== null && via.longitude !== null) {
    return [...arcPoints(from, via), ...arcPoints(via, to).slice(1)]
  }
  return arcPoints(from, to)
}
function redraw() {
  if (!viewer || viewer.isDestroyed()) return
  viewer.entities.removeAll()
  nodeEntities.clear()
  const byKey = new Map(props.nodes.map(node => [node.key, node]))
  for (const link of props.links) {
    const points = linkPoints(link, byKey)
    if (!points) continue
    const highlight = props.selectedLinkKey === link.key
    const color = link.kind === 'p2p' ? Cesium.Color.fromCssColorString('#3ae6ff')
      : link.kind === 'relay' ? Cesium.Color.fromCssColorString('#ffae55')
      : Cesium.Color.fromCssColorString('#b1bfd0')
    viewer.entities.add({
      id: 'map-link:' + link.key,
      polyline: {
        positions: points,
        width: highlight ? 5 : link.kind === 'p2p' ? 3.1 : 2.6,
        material: link.kind === 'p2p'
          ? new Cesium.PolylineGlowMaterialProperty({ glowPower: highlight ? 0.28 : 0.17, color })
          : new Cesium.PolylineDashMaterialProperty({ color, dashLength: 16 }),
        arcType: Cesium.ArcType.NONE
      }
    })
    // 粒子仅在连续两次真实流量采样证明存在数据变化时出现。
    const bps = (link.rxBps || 0) + (link.txBps || 0)
    if (!props.showTraffic || bps <= 0) continue
    const count = Math.min(5, Math.max(1, Math.ceil(Math.log10(bps + 1) / 1.7)))
    const speed = Math.min(0.37, 0.06 + Math.log10(bps + 1) * 0.03)
    for (let i = 0; i < count; i++) {
      const offset = i / count
      const position = new Cesium.CallbackPositionProperty((_time, result) => {
        const fraction = ((performance.now() / 1000 * speed) + offset) % 1
        const segment = Math.min(points.length - 2, Math.floor(fraction * (points.length - 1)))
        const weight = fraction * (points.length - 1) - segment
        return Cesium.Cartesian3.lerp(points[segment], points[segment + 1], weight, result || new Cesium.Cartesian3())
      }, false)
      viewer.entities.add({
        position,
        point: { pixelSize: 5, color: Cesium.Color.WHITE, outlineColor: color, outlineWidth: 2 }
      })
    }
  }
  for (const node of located()) {
    const selected = props.selectedNodeKey === node.key
    const publicServer = node.flags.includes('公共服务器')
    const color = publicServer ? Cesium.Color.fromCssColorString('#8be9b9')
      : Cesium.Color.fromCssColorString('#36dcec')
    const entity = viewer.entities.add({
      id: 'map-node:' + node.key,
      position: Cesium.Cartesian3.fromDegrees(node.longitude!, node.latitude!, 2500),
      point: {
        pixelSize: new Cesium.CallbackProperty(() => (selected ? 16 : 11) +
          1.6 * Math.sin(performance.now() / 600), false),
        color, outlineColor: Cesium.Color.WHITE, outlineWidth: selected ? 3 : 1.5,
        disableDepthTestDistance: Number.POSITIVE_INFINITY
      },
      label: {
        text: node.label,
        font: '600 13px sans-serif',
        fillColor: Cesium.Color.WHITE,
        showBackground: true,
        backgroundColor: Cesium.Color.fromCssColorString('#081d32').withAlpha(0.85),
        backgroundPadding: new Cesium.Cartesian2(8, 5),
        style: Cesium.LabelStyle.FILL,
        pixelOffset: new Cesium.Cartesian2(0, -26),
        distanceDisplayCondition: new Cesium.DistanceDisplayCondition(0, 29000000),
        disableDepthTestDistance: Number.POSITIVE_INFINITY
      }
    })
    nodeEntities.set(node.key, entity)
  }
}
function focusNode(key: string) {
  if (!viewer) return
  const entity = nodeEntities.get(key)
  if (entity) void viewer.flyTo(entity, {
    duration: 1.3, offset: new Cesium.HeadingPitchRange(0, -Math.PI / 2, 900000)
  })
}
function resetCamera() {
  if (!viewer) return
  viewer.camera.flyTo({
    destination: Cesium.Cartesian3.fromDegrees(108, 19, 25500000), duration: 1.5
  })
}
function zoomIn() {
  if (viewer) viewer.camera.zoomIn(Math.max(3000, viewer.camera.positionCartographic.height * 0.27))
}
function zoomOut() {
  if (viewer) viewer.camera.zoomOut(Math.max(3000, viewer.camera.positionCartographic.height * 0.38))
}
defineExpose({ focusNode, resetCamera, zoomIn, zoomOut })
watch(() => props.basemap, setBaseLayer)
watch(() => [props.nodes, props.links, props.showTraffic, props.selectedNodeKey, props.selectedLinkKey],
  redraw, { deep: true })
onMounted(() => {
  if (!globe.value) return
  try {
    viewer = new Cesium.Viewer(globe.value, {
      animation: false, timeline: false, homeButton: false, geocoder: false,
      navigationHelpButton: false, sceneModePicker: false, baseLayerPicker: false,
      fullscreenButton: false, selectionIndicator: false, infoBox: false,
      baseLayer: false, terrainProvider: new Cesium.EllipsoidTerrainProvider(),
      requestRenderMode: false
    })
    viewer.scene.globe.baseColor = Cesium.Color.fromCssColorString('#c4d9e5')
    viewer.scene.globe.enableLighting = false
    viewer.scene.fog.enabled = false
    viewer.scene.backgroundColor = Cesium.Color.fromCssColorString('#061323')
    if (viewer.scene.skyAtmosphere) viewer.scene.skyAtmosphere.show = true
    viewer.scene.screenSpaceCameraController.minimumZoomDistance = 350
    viewer.scene.screenSpaceCameraController.maximumZoomDistance = 45000000
    setBaseLayer()
    redraw()
    viewer.screenSpaceEventHandler.setInputAction((movement: { position: Cesium.Cartesian2 }) => {
      if (!viewer) return
      const picked = viewer.scene.pick(movement.position) as { id?: Cesium.Entity } | undefined
      const id = picked?.id?.id
      if (id?.startsWith('map-node:')) emit('nodeClick', id.slice('map-node:'.length))
      else if (id?.startsWith('map-link:')) emit('linkClick', id.slice('map-link:'.length))
    }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
    resetCamera()
  } catch (cause) {
    error.value = '3D 地球初始化失败，请检查 WebGL 和 Cesium 静态资源：' +
      (cause instanceof Error ? cause.message : '未知异常')
    if (viewer && !viewer.isDestroyed()) viewer.destroy()
    viewer = null
  } finally {
    if (!cancelled) loading.value = false
  }
})
onUnmounted(() => {
  cancelled = true
  if (viewer && !viewer.isDestroyed()) viewer.destroy()
  viewer = null
})
</script>

<template>
  <div class="earth-shell">
    <div ref="globe" class="earth-canvas" aria-label="三维地球网络地图，可拖动旋转与滚轮缩放" />
    <div v-if="loading" class="map-notice">正在启动 3D 地球…</div>
    <div v-if="error" class="map-notice error-notice" role="alert">{{ error }}</div>
    <div class="zoom-tools">
      <button title="放大" type="button" @click="zoomIn">+</button>
      <button title="缩小" type="button" @click="zoomOut">−</button>
      <button title="全球视角" type="button" @click="resetCamera">◎</button>
    </div>
  </div>
</template>

<style scoped>
.earth-shell { position: relative; width: 100%; height: 100%; min-height: 320px; overflow: hidden;
  background: radial-gradient(circle at 45% 50%, #214563, #071423 72%); }
.earth-canvas { position: absolute; inset: 0; }
:deep(.cesium-viewer), :deep(.cesium-viewer-cesiumWidgetContainer), :deep(.cesium-widget) { height: 100%; width: 100%; }
:deep(.cesium-viewer-bottom) { max-width: 78%; }
.map-notice { position: absolute; z-index: 2; top: 16px; left: 16px; right: 16px;
  max-width: 560px; border: 1px solid #3b677d; border-radius: 9px; background: #0b2738ed;
  color: #d8f4ff; font-size: 12px; padding: 9px 13px; pointer-events: none; }
.error-notice { border-color: #bc795a; background: #372820ef; color: #ffdfba; }
.zoom-tools { position: absolute; z-index: 3; right: 18px; bottom: 46px; display: grid; gap: 7px; }
.zoom-tools button { border: 1px solid #4e7190; background: #102e45e8; color: #e4faff;
  border-radius: 9px; width: 36px; height: 36px; cursor: pointer; font-size: 22px; }
.zoom-tools button:hover { background: #1e526d; }
</style>
