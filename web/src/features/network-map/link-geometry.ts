// 城市内的连线按画布像素控制弓高；长距离连接仍沿地球测地线抬升。
import { Cartesian2, Cartesian3, Cartographic, Ellipsoid, EllipsoidGeodesic, IntersectionTests, Ray, type Scene } from 'cesium'
import type { MapNode } from './topology'

export const LINK_ENDPOINT_HEIGHT = 20
const LOCAL_DISTANCE = 250_000
const globe = Ellipsoid.WGS84
const linkSurface = new Ellipsoid(
  globe.radii.x + LINK_ENDPOINT_HEIGHT, globe.radii.y + LINK_ENDPOINT_HEIGHT, globe.radii.z + LINK_ENDPOINT_HEIGHT
)

type Pixel = { x: number; y: number }

// 二次贝塞尔曲线位于两端和控制点组成的三角形内。限制控制点即可限制整段，光晕另留边距。
export function screenCurveControl(start: Pixel, end: Pixel, width: number, height: number, lane?: number): Cartesian2 {
  const middle = new Cartesian2((start.x + end.x) / 2, (start.y + end.y) / 2)
  const dx = end.x - start.x, dy = end.y - start.y
  const span = Math.hypot(dx, dy)
  if (!span || !width || !height) return middle
  const nx = -dy / span, ny = dx / span
  const viewport = Math.min(width, height)
  // 从远景的小弯过渡到近景的明显弧线，弓高最多占画布短边的一成。
  const target = span * (0.12 + 0.18 * Math.min(span / viewport, 1))
  const limit = viewport * 0.10
  const bow = limit * -Math.expm1(-target / limit)
  const usable = (position: number, axis: number, extent: number) => {
    if (axis === 0) return Infinity
    const edge = axis > 0 ? extent - 7 : 7
    return Math.max(0, (edge - position) / axis)
  }
  const room = (sign: number) => Math.min(
    usable(middle.x, nx * sign, width), usable(middle.y, ny * sign, height)
  )
  // 中点弓高是控制点偏移的一半。单线贴边时逐渐转向画面内侧，平移不会突然翻转。
  let offset: number
  if (lane === undefined) {
    const desired = 2 * bow * 0.8
    offset = Math.max(-room(-1), Math.min(desired, 2 * room(1) - desired))
  } else {
    // 并行线路共享同侧的可用空间，再按车道分配弧高。
    offset = Math.min(2 * bow, room(Math.sign(lane))) * lane / (0.5 + Math.abs(lane))
  }
  return new Cartesian2(middle.x + nx * offset, middle.y + ny * offset)
}

function surfacePoint(scene: Scene, pixel: Cartesian2, ray: Ray): Cartesian3 | undefined {
  if (!scene.camera.getPickRay(pixel, ray)) return undefined
  const hit = IntersectionTests.rayEllipsoid(ray, linkSurface)
  return hit && hit.start > 0 ? Ray.getPoint(ray, hit.start, new Cartesian3()) : undefined
}

function localPoints(scene: Scene, from: Cartesian3, to: Cartesian3, lane?: number): Cartesian3[] | undefined {
  // 地平线背面的节点不可通过近侧射线反算，以免线路跳到地球另一面。
  const front = (point: Cartesian3) => Cartesian3.dot(
    Cartesian3.subtract(scene.camera.positionWC, point, new Cartesian3()),
    globe.geodeticSurfaceNormal(point, new Cartesian3())
  ) > 0
  if (!front(from) || !front(to)) return undefined
  const a = scene.cartesianToCanvasCoordinates(from), b = scene.cartesianToCanvasCoordinates(to)
  if (!a || !b) return undefined
  const { clientWidth, clientHeight } = scene.canvas
  const control = screenCurveControl(a, b, clientWidth, clientHeight, lane)
  const ray = new Ray(), pixel = new Cartesian2()
  const middle = Cartesian2.lerp(a, b, 0.5, new Cartesian2())
  // 控制点接近地球轮廓时向弦中点收拢，确保插值像素也可投回可见半球。
  for (let i = 0; i < 8 && !surfacePoint(scene, control, ray); i++) {
    Cartesian2.lerp(control, middle, 0.5, control)
  }
  const steps = Math.min(64, Math.max(8, Math.ceil(Cartesian2.distance(a, b) / 12)))
  const points = [from]
  for (let i = 1; i < steps; i++) {
    const t = i / steps, u = 1 - t
    pixel.x = u * u * a.x + 2 * u * t * control.x + t * t * b.x
    pixel.y = u * u * a.y + 2 * u * t * control.y + t * t * b.y
    const point = surfacePoint(scene, pixel, ray)
    if (!point) return undefined
    points.push(point)
  }
  return [...points, to]
}

export function makeLinkSegment(start: MapNode, end: MapNode, lane?: number) {
  if (start.latitude === null || start.longitude === null || end.latitude === null || end.longitude === null) return null
  const startGeo = Cartographic.fromDegrees(start.longitude, start.latitude, LINK_ENDPOINT_HEIGHT)
  const endGeo = Cartographic.fromDegrees(end.longitude, end.latitude, LINK_ENDPOINT_HEIGHT)
  const from = globe.cartographicToCartesian(startGeo), to = globe.cartographicToCartesian(endGeo)
  if (Cartesian3.distance(from, to) < 0.001) return null
  const geodesic = new EllipsoidGeodesic(startGeo, endGeo)
  const distance = geodesic.surfaceDistance
  const isLocal = distance <= LOCAL_DISTANCE
  let base: Cartesian3[] | undefined
  let sides: Cartesian3[] | undefined
  function geographicPoints(separation: number) {
    if (!base) {
      const lift = Math.min(1_150_000, distance * 0.115)
      base = Array.from({ length: 73 }, (_, i) => {
        if (i === 0) return from
        if (i === 72) return to
        const point = geodesic.interpolateUsingFraction(i / 72)
        point.height = LINK_ENDPOINT_HEIGHT + Math.sin(Math.PI * i / 72) * lift
        return globe.cartographicToCartesian(point)
      })
    }
    if (!lane) return base
    if (!sides) {
      const path = base
      sides = path.map((point, i) => {
        if (i === 0 || i === path.length - 1) return Cartesian3.ZERO
        const tangent = Cartesian3.subtract(path[i + 1], path[i - 1], new Cartesian3())
        const side = Cartesian3.cross(tangent, globe.geodeticSurfaceNormal(point, new Cartesian3()), new Cartesian3())
        return Cartesian3.multiplyByScalar(Cartesian3.normalize(side, side), Math.sin(Math.PI * i / 72), side)
      })
    }
    const offset = isLocal ? Math.min(separation, distance * 0.15) * lane / (1 + Math.abs(lane)) : separation * lane
    return base.map((point, i) => Cartesian3.add(point, Cartesian3.multiplyByScalar(sides![i], offset, new Cartesian3()), new Cartesian3()))
  }
  return {
    viewDependent: isLocal,
    positions(scene: Scene, separationMeters: number) {
      if (isLocal) {
        const projected = localPoints(scene, from, to, lane)
        if (projected) return projected
      }
      return geographicPoints(separationMeters)
    }
  }
}
