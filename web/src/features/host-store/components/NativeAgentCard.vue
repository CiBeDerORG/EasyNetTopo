<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Refresh, Download, Lightning } from '@element-plus/icons-vue'
import nativeAgentIcon from '../../../assets/easyhostagent.svg'
import type { AgentMaintenanceState, AgentUpdateAction, ExecutionTask } from '../types'
import { formatTaskTime, taskActive, taskSourceLabel } from '../task-status'

const props = defineProps<{
  softwareRoot?: string
  platform?: string
  task?: ExecutionTask
  connected: boolean
  paired: boolean
  checking: boolean
  connectionError: boolean
  maintenance: AgentMaintenanceState | null
  busy: boolean
}>()

const emit = defineEmits<{
  update: [action: AgentUpdateAction]
  resolve: []
}>()

const status = computed(() => {
  if (props.task && taskActive(props.task)) return (props.task.state === 'queued' ? '排队中' : '更新中') +
    ' · ' + taskSourceLabel(props.task.trigger_source)
  if (props.task?.state === 'interrupted') return '更新未确认 · 需要排查'
  if (props.task?.state === 'failed') return '更新失败 · ' + taskSourceLabel(props.task.trigger_source)
  if (props.maintenance?.state === 'timeout') return props.connected ?
    '更新未确认完成 · 需要排查' : '更新后连接异常 · 需要排查'
  if (props.maintenance?.state === 'waiting') return '正在更新 · 短暂断联属于正常现象'
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
async function showInstallCommand() {
  const command = (validRoot.value ? 'cd ' + validRoot.value + '/deploy && ' : '') +
    (props.platform === 'macos' ? 'bash' : 'sudo bash') + ' deploy.sh easyhostagent install'
  try {
    await ElMessageBox.alert('首次安装需要在目标主机终端执行：\n\n' + command,
      'EasyHostAgent · 安装', { confirmButtonText: '复制命令', type: 'info' })
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard API unavailable')
      await navigator.clipboard.writeText(command)
    } catch {
      const field = document.createElement('textarea')
      field.value = command
      field.style.position = 'fixed'
      field.style.opacity = '0'
      document.body.appendChild(field)
      field.select()
      const ok = document.execCommand('copy')
      field.remove()
      if (!ok) throw new Error('copy')
    }
    ElMessage.success('命令已复制，请在目标主机可信终端执行。')
  } catch {
    // 取消或复制失败不触发远程操作。
  }
}
</script>

<template>
  <article class="native-card">
    <div class="heading">
      <div class="native-logo"><img :src="nativeAgentIcon" alt="EasyHostAgent 图标" /></div>
      <div>
        <h4>EasyHostAgent</h4>
        <span class="kind">宿主机原生服务 · {{ platform === 'macos' ? 'launchd' : 'systemd' }}</span>
      </div>
    </div>
    <p class="description">自动关联已配对主机。更新由系统独立维护任务执行，服务会自动重启。</p>
    <div class="state" :class="{ online: connected && !maintenance && !taskActive(task) && task?.state !== 'failed' && task?.state !== 'interrupted', danger: maintenance?.state === 'timeout' || task?.state === 'failed' || task?.state === 'interrupted' }">{{ status }}</div>
    <p v-if="task && task.state === 'succeeded'" class="foot">最近自动更新：{{ formatTaskTime(task) }}</p>
    <p v-else-if="task" class="foot">任务 {{ task.id }} · {{ taskSourceLabel(task.trigger_source) }}<span v-if="task.message && (task.state === 'failed' || task.state === 'interrupted')"> · {{ task.message }}</span></p>
    <p v-if="maintenance?.state === 'timeout' || task?.state === 'interrupted'" class="diagnostic" role="alert">
      本次更新结果尚未确认，可能是下载失败、未重启或网络断联。请先在目标主机核对更新任务已经停止，再人工解除阻塞：
      <template v-if="platform === 'macos'">
        <code>launchctl print gui/$(id -u)/org.cibeder.easyhostagent</code>、
        <code>deploy/state/easyhostagent-update.stderr.log</code>
      </template>
      <template v-else>
        <code>systemctl status easyhostagent</code>、
        <code>journalctl -u easyhostagent-self-update.service -n 100</code>
      </template>
      和网络连接。
    </p>
    <div class="actions">
      <el-button v-if="task?.state === 'interrupted'" size="small" type="warning" @click="emit('resolve')">核实后解除阻塞</el-button>
      <el-button v-if="!paired" size="small" type="primary" @click="showInstallCommand">
        <el-icon><Download /></el-icon>安装
      </el-button>
      <el-button size="small" :loading="busy" :disabled="!connected || !!maintenance || !!task && (taskActive(task) || task.state === 'interrupted') || busy"
        @click="emit('update', 'update')"><el-icon><Refresh /></el-icon>更新</el-button>
      <el-button size="small" type="warning" :disabled="!connected || !!maintenance || busy"
        @click="emit('update', 'force-update')"><el-icon><Lightning /></el-icon>强制更新</el-button>
    </div>
    <p class="foot"><el-icon><CopyDocument /></el-icon>更新指令受理后不会立即报告成功；重启后自动验证主机重新连接。</p>
  </article>
</template>

<style scoped>
.native-card{display:flex;min-width:0;min-height:220px;flex-direction:column;padding:16px;border:1px solid #36536a;border-radius:13px;background:linear-gradient(145deg,#16253a,#102235)}
.heading{display:flex;align-items:center;gap:10px}
.native-logo{display:grid;place-items:center;width:43px;height:43px;flex:none;border-radius:12px;background:#214b60;color:#96e4fd;font-weight:800}
.native-logo img{display:block;width:100%;height:100%;object-fit:contain;border-radius:inherit}
h4{margin:0 0 5px;font-size:14px}.kind{color:#80a6c3;font-size:10px}.description{color:#87a5bf;font-size:11px;line-height:1.7;margin:14px 0}
.state{color:#e0bb77;font-size:12px}.state.online{color:#8ce4b4}.state.danger{color:#ff9d92}.actions{display:flex;flex-wrap:wrap;gap:5px;margin-top:auto;padding-top:14px}.actions :deep(.el-button){margin:0}
.foot{display:flex;align-items:center;gap:4px;color:#7f9ab4;font-size:10px;margin:10px 0 0;line-height:1.4}
.diagnostic{color:#ffb5aa;font-size:11px;line-height:1.8;margin:9px 0}.diagnostic code{font-size:10px;color:#f2c9a6}
</style>
