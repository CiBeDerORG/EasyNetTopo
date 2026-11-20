import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { apiRequest } from '../../lib/api-client'
import type { Host } from '../host-store/types'
import { mergedNetworkRows, publicIPv4Candidate, type NetworkRow } from '../network-nodes/data'
import { networkNodeAttributeKey, networkNodeIdentityKey } from '../network-nodes/identity'
import { listNodeAttributes, type NodeAttributes } from '../network-nodes/attributes'
import { getNodeDevices } from './api'
import type { Device } from './types'

export type LiveNodeOption = {
  networkKey: string
  peerId: string
  hostname: string
  identityKey?: string
  label: string
  instance: string
  ip: string
  isOnline: true
  row: NetworkRow
  hostId: string
}
export function nodeValue(node: { hostname: string }): string {
  const hostname = node.hostname.trim().toLowerCase()
  return hostname ? 'hostname:' + hostname : ''
}

export function useDeviceNetwork(hosts: () => Host[]) {
  const attributesQuery = useQuery({
    queryKey: ['network-node-attributes'], queryFn: listNodeAttributes, retry: 0
  })
  const nodeDevicesQuery = useQuery({
    queryKey: ['device-node-devices'], queryFn: getNodeDevices, retry: 0, staleTime: 10_000
  })
  const networkEntries = computed(() => hosts().filter(host => host.network_entry))
  const nodesQuery = useQuery({
    queryKey: computed(() => ['device-network-nodes', networkEntries.value.map(host => host.id).join('|')]),
    queryFn: async ({ signal }) => {
      const settled = await Promise.allSettled(networkEntries.value.map(async host => {
        const response = await apiRequest<{ data: unknown }>(
          '/api/v1/hosts/' + encodeURIComponent(host.id) + '/network/route', { signal })
        return mergedNetworkRows(response.data).filter(row => row.id !== '—').map(row => ({
          row, hostId: host.id, networkKey: networkNodeAttributeKey(row, host.id)
        }))
      }))
      const nodes = settled.flatMap(result => result.status === 'fulfilled' ? result.value : [])
      const errors = settled.filter(result => result.status === 'rejected').length
      if (!nodes.length && errors === settled.length && errors > 0) {
        throw new Error('暂时无法读取现有网络节点，请检查组网入口状态')
      }
      const peersByIdentity = new Map<string, Set<string>>()
      for (const node of nodes) {
        const identity = networkNodeIdentityKey(node.networkKey, node.row.rawHostname, node.row.id)
        const peers = peersByIdentity.get(identity) || new Set<string>()
        peers.add(node.row.id)
        peersByIdentity.set(identity, peers)
      }
      // 如果同一网络内原始主机名冲突，不将不同 Peer ID 静默合并。
      const unique = new Map<string, { row: NetworkRow; hostId: string; networkKey: string; identityKey: string }>()
      for (const node of nodes) {
        const identity = networkNodeIdentityKey(node.networkKey, node.row.rawHostname, node.row.id)
        const key = (peersByIdentity.get(identity)?.size || 0) > 1
          ? networkNodeIdentityKey(node.networkKey, '', node.row.id) : identity
        if (!unique.has(key)) unique.set(key, { ...node, identityKey: key })
      }
      return { nodes: [...unique.values()], localNodes: nodes.filter(node => node.row.kind === '入口实例'), errors }
    },
    enabled: computed(() => networkEntries.value.length > 0),
    retry: 0,
    staleTime: 10_000,
    refetchInterval: 15_000
  })
  const attributesByKey = computed(() => new Map<string, NodeAttributes>(
    (attributesQuery.data.value?.attributes || []).map(attr => [attr.network_key + '|' + attr.peer_id, attr])
  ))
  const attributesByHostname = computed(() => new Map<string, NodeAttributes>(
    (attributesQuery.data.value?.attributes || []).filter(attr => attr.hostname)
      .map(attr => [attr.network_key + '|' + attr.hostname!.toLocaleLowerCase(), attr])
  ))
  const liveNodeOptions = computed<LiveNodeOption[]>(() => (nodesQuery.data.value?.nodes || []).map(node => {
    const hostname = node.row.rawHostname || ''
    const attr = (hostname && node.identityKey === networkNodeIdentityKey(node.networkKey, hostname, node.row.id)
      ? attributesByHostname.value.get(node.networkKey + '|' + hostname.toLocaleLowerCase()) : undefined)
      || attributesByKey.value.get(node.networkKey + '|' + node.row.id)
    return {
      networkKey: node.networkKey, peerId: node.row.id, hostname,
      identityKey: node.identityKey,
      label: attr?.alias || node.row.hostname, instance: node.row.instance, ip: node.row.ip,
      isOnline: true, row: node.row, hostId: node.hostId
    }
  }))
  const liveNodesByHostname = computed(() => {
    const groups = new Map<string, LiveNodeOption[]>()
    for (const node of liveNodeOptions.value) {
      const key = nodeValue(node)
      if (key) groups.set(key, [...(groups.get(key) || []), node])
    }
    return groups
  })
  const conflictingNodeNames = computed(() => {
    const conflicts = new Set([...liveNodesByHostname.value].filter(([, nodes]) => nodes.length > 1).map(([key]) => key))
    const seen = new Set<string>()
    for (const device of nodeDevicesQuery.data.value || []) {
      const key = nodeValue({ hostname: device.node_hostname || '' })
      if (!key) continue
      if (seen.has(key)) conflicts.add(key)
      seen.add(key)
    }
    return conflicts
  })
  function deviceNetworkIP(device: Device) {
    const key = nodeValue({ hostname: device.node_hostname || '' })
    if (!key || conflictingNodeNames.value.has(key)) return ''
    return liveNodesByHostname.value.get(key)?.[0]?.ip || ''
  }
  const serviceEntryPublicIPs = computed(() => {
    const result: Record<string, Record<string, string>> = {}
    for (const { row, hostId, networkKey } of nodesQuery.data.value?.localNodes || []) {
      const rawName = row.rawHostname?.toLocaleLowerCase()
      const attr = (rawName ? attributesByHostname.value.get(networkKey + '|' + rawName) : undefined)
        || attributesByKey.value.get(networkKey + '|' + row.id)
      const candidates = new Set(row.publicIP.split('、').map(publicIPv4Candidate).filter(Boolean))
      const address = attr?.has_public_ip && attr.public_ip
        ? attr.public_ip : candidates.size === 1 ? [...candidates][0] : ''
      if (address) (result[hostId] ??= {})[row.instanceID] = address
    }
    return result
  })
  function serviceDeviceNetworkIP(device: Device) {
    if (device.network_ip) return device.network_ip
    const nodeIP = deviceNetworkIP(device)
    return device.is_node_device || (nodeIP && nodeIP === device.ip_address) ? nodeIP : ''
  }
  return {
    networkEntries, nodesQuery, nodeDevicesQuery, attributesQuery, attributesByKey, liveNodeOptions,
    conflictingNodeNames, deviceNetworkIP, serviceDeviceNetworkIP, serviceEntryPublicIPs
  }
}
