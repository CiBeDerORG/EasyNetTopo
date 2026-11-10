// 瓦片坐标使用 Web Mercator（EPSG:3857），节点经纬度仍保存为 WGS84。
const MAX_LAT = 85.05112878
const TILE_SIZE = 256

export function mercatorWorldSize(zoom: number): number {
  return TILE_SIZE * Math.pow(2, zoom)
}

export function projectMercator(latitude: number, longitude: number, zoom: number) {
  const length = mercatorWorldSize(zoom)
  const lat = Math.max(-MAX_LAT, Math.min(MAX_LAT, latitude)) * Math.PI / 180
  return {
    x: (longitude + 180) / 360 * length,
    y: (1 - Math.log(Math.tan(lat) + 1 / Math.cos(lat)) / Math.PI) / 2 * length
  }
}

export function unprojectMercator(x: number, y: number, zoom: number) {
  const length = mercatorWorldSize(zoom)
  const latitude = Math.atan(Math.sinh(Math.PI * (1 - 2 * Math.max(0, Math.min(length, y)) / length))) * 180 / Math.PI
  // x=0 对应西经 180°，不是 0°：反算时绝不能额外加 180°。
  // 仅对瓦片横坐标做环绕，以便正常跨越国际日期变更线。
  const longitude = ((x / length * 360) % 360 + 360) % 360 - 180
  return { latitude: Number(latitude.toFixed(6)), longitude: Number(longitude.toFixed(6)) }
}
