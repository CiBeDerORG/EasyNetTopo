<script setup lang="ts">
import { onMounted, ref } from 'vue'
const emit = defineEmits<{ paired: []; changed: [] }>()

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
  docker?: { available?: boolean; status?: string }
}
type Resource = { resource_id: string; name: string; state: string; kind: string; managed: boolean }
type Inventory = {
  agent_id: string; observed_at: string; status: string
  complete: boolean; resources: Resource[]; error?: string
}

const hosts = ref<Host[]>([])
const name = ref('')
const endpoint = ref('')
const selectedID = ref('')
const agentID = ref('')
const fingerprint = ref('')
const pairingCode = ref('')
const editingID = ref('')
const editName = ref('')
const editEndpoint = ref('')
const busy = ref(false)
const error = ref('')
const message = ref('')
const snapshot = ref<Snapshot | null>(null)
const inventory = ref<Inventory | null>(null)

async function call<T>(url: string, method = 'GET', payload?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (method !== 'GET') headers['X-ENT-Request'] = '1'
  if (payload !== undefined) headers['Content-Type'] = 'application/json'
  const response = await fetch(url, {
    method, credentials: 'same-origin', cache: 'no-store', headers,
    ...(payload === undefined ? {} : { body: JSON.stringify(payload) })
  })
  if (!response.ok) {
    if (url.endsWith('/pair')) {
      const detail = (await response.json().catch(() => null)) as
        | { code?: string; stage?: string; message?: string }
        | null
      if (detail && typeof detail.message === 'string' && typeof detail.code === 'string') {
        throw new Error(detail.message + '（' + detail.code + ' / ' + (detail.stage || 'unknown') + '）')
      }
    }
    throw new Error('请求失败（HTTP ' + response.status + '），请检查主机状态与认证配置')
  }
  if (response.status === 204) return undefined as T
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
    selectHost(host.id)
    editingID.value = host.id
    editName.value = host.name
    editEndpoint.value = host.endpoint
    message.value = '主机登记成功。请在该主机展开的编辑区域输入本机终端的 UUID、指纹和一次性配对码。'
    await loadHosts()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登记失败'
  } finally {
    busy.value = false
  }
}
async function pairHost() {
  if (!selectedID.value) return
  const registered = hosts.value.find(host => host.id === selectedID.value)
  if (registered && (editName.value.trim() !== registered.name ||
      editEndpoint.value.trim() !== registered.endpoint)) {
    error.value = '请先保存名称或连接地址，再进行配对。'
    return
  }
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await call<Host>('/api/v1/hosts/' + selectedID.value + '/pair', 'POST', {
      agent_id: agentID.value.trim(), spki_sha256: fingerprint.value.trim(),
      pairing_code: pairingCode.value.trim()
    })
    pairingCode.value = ''
    editingID.value = ''
    emit('paired')
    message.value = '配对成功。Agent 已将此控制端设为唯一有效控制中心。'
    await loadHosts()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '配对失败，请检查 EasyNetTopo 服务端日志'
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
    }
    selectedID.value = id
  } catch {
    error.value = 'Agent 查询失败。网络/授权错误不代表主机或容器已被删除。'
  } finally {
    busy.value = false
  }
}
function selectHost(id: string) {
  selectedID.value = id
  editingID.value = ''
  snapshot.value = null
  inventory.value = null
  agentID.value = ''
  fingerprint.value = ''
  pairingCode.value = ''
}
function toggleEditor(host: Host) {
  if (editingID.value === host.id) {
    editingID.value = ''
    pairingCode.value = ''
    return
  }
  selectHost(host.id)
  editingID.value = host.id
  editName.value = host.name
  editEndpoint.value = host.endpoint
  agentID.value = host.agent_id
  fingerprint.value = host.spki_sha256
}
async function saveHost() {
  if (!editingID.value) return
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await call<Host>('/api/v1/hosts/' + editingID.value, 'PATCH', {
      name: editName.value, endpoint: editEndpoint.value.trim()
    })
    await loadHosts()
    emit('changed')
    message.value = '主机名称和连接地址已保存，原有配对凭据保留。'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存主机失败'
  } finally {
    busy.value = false
  }
}
async function deleteHost(host: Host) {
  const warning = host.agent_id
    ? '该主机已配对。删除仅移除 EasyNetTopo 本地登记和已保存的凭据，不会撤销 Agent 端授权、停止 Agent 或删除 Docker 应用。'
    : '删除仅移除 EasyNetTopo 的主机登记，不影响 Agent 及其 Docker 应用。'
  if (!window.confirm('确定删除主机「' + host.name + '」？\n\n' + warning)) return
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await call<void>('/api/v1/hosts/' + host.id, 'DELETE')
    if (selectedID.value === host.id) {
      selectHost('')
    }
    if (editingID.value === host.id) editingID.value = ''
    await loadHosts()
    emit('changed')
    message.value = '主机「' + host.name + '」已从 EasyNetTopo 删除。Agent 和 Docker 应用未受影响。'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除主机失败'
  } finally {
    busy.value = false
  }
}
onMounted(loadHosts)
</script>

<template>
  <section class="panel">
    <div class="heading">
      <div>
        <h2>主机与 EasyHostAgent</h2>
        <p class="muted">支持 IP:port（默认 HTTPS）或 http://IP:port（显式 HTTP）。HTTP 明文传输凭据与操作参数，只适合可信网络。</p>
      </div>
      <button type="button" class="secondary" @click="loadHosts">刷新主机</button>
    </div>

    <form class="form" @submit.prevent="registerHost">
      <label for="host-name">主机名称</label>
      <input id="host-name" v-model="name" placeholder="例如：fnOS NAS" maxlength="100" required />
      <label for="host-address">Agent 地址</label>
      <input id="host-address" v-model="endpoint" placeholder="例如：http://192.168.1.20:7018" required />
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
            <button type="button" class="secondary" :disabled="busy" :aria-expanded="editingID === host.id" :aria-controls="'host-edit-' + host.id" @click="toggleEditor(host)">{{ editingID === host.id ? '收起编辑' : '编辑' }}</button>
            <button v-if="host.agent_id" type="button" :disabled="busy" @click="queryHost(host.id, 'snapshot')">主机信息</button>
            <button v-if="host.agent_id" type="button" :disabled="busy" @click="queryHost(host.id, 'docker')">Docker 资源</button>
            <button type="button" class="danger" :disabled="busy" @click="deleteHost(host)">删除</button>
          </div>
        </div>
        <small v-if="host.agent_id" class="muted">Agent UUID：{{ host.agent_id }}</small>
        <div v-if="editingID === host.id" :id="'host-edit-' + host.id" class="edit-panel">
          <form class="form" @submit.prevent="saveHost">
            <h3>编辑主机信息</h3>
            <label :for="'edit-name-' + host.id">主机名称</label>
            <input :id="'edit-name-' + host.id" v-model="editName" maxlength="100" required />
            <label :for="'edit-endpoint-' + host.id">Agent 地址</label>
            <input :id="'edit-endpoint-' + host.id" v-model="editEndpoint" required />
            <button type="submit" :disabled="busy">{{ busy ? '处理中…' : '保存修改' }}</button>
          </form>
          <details class="pair-advanced">
            <summary>{{ host.agent_id ? '重新配对 Agent（仅手动操作时生效）' : '首次配对 Agent' }}</summary>
            <form class="form pair" @submit.prevent="pairHost">
            <h3>{{ host.agent_id ? '重新配对 Agent' : '配对 Agent' }}</h3>
            <p class="muted">只有点击「安全配对」才会更新配对授权；普通「保存修改」不会触碰 Agent ID、指纹、Token 或配对时间。请先保存可能修改过的连接地址。HTTPS 会校验指纹，HTTP 不提供加密和服务器身份验证。重新配对可能撤销先前的控制端授权。</p>
            <label :for="'agent-id-' + host.id">Agent UUID</label>
            <input :id="'agent-id-' + host.id" v-model="agentID" placeholder="终端核对后的 UUID" autocomplete="off" required />
            <label :for="'agent-fingerprint-' + host.id">Agent 公钥指纹（64 位十六进制）</label>
            <input :id="'agent-fingerprint-' + host.id" v-model="fingerprint" placeholder="终端核对后的公钥指纹" autocomplete="off" spellcheck="false" required />
            <label :for="'pair-code-' + host.id">一次性配对码</label>
            <input :id="'pair-code-' + host.id" v-model="pairingCode" type="password" autocomplete="off" placeholder="本机生成的 32 位十六进制码" required />
            <button type="submit" :disabled="busy">{{ busy ? '处理中…' : '安全配对' }}</button>
            </form>
          </details>
        </div>
      </li>
    </ul>

    <section v-if="snapshot" class="result">
      <h3>主机实时快照</h3>
      <p><strong>{{ snapshot.hostname }}</strong> · {{ snapshot.platform }} / {{ snapshot.arch }}</p>
      <p>软件目录：<code>{{ snapshot.software_root }}</code></p>
      <p class="muted">采集时间：{{ snapshot.observed_at }}</p>
    </section>
    <section v-if="inventory" class="result">
      <div class="heading">
        <h3>Docker 资源</h3>
        <span :class="inventory.complete ? 'ok' : 'warning'">{{ inventory.complete ? '查询完整' : '查询不完整，不据此删除资源' }}</span>
      </div>
      <p class="muted">采集时间：{{ inventory.observed_at }}</p>
      <div v-for="resource in inventory.resources" :key="resource.resource_id" class="resource">
        <strong>{{ resource.name }}</strong>
        <span class="muted">{{ resource.kind }} · {{ resource.state }} · {{ resource.managed ? '受管资源' : '外部资源（只读）' }}</span>
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
.danger{background:#b43835;color:#fff}
.edit-panel{margin-top:12px;padding:12px 16px;border:1px solid #d8e2ec;border-radius:10px;background:#f8fbfe}
.edit-panel .form{max-width:100%;padding:10px 0}
.pair-advanced{border-top:1px solid #d8e2ec;margin-top:12px;padding-top:10px}
.pair-advanced summary{cursor:pointer;padding:8px 0;font-size:14px;font-weight:600;color:#176a9b}
.pair-advanced .form{padding-top:12px}
button:disabled{opacity:.55;cursor:not-allowed}
.hosts{list-style:none;padding:0;margin:15px 0}
.hosts li{padding:14px 0;border-top:1px solid #e4eaf1}
.row p{margin:4px 0}
.actions{display:flex;gap:8px;flex-wrap:wrap}
.result{background:#f6f9fc;padding:18px;border-radius:10px;margin-top:16px}
.resource{border-top:1px solid #e3eaf1;padding:10px 0;display:flex;gap:15px;justify-content:space-between;flex-wrap:wrap}
.ok{color:#247852}.warning{color:#b25216}
code{overflow-wrap:anywhere}
@media(max-width:650px){.panel{padding:14px}.form{grid-template-columns:1fr}.form button{grid-column:1}}
</style>
