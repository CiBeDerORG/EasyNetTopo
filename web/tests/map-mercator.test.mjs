import assert from 'node:assert/strict'
import test from 'node:test'
import { mercatorWorldSize, projectMercator, unprojectMercator } from '../src/features/network-nodes/map-mercator.ts'

test('Web Mercator 经纬度正反转换不发生 180° 经度跳转', () => {
  for (const [lat, lon] of [
    [39.9042, 116.4074],   // 北京
    [31.2304, 121.4737],   // 上海
    [22.5431, 114.0579],   // 深圳
    [37.7749, -122.4194],  // 旧金山
    [1.3521, 103.8198],    // 新加坡
    [-33.8688, 151.2093]  // 悉尼
  ]) {
    for (const zoom of [2, 4, 8, 12, 17]) {
      const point = projectMercator(lat, lon, zoom)
      const actual = unprojectMercator(point.x, point.y, zoom)
      assert.ok(Math.abs(actual.latitude - lat) < 0.000001,
        `纬度跳转：${lat}, zoom=${zoom}, actual=${actual.latitude}`)
      assert.ok(Math.abs(actual.longitude - lon) < 0.000001,
        `经度跳转：${lon}, zoom=${zoom}, actual=${actual.longitude}`)
    }
  }
})

test('点击或小幅拖动地图不会从中国跳到美洲', () => {
  const zoom = 4
  const center = projectMercator(31.2304, 121.4737, zoom)
  const clicked = unprojectMercator(center.x + 10, center.y + 8, zoom)
  const dragged = unprojectMercator(center.x - 30, center.y + 12, zoom)
  for (const actual of [clicked, dragged]) {
    assert.ok(actual.longitude > 110 && actual.longitude < 130, JSON.stringify(actual))
    assert.ok(actual.latitude > 20 && actual.latitude < 40, JSON.stringify(actual))
  }
})

test('横向拖动越过国际日期变更线能够连续环绕', () => {
  for (const zoom of [2, 6, 12]) {
    const origin = projectMercator(0, 179, zoom)
    const wrapped = unprojectMercator(origin.x + mercatorWorldSize(zoom), origin.y, zoom)
    assert.equal(wrapped.longitude, 179)
    const east = unprojectMercator(origin.x + mercatorWorldSize(zoom) / 60, origin.y, zoom)
    assert.ok(east.longitude < -170, JSON.stringify(east))
  }
})
