import type { NetworkRow } from './data'

// 与网络节点属性表共用同一身份规则；无 network_name 的实例必须按入口主机隔离。
export function networkNodeAttributeKey(row: NetworkRow, entryHostId: string): string {
  return row.networkKey.startsWith('instance:')
    ? 'entry:' + entryHostId + ':' + row.networkKey
    : row.networkKey
}
