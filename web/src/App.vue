<script setup lang="ts">
import { onMounted, ref } from 'vue'

const status = ref<'loading' | 'ready' | 'error'>('loading')
const message = ref('正在检查控制端状态')

onMounted(async () => {
  try {
    const response = await fetch('/api/v1/health', { cache: 'no-store' })
    if (!response.ok) throw new Error('HTTP ' + response.status)
    const data: unknown = await response.json()
    if (typeof data !== 'object' || data === null || !('status' in data) || data.status !== 'ok') {
      throw new Error('健康响应无效')
    }
    status.value = 'ready'
    message.value = '控制端已连接'
  } catch {
    status.value = 'error'
    message.value = '控制端暂时不可用'
  }
})
</script>

<template>
  <main class="page">
    <div class="shell">
      <p class="eyebrow">INTEGRATION PLATFORM</p>
      <h1>EasyNetTopo</h1>
      <p class="intro">统一集成管理平台</p>
      <div class="state" role="status" aria-live="polite">
        <span class="dot" :class="status"></span>
        <span>{{ message }}</span>
      </div>
      <p class="note">基础服务已就绪。Git 应用源、主机管理与任务功能将在完成身份认证和协议联调后开放。</p>
    </div>
  </main>
</template>

<style scoped>
.page { min-height: 100vh; display: grid; place-items: center; padding: 24px; background: #f6f8fb; color: #172b4d; font-family: system-ui, -apple-system, "Segoe UI", sans-serif; }
.shell { width: min(100%, 660px); background: #fff; padding: clamp(28px, 5vw, 56px); border: 1px solid #e4eaf1; border-radius: 16px; box-shadow: 0 8px 40px #152a4810; }
.eyebrow { font-size: 12px; letter-spacing: 0.16em; font-weight: 700; color: #577899; }
h1 { font-size: clamp(36px, 5vw, 54px); letter-spacing: -0.04em; margin: 10px 0 6px; }
.intro { font-size: 19px; color: #5d7188; margin: 0 0 34px; }
.state { display: inline-flex; gap: 10px; align-items: center; padding: 12px 16px; border-radius: 10px; background: #f2f6fa; font-weight: 600; }
.dot { width: 9px; height: 9px; border-radius: 50%; background: #9ca9b6; }
.dot.ready { background: #238b63; }
.dot.error { background: #c44d4d; }
.note { font-size: 14px; color: #64768a; line-height: 1.75; margin-top: 32px; }
</style>
