<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import * as Cesium from 'cesium'
import 'cesium/Build/Cesium/Widgets/widgets.css'
import type { MapLink, MapNode } from './topology'

type Basemap = 'auto' | 'standard' | 'satellite' | 'dark'
type Theme = 'light' | 'dark'

const props = defineProps<{
  nodes: MapNode[]
  links: MapLink[]
  basemap: Basemap
  showTraffic: boolean
  selectedNodeKey: string
  selectedLinkKey: string
}>()
const emit = defineEmits<{ nodeClick: [key: string]; linkClick: [key: string] }>()
const globe = ref<HTMLElement | null>(null)
const loading = ref(true)
const error = ref('')
const activeTheme = ref<Theme>('dark')
let viewer: Cesium.Viewer | null = null
let themeObserver: MutationObserver | null = null
let cancelled = false
let tileFailureCount = 0
const nodeEntities = new Map<string, Cesium.Entity>()
const located = () => props.nodes.filter(node => node.latitude !== null && node.longitude !== null)

function readTheme(): Theme {
  return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark'
}
function resolvedBasemap(): Exclude<Basemap, 'auto'> {
  if (props.basemap !== 'auto') return props.basemap
  return activeTheme.value === 'dark' ? 'dark' : 'standard'
}
function linkColor(kind: MapLink['kind']): Cesium.Color {
  if (kind === 'p2p') return Cesium.Color.fromCssColorString(
    activeTheme.value === 'dark' ? '#38d8ff' : '#087f9a')
  if (kind === 'relay') return Cesium.Color.fromCssColorString(
    activeTheme.value === 'dark' ? '#ffb45c' : '#a85b0a')
  return Cesium.Color.fromCssColorString(
    activeTheme.value === 'dark' ? '#b1bfd0' : '#60768a')
}
function imageryProviders(key: Exclude<Basemap, 'auto'>): Array<{ provider: Cesium.ImageryProvider; name: string }> {
  const url = (template: string, credit: string) => ({
    provider: new Cesium.UrlTemplateImageryProvider({
      url: template,
      credit,
      maximumLevel: 19
    }),
    name: key
  })
  if (key === 'satellite') return [url(
    'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    'Powered by Esri · Source: Esri, Vantor, Earthstar Geographics, and the GIS User Community'
  )]
  if (key === 'dark') return [
    url(
      'https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}',
      'Powered by Esri · Esri, HERE, Garmin, © OpenStreetMap contributors, and the GIS user community'
    ),
    {
      provider: new Cesium.UrlTemplateImageryProvider({
        url: 'https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Reference/MapServer/tile/{z}/{y}/{x}',
        credit: 'Powered by Esri · Esri, HERE, Garmin, © OpenStreetMap contributors, and the GIS user community',
        maximumLevel: 19
      }),
      name: 'dark labels'
    }
  ]
  return [url(
    'https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}',
    'Powered by Esri · Sources: Esri, DeLorme, HERE, USGS, Intermap, iPC, NRCAN, Esri Japan, METI, Esri China (Hong Kong), Esri (Thailand), MapmyIndia, and TomTom'
  )]
}
function applySceneTheme() {
  if (!viewer || viewer.isDestroyed()) return
  const dark = activeTheme.value === 'dark'
  viewer.scene.backgroundColor = Cesium.Color.fromCssColorString(dark ? '#061323' : '#dceaf4')
  viewer.scene.globe.baseColor = Cesium.Color.fromCssColorString(dark ? '#14283b' : '#e5eff5')
  viewer.scene.globe.enableLighting = false
  viewer.scene.globe.showGroundAtmosphere = true
  viewer.scene.globe.maximumScreenSpaceError = 1.35
  viewer.scene.fog.enabled = false
  if (viewer.scene.skyAtmosphere) viewer.scene.skyAtmosphere.show = true
  if (viewer.scene.postProcessStages.fxaa) viewer.scene.postProcessStages.fxaa.enabled = true
  viewer.scene.requestRender()
}
function setBaseLayer() {
  if (!viewer || viewer.isDestroyed()) return
  tileFailureCount = 0
  viewer.imageryLayers.removeAll(true)
  try {
    const providers = imageryProviders(resolvedBasemap())
    for (const item of providers) {
      item.provider.errorEvent.addEventListener(() => {
        tileFailureCount += 1
        if (tileFailureCount >= 3) {
          error.value = '地图底图加载失败（' + item.name +
            '），请检查网络；节点及连接数据不会丢失。'
        }
      })
      viewer.imageryLayers.addImageryProvider(item.provider)
    }
    error.value = ''
    viewer.scene.requestRender()
  } catch {
    error.value = '地图底图初始化失败。地球仍可操作，请切换底图重试。'
  }
}
function arcPoints(start: MapNode, end: MapNode): Cesium.Cartesian3[] {
  const startPoint = Cesium.Cartographic.fromDegrees(start.longitude!, start.latitude!)
  const endPoint = Cesium.Cartographic.fromDegrees(end.longitude!, end.latitude!)
  const geodesic = new Cesium.EllipsoidGeodesic(startPoint, endPoint)
  const distance = geodesic.surfaceDistance
  const lift = Math.max(110000, Math.min(1150000, distance * 0.115))
  const positions: Cesium.Cartesian3[] = []
  for (let i = 0; i <= 72; i++) {
    const fraction = i / 72
    const point = geodesic.interpolateUsingFraction(fraction)
    const height = 1600 + Math.sin(Math.PI * fraction) * lift
    positions.push(Cesium.Cartesian3.fromRadians(point.longitude, point.latitude, height))
  }
  return positions
}
function linkPoints(link: MapLink, byKey: Map<string, MapNode>): Cesium.Cartesian3[] | null {
  const from = byKey.get(link.from), to = byKey.get(link.to)
  if (!from || !to || from.latitude === null || from.longitude === null ||
      to.latitude === null || to.longitude === null) return null
  if (from.latitude === to.latitude && from.longitude === to.longitude) return null
  const via = link.via ? byKey.get(link.via) : undefined
  if (via && via.latitude !== null && via.longitude !== null) {
    return [...arcPoints(from, via), ...arcPoints(via, to).slice(1)]
  }
  return arcPoints(from, to)
}
function addTrafficParticles(link: MapLink, points: Cesium.Cartesian3[], color: Cesium.Color) {
  const bps = (link.rxBps || 0) + (link.txBps || 0)
  if (!props.showTraffic || bps <= 0) return
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
    viewer?.entities.add({
      position,
      point: { pixelSize: 7, color: Cesium.Color.WHITE, outlineColor: color, outlineWidth: 2 }
    })
  }
}
function redraw() {
  if (!viewer || viewer.isDestroyed()) return
  viewer.entities.removeAll()
  nodeEntities.clear()
  const byKey = new Map(props.nodes.map(node => [node.key, node]))

  // 先绘制中继/未知线路，再绘制 P2P，确保连续直连线不会被虚线覆盖。
  const rank: Record<MapLink['kind'], number> = { relay: 0, unknown: 1, p2p: 2 }
  const orderedLinks = [...props.links].sort((a, b) => rank[a.kind] - rank[b.kind])
  for (const link of orderedLinks) {
    const points = linkPoints(link, byKey)
    if (!points) continue
    const highlight = props.selectedLinkKey === link.key
    const color = linkColor(link.kind)
    if (link.kind === 'p2p') {
      // P2P 主线使用纯色材质，确保始终是连续实线；宽光晕仅作视觉衬底。
      viewer.entities.add({
        id: 'map-link-glow:' + link.key,
        polyline: {
          positions: points, width: highlight ? 13 : 10,
          material: new Cesium.ColorMaterialProperty(color.withAlpha(highlight ? 0.32 : 0.2)),
          arcType: Cesium.ArcType.NONE, clampToGround: false
        }
      })
      viewer.entities.add({
        id: 'map-link:' + link.key,
        polyline: {
          positions: points, width: highlight ? 7 : 5.2,
          material: new Cesium.ColorMaterialProperty(color.withAlpha(0.98)),
          arcType: Cesium.ArcType.NONE, clampToGround: false
        }
      })
    } else {
      viewer.entities.add({
        id: 'map-link:' + link.key,
        polyline: {
          positions: points, width: highlight ? 6 : 4.4,
          material: new Cesium.PolylineDashMaterialProperty({
            color: color.withAlpha(0.98),
            gapColor: color.withAlpha(0.035),
            dashLength: 24
          }),
          arcType: Cesium.ArcType.NONE, clampToGround: false
        }
      })
    }
    addTrafficParticles(link, points, color)
  }

  const relayNodeKeys = new Set(props.links
    .filter(link => link.kind === 'relay' && link.via)
    .map(link => link.via!))
  for (const node of located()) {
    const selected = props.selectedNodeKey === node.key
    const relayNode = relayNodeKeys.has(node.key)
    const color = Cesium.Color.fromCssColorString(relayNode
      ? (activeTheme.value === 'dark' ? '#ffb45c' : '#a85b0a')
      : node.flags.includes('公共服务器') ? '#22a879' : '#16a8c6')
    const position = Cesium.Cartesian3.fromDegrees(node.longitude!, node.latitude!, 3500)
    const outline = Cesium.Color.fromCssColorString(activeTheme.value === 'dark' ? '#f4fbff' : '#12364b')
    const labelFill = Cesium.Color.fromCssColorString(activeTheme.value === 'dark' ? '#eaf7ff' : '#17344d')
    const labelOutline = Cesium.Color.fromCssColorString(activeTheme.value === 'dark' ? '#061323' : '#ffffff')
    const labelBackground = Cesium.Color.fromCssColorString(activeTheme.value === 'dark' ? '#071a2d' : '#ffffff').withAlpha(0.9)

    // 外圈光晕与中心实心点分层，远距离也能快速辨认节点坐标。
    viewer.entities.add({
      id: 'map-node-halo:' + node.key,
      position,
      point: {
        pixelSize: new Cesium.CallbackProperty(() => (selected ? 48 : 38) +
          2.5 * Math.sin(performance.now() / 700), false),
        color: color.withAlpha(activeTheme.value === 'dark' ? 0.2 : 0.16),
        outlineColor: color.withAlpha(0.12), outlineWidth: 1,
        disableDepthTestDistance: Number.POSITIVE_INFINITY
      }
    })
    const entity = viewer.entities.add({
      id: 'map-node:' + node.key,
      position,
      point: {
        pixelSize: new Cesium.CallbackProperty(() => (selected ? 27 : 20) +
          1.8 * Math.sin(performance.now() / 600), false),
        color, outlineColor: outline, outlineWidth: selected ? 4 : 3,
        disableDepthTestDistance: Number.POSITIVE_INFINITY
      },
      label: {
        text: node.label,
        font: '700 14px sans-serif',
        fillColor: labelFill,
        outlineColor: labelOutline,
        outlineWidth: 3,
        showBackground: true,
        backgroundColor: labelBackground,
        backgroundPadding: new Cesium.Cartesian2(9, 6),
        style: Cesium.LabelStyle.FILL_AND_OUTLINE,
        pixelOffset: new Cesium.Cartesian2(0, -36),
        distanceDisplayCondition: new Cesium.DistanceDisplayCondition(0, 42000000),
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
    duration: 1.3, offset: new Cesium.HeadingPitchRange(0, -Math.PI / 2, 780000)
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
watch(() => [props.basemap, activeTheme.value], () => {
  setBaseLayer()
  applySceneTheme()
})
watch(() => [props.nodes, props.links, props.showTraffic, props.selectedNodeKey, props.selectedLinkKey],
  redraw, { deep: true })
onMounted(() => {
  if (!globe.value) return
  activeTheme.value = readTheme()
  try {
    viewer = new Cesium.Viewer(globe.value, {
      animation: false, timeline: false, homeButton: false, geocoder: false,
      navigationHelpButton: false, sceneModePicker: false, baseLayerPicker: false,
      fullscreenButton: false, selectionIndicator: false, infoBox: false,
      baseLayer: false, terrainProvider: new Cesium.EllipsoidTerrainProvider(),
      requestRenderMode: false, shouldAnimate: true
    })
    applySceneTheme()
    setBaseLayer()
    redraw()
    viewer.screenSpaceEventHandler.setInputAction((movement: { position: Cesium.Cartesian2 }) => {
      if (!viewer) return
      const picked = viewer.scene.pick(movement.position) as { id?: Cesium.Entity } | undefined
      const id = picked?.id?.id
      if (id?.startsWith('map-node:')) emit('nodeClick', id.slice('map-node:'.length))
      else if (id?.startsWith('map-node-halo:')) emit('nodeClick', id.slice('map-node-halo:'.length))
      else if (id?.startsWith('map-link:')) emit('linkClick', id.slice('map-link:'.length))
      else if (id?.startsWith('map-link-glow:')) emit('linkClick', id.slice('map-link-glow:'.length))
    }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
    themeObserver = new MutationObserver(() => {
      const next = readTheme()
      if (next !== activeTheme.value) activeTheme.value = next
    })
    themeObserver.observe(document.documentElement, {
      attributes: true, attributeFilter: ['data-theme']
    })
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
  themeObserver?.disconnect()
  themeObserver = null
  if (viewer && !viewer.isDestroyed()) viewer.destroy()
  viewer = null
})
</script>

<template>
  <div class="earth-shell" :class="'theme-' + activeTheme">
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
  background: radial-gradient(circle at 45% 50%, #214563, #071423 72%); transition: background .2s ease; }
.earth-shell.theme-light { background: radial-gradient(circle at 45% 50%, #f7fbff, #d7e6f0 72%); }
.earth-canvas { position: absolute; inset: 0; }
:deep(.cesium-viewer), :deep(.cesium-viewer-cesiumWidgetContainer), :deep(.cesium-widget) { height: 100%; width: 100%; }
:deep(.cesium-viewer-bottom) { max-width: 78%; }
.map-notice { position: absolute; z-index: 2; top: 16px; left: 16px; right: 16px;
  max-width: 560px; border: 1px solid #3b677d; border-radius: 9px; background: #0b2738ed;
  color: #d8f4ff; font-size: 12px; padding: 9px 13px; pointer-events: none; }
.error-notice { border-color: #bc795a; background: #372820ef; color: #ffdfba; }
.theme-light .map-notice { border-color: #b6cad9; background: #ffffffed; color: #29465e; }
.theme-light .error-notice { border-color: #e1b6a0; background: #fff6f0ed; color: #914323; }
.zoom-tools { position: absolute; z-index: 3; right: 18px; bottom: 46px; display: grid; gap: 7px; }
.zoom-tools button { border: 1px solid #4e7190; background: #102e45e8; color: #e4faff;
  border-radius: 9px; width: 40px; height: 40px; cursor: pointer; font-size: 23px; }
.zoom-tools button:hover { background: #1e526d; }
.theme-light .zoom-tools button { border-color: #b5c8d8; background: #ffffffed; color: #23445e; box-shadow: 0 2px 9px #45678020; }
.theme-light .zoom-tools button:hover { background: #eaf3f9; }
</style>
