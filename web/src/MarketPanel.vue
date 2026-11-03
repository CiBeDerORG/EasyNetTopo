<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

type AppRef = {source_id: string; app_id: string}
type App = AppRef & {name: string; summary: string; revision: string}
type Host = {id: string; name: string; agent_id?: string}
type Status = AppRef & {managed: boolean; allowed: boolean; external?: boolean; blocked_reason?: string; state: string; action?: string; result?: string; message?: string}
type Field = {key: string; type: string; label: string; description?: string; required: boolean; editable?: boolean; default?: string | boolean | number; minimum?: number; maximum?: number}
const props = defineProps<{apps: App[]; sources: {id: string; name: string}[]; hostVersion: number; controllerId: string}>()
const hosts = ref<Host[]>([])
const host = ref('')
const rows = ref<Record<string,Status>>({})
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const statusError = ref('')
const form = ref<{app: App; revision: string; fields: Field[]; values: Record<string,string>} | null>(null)
type AppAction = 'install' | 'update' | 'uninstall'
type Toast = { id: number; kind: 'success' | 'error' | 'info' | 'warning'; text: string }
const toast = ref<Toast | null>(null)
const uninstallTarget = ref<App | null>(null)
const showGrantHelp = ref('')
const copyStatus = ref('')
function authorizationValue(sourceId: string) {
  return props.controllerId + '/' + sourceId
}
async function copyGrant(sourceId: string) {
  if (!props.controllerId) return
  try {
    await navigator.clipboard.writeText(authorizationValue(sourceId))
    copyStatus.value = '授权项已复制。请追加到 fnOS 的 EASYHOSTAGENT_APP_GRANTS，保留原有授权。'
  } catch {
    copyStatus.value = '复制失败，请手动选中授权项复制。'
  }
}
const pending = ref<{ host: string; key: string; action: AppAction; app: string; errors: number } | null>(null)
let toastSerial = 0
let toastTimer: ReturnType<typeof setTimeout> | undefined
function notify(kind: Toast['kind'], text: string) {
  clearTimeout(toastTimer)
  toast.value = { id: ++toastSerial, kind, text }
  toastTimer = setTimeout(() => { toast.value = null }, kind === 'error' ? 11000 : 7500)
}
const states: Record<string,string> = { running:'运行中', stopped:'已停止', partial:'部分运行', not_installed:'未安装', unknown:'未知', missing:'已登记，容器缺失', external:'同名外部项目' }
const results: Record<string,string> = {running:'进行中', succeeded:'成功', failed:'失败', unknown:'结果未知'}
const key = (a: AppRef) => a.source_id + '/' + a.app_id
const refFor = (a: AppRef) => ({source_id:a.source_id, app_id:a.app_id})
const catalogKeys = computed(() => new Set(props.apps.map(key)))
const visibleApps = computed(() => [...props.apps,...Object.values(rows.value)
  .filter(r => r.managed && !catalogKeys.value.has(key(r)))
  .map(r => ({...r,name:r.app_id,summary:'本机安装记录',revision:''}))])
const working = computed(() => Object.values(rows.value).some(r => r.result === 'running'))
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false
const active = computed(() => busy.value || !!pending.value || working.value)

async function call<T>(url: string, body?: unknown): Promise<T> {
  const res = await fetch(url,{method: body === undefined ? 'GET':'POST', credentials:'same-origin', cache:'no-store',
    ...(body === undefined ? {} : {headers:{'Content-Type':'application/json','X-ENT-Request':'1'},body:JSON.stringify(body)})})
  if (!res.ok) throw new Error((await res.text()).trim() || '请求失败')
  return res.json()
}
function message(e: unknown) { return e instanceof Error ? e.message : '请求失败' }
function endpoint(id: string, action: string){return '/api/v1/hosts/'+encodeURIComponent(id)+'/apps/'+action}

async function loadHosts(){
  try {
    const response=await call<{hosts: Host[]}>('/api/v1/hosts')
    if (disposed) return
    hosts.value=(response.hosts || []).filter(h => h.agent_id)
    if (!hosts.value.some(h=>h.id===host.value)) host.value=hosts.value[0]?.id || ''
  } catch(e) { error.value=message(e) }
}
async function refresh(){
  clearTimeout(timer)
  const seq=++generation, id=host.value
  if (!id) {rows.value={}; loading.value=false; return}
  loading.value=true
  const refs=props.apps.map(refFor), next:Record<string,Status>={}
  try {
    for (let start=0; start<Math.max(refs.length,1); start+=256) {
      const result=await call<{apps:Status[]}>(endpoint(id,'status'),{apps:refs.slice(start,start+256),include_managed:start===0})
      if (seq!==generation || disposed) return
      for(const row of result.apps) next[key(row)]=row
    }
    rows.value=next; statusError.value=''
    const job = pending.value
    if (job && job.host === id) {
      const row = next[job.key]
      if (row && row.action === job.action && row.result && row.result !== 'running') {
        pending.value = null
        if (row.result === 'failed') {
          notify('error', job.app + ' ' + actionNames[job.action] + '失败：' + (row.message || '请检查 Agent 运行状态'))
        } else if (row.result === 'unknown') {
          notify('warning', job.app + ' 操作结果未知，请核对目标主机实际状态')
        } else if (row.result === 'succeeded' &&
          (job.action === 'uninstall' ? row.state === 'not_installed' : row.state === 'running')) {
          notify('success', job.app + ' ' + actionNames[job.action] + '成功')
        } else {
          notify('warning', job.app + ' 脚本执行完成，但当前状态为「' + (states[row.state] || '未知') + '」，请核查')
        }
      }
    }
    if (working.value || pending.value?.host === id) timer=setTimeout(refresh,1800)
  } catch(e) {
    if (seq===generation && !disposed) {
      rows.value={}; statusError.value='主机状态未知。'+message(e)
      if (pending.value?.host === id) {
        pending.value.errors++
        if (pending.value.errors >= 3) {
          notify('warning', '无法获取操作最终结果，请恢复连接后刷新主机状态，不能据此判断成功或失败')
          pending.value = null
        } else timer=setTimeout(refresh,2500)
      }
    }
  } finally {if(seq===generation) loading.value=false}
}
async function configure(app: App){
  const id=host.value
  busy.value=true; error.value=''; form.value=null
  try {
    const data=await call<{revision:string;config:Field[]}>('/api/v1/catalog/apps/'+key(app)+'/config')
    if(id!==host.value || disposed) return
    const fields=(data.config || []).filter(f=>f.editable!==false)
    const values=Object.fromEntries(fields.map(f=>[f.key,String(f.default ?? '')]))
    form.value={app,revision:data.revision,fields,values}
  } catch(e){error.value=message(e)} finally {busy.value=false}
}
const actionNames: Record<AppAction,string> = {install:'安装', update:'更新', uninstall:'卸载'}
async function submit(app: App, action: AppAction) {
  const id=host.value, install=form.value
  if (!id || active.value) return
  if (action==='install' && (!install || key(install.app)!==key(app))) return
  if (action==='update' && !canUpdate(app)) return
  if (action==='uninstall' && !canUninstall(app)) return
  busy.value=true
  error.value=''
  try {
    const payload=action==='install' && install
      ? {...refFor(app),revision:install.revision,config:install.values}
      : action==='update' ? {...refFor(app),revision:app.revision} : refFor(app)
    const accepted=await call<{action?: string;result?: string;message?: string}>(endpoint(id,action),payload)
    uninstallTarget.value=null
    if (id !== host.value) return
    form.value=null
    if (accepted.result === 'running') {
      pending.value={host:id,key:key(app),action,app:app.name,errors:0}
      notify('info',app.name+' '+actionNames[action]+'请求已受理，正在等待 Agent 执行结果')
    } else if (accepted.result === 'succeeded') {
      notify('success', app.name+' '+actionNames[action]+'已完成')
    } else {
      notify('warning', app.name+' 已受理，但结果尚未确认，请刷新状态核对')
    }
    await refresh()
  } catch(e) {
    if(id===host.value) {
      error.value=message(e)
      notify('error',app.name+' '+actionNames[action]+'失败：'+message(e))
      await refresh()
    }
  } finally {busy.value=false}
}
function askUninstall(app: App) {
  if (!active.value && canUninstall(app)) uninstallTarget.value=app
}
async function confirmUninstall() {
  const app=uninstallTarget.value
  if (app) await submit(app,'uninstall')
}
function canInstall(a:App){
  const row=rows.value[key(a)]
  return !!row?.allowed && !row.external && row.state!=='unknown' &&
    (!row.managed || row.state==='missing' || (row.action==='install' && ['failed','unknown'].includes(row.result || '') && row.state==='not_installed'))
}
function canUpdate(a:App) {
  const row=rows.value[key(a)]
  return !!row?.allowed && !!row.managed && !row.external && !!a.revision &&
    ['running','stopped','partial'].includes(row.state)
}
function canUninstall(a:App) {
  const row=rows.value[key(a)]
  return !!row?.allowed && !!row.managed && !row.external && row.state!=='unknown'
}
function isExternal(a: App) {
  return !!rows.value[key(a)]?.external
}
function isManaged(a: App) {
  return !!rows.value[key(a)]?.managed
}
function blockReason(a: App) {
  const row = rows.value[key(a)]
  if (!row) return '尚未获取应用状态，点击刷新状态后再操作。'
  if (row.external || row.blocked_reason === 'external_project') {
    return '同名 Docker 项目由其他部署管理。为保护已有应用和数据，不能直接覆盖或卸载；需要先进行明确的受控接管。'
  }
  if (row.blocked_reason === 'source_conflict') {
    return '此应用已有其他 Git 来源的安装记录，不能用当前来源覆盖。'
  }
  if (!row.allowed) {
    return '当前控制端尚未获得此 Git 应用源的 Agent 操作授权。请先在 fnOS 上授权，再刷新状态。'
  }
  if (row.state === 'unknown') return '无法确认 Docker 状态，暂时禁止安装、更新或卸载。'
  if (row.state === 'missing') return '已有受管安装记录，但服务容器缺失；可以重新安装。'
  return ''
}
function grantRequired(a: App) {
  const row = rows.value[key(a)]
  return !!row && !row.external &&
    (row.blocked_reason === 'grant_required' ||
      (!row.allowed && !row.blocked_reason))
}
function showInstallAction(a: App) {
  const row = rows.value[key(a)]
  return !row || (!row.managed && !row.external) || row.state === 'missing'
}

watch(host,()=>{form.value=null; uninstallTarget.value=null; showGrantHelp.value=''; copyStatus.value=''; rows.value={};error.value='';statusError.value='';void refresh()})
watch(()=>props.apps,()=>{form.value=null;void refresh()})
watch(()=>props.hostVersion,loadHosts)
onMounted(loadHosts)
onBeforeUnmount(()=>{disposed=true;generation++;clearTimeout(timer);clearTimeout(toastTimer)})
</script>

<template>
<section class="market">
  <h2>应用商店</h2>
  <div class="toolbar"><label>目标主机 <select v-model="host" :disabled="active || !hosts.length">
    <option v-if="!hosts.length" value="">请先配对主机</option>
    <option v-for="h in hosts" :key="h.id" :value="h.id">{{h.name}}</option>
  </select></label><button :disabled="!host || busy || loading" @click="refresh">刷新状态</button></div>
  <div v-if="toast" class="toast-wrap" role="status" aria-live="polite">
    <div class="toast" :class="'toast-'+toast.kind">
      <span>{{ toast.text }}</span>
      <button type="button" class="toast-close" aria-label="关闭提示" @click="toast=null">×</button>
    </div>
  </div>
  <p v-if="error" class="error" role="alert">{{error}}</p>
  <p v-if="statusError" class="error" role="status">{{statusError}}</p>
  <p v-if="!visibleApps.length">暂无应用，请先同步 Git 来源。</p>
  <div class="cards">
    <article v-for="app in visibleApps" :key="key(app)">
      <h3>{{app.name}}</h3><p>{{app.summary}}</p>
      <small>{{sources.find(s=>s.id===app.source_id)?.name || app.source_id}} · {{app.revision.slice(0,12) || '本机保留版本'}}</small>
      <p v-if="host" role="status">服务状态 · {{states[rows[key(app)]?.state] || '未知'}}
        <span v-if="rows[key(app)]?.result"><br/>最近{{actionNames[rows[key(app)].action as AppAction] || '操作'}} · {{results[rows[key(app)].result!]}} {{rows[key(app)].message}}</span></p>
      <p v-if="blockReason(app)" class="restriction"><small>{{blockReason(app)}}</small></p>
      <div v-if="grantRequired(app)" class="grant-tools">
        <button type="button" class="secondary" @click="showGrantHelp = showGrantHelp === app.source_id ? '' : app.source_id">
          {{ showGrantHelp === app.source_id ? '收起授权说明' : '查看授权方法' }}
        </button>
        <div v-if="showGrantHelp === app.source_id" class="grant-help">
          <p>在安装 EasyHostAgent 的 fnOS 主机上，编辑 <code>/vol3/1000/software/deploy/.env</code>，将下面的授权项追加到 <code>EASYHOSTAGENT_APP_GRANTS</code>。如果已经有其他授权，用英文逗号分隔，<strong>不要覆盖旧值</strong>。</p>
          <code class="grant-value">{{ authorizationValue(app.source_id) }}</code>
          <button type="button" :disabled="!controllerId" @click="copyGrant(app.source_id)">复制授权项</button>
          <p v-if="copyStatus" role="status">{{copyStatus}}</p>
          <p>保存后在 fnOS 执行 <code>sudo bash deploy.sh easyhostagent update</code> 使授权进入 Agent 原生配置，然后返回这里点击「刷新状态」。</p>
        </div>
      </div>
      <form v-if="form && key(form.app)===key(app)" @submit.prevent="submit(app,'install')">
        <label v-for="f in form.fields" :key="f.key">{{f.label}} <small>{{f.required?'必填':'选填'}}</small>
          <select v-if="f.type==='boolean'" v-model="form.values[f.key]">
            <option v-if="!f.required" value="">未设置</option><option value="true">是</option><option value="false">否</option>
          </select>
          <input v-else v-model="form.values[f.key]" :type="f.type==='secret'?'password':['number','port'].includes(f.type)?'number':'text'"
            :min="f.minimum" :max="f.maximum" :required="f.required && f.type!=='secret'" :autocomplete="f.type==='secret'?'new-password':'off'"/>
          <small v-if="f.description">{{f.description}}</small>
        </label>
        <button :disabled="active">确认安装</button><button type="button" :disabled="busy" @click="form=null">取消</button>
      </form>
      <template v-else>
        <button v-if="catalogKeys.has(key(app)) && showInstallAction(app)"
          type="button" :disabled="active || !canInstall(app)" :title="canInstall(app) ? '执行安装脚本' : blockReason(app)"
          @click="configure(app)">{{rows[key(app)]?.state === 'missing' ? '重新安装' : '安装'}}</button>
        <template v-if="isManaged(app) || isExternal(app)">
          <button v-if="catalogKeys.has(key(app))" type="button" :disabled="active || !canUpdate(app)"
            :title="canUpdate(app) ? '执行更新脚本' : blockReason(app)" @click="submit(app,'update')">更新</button>
          <button type="button" :disabled="active || !canUninstall(app)" class="uninstall"
            :title="canUninstall(app) ? '执行卸载脚本' : blockReason(app)" @click="askUninstall(app)">卸载</button>
        </template>
      </template>
    </article>
  </div>
  <div v-if="uninstallTarget" class="dialog-backdrop" @click.self="uninstallTarget=null">
    <section class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="app-uninstall-title" aria-describedby="app-uninstall-detail">
      <h3 id="app-uninstall-title">确认卸载「{{uninstallTarget.name}}」？</h3>
      <p id="app-uninstall-detail">此操作将通过目标主机 EasyHostAgent 执行 Deploy 应用清单声明的卸载动作。默认保留持久化数据，不会卸载 EasyHostAgent 本身。请确认目标主机和应用正确。</p>
      <div class="confirm-actions">
        <button type="button" :disabled="busy" @click="uninstallTarget=null">取消</button>
        <button type="button" class="uninstall" :disabled="active" @click="confirmUninstall">确认卸载</button>
      </div>
    </section>
  </div>
</section>
</template>

<style scoped>
.market{border:1px solid #e4eaf1;background:white;border-radius:14px;padding:24px;margin:22px 0}
h2,h3{margin:0 0 12px}.toolbar,.cards{display:flex;gap:12px;flex-wrap:wrap;align-items:center}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,245px),1fr));align-items:start;margin-top:16px}
article{border:1px solid #dce4ed;border-radius:10px;padding:16px;overflow-wrap:anywhere}p{line-height:1.55}
button,select,input{font:inherit;border-radius:6px;padding:7px 10px}button{cursor:pointer;border:0;background:#e9f3f9;color:#155478;margin:3px}
button:disabled{opacity:.5;cursor:default}small{color:#586a7e}form label{display:grid;gap:4px;margin:10px 0}
input,select{border:1px solid #becbda;min-width:0;box-sizing:border-box}.error{color:#a32831}
.uninstall{background:#a42d36;color:white}
.dialog-backdrop{position:fixed;inset:0;z-index:30;display:grid;place-items:center;background:#14263d9c;padding:20px}
.confirm-dialog{background:#fff;padding:24px;max-width:470px;width:100%;box-shadow:0 15px 45px #0003;border-radius:12px}
.confirm-actions{display:flex;justify-content:flex-end;gap:12px;margin-top:20px}
.toast-wrap{position:fixed;right:20px;top:20px;z-index:40;max-width:min(430px,calc(100vw - 40px))}
.toast{display:flex;align-items:flex-start;gap:12px;padding:14px 16px;background:#f0f7ff;border-left:4px solid #236aa0;box-shadow:0 8px 25px #0002;border-radius:9px}
.toast-success{background:#edfaf2;border-color:#27845a}
.toast-error{background:#fff1f1;border-color:#b83d43}
.toast-warning{background:#fff7e8;border-color:#b67d27}
.toast-close{background:transparent;color:inherit;margin:0;padding:0 5px;font-size:21px}
.restriction{margin:10px 0;color:#5c6575}
.grant-tools{border-top:1px solid #e4eaf1;margin-top:12px;padding-top:8px}
.grant-help{padding:10px 0;font-size:13px;line-height:1.6;overflow-wrap:anywhere}
.grant-help p{margin:6px 0}
.grant-value{display:block;white-space:normal;overflow-wrap:anywhere;padding:8px 10px;border:1px solid #dce4ed;background:#f6f8fa;border-radius:5px}
.grant-tools .secondary{background:#e9f3f9;color:#155478}

</style>
