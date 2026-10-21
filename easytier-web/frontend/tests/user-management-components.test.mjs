import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import vue from '@vitejs/plugin-vue';
import yaml from '@modyfi/vite-plugin-yaml';
import { chromium } from 'playwright';
// Real browser mounting; all account data and events are mocked, not backend authorization.
const root = fileURLToPath(new URL('../', import.meta.url));
let server, browser, base;
const fixture = `
import { createApp, h, reactive } from 'vue';
import { createI18n } from 'vue-i18n';
import PrimeVue from 'primevue/config';
import UserManagement from '/src/components/users/UserManagement.vue';
import en from '/@fs/${root}../frontend-lib/src/locales/en.yaml';
import cn from '/@fs/${root}../frontend-lib/src/locales/cn.yaml';
const user={id:2,username:'test-user-'+'long'.repeat(30),role:'user',web_login_enabled:true,source:'unknown',local_password_enabled:null,must_change_password:false,created_at:null,last_login_at:null};
const state=reactive({currentUser:{...user,id:1,role:'admin',capabilities:['users.manage'],default_password_warning:false},users:{items:[user],total:41,page:1,page_size:20},events:{items:[],total:0,page:1,page_size:20},selectedUser:null,loading:false,busy:false,stale:false,error:null,credential:null});
const calls=[],handlers={};
for(const event of ['query','select','create','set-access','set-role','reset-password','revoke-sessions','query-events','dismiss-credential','retry']) handlers['on'+event.split('-').map(x=>x[0].toUpperCase()+x.slice(1)).join('')]=value=>{calls.push({event,value});if(event==='select')state.selectedUser=state.users.items.find(x=>x.id===value);if(event==='dismiss-credential')state.credential=null;if(['create','set-access','set-role','reset-password','revoke-sessions'].includes(event))state.busy=true;};
window.fixture={state,calls};
createApp({render:()=>h(UserManagement,{...state,...handlers})}).use(PrimeVue).use(createI18n({legacy:false,locale:new URLSearchParams(location.search).get('lang')||'en',messages:{en,cn}})).mount('#app');`;
before(async()=>{
 server=await createServer({root,configFile:false,plugins:[vue(),yaml(),{name:'wum-fixture',resolveId(id){if(id==='/wum-fixture.js')return id;},load(id){if(id==='/wum-fixture.js')return fixture;},configureServer(server){server.middlewares.use((req,res,next)=>{if(req.url?.split('?')[0]!=='/wum-test')return next();res.setHeader('Content-Type','text/html');res.end('<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><div id="app"></div><script type="module" src="/wum-fixture.js"></script>');});}}],server:{host:'127.0.0.1',port:0,fs:{allow:[fileURLToPath(new URL('../../',import.meta.url))]}}});
 await server.listen();base=`http://127.0.0.1:${server.httpServer.address().port}/wum-test`;
 browser=await chromium.launch({headless:true,executablePath:process.env.WEB_TEST_CHROMIUM||'/usr/bin/chromium'});
});
after(async()=>{await browser?.close();await server?.close();});
async function open(t,lang='en',width=1200){const page=await browser.newPage({viewport:{width,height:900}});t.after(()=>page.close());page.setDefaultTimeout(10000);await page.goto(`${base}?lang=${lang}`);await page.locator('h1').waitFor();return page;}
test('search, stable pagination and unknown/stale/error states',async t=>{
 const p=await open(t);assert.match(await p.locator('table').innerText(),/Source not recorded/);assert.equal(await p.getByRole('cell',{name:'Unknown',exact:true}).count(),2);
 await p.getByRole('textbox',{name:'Search username'}).fill('test_%');await p.getByRole('button',{name:'Search username',exact:true}).click();
 assert.deepEqual(await p.evaluate(()=>window.fixture.calls[0]),{event:'query',value:{search:'test_%',role:undefined,web_login_enabled:undefined,page:1,page_size:20}});
 await p.getByRole('button',{name:'Next',exact:true}).click();assert.equal(await p.evaluate(()=>window.fixture.calls.at(-1).value.page),2);
 await p.evaluate(()=>{window.fixture.state.error={code:'network_error',message:'untrusted diagnostic'};window.fixture.state.stale=true;});
 await p.getByText('The server could not be reached.',{exact:false}).waitFor();assert.equal(await p.getByRole('table').count(),1);assert.equal(await p.getByText('untrusted diagnostic').count(),0);assert.equal(await p.getByText('No users yet.').count(),0);
});
test('create retains draft on failure, prevents duplicate submissions, returns focus',async t=>{
 const p=await open(t);await p.getByRole('button',{name:'Create user',exact:true}).click();const d=p.getByRole('dialog');await d.getByLabel('Username',{exact:true}).fill('test_new');await d.getByRole('button',{name:'Create user',exact:true}).click();assert.equal(await d.getByRole('button',{name:'Create user',exact:true}).isDisabled(),true);assert.equal(await p.evaluate(()=>window.fixture.calls.filter(x=>x.event==='create').length),1);
 await p.evaluate(()=>{window.fixture.state.error={code:'username_conflict',message:'conflict'};window.fixture.state.busy=false;});await d.getByText('This username is already in use. Choose another.').waitFor();assert.equal(await d.getByLabel('Username',{exact:true}).inputValue(),'test_new');await p.keyboard.press('Escape');await d.waitFor({state:'hidden'});assert.equal(await p.getByRole('button',{name:'Create user',exact:true}).evaluate(el=>el===document.activeElement),true);
});
test('confirmation explains console boundary and server rejection leaves status unchanged',async t=>{
 const p=await open(t);await p.getByRole('button',{name:/Details:/}).click();assert.equal(await p.getByRole('button',{name:'Reset local password',exact:true}).isDisabled(),true);await p.getByRole('button',{name:'Disable console access',exact:true}).click();const d=p.getByRole('dialog');assert.match(await d.innerText(),/Devices and networks are not stopped/);assert.equal(await p.evaluate(()=>window.fixture.calls.filter(x=>x.event==='set-access').length),0);await d.getByRole('button',{name:'Confirm operation',exact:true}).click();assert.deepEqual(await p.evaluate(()=>window.fixture.calls.at(-1)),{event:'set-access',value:{id:2,web_login_enabled:false}});
 await p.evaluate(()=>{window.fixture.state.error={code:'last_admin',message:'denied'};window.fixture.state.busy=false;});await d.getByText('At least one enabled administrator must remain.').waitFor();assert.equal(await p.evaluate(()=>window.fixture.state.selectedUser.web_login_enabled),true);
});
test('credential is masked, explicitly revealed, cleared on close and never persisted',async t=>{
 const p=await open(t);await p.evaluate(()=>{window.fixture.state.credential={user:window.fixture.state.users.items[0],temporary_password:'fictional-test-only-secret',expires_at:'2026-10-22T00:00:00Z'};});const input=p.locator('input#temporary-password');assert.equal(await input.getAttribute('type'),'password');await p.getByRole('button',{name:'Reveal password',exact:true}).click();assert.equal(await input.getAttribute('type'),'text');await p.getByRole('button',{name:'Close',exact:true}).last().click();await p.getByRole('dialog').waitFor({state:'hidden'});assert.equal(await p.evaluate(()=>window.fixture.state.credential),null);assert.equal(await p.evaluate(()=>Object.keys(localStorage).length),0);
});
test('Chinese, narrow screen, keyboard focus and revoked capabilities',async t=>{
 const p=await open(t,'cn',375);assert.equal(await p.locator('h1').innerText(),'用户管理');assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);await p.getByRole('button',{name:'创建用户',exact:true}).click();await p.getByRole('dialog').waitFor();await p.keyboard.press('Tab');assert.equal(await p.evaluate(()=>document.activeElement.closest('[role="dialog"]')!==null),true);await p.evaluate(()=>{window.fixture.state.currentUser.capabilities=[];});await p.getByRole('dialog').waitFor({state:'hidden'});assert.equal(await p.getByRole('button',{name:'创建用户',exact:true}).isDisabled(),true);await p.getByText('你没有执行此操作的权限。').waitFor();
});
test('role/status filters, loading and empty results remain distinct',async t=>{
 const p=await open(t);await p.getByLabel('Role',{exact:true}).selectOption('admin');await p.getByLabel('Console access',{exact:true}).selectOption('disabled');await p.getByRole('button',{name:'Search username',exact:true}).click();assert.deepEqual(await p.evaluate(()=>window.fixture.calls.at(-1).value),{search:undefined,role:'admin',web_login_enabled:false,page:1,page_size:20});
 await p.evaluate(()=>{window.fixture.state.users.items=[];window.fixture.state.users.total=0;window.fixture.state.loading=true;});await p.getByText('Loading users…',{exact:true}).waitFor();assert.equal(await p.getByText('No matching users.',{exact:true}).count(),0);
 await p.evaluate(()=>window.fixture.state.loading=false);await p.getByText('No matching users.',{exact:true}).waitFor();assert.equal(await p.getByRole('button',{name:'Next',exact:true}).isDisabled(),true);
});
test('role, reset and standalone session revocation emit distinct confirmed actions; audit is paginated',async t=>{
 const p=await open(t);await p.evaluate(()=>window.fixture.state.users.items[0].local_password_enabled=true);await p.getByRole('button',{name:/Details:/}).click();
 for(const [label,event,value] of [['Grant administrator role','set-role',{id:2,role:'admin'}],['Reset local password','reset-password',2],['Revoke all Web sessions','revoke-sessions',2]]){
  await p.getByRole('button',{name:label,exact:true}).click();await p.getByRole('dialog').getByRole('button',{name:'Confirm operation',exact:true}).click();assert.deepEqual(await p.evaluate(()=>window.fixture.calls.at(-1)),{event,value});await p.evaluate(()=>window.fixture.state.busy=false);await p.getByRole('dialog').waitFor({state:'hidden'});
 }
 await p.evaluate(()=>{window.fixture.state.events={items:[{id:5,actor_id:1,target_id:2,action:'revoke_sessions',occurred_at:'2026-10-21T00:00:00Z',result:'success'}],page:1,page_size:20,total:21};});await p.getByText(/2026-10-21T00:00:00Z · Revoke sessions · Success/).waitFor();await p.getByRole('navigation',{name:'Management audit',exact:true}).getByRole('button',{name:'Next',exact:true}).click();assert.deepEqual(await p.evaluate(()=>window.fixture.calls.at(-1)),{event:'query-events',value:{target_id:2,page:2,page_size:20}});
});
