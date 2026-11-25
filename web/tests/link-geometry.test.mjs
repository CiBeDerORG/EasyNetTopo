import test from 'node:test'
import assert from 'node:assert/strict'
import { Cartesian2, Cartesian3 } from 'cesium'
import { LINK_ENDPOINT_HEIGHT, makeLinkSegment, screenCurveControl } from '../src/features/network-map/link-geometry.ts'

function pixelAt(a, b, control, t) {
  const u = 1 - t
  return {
    x: u * u * a.x + 2 * u * t * control.x + t * t * b.x,
    y: u * u * a.y + 2 * u * t * control.y + t * t * b.y
  }
}

test('同一对节点放大时像素弓高递增，缩小时收平，最大值受短边限制', () => {
  for (const [width, height] of [[1280, 600], [360, 280]]) {
    const spans = [4, 15, 55, 140, width - 30]
    const bows = spans.map(span => {
      const a = { x: (width - span) / 2, y: height / 2 }
      const b = { x: (width + span) / 2, y: height / 2 }
      return pixelAt(a, b, screenCurveControl(a, b, width, height), 0.5).y - a.y
    })
    for (let i = 1; i < bows.length; i++) assert.ok(bows[i] > bows[i - 1])
    assert.ok(bows[0] < 1 && bows.at(-1) <= height * 0.1)
  }
})

test('靠近画布边缘和角落时整条曲线仍在可见范围内，端点不移位', () => {
  const width = 390, height = 295
  for (const [a, b] of [
    [{ x: 4, y: 6 }, { x: 360, y: 11 }],
    [{ x: 28, y: 281 }, { x: 374, y: 283 }],
    [{ x: 8, y: 12 }, { x: 13, y: 285 }],
    [{ x: 380, y: 4 }, { x: 388, y: 281 }],
    [{ x: 4, y: 2 }, { x: 389, y: 294 }]
  ]) {
    for (const lane of [undefined, -2, -0.5, 0, 0.5, 2]) {
      const control = screenCurveControl(a, b, width, height, lane)
      assert.deepEqual(pixelAt(a, b, control, 0), a)
      assert.deepEqual(pixelAt(a, b, control, 1), b)
      for (let i = 0; i <= 50; i++) {
        const p = pixelAt(a, b, control, i / 50)
        assert.ok(p.x >= 0 && p.x <= width && p.y >= 0 && p.y <= height)
      }
    }
  }
  // 两点都贴在下边缘时，单线向画面内弯，依然可以看出弧度。
  const a = { x: 75, y: height }, b = { x: 315, y: height }
  assert.ok(pixelAt(a, b, screenCurveControl(a, b, width, height), 0.5).y < height - 1)
})

test('短线路共享端点时车道次序稳定，反转端点并反转车道产生同一个控制点', () => {
  const a = { x: 60, y: 50 }, b = { x: 156, y: 50 }
  const lanes = [-2, -1, 0, 1, 2]
  const controls = lanes.map(lane => screenCurveControl(a, b, 400, 250, lane))
  for (let i = 1; i < controls.length; i++) assert.ok(controls[i].y > controls[i - 1].y)
  assert.deepEqual(screenCurveControl(b, a, 400, 250, -2), controls.at(-1))
})

test('可见的短线路投回地球表面后，弧顶仍显示在计算出的画布位置', () => {
  const segment = makeLinkSegment({ latitude: 0, longitude: -0.01 }, { latitude: 0, longitude: 0.01 })
  const eye = Cartesian3.fromDegrees(0, 0, 150_000)
  const project = point => new Cartesian2(200 + point.y / 10, 150 - point.z / 10)
  const scene = {
    canvas: { clientWidth: 400, clientHeight: 300 },
    cartesianToCanvasCoordinates: project,
    camera: {
      positionWC: eye,
      getPickRay(pixel, ray) {
        ray.origin = new Cartesian3(eye.x, (pixel.x - 200) * 10, (150 - pixel.y) * 10)
        ray.direction = new Cartesian3(-1, 0, 0)
        return ray
      }
    }
  }
  const points = segment.positions(scene, 900).map(project)
  const a = points[0], b = points.at(-1)
  const control = screenCurveControl(a, b, 400, 300)
  points.forEach((point, index) => {
    const expected = pixelAt(a, b, control, index / (points.length - 1))
    assert.ok(Math.hypot(expected.x - point.x, expected.y - point.y) < 1e-6)
  })
})

test('短距离线路回退以实际距离抬升，两端与节点贴合，重合或缺坐标的段可跳过', () => {
  const a = { latitude: 31, longitude: 121 }, b = { latitude: 31, longitude: 121.01 }
  assert.equal(makeLinkSegment(a, a), null)
  assert.equal(makeLinkSegment(a, { latitude: null, longitude: 121 }), null)
  const segment = makeLinkSegment(a, b)
  assert.equal(segment.viewDependent, true)
  // 地球背面的节点使用原地理路径，不投射到可见半球。
  const points = segment.positions({ camera: { positionWC: Cartesian3.ZERO } }, 240000)
  const from = Cartesian3.fromDegrees(a.longitude, a.latitude, LINK_ENDPOINT_HEIGHT)
  const to = Cartesian3.fromDegrees(b.longitude, b.latitude, LINK_ENDPOINT_HEIGHT)
  assert.ok(Cartesian3.distance(points[0], from) < 1e-7)
  assert.ok(Cartesian3.distance(points.at(-1), to) < 1e-7)
  assert.ok(Cartesian3.distance(points[36], Cartesian3.midpoint(from, to, new Cartesian3())) < 130)
})
