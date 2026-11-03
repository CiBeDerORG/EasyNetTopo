<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Refresh, Download, Delete, Lightning } from '@element-plus/icons-vue'

const props = defineProps<{
  softwareRoot?: string
  connected: boolean
  paired: boolean
  checking: boolean
  connectionError: boolean
}>()

type NativeAction = 'install' | 'uninstall' | 'update' | 'force-update'
const status = computed(() => {
  if (props.checking) return '检查中'
  if (props.connected) return '运行中 · 已关联'
  if (props.connectionError) return '连接异常 · 状态未知'
  if (props.paired) return '已配对 · 等待检查'
  return '未配对 · 状态未知'
})
const validRoot = computed(() => {
  const root = props.softwareRoot || ''
  if (!/^\/[A-Za-z0-9_./-]+$/.test(root) || root.split('/').includes('..')) return ''
  return root.replace(/\/$/, '')
})
function command(action: NativeAction): string {
  const verb = action
  const prefix = validRoot.value ? 'cd ' + validRoot.value + '/deploy && ' : ''
  return prefix + 'sudo bash deploy.sh easyhostagent ' + verb
}
async function showCommand(action: NativeAction) {
  const title = ({ install: '安装', uninstall: '卸载', update: '更新', 'force-update': '重新下载并更新' } as Record<NativeAction, string>)[action]
  const note = action === 'uninstall'
    ? '卸载当前 Agent 会立即断开该主机与 EasyNetTopo 的管理连接，请在目标主机终端操作。'
    : action === 'force-update'
      ? '原生 force-update 会忽略本地安装包路径，重新下载并校验当前 EASYHOSTAGENT_RELEASE 指定的发布资产，保留原配对、授权和数据。'
      : 'EasyHostAgent 是宿主机原生 systemd 服务；控制端不能通过自身尚未运行的 Agent 安装或安全地自卸载。请在目标主机可信终端操作。'
  try {
    await ElMessageBox.alert(note + '\n\n' + command(action), 'EasyHostAgent · ' + title, {
      confirmButtonText: '复制命令',
      dangerouslyUseHTMLString: false,
      type: action === 'uninstall' ? 'warning' : 'info'
    })
    await navigator.clipboard.writeText(command(action))
    ElMessage.success('命令已复制，请在目标主机可信终端执行。')
  } catch {
    // 用户关闭弹窗或浏览器阻止剪贴板时不标记为执行成功。
  }
}
</script>

<template>
  <article class="native-card">
    <div class="heading">
      <div class="native-logo">EH</div>
      <div>
        <h4>EasyHostAgent</h4>
        <span class="kind">宿主机原生服务 · systemd</span>
      </div>
    </div>
    <p class="description">EasyNetTopo 主机管理代理，自动关联当前已配对并可访问的 Agent。</p>
    <div class="state" :class="{ online: connected }">{{ status }}</div>
    <div class="actions">
      <el-button size="small" type="primary" @click="showCommand('install')"><el-icon><Download /></el-icon>安装</el-button>
      <el-button size="small" @click="showCommand('update')"><el-icon><Refresh /></el-icon>更新</el-button>
      <el-button size="small" type="warning" @click="showCommand('force-update')"><el-icon><Lightning /></el-icon>强制更新</el-button>
      <el-button size="small" type="danger" plain @click="showCommand('uninstall')"><el-icon><Delete /></el-icon>卸载</el-button>
    </div>
    <p class="foot"><el-icon><CopyDocument /></el-icon> 原生维护须在目标主机终端执行，按钮提供操作命令。</p>
  </article>
</template>

<style scoped>
.native-card{display:flex;min-width:0;min-height:220px;flex-direction:column;padding:16px;border:1px solid #36536a;border-radius:13px;background:linear-gradient(145deg,#16253a,#102235)}
.heading{display:flex;align-items:center;gap:10px}
.native-logo{display:grid;place-items:center;width:43px;height:43px;flex:none;border-radius:12px;background:#214b60;color:#96e4fd;font-weight:800}
h4{margin:0 0 5px;font-size:14px}.kind{color:#80a6c3;font-size:10px}.description{color:#87a5bf;font-size:11px;line-height:1.7;margin:14px 0}
.state{color:#e0bb77;font-size:12px}.state.online{color:#8ce4b4}.actions{display:flex;flex-wrap:wrap;gap:5px;margin-top:auto;padding-top:14px}.actions :deep(.el-button){margin:0}
.foot{display:flex;align-items:center;gap:4px;color:#7f9ab4;font-size:10px;margin:10px 0 0;line-height:1.4}
</style>
