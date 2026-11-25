import type { NetworkRow } from './data'

// 与网络节点属性表共用同一身份规则；无 network_name 的实例必须按入口主机隔离。
export function networkNodeAttributeKey(row: NetworkRow, entryHostId: string): string {
  return row.networkKey.startsWith('instance:')
    ? 'entry:' + entryHostId + ':' + row.networkKey
    : row.networkKey
}


// 节点持久身份只来自组网作用域 + EasyTier 原始主机名。
// Peer ID 仅在原始主机名缺失时作为临时标识，不能以用户可编辑别名代替 hostname。
export function networkNodeIdentityKey(networkKey: string, hostname: string | undefined, peerID: string): string {
  const raw = (hostname || '').trim()
  const normalized = raw && raw !== '—' ? raw.toLocaleLowerCase() : ''
  return JSON.stringify([networkKey, normalized ? 'hostname' : 'peer', normalized || peerID])
}

export function stableNetworkRowKey(row: NetworkRow, entryHostId: string): string {
  return networkNodeIdentityKey(networkNodeAttributeKey(row, entryHostId), row.rawHostname, row.id)
}

// 同一网络若同时报告同名但不同的真实节点，禁止用 hostname 把它们错误合成一个节点。
// 冲突仅在本次快照中使用运行时 key 区分，后台同步会单独拒绝含歧义的批次。
export function stableNetworkRows(rows: NetworkRow[], entryHostId: string): NetworkRow[] {
  const counts = new Map<string, number>()
  for (const row of rows) {
    const key = stableNetworkRowKey(row, entryHostId)
    counts.set(key, (counts.get(key) || 0) + 1)
  }
  return rows.map(row => {
    const key = stableNetworkRowKey(row, entryHostId)
    return {
      ...row,
      key: (counts.get(key) || 0) > 1
        ? JSON.stringify([networkNodeAttributeKey(row, entryHostId), 'conflict', row.instanceID, row.id, row.kind])
        : key
    }
  })
}

export function nodeEditingIssue(row: NetworkRow, entryId: string): string {
  if (!row.id || row.id === '—') return '该节点尚未报告有效节点 ID。'
  // 保存接口按网络作用域和原始主机名换绑 Peer ID，同名冲突时不能只靠运行时 ID 写入。
  if (row.key !== stableNetworkRowKey(row, entryId)) return '当前网络内存在身份冲突，请先确认节点主机名。'
  return ''
}
