<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

type AppRef = {source_id: string; app_id: string}
type App = AppRef & {name: string; summary: string; revision: string}
type Host = {id: string; name: string; agent_id?: string}
type Status = AppRef & {managed: boolean; allowed: boolean; external?: boolean; state: string; action?: string; result?: string; message?: string}
type Field = {key: string; type: string; label: string; description?: string; required: boolean; editable?: boolean; default?: string | boolean | number; minimum?: number; maximum?: number}
const props = defineProps<{apps: App[]; sources: {id: string; name: string}[]; hostVersion: number}>()
const hosts = ref<Host[]>([])
const host = ref('')
const rows = ref<Record<string,Status>>({})
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const statusError = ref('')
const form = ref<{app: App; revision: string; fields: Field[]; values: Record<string,string>} | null>(null)
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
    if (working.value) timer=setTimeout(refresh,1800)
  } catch(e) {
    if (seq===generation && !disposed) {rows.value={}; statusError.value='主机状态未知。'+message(e)}
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
async function submit(app: App, action:'install'|'uninstall'){
  const id=host.value, install=form.value
  if(action==='install' && (!install || key(install.app)!==key(app))) return
  if(action==='uninstall' && !window.confirm('卸载 '+app.name+'？持久化数据会保留。')) return
  busy.value=true; error.value=''
  try{
    const payload=action==='install' && install ? {...refFor(app),revision:install.revision,config:install.values} : refFor(app)
    await call(endpoint(id,action),payload)
    if(id===host.value){ form.value=null; await refresh() }
  } catch(e){if(id===host.value){error.value=message(e); await refresh()}} finally{busy.value=false}
}
function canInstall(a:App){
  const row=rows.value[key(a)]
  return row?.allowed && row.state!=='unknown' && !row.external && (!row.managed || row.state==='missing' || (row.action==='install' && ['failed','unknown'].includes(row.result || '')))
}
watch(host,()=>{form.value=null; rows.value={};error.value='';statusError.value='';void refresh()})
watch(()=>props.apps,()=>{form.value=null;void refresh()})
watch(()=>props.hostVersion,loadHosts)
onMounted(loadHosts)
onBeforeUnmount(()=>{disposed=true;generation++;clearTimeout(timer)})
</script>

<template>
<section class="market">
  <h2>应用商店</h2>
  <div class="toolbar"><label>目标主机 <select v-model="host" :disabled="busy || !hosts.length">
    <option v-if="!hosts.length" value="">请先配对主机</option>
    <option v-for="h in hosts" :key="h.id" :value="h.id">{{h.name}}</option>
  </select></label><button :disabled="!host || busy || loading" @click="refresh">刷新状态</button></div>
  <p v-if="error" class="error" role="alert">{{error}}</p>
  <p v-if="statusError" class="error" role="status">{{statusError}}</p>
  <p v-if="!visibleApps.length">暂无应用，请先同步 Git 来源。</p>
  <div class="cards">
    <article v-for="app in visibleApps" :key="key(app)">
      <h3>{{app.name}}</h3><p>{{app.summary}}</p>
      <small>{{sources.find(s=>s.id===app.source_id)?.name || app.source_id}} · {{app.revision.slice(0,12) || '本机保留版本'}}</small>
      <p v-if="host" role="status">服务状态 · {{states[rows[key(app)]?.state] || '未知'}}
        <span v-if="rows[key(app)]?.result"><br/>最近{{rows[key(app)].action==='install'?'安装':'卸载'}} · {{results[rows[key(app)].result!]}} {{rows[key(app)].message}}</span></p>
      <p v-if="rows[key(app)]?.external"><small>同名项目由主机上的其他部署管理。</small></p>
      <p v-else-if="rows[key(app)] && !rows[key(app)].allowed"><small>此来源在该主机上不可操作，请核对主机授权和应用归属。</small></p>
      <form v-if="form && key(form.app)===key(app)" @submit.prevent="submit(app,'install')">
        <label v-for="f in form.fields" :key="f.key">{{f.label}} <small>{{f.required?'必填':'选填'}}</small>
          <select v-if="f.type==='boolean'" v-model="form.values[f.key]">
            <option v-if="!f.required" value="">未设置</option><option value="true">是</option><option value="false">否</option>
          </select>
          <input v-else v-model="form.values[f.key]" :type="f.type==='secret'?'password':['number','port'].includes(f.type)?'number':'text'"
            :min="f.minimum" :max="f.maximum" :required="f.required && f.type!=='secret'" :autocomplete="f.type==='secret'?'new-password':'off'"/>
          <small v-if="f.description">{{f.description}}</small>
        </label>
        <button :disabled="busy || working">确认安装</button><button type="button" :disabled="busy" @click="form=null">取消</button>
      </form>
      <template v-else>
        <button v-if="catalogKeys.has(key(app))" :disabled="busy || working || !canInstall(app)" @click="configure(app)">安装</button>
        <button v-if="rows[key(app)]?.managed" :disabled="busy || working || !rows[key(app)].allowed" @click="submit(app,'uninstall')">卸载</button>
      </template>
    </article>
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
</style>
