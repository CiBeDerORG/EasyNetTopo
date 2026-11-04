import { computed, type ComputedRef, type Ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { getApps, getDockerInventory, getHostSnapshot, getHosts, getSources, getAppStatuses } from '../api'
import { appKey } from '../rules'
import type { AppStatus, CatalogApp } from '../types'

export function useHostStoreData(
  selectedHostId: ComputedRef<string>,
  authenticated: ComputedRef<boolean>,
  shouldPoll: ComputedRef<boolean>,
  dockerOpen: Ref<boolean>,
  agentRestarting: ComputedRef<boolean>
) {
  const hostsQuery = useQuery({
    queryKey: ['hosts'], queryFn: getHosts, enabled: authenticated, staleTime: 10_000
  })
  const sourcesQuery = useQuery({
    queryKey: ['catalog-sources'], queryFn: getSources, enabled: authenticated, staleTime: 15_000
  })
  const appsQuery = useQuery({
    queryKey: ['catalog-apps'], queryFn: getApps, enabled: authenticated, staleTime: 15_000
  })

  const hosts = computed(() => hostsQuery.data.value || [])
  const sources = computed(() => sourcesQuery.data.value || [])
  const catalogApps = computed(() => appsQuery.data.value || [])
  const selectedHost = computed(() =>
    hosts.value.find(host => host.id === selectedHostId.value) || null
  )
  const appSignature = computed(() => catalogApps.value.map(appKey).join('|'))

  const snapshotQuery = useQuery({
    queryKey: computed(() => ['host-snapshot', selectedHostId.value]),
    queryFn: () => getHostSnapshot(selectedHostId.value),
    enabled: computed(() => authenticated.value && !!selectedHostId.value && !!selectedHost.value?.agent_id),
    retry: 0,
    staleTime: 10_000,
    refetchInterval: computed(() => agentRestarting.value ? 3000 : false)
  })
  const statusQuery = useQuery({
    queryKey: computed(() => ['host-app-status', selectedHostId.value, appSignature.value]),
    queryFn: () => getAppStatuses(selectedHostId.value, catalogApps.value),
    enabled: computed(() => authenticated.value && !!selectedHostId.value &&
      !!selectedHost.value?.agent_id && appsQuery.isSuccess.value),
    retry: 0,
    refetchInterval: query => shouldPoll.value ||
      (query.state.data as AppStatus[] | undefined)?.some(row => row.result === 'running') ? 1800 : false
  })
  const dockerQuery = useQuery({
    queryKey: computed(() => ['host-docker', selectedHostId.value]),
    queryFn: () => getDockerInventory(selectedHostId.value),
    enabled: computed(() => authenticated.value && dockerOpen.value && !!selectedHost.value?.agent_id),
    retry: 0,
    staleTime: 0
  })

  const rows = computed(() => statusQuery.data.value || [])
  const statusByKey = computed(() => Object.fromEntries(rows.value.map(row => [appKey(row), row])))
  const visibleApps = computed<CatalogApp[]>(() => {
    const known = new Set(catalogApps.value.map(appKey))
    const local = rows.value.filter(row => row.managed && !known.has(appKey(row))).map(row => ({
      source_id: row.source_id, app_id: row.app_id, name: row.app_id,
      summary: '本机安装记录', category: '本机', revision: '', manifest_sha256: ''
    }))
    return [...catalogApps.value, ...local]
  })

  return {
    hostsQuery, sourcesQuery, appsQuery, snapshotQuery, statusQuery, dockerQuery,
    hosts, sources, catalogApps, selectedHost, rows, statusByKey, visibleApps
  }
}
