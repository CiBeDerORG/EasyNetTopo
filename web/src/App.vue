<script setup lang="ts">
import { onMounted, ref } from 'vue'

type AuthStatus = { initialized: boolean; authenticated: boolean }
const loading = ref(true)
const initialized = ref(false)
const authenticated = ref(false)
const username = ref('admin')
const password = ref('')
const error = ref('')
const controllerID = ref('')

async function refresh() {
  error.value = ''
  try {
    const response = await fetch('/api/v1/auth/status', { credentials: 'same-origin', cache: 'no-store' })
    if (!response.ok) throw new Error('无法查询平台状态')
    const data = (await response.json()) as AuthStatus
    initialized.value = data.initialized
    authenticated.value = data.authenticated
    if (authenticated.value) {
      const info = await fetch('/api/v1/system/info', { credentials: 'same-origin', cache: 'no-store' })
      if (!info.ok) throw new Error('会话已过期')
      controllerID.value = (await info.json() as { controller_id: string }).controller_id
    }
  } catch {
    error.value = '控制端暂时不可用'
  } finally {
    loading.value = false
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
  } catch (e) {
    error.value = e instanceof Error ? e.message : '登录失败'
  }
}
async function logout() {
  await fetch('/api/v1/auth/logout', {
    method: 'POST', credentials: 'same-origin', headers: { 'X-ENT-Request': '1' }
  })
  authenticated.value = false
  controllerID.value = ''
}
onMounted(refresh)
</script>

<template>
  <main class="page">
    <section class="shell">
      <p class="eyebrow">INTEGRATION PLATFORM</p>
      <h1>EasyNetTopo</h1>
      <p class="intro">统一集成管理平台</p>
      <p v-if="loading" role="status">正在连接控制端…</p>
      <template v-else-if="!initialized">
        <h2>首次初始化</h2>
        <p>请在 EasyNetTopo 服务器终端执行：</p>
        <code>easynettopo admin init</code>
        <p>初始化完成后刷新页面。管理员密码不会通过初始化页面传输。</p>
        <button type="button" @click="refresh">刷新状态</button>
      </template>
      <form v-else-if="!authenticated" @submit.prevent="login">
        <h2>管理员登录</h2>
        <label for="account">账号</label>
        <input id="account" v-model="username" autocomplete="username" required />
        <label for="password">密码</label>
        <input id="password" v-model="password" type="password" autocomplete="current-password" required />
        <button type="submit">登录</button>
      </form>
      <section v-else>
        <h2>控制端已就绪</h2>
        <p>当前管理员：admin</p>
        <p class="muted">控制端 ID：<code>{{ controllerID }}</code></p>
        <p>应用源、主机和任务能力将在下一阶段独立接入。</p>
        <button type="button" @click="logout">退出登录</button>
      </section>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </section>
  </main>
</template>

<style scoped>
.page { min-height:100vh; display:grid; place-items:center; padding:24px; background:#f6f8fb; color:#172b4d; font-family:system-ui,-apple-system,"Segoe UI",sans-serif; }
.shell { width:min(100%,660px); background:#fff; padding:clamp(28px,5vw,56px); border:1px solid #e4eaf1; border-radius:16px; box-shadow:0 8px 40px #152a4810; }
.eyebrow { font-size:12px; letter-spacing:.16em; font-weight:700; color:#577899; }
h1 { font-size:clamp(36px,5vw,54px); letter-spacing:-.04em; margin:10px 0 6px; }
.intro { font-size:19px; color:#5d7188; margin:0 0 34px; }
h2 { font-size:19px; margin-top:20px; }
label { display:block; margin:18px 0 6px; font-weight:600; }
input { width:100%; box-sizing:border-box; font:inherit; border:1px solid #c8d2df; padding:11px 12px; border-radius:8px; }
button { display:block; margin-top:22px; background:#176a9b; border:0; border-radius:8px; color:white; font:inherit; padding:11px 20px; cursor:pointer; }
code { overflow-wrap:anywhere; color:#235780; }
.muted { color:#617286; font-size:13px; }
.error { color:#b42832; margin-top:16px; }
</style>
