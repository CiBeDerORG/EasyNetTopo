// Real isolated SQLite/API + a frozen frontend; no Core, VPN, real identities or API mocks.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, rm, writeFile, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'node:net';
import { createHash } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from 'playwright';
import { preview } from 'vite';
import { resolveWebTestBinary } from './helpers/webTestBinary.mjs';

const binary = resolveWebTestBinary();
const dist = process.env.WEB_TEST_DIST;
if (!dist) throw new Error('WEB_TEST_DIST must point to the frozen frontend being verified.');
const evidence = fileURLToPath(new URL('../../../process/web-ui-optimization-done/plan/verification/', import.meta.url));
const username = 'fictional-wui-admin';
const password = 'Fictional-wui-password-2026';
const headers = { 'X-Requested-With': 'EasyTier' };
const environment = Object.fromEntries(Object.entries(process.env).filter(([key]) => !/^(ET_|OIDC_|RUST_LOG$|(?:HTTP|HTTPS|ALL)_PROXY$)/i.test(key)));
let directory, backend, server, browser, admin, base, network, backendBase;
const contexts = new Set();
async function api(context, path, method = 'GET', data, status = 200) {
    const response = await context.request.fetch(`/api/v1${path}`, { method, data, headers });
    assert.equal(response.status(),status,`${method} ${path}: unexpected status`);
    return status === 204 ? null : response.json();
}
async function openContext({ language = 'en', theme = 'light', width = 1440, authenticated = true } = {}) {
    const context = await browser.newContext({baseURL:base, viewport:{width,height:900}, permissions:['clipboard-read','clipboard-write'], ...(authenticated ? {storageState:await admin.storageState()} : {})});
    contexts.add(context);
    await context.addInitScript(({language,theme})=>{localStorage.setItem('lang',language);localStorage.setItem('console-theme-mode',theme);},{language,theme});
    context.setDefaultTimeout(10000);
    return context;
}
async function closeContext(context) { contexts.delete(context); await context.close(); }
before(async () => {
    await mkdir(evidence,{recursive:true}); directory=await mkdtemp(join(tmpdir(),'wui112-isolated-'));
    const database=join(directory,'isolated.sqlite');
    const recovered=spawnSync(binary,['--db',database,'--recover-admin',username],{input:`${password}\n`,encoding:'utf8',env:environment});
    assert.equal(recovered.status,0,'isolated administrator recovery');
    const reservation=createServer(); await new Promise(resolve=>reservation.listen(0,'127.0.0.1',resolve));
    const port=reservation.address().port; await new Promise(resolve=>reservation.close(resolve));
    backendBase=`http://127.0.0.1:${port}`;
    backend=spawn(binary,['--db',database,'--api-server-addr','127.0.0.1','--api-server-port',String(port),'--config-server-protocol','tcp','--config-server-port','0','--disable-registration','--console-log-level','warn'],{env:environment,stdio:'ignore'});
    let ready=false;
    for(let attempt=0;attempt<120;attempt++){if(await fetch(`${backendBase}/api/v1/auth/policy`).then(r=>r.ok).catch(()=>false)){ready=true;break;}if(backend.exitCode!==null)break;await delay(250);}
    assert.ok(ready,'isolated backend starts');
    server=await preview({configFile:false,build:{outDir:dist},preview:{host:'127.0.0.1',port:0,cors:false,proxy:{'/api/':{target:backendBase,changeOrigin:false},'/api_meta.js':{target:backendBase,changeOrigin:false}}}});
    base=`http://127.0.0.1:${server.httpServer.address().port}`;
    browser=await chromium.launch({headless:true,executablePath:process.env.WEB_TEST_CHROMIUM||'/usr/bin/chromium'});
    admin=await openContext({authenticated:false});
    await api(admin,'/auth/login','POST',{username,password:createHash('md5').update(password).digest('hex')});
    network=await api(admin,'/networks','POST',{settings:{display_name:'fictional-wui-long-network-name-for-responsive-layout',network_name:null,networking_method:'Standalone',peer_urls:[],public_server_url:null,virtual_cidr:null,secure_mode:false}});
    for(const name of ['fictional-long-user-layout','fictional-short']) await api(admin,'/admin/users','POST',{username:name},201);
    await writeFile(join(evidence,'environment.json'),JSON.stringify({frontendCommit:'97112bef',frontendIndexSha256:createHash('sha256').update(await readFile(join(dist,'index.html'))).digest('hex'),binary,backendRebuilt:false,base,backendBase,mode:'real isolated API and SQLite; no connected devices, no VPN; config-server TCP port 0'},null,2)+'\n');
}, {timeout:45000});
after(async () => {
    for(const context of contexts) await context.close(); await browser?.close();
    if(server) await new Promise(resolve=>server.httpServer.close(resolve));
    if(backend && backend.exitCode===null){backend.kill('SIGTERM');await Promise.race([new Promise(resolve=>backend.once('exit',resolve)),delay(3000)]);if(backend.exitCode===null)backend.kill('SIGKILL');}
    if(directory) await rm(directory,{recursive:true,force:true});
});

test('real API page matrix: eight pages across CN/EN, themes and three widths', async () => {
    const matrix=[];
    const routes=[['login','/auth'],['register','/auth/register'],['password','/h/password'],['users','/h/users'],['dashboard','/h'],['devices','/h/deviceList'],['networks','/h/networks'],['network-detail',`/h/networks/${network.network_id}`]];
    for(const language of ['en','cn']) for(const theme of ['light','dark']) for(const width of [360,768,1440]) {
        const context=await openContext({language,theme,width}); const page=await context.newPage(); const errors=[];
        page.on('pageerror',error=>errors.push(error.message));
        for(const [name,path] of routes){
            await page.goto(`/#${path}`);
            if(name==='login') await page.locator('#username').waitFor();
            else if(name==='register') await page.getByText(/registration is disabled|公开注册已关闭/i).first().waitFor();
            else await page.locator('.console-page, .user-management, .p-card').first().waitFor();
            await page.waitForTimeout(300);
            const measured=await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,headings:[...document.querySelectorAll('h1,h2')].map(el=>el.textContent.trim())}));
            const screenshot=(language==='en'&&theme==='light'&&width===1440)||(language==='cn'&&theme==='dark'&&width===360);
            if(screenshot) await page.screenshot({path:join(evidence,`${name}-${language}-${theme}-${width}.png`),fullPage:true,animations:'disabled'});
            matrix.push({page:name,language,theme,width,...measured,pageErrors:[...errors],screenshot:screenshot?`${name}-${language}-${theme}-${width}.png`:null});
            assert.equal(measured.scrollWidth>width,false,`${name}/${language}/${theme}/${width}: overflow`);
            assert.deepEqual(errors,[]);
        }
        await closeContext(context);
    }
    await writeFile(join(evidence,'page-matrix.json'),JSON.stringify(matrix,null,2)+'\n');
    assert.equal(matrix.length,96);
});

test('real network ACL and settings writes persist, missing network is not an empty success', async () => {
    const context=await openContext();const page=await context.newPage();await page.goto(`/#/h/networks/${network.network_id}`);
    await page.getByRole('tab',{name:'Access Control',exact:true}).click();
    await page.getByRole('button',{name:'Deny by default',exact:true}).click();
    await page.getByRole('button',{name:'Save Policy',exact:true}).click();await page.getByText('Access policy saved',{exact:true}).waitFor();
    assert.equal((await api(context,`/networks/${network.network_id}/acl-policy`)).policy.default_action,'deny');
    await page.getByRole('tab',{name:'Settings',exact:true}).click();
    assert.equal(await page.getByLabel('Network Secret',{exact:true}).getAttribute('type'),'password');
    await page.locator('#settings-display-name').fill('fictional-wui-long-network-name-for-responsive-layout-edited');
    await page.getByRole('button',{name:'Save',exact:true}).click();
    await page.getByText('Config saved',{exact:false}).first().waitFor();
    network = await api(context,`/networks/${network.network_id}`);
    assert.equal(network.display_name,'fictional-wui-long-network-name-for-responsive-layout-edited');
    await page.goto('/#/h/networks/00000000-0000-0000-0000-000000000099');
    await page.getByText('Unable to refresh data',{exact:false}).waitFor();
    assert.equal(await page.getByRole('tab',{name:'Access Control',exact:true}).count(),0);
    await closeContext(context);
});

test('real mobile focus, long-title sizing, user table and exact enrollment copy', async () => {
    const context=await openContext({language:'cn',theme:'dark',width:360});const page=await context.newPage();
    await page.goto('/#/h/users');await page.locator('.user-management tbody tr').first().waitFor();
    const userTable=await page.evaluate(()=>({tableWidth:document.querySelector('.table-scroll table').getBoundingClientRect().width,containerWidth:document.querySelector('.table-scroll').clientWidth,usernameWidth:document.querySelector('.user-management tbody td').getBoundingClientRect().width}));
    assert.ok(userTable.usernameWidth>=140);
    const toggle=page.locator('.mobile-nav-toggle');
    for(let i=0;i<3;i++){await toggle.focus();await page.keyboard.press('Enter');await page.locator('.console-mobile-nav').waitFor();await page.keyboard.press('Escape');await page.locator('.console-mobile-nav').waitFor({state:'hidden'});await page.waitForFunction(()=>document.activeElement?.classList.contains('mobile-nav-toggle'));}
    await page.goto(`/#/h/networks/${network.network_id}`);await page.getByRole('heading',{name:network.display_name,exact:true}).waitFor();
    const title=await page.evaluate(()=>({height:document.querySelector('.console-topbar').getBoundingClientRect().height,buttonWidth:document.querySelector('.mobile-nav-toggle').getBoundingClientRect().width}));assert.ok(title.height<=80);assert.ok(title.buttonWidth>=28);
    await page.goto('/#/h/deviceList');await page.locator('.console-empty-state code').waitFor();
    const command=await page.locator('.console-empty-state code').innerText();await page.locator('.console-empty-state button').click();
    assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),command);assert.match(command,/^easytier-core --config-server tcp:\/\/127\.0\.0\.1:0\/fictional-wui-admin$/);
    await writeFile(join(evidence,'interaction-observations.json'),JSON.stringify({userTable,title,escapeFocusRestored:3,enrollmentMatchesRendered:true,configServerCommandStructureUnchanged:true,commandExecuted:false},null,2)+'\n');
    await closeContext(context);
});

test('idle network list keeps existing list/shell requests; leaving devices stops machine polling', async () => {
    const context=await openContext();const page=await context.newPage();const requests=[];
    page.on('request',request=>{const url=new URL(request.url());if(url.pathname.startsWith('/api/v1/'))requests.push({path:url.pathname,method:request.method(),at:Date.now()});});
    await page.goto('/#/h/deviceList');await page.locator('.console-empty-state code').waitFor();await page.waitForTimeout(2200);
    assert.ok(requests.filter(item=>item.path==='/api/v1/machines').length>=2);
    await page.goto('/#/h/networks');await page.locator('.page-heading').waitFor();await page.waitForTimeout(500);requests.length=0;
    await page.waitForTimeout(22000);
    assert.equal(requests.some(item=>item.path==='/api/v1/machines'),false);
    assert.equal(requests.some(item=>item.method!=='GET'),false);
    const allowed=new Set(['/api/v1/auth/me','/api/v1/networks','/api/v1/blocked-devices']);
    assert.ok(requests.every(item=>allowed.has(item.path)));
    await writeFile(join(evidence,'idle-requests.json'),JSON.stringify({durationMs:22000,requests,interpretation:'existing 2-second NetworkList and 10-second shell refresh; no device polling after navigation'},null,2)+'\n');
    await closeContext(context);
});
