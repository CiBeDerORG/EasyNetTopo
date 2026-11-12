import test from 'node:test'
import assert from 'node:assert/strict'
import { Rectangle, Cartographic, Math as CesiumMath } from 'cesium'
import { nodeViewRectangle, paddedCameraHeight } from '../src/features/network-map/camera-fit.ts'

const point = (longitude, latitude) => ({ longitude, latitude })
test('范围覆盖最远两端及侧向节点，与输入顺序无关', () => {
  const nodes = [point(118, 29), point(122, 31), point(120, 33)]
  const rectangle = nodeViewRectangle(nodes)
  for (const node of nodes) assert.ok(Rectangle.contains(rectangle, Cartographic.fromDegrees(node.longitude, node.latitude)))
  assert.deepEqual(rectangle, nodeViewRectangle([...nodes].reverse()))
})
test('跨日期变更线使用短范围，不缩到整个世界', () => {
  const rectangle = nodeViewRectangle([point(179, 20), point(-179, 22)])
  assert.ok(Math.abs(CesiumMath.toDegrees(rectangle.width) - 2) < 1e-8)
})
test('无坐标不定位，单点和重合点保留有效中心及最小高度', () => {
  assert.equal(nodeViewRectangle([point(null, null), point(NaN, 2)]), null)
  const rectangle = nodeViewRectangle([point(120, 30), point(120, 30)])
  assert.equal(rectangle.width, 0)
  assert.equal(rectangle.height, 0)
  assert.equal(paddedCameraHeight(0, 1000, 600), 30000)
})
test('窄窗口增加标签余量，正常窗口保持紧凑', () => {
  const wide = paddedCameraHeight(100000, 1200, 800)
  const narrow = paddedCameraHeight(100000, 360, 800)
  assert.ok(wide > 100000 && wide < 140000)
  assert.ok(narrow > wide)
})
