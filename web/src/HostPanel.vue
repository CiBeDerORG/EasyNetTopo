<script setup lang="ts">
import { onMounted, ref } from 'vue'

type Host = {
  id: string
  name: string
  endpoint: string
  agent_id: string
  spki_sha256: string
  paired_at?: string
}
type Snapshot = {
  agent_id: string
  hostname: string
  platform: string
  arch: string
  software_root: string
  observed_at: string
  docker?: { available?: boolean; engine_version?: string; compose_available?: boolean; compose_version?: string; error?: string }
}
type Resource = { resource_id: string; name: string; state: string; kind: string; managed: boolean }
type Inventory = {
  agent_id: string; observed_at: string; status: string
  complete: boolean; resources: Resource[]; error?: string
}
type ContainerStats = {
  container_id: string; state: string; available: boolean
  cpu_percent: number; memory_bytes: number; memory_limit_bytes: number
}
type ResourceStats = {
  agent_id: string; resource_id: string; observed_at: string
  complete: boolean; containers: ContainerStats[]
}
type ResourceLogs = { agent_id: string; resource_id: string; text: string; truncated: boolean }

const hosts = ref<Host[]>([])
const name = ref('')
const endpoint = ref('')
const selectedID = ref('')
const agentID = ref('')
const fingerprint = ref('')
const pairingCode = ref('')
const busy = ref(false)
const error = ref('')
const message = ref('')
const snapshot = ref<Snapshot | null>(null)
const inventory = ref<Inventory | null>(null)
const selectedResourceID = ref('')
const resourceStats = ref<ResourceStats | null>(null)
const resourceLogs = ref<ResourceLogs | null>(null)

async function call<T>(url: string, method = 'GET', payload?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (method !== 'GET') headers['X-ENT-Request'] = '1'
  if (payload !== undefined) headers['Content-Type'] = 'application/json'
  const response = await fetch(url, {
    method, credentials: 'same-origin', cache: 'no-store', headers,
    ...(payload === undefined ? {} : { body: JSON.stringify(payload) })
  })
  if (!response.ok) throw new Error('请求失败（HTTP ' + response.status + '），请检查主机状态与认证配置')
  return (await response.json()) as T
}
async function loadHosts() {
  error.value = ''
  try {
    const result = await call<{ hosts: Host[] }>('/api/v1/hosts')
    hosts.value = result.hosts
  } catch {
    error.value = '无法读取主机列表'
  }
}
async function registerHost() {
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    const host = await call<Host>('/api/v1/hosts', 'POST', {
      name: name.value, endpoint: endpoint.value
    })
    name.value = ''
    endpoint.value = ''
    selectedID.value = host.id
    message.value = '主机登记成功。请从该主机可信终端核验 Agent ID 与 TLS SPKI 指纹后再配对。'
    await loadHosts()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登记失败'
  } finally {
    busy.value = false
  }
}
async function pairHost() {
  if (!selectedID.value) return
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await call<Host>('/api/v1/hosts/' + selectedID.value + '/pair', 'POST', {
      agent_id: agentID.value.trim(), spki_sha256: fingerprint.value.trim(),
      pairing_code: pairingCode.value.trim()
    })
    pairingCode.value = ''
    message.value = '配对成功。Agent 已将此控制端设为唯一有效控制中心。'
    await loadHosts()
  } catch {
    error.value = '配对失败：请确认 Agent 终端的真实指纹、UUID、有效一次性码和主机 IP。'
  } finally {
    busy.value = false
  }
}
async function queryHost(id: string, type: 'snapshot' | 'docker') {
  busy.value = true
  error.value = ''
  try {
    if (type === 'snapshot') {
      snapshot.value = await call<Snapshot>('/api/v1/hosts/' + id + '/snapshot')
    } else {
      inventory.value = await call<Inventory>('/api/v1/hosts/' + id + '/docker')
      selectedResourceID.value = ''
      resourceStats.value = null
      resourceLogs.value = null
    }
    selectedID.value = id
  } catch {
    error.value = 'Agent 查询失败。网络/授权错误不代表主机或容器已被删除。'
  } finally {
    busy.value = false
  }
}
async function queryResource(resourceID: string, detail: 'logs' | 'stats') {
  if (!selectedID.value || !inventory.value?.complete) return
  busy.value = true
  error.value = ''
  selectedResourceID.value = resourceID
  resourceStats.value = null
  resourceLogs.value = null
  try {
    const path = '/api/v1/hosts/' + selectedID.value + '/docker/resources/' + encodeURIComponent(resourceID) + '/' + detail
    if (detail === 'logs') {
      resourceLogs.value = await call<ResourceLogs>(path + '?tail=100')
    } else {
      resourceStats.value = await call<ResourceStats>(path)
    }
  } catch {
    error.value = '资源详情查询失败，不能据此推断容器已删除。'
  } finally {
    busy.value = false
  }
}
function bytes(value: number): string {
  if (!Number.isFinite(value) || value < 0) return '未知'
  return (value / 1024 / 1024).toFixed(1) + ' MiB'
}
function selectHost(id: string) {
  selectedID.value = id
  snapshot.value = null
  inventory.value = null
  selectedResourceID.value = ''
  resourceStats.value = null
  resourceLogs.value = null
  agentID.value = ''
  fingerprint.value = ''
  pairingCode.value = ''
}
onMounted(loadHosts)
</script>

<template>
  <section class="panel">
    <div class="heading">
      <div>
        <h2>主机与 EasyHostAgent</h2>
        <p class="muted">使用 IP:port + HTTPS 直接连接。配对前必须在 Agent 所在机器可信终端核对指纹。</p>
      </div>
      <button type="button" class="secondary" @click="loadHosts">刷新主机</button>
    </div>

    <form class="form" @submit.prevent="registerHost">
      <label for="host-name">主机名称</label>
      <input id="host-name" v-model="name" placeholder="例如：fnOS NAS" maxlength="100" required />
      <label for="host-address">Agent IP:端口</label>
      <input id="host-address" v-model="endpoint" placeholder="例如：192.168.1.20:7014" required />
      <button type="submit" :disabled="busy">添加主机</button>
    </form>

    <div v-if="hosts.length === 0" class="muted">当前没有登记的主机。</div>
    <ul v-else class="hosts">
      <li v-for="host in hosts" :key="host.id">
        <div class="row">
          <div>
            <strong>{{ host.name }}</strong>
            <p class="muted">{{ host.endpoint }} · {{ host.agent_id ? '已配对' : '未配对' }}</p>
          </div>
          <div class="actions">
            <button type="button" class="secondary" :disabled="busy" @click="selectHost(host.id)">选择</button>
            <button v-if="host.agent_id" type="button" :disabled="busy" @click="queryHost(host.id, 'snapshot')">主机信息</button>
            <button v-if="host.agent_id" type="button" :disabled="busy" @click="queryHost(host.id, 'docker')">Docker 资源</button>
          </div>
        </div>
        <small v-if="host.agent_id" class="muted">Agent UUID：{{ host.agent_id }}</small>
      </li>
    </ul>

    <form v-if="selectedID" class="form pair" @submit.prevent="pairHost">
      <h3>配对或重新配对选中的主机</h3>
      <p class="muted">配对前，必须从该主机本地可信终端获取 Agent UUID、TLS SPKI SHA-256 指纹与有效的一次性配对码。不要仅信任网络返回的指纹；重新配对成功会撤销 Agent 原控制端的授权。</p>
      <label for="agent-id">Agent UUID</label>
      <input id="agent-id" v-model="agentID" placeholder="终端核对后的 UUID" autocomplete="off" required />
      <label for="agent-fingerprint">TLS SPKI SHA-256（64 位十六进制）</label>
      <input id="agent-fingerprint" v-model="fingerprint" placeholder="终端核对后的公钥指纹" autocomplete="off" spellcheck="false" required />
      <label for="pair-code">一次性配对码</label>
      <input id="pair-code" v-model="pairingCode" type="password" autocomplete="off" placeholder="主机本地生成的 32 位十六进制码" required />
      <button type="submit" :disabled="busy">{{ busy ? '处理中…' : '安全配对' }}</button>
    </form>

    <section v-if="snapshot" class="result">
      <h3>主机实时快照</h3>
      <p><strong>{{ snapshot.hostname }}</strong> · {{ snapshot.platform }} / {{ snapshot.arch }}</p>
      <p>软件目录：<code>{{ snapshot.software_root }}</code></p>
      <p v-if="snapshot.docker">
        Docker：{{ snapshot.docker.available ? '已连接' : '暂不可用' }}
        <span v-if="snapshot.docker.engine_version"> · Engine {{ snapshot.docker.engine_version }}</span>
        <span v-if="snapshot.docker.compose_available"> · Compose {{ snapshot.docker.compose_version }}</span>
      </p>
      <p class="muted">采集时间：{{ snapshot.observed_at }}</p>
    </section>
    <section v-if="inventory" class="result">
      <div class="heading">
        <h3>Docker 资源</h3>
        <span :class="inventory.complete ? 'ok' : 'warning'">{{ inventory.complete ? '查询完整' : '查询不完整，不据此删除资源' }}</span>
      </div>
      <p class="muted">采集时间：{{ inventory.observed_at }}</p>
      <div v-if="!inventory.complete" class="warning">{{ inventory.error || 'Docker Engine 当前不可用' }}，保留旧的资源认知，不将空列表作为删除依据。</div>
      <div v-for="resource in inventory.resources" :key="resource.resource_id" class="resource">
        <div>
          <strong>{{ resource.name }}</strong>
          <p class="muted">{{ resource.kind }} · {{ resource.state }} · {{ resource.managed ? '受管资源' : '外部资源（只读）' }}</p>
        </div>
        <div class="actions">
          <button type="button" class="secondary" :disabled="busy || !inventory.complete" @click="queryResource(resource.resource_id, 'stats')">资源占用</button>
          <button type="button" class="secondary" :disabled="busy || !inventory.complete" @click="queryResource(resource.resource_id, 'logs')">查看日志</button>
        </div>
        <div v-if="selectedResourceID === resource.resource_id && resourceStats" class="detail">
          <p v-for="item in resourceStats.containers" :key="item.container_id">
            <code>{{ item.container_id.slice(0, 12) }}</code> · {{ item.state }}
            <span v-if="item.available"> · CPU {{ item.cpu_percent.toFixed(1) }}% · 内存 {{ bytes(item.memory_bytes) }} / {{ bytes(item.memory_limit_bytes) }}</span>
            <span v-else> · 无实时统计</span>
          </p>
          <p class="muted">按需采集于 {{ resourceStats.observed_at }}</p>
        </div>
        <div v-if="selectedResourceID === resource.resource_id && resourceLogs" class="detail">
          <p v-if="resourceLogs.truncated" class="warning">日志已截断，仅显示有限内容</p>
          <pre class="log">{{ resourceLogs.text }}</pre>
        </div>
      </div>
    </section>
    <p v-if="message" class="ok" role="status">{{ message }}</p>
    <p v-if="error" class="warning" role="alert">{{ error }}</p>
  </section>
</template>

<style scoped>
.panel{border:1px solid #e4eaf1;background:#fff;border-radius:14px;padding:24px;margin:22px 0}
.heading,.row{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
h2{font-size:20px;margin:0 0 10px}h3{font-size:17px}
.muted{font-size:13px;color:#65778b}
.form{display:grid;grid-template-columns:160px minmax(0,1fr);gap:12px;align-items:center;max-width:840px;padding:20px 0}
.form h3,.form p{grid-column:1/-1;margin:0}
.form button{grid-column:2;justify-self:start}
label{font-size:13px;font-weight:600}
input{min-width:0;padding:10px 12px;border:1px solid #c8d2df;border-radius:8px;font:inherit}
button{background:#176a9b;color:#fff;border:0;border-radius:8px;padding:10px 13px;cursor:pointer}
.secondary{background:#e9f3f9;color:#176a9b}
button:disabled{opacity:.55;cursor:not-allowed}
.hosts{list-style:none;padding:0;margin:15px 0}
.hosts li{padding:14px 0;border-top:1px solid #e4eaf1}
.row p{margin:4px 0}
.actions{display:flex;gap:8px;flex-wrap:wrap}
.result{background:#f6f9fc;padding:18px;border-radius:10px;margin-top:16px}
.resource{border-top:1px solid #e3eaf1;padding:10px 0;display:flex;gap:15px;justify-content:space-between;align-items:center;flex-wrap:wrap}
.detail{flex-basis:100%;min-width:0;padding:8px 12px;background:#fff;border-radius:8px}
.detail p{margin:6px 0;font-size:13px}
.log{max-height:320px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px}
.ok{color:#247852}.warning{color:#b25216}
code{overflow-wrap:anywhere}
@media(max-width:650px){.panel{padding:14px}.form{grid-template-columns:1fr}.form button{grid-column:1}}
</style>
