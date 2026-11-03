<script setup lang="ts">
import { onMounted, ref } from 'vue'
import HostPanel from './HostPanel.vue'
import MarketPanel from './MarketPanel.vue'

type AuthStatus = { initialized: boolean; authenticated: boolean }
type Source = {
  id: string; name: string; url: string; branch: string
  current_revision: string; last_error?: string; updated_at?: string
}
type CatalogApp = {
  source_id: string; app_id: string; name: string; summary: string
  category: string; revision: string; manifest_sha256: string
}
const loading = ref(true)
const initialized = ref(false)
const authenticated = ref(false)
const username = ref('admin')
const password = ref('')
const error = ref('')
const catalogError = ref('')
const controllerID = ref('')
const sources = ref<Source[]>([])
const apps = ref<CatalogApp[]>([])
const sourceName = ref('Deploy')
const remoteURL = ref('https://github.com/CiBeDerORG/deploy.git')
const branch = ref('main')
const busySource = ref('')
const deletingSource = ref('')
const saving = ref(false)
const hostVersion = ref(0)

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: 'same-origin', cache: 'no-store' })
  if (!response.ok) throw new Error('请求失败：HTTP ' + response.status)
  return (await response.json()) as T
}
async function refresh() {
  error.value = ''
  try {
    const state = await fetchJSON<AuthStatus>('/api/v1/auth/status')
    initialized.value = state.initialized
    authenticated.value = state.authenticated
    if (authenticated.value) {
      const info = await fetchJSON<{ controller_id: string }>('/api/v1/system/info')
      controllerID.value = info.controller_id
      await loadCatalog()
    }
  } catch {
    error.value = '控制端暂时不可用'
  } finally {
    loading.value = false
  }
}
async function loadCatalog() {
  catalogError.value = ''
  try {
    const [sourcesResult, appsResult] = await Promise.all([
      fetchJSON<{ sources: Source[] }>('/api/v1/catalog/sources'),
      fetchJSON<{ apps: CatalogApp[] }>('/api/v1/catalog/apps')
    ])
    sources.value = sourcesResult.sources
    apps.value = appsResult.apps
  } catch {
    catalogError.value = '无法读取应用目录，请检查管理端状态'
  }
}
async function login() {
  error.value = ''
  try {
    const response = await fetch('/api/v1/auth/login', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-ENT-Request': '1' },
      body: JSON.stringify({ username: username.value, password: password.value })
    })
    if (!response.ok) throw new Error(response.status === 429 ? '登录尝试过多，请稍后重试' : '账号或密码错误')
    password.value = ''
    await refresh()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  }
}
async function logout() {
  const response = await fetch('/api/v1/auth/logout', {
    method: 'POST', credentials: 'same-origin', headers: { 'X-ENT-Request': '1' }
  })
  if (!response.ok) { error.value = '退出失败，请重试'; return }
  authenticated.value = false
  controllerID.value = ''
  sources.value = []
  apps.value = []
}
async function addSource() {
  if (!sourceName.value.trim() || !remoteURL.value.trim() || !branch.value.trim()) return
  saving.value = true
  catalogError.value = ''
  try {
    const response = await fetch('/api/v1/catalog/sources', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-ENT-Request': '1' },
      body: JSON.stringify({ name: sourceName.value, url: remoteURL.value, branch: branch.value })
    })
    if (!response.ok) throw new Error('添加失败：请检查 Git 地址、分支或是否已存在')
    await loadCatalog()
  } catch (err) {
    catalogError.value = err instanceof Error ? err.message : '无法添加应用源'
  } finally {
    saving.value = false
  }
}
async function deleteSource(source: Source) {
  const confirmed = window.confirm(
    `确认删除来源“${source.name}”（${source.branch}）？\n此来源的应用商店条目将被移除，但不会卸载已安装应用。`
  )
  if (!confirmed) return
  deletingSource.value = source.id
  catalogError.value = ''
  try {
    const response = await fetch('/api/v1/catalog/sources/' + encodeURIComponent(source.id), {
      method: 'DELETE', credentials: 'same-origin', headers: { 'X-ENT-Request': '1' }
    })
    if (!response.ok) throw new Error(response.status === 404 ? '来源不存在，请刷新列表' : '删除来源失败')
    await loadCatalog()
  } catch (err) {
    catalogError.value = err instanceof Error ? err.message : '删除来源失败'
  } finally {
    deletingSource.value = ''
  }
}
async function syncSource(source: Source) {
  busySource.value = source.id
  catalogError.value = ''
  try {
    const response = await fetch('/api/v1/catalog/sources/' + encodeURIComponent(source.id) + '/refresh', {
      method: 'POST', credentials: 'same-origin', headers: { 'X-ENT-Request': '1' }
    })
    if (!response.ok) throw new Error('同步失败：请检查控制端的 SSH/Git 授权及仓库应用描述')
    await loadCatalog()
  } catch (err) {
    catalogError.value = err instanceof Error ? err.message : '同步失败'
    await loadCatalog()
  } finally {
    busySource.value = ''
  }
}
onMounted(refresh)
</script>

<template>
  <main class="page">
    <div class="shell">
      <header>
        <div>
          <p class="eyebrow">INTEGRATION PLATFORM</p>
          <h1>EasyNetTopo</h1>
          <p class="intro">统一集成管理平台</p>
        </div>
        <button v-if="authenticated" class="secondary" type="button" @click="logout">退出登录</button>
      </header>

      <p v-if="loading" role="status">正在连接控制端…</p>
      <template v-else-if="!initialized">
        <h2>首次初始化</h2>
        <p>请先在服务器可信终端运行 <code>easynettopo admin init</code>。</p>
        <button type="button" @click="refresh">刷新状态</button>
      </template>
      <form v-else-if="!authenticated" class="login" @submit.prevent="login">
        <h2>管理员登录</h2>
        <label for="account">账号</label>
        <input id="account" v-model="username" autocomplete="username" required />
        <label for="password">密码</label>
        <input id="password" v-model="password" type="password" autocomplete="current-password" required />
        <button type="submit">登录</button>
      </form>

      <template v-else>
        <p class="muted">控制端：<code>{{ controllerID }}</code></p>
        <section class="panel">
          <div class="heading">
            <div>
              <h2>Git 应用源</h2>
              <p class="muted">支持 SSH、HTTPS、HTTP Git 地址。HTTP 明文来源须由服务端管理员显式授权，禁止在 URL 中填写账号或密码。</p>
            </div>
            <button type="button" class="secondary" @click="loadCatalog">刷新列表</button>
          </div>
          <form class="source-form" @submit.prevent="addSource">
            <label for="source-name">名称</label>
            <input id="source-name" v-model="sourceName" maxlength="80" required />
            <label for="source-url">Git 地址</label>
            <div class="source-url-field">
              <input id="source-url" v-model="remoteURL" spellcheck="false" autocomplete="off" aria-describedby="git-source-help" required />
              <small id="git-source-help" class="muted">公开仓库推荐 HTTPS：https://github.com/owner/repo.git；私有仓库可使用已授权的 SSH。明文 HTTP 需后端开启。</small>
            </div>
            <label for="source-branch">分支</label>
            <input id="source-branch" v-model="branch" required />
            <button type="submit" :disabled="saving">{{ saving ? '添加中…' : '添加来源' }}</button>
          </form>
          <div v-if="sources.length === 0" class="empty">尚未添加 Git 应用源</div>
          <ul v-else class="sources">
            <li v-for="source in sources" :key="source.id">
              <div class="source-info">
                <strong>{{ source.name }}</strong>
                <code>{{ source.url }}</code>
                <small class="muted">来源 ID：{{ source.id }}</small>
                <span class="muted">分支：{{ source.branch }} · {{ source.current_revision ? '当前版本 ' + source.current_revision.slice(0, 12) : '尚未同步' }}</span>
                <span v-if="source.last_error" class="error">{{ source.last_error }}</span>
              </div>
              <div class="source-actions">
                <button type="button" :disabled="!!busySource || !!deletingSource" @click="syncSource(source)">
                  {{ busySource === source.id ? '同步中…' : '手动同步' }}
                </button>
                <button type="button" class="danger" :disabled="!!busySource || !!deletingSource" @click="deleteSource(source)">
                  {{ deletingSource === source.id ? '删除中…' : '删除' }}
                </button>
              </div>
            </li>
          </ul>
        </section>

        <MarketPanel :apps="apps" :sources="sources" :host-version="hostVersion" />
        <HostPanel @paired="hostVersion++" />
        <p v-if="catalogError" class="error" role="alert">{{ catalogError }}</p>
      </template>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </div>
  </main>
</template>

<style scoped>
.page{min-height:100vh;background:#f6f8fb;color:#172b4d;font-family:system-ui,-apple-system,"Segoe UI",sans-serif;padding:24px;box-sizing:border-box}
.shell{max-width:1050px;margin:16px auto 50px}
header,.heading{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}
.eyebrow{font-size:12px;letter-spacing:.16em;font-weight:700;color:#577899}
h1{font-size:clamp(36px,5vw,54px);letter-spacing:-.04em;margin:8px 0}
.intro{font-size:19px;color:#5d7188;margin:0 0 26px}
h2{font-size:20px;margin:0 0 10px}h3{font-size:18px;margin:0}
.panel,.login{border:1px solid #e4eaf1;background:white;border-radius:14px;padding:24px;margin:22px 0}
.login{max-width:420px}label{font-size:13px;font-weight:700;margin:10px 0 4px}
input{min-width:0;box-sizing:border-box;width:100%;padding:10px 12px;font:inherit;border:1px solid #c8d2df;border-radius:8px}
button{background:#176a9b;color:white;border:0;border-radius:8px;padding:10px 16px;cursor:pointer;font:inherit}
button:disabled{opacity:.55;cursor:not-allowed}.secondary{color:#176a9b;background:#e9f3f9}
.source-form{display:grid;grid-template-columns:75px minmax(0,1fr);gap:10px 12px;align-items:center;max-width:780px;margin-top:20px}
.source-form button{grid-column:2;justify-self:start}
.source-url-field{min-width:0}.source-url-field small{display:block;margin-top:5px;line-height:1.5;overflow-wrap:anywhere}
.source-actions{display:flex;gap:8px;flex-wrap:wrap}.source-actions .danger{background:#fff1f2;color:#b42318;border:1px solid #fda4af}
.sources{padding:0;margin:24px 0 0;list-style:none}
.sources li{display:flex;justify-content:space-between;gap:15px;align-items:center;border-top:1px solid #edf0f3;padding:16px 0;flex-wrap:wrap}
.source-info{display:flex;flex-direction:column;gap:5px;min-width:0;max-width:100%}
.source-info code{overflow-wrap:anywhere}.apps{display:grid;grid-template-columns:repeat(auto-fit,minmax(245px,1fr));gap:14px}
.app{border:1px solid #e4eaf1;border-radius:12px;padding:16px}.app p{line-height:1.5}
.muted,.empty{color:#65778b;font-size:13px}.empty{padding:20px 0}
.pill{display:inline-block;padding:5px 9px;background:#edf2f7;color:#61758c;font-size:12px;border-radius:999px}
.error{color:#b32832}
@media(max-width:640px){.source-form{grid-template-columns:1fr}.source-form button{grid-column:1}.panel{padding:16px}.page{padding:12px}}
</style>
