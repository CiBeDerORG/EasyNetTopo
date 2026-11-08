import { computed, type ComputedRef, type Ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { getApps, getDockerInventory, getHostSnapshot, getHosts, getSources, getAppStatuses, getExecutionTasks } from '../api'
import { appKey } from '../rules'
import type { AppStatus, CatalogApp } from '../types'

export function useHostStoreData(
  selectedHostId: ComputedRef<string>,
  authenticated: ComputedRef<boolean>,
  shouldPoll: ComputedRef<boolean>,
  agentRestarting: ComputedRef<boolean>
) {
  const tasksQuery = useQuery({
    queryKey: ['execution-tasks'],
    queryFn: getExecutionTasks,
    enabled: authenticated,
    retry: 0,
    refetchInterval: 2500
  })
  const hostsQuery = useQuery({
    queryKey: ['hosts'], queryFn: getHosts, enabled: authenticated, staleTime: 10_000
  })
  const sourcesQuery = useQuery({
    queryKey: ['catalog-sources'], queryFn: getSources, enabled: authenticated, staleTime: 15_000
  })
  const appsQuery = useQuery({
    queryKey: ['catalog-apps'], queryFn: getApps, enabled: authenticated, staleTime: 15_000
  })

  const tasks = computed(() => tasksQuery.data.value || [])
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
    refetchInterval: computed(() => agentRestarting.value || tasks.value.some(task =>
      task.host_id === selectedHostId.value && task.source_id === 'native' &&
      task.app_id === 'easyhostagent' && ['queued', 'preparing', 'running'].includes(task.state)
    ) ? 3000 : false)
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
    // 应用卡片也需要使用同一份 Docker 快照展示宿主机映射端口，
    // 不再等到用户打开资源抽屉时才读取。
    enabled: computed(() => authenticated.value && !!selectedHost.value?.agent_id),
    retry: 0,
    staleTime: 10_000
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
    hostsQuery, tasksQuery, sourcesQuery, appsQuery, snapshotQuery, statusQuery, dockerQuery,
    hosts, tasks, sources, catalogApps, selectedHost, rows, statusByKey, visibleApps
  }
}
