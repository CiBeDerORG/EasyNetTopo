import { Cartographic, Rectangle } from 'cesium'

// 使用全部点位的外沿，避免只考虑最远点对时漏掉侧向节点。
export function nodeViewRectangle(nodes: ReadonlyArray<{ latitude: number | null; longitude: number | null }>): Rectangle | null {
  const points = nodes.filter(node => node.latitude !== null && node.longitude !== null &&
    Number.isFinite(node.latitude) && Number.isFinite(node.longitude) &&
    Math.abs(node.latitude) <= 90 && Math.abs(node.longitude) <= 180)
    .map(node => Cartographic.fromDegrees(node.longitude!, node.latitude!))
  return points.length ? Rectangle.fromCartographicArray(points) : null
}

export function paddedCameraHeight(height: number, width: number, viewportHeight: number): number {
  // 给标签、光晕和地图控件留出像素边距，小窗口最多使用一半边距比例。
  const usable = Math.min(Math.max(0.5, 1 - 200 / width), Math.max(0.5, 1 - 140 / viewportHeight))
  return Math.max(30000, height / usable + 3500)
}
