// Production components with fictional HTTP fixtures, never real VPN devices.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';
let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });
async function openDetail(t, options = {}) {
    const state = { aclFailure: false, saveFailure: false, writes: [], requests: [], hold: null, release: null, ...options };
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width: options.width ?? 360, height: 900 } });
    t.after(async () => { state.release?.(); await context.close(); });
    await context.addInitScript(({language, theme}) => { localStorage.setItem('lang', language); localStorage.setItem('console-theme-mode', theme); }, { language: options.language ?? 'en', theme: options.theme ?? 'light' });
    await context.route('**/api_meta.js', route => route.fulfill({ body: '' }));
    await context.route('**/api/v1/**', async route => {
        const path = new URL(route.request().url()).pathname.replace('/api/v1', '');
        state.requests.push(path);
        if (path === '/networks/fixture-a' && route.request().method() === 'PATCH') {
            state.writes.push(route.request().postDataJSON()); if (state.hold) await state.hold;
            return route.fulfill({status:503,json:{message:'Fixture settings failed'}});
        }
        if (path.endsWith('/acl-policy')) {
            if (route.request().method() === 'PUT') {
                state.writes.push(route.request().postDataJSON()); if (state.hold) await state.hold;
                return route.fulfill({ status: state.saveFailure ? 503 : 200, json: state.saveFailure ? {message:'Fixture save failed'} : {policy:state.writes.at(-1)} });
            }
            return route.fulfill({status:state.aclFailure ? 503 : 200, json: state.aclFailure ? {} : {policy:state.policy ?? {default_action:'allow',rules:[]}}});
        }
        if (path === '/networks/fixture-a' && state.networkHold) await state.networkHold;
        const data = {
            '/auth/me': {id:1,username:'fixture-admin',role:'admin',capabilities:['users.manage'],source:'local',local_password_enabled:true,web_login_enabled:true,must_change_password:false},
            '/console-info':{username:'fixture-admin',webhook_auth:false},
            '/networks':{networks:[]}, '/machines':{machines:[]},
            '/networks/gateway-info':{enabled:false},
            '/networks/fixture-a':{network_id:'fixture-a',display_name:'Fictional network',network_name:'fixture-network',network_secret:'FICTIONAL-TEST-SECRET',networking_method:'Standalone',secure_mode:false,member_count:0,online_member_count:0,peer_urls:[]},
            '/networks/fixture-a/members':{members:[],temporary_peers:[]},
        };
        return route.fulfill({status:path in data ? 200 : 404,json:data[path] ?? {code:'not_found'}});
    });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    await page.goto('/#/h/networks/fixture-a');
    if (!options.skipAcl) await page.getByRole('tab', {name:/Access Control|访问控制/}).click();
    return {page,state};
}
test('ACL failure is not default-allow empty state and explicit retry restores policy', async t => {
    const {page,state} = await openDetail(t,{aclFailure:true});
    await page.getByText('Unable to refresh data', {exact:false}).first().waitFor();
    assert.equal(await page.getByText('The network keeps default-allow behavior; unmatched traffic is unrestricted', {exact:true}).count(),0);
    assert.equal(await page.getByRole('button',{name:'Add Rule',exact:true}).count(),0);
    state.aclFailure=false;
    await page.getByRole('button',{name:'Retry',exact:true}).click();
    await page.getByRole('button',{name:'Add Rule',exact:true}).waitFor();
});
test('pending ACL save locks draft and ignores duplicate click; failed save retains draft', async t => {
    const {page,state}=await openDetail(t);
    await page.getByRole('button',{name:'Deny by default',exact:true}).click();
    state.saveFailure=true; state.hold=new Promise(resolve=>{state.release=resolve;});
    const request=page.waitForRequest(req=>req.method()==='PUT');
    await page.getByRole('button',{name:'Save Policy',exact:true}).click(); await request;
    assert.equal(await page.getByRole('button',{name:'Add Rule',exact:true}).isDisabled(),true);
    await page.getByRole('button',{name:'Save Policy',exact:true}).dispatchEvent('click');
    state.release(); state.hold=null;
    await page.getByText(/Fixture save failed|Save failed/).first().waitFor();
    assert.equal(state.writes.length,1);
    assert.equal(state.writes[0].default_action,'deny');
    assert.equal(await page.getByRole('button',{name:'Save Policy',exact:true}).isEnabled(),true);
});

test('leaving during initial detail read discards its result and stops follow-up requests', async t => {
    let release;
    const networkHold = new Promise(resolve => { release = resolve; });
    const {page,state} = await openDetail(t,{networkHold,skipAcl:true});
    t.after(release);
    assert.equal(state.requests.includes('/networks/fixture-a'),true);
    await page.locator('.page-heading button').click();
    await page.waitForURL('**/#/h/networks');
    release();
    await page.waitForTimeout(2600);
    assert.equal(state.requests.filter(path => path === '/networks/fixture-a').length,1);
    assert.equal(state.requests.filter(path => path.endsWith('/members')).length,0);
});

test('settings secret starts masked, pending save blocks edits and repeat writes, failure keeps draft', async t => {
    const {page,state}=await openDetail(t,{skipAcl:true});
    await page.getByRole('tab',{name:'Settings',exact:true}).click();

    assert.equal(await page.getByLabel('Network Secret',{exact:true}).getAttribute('type'),'password');
    await page.locator('#settings-display-name').fill('Preserved fictional draft');
    state.hold=new Promise(resolve=>{state.release=resolve;});
    const request=page.waitForRequest(req=>req.method()==='PATCH');
    await page.getByRole('button',{name:'Save',exact:true}).click(); await request;
    assert.equal(await page.locator('#settings-display-name').isDisabled(),true);
    await page.getByRole('button',{name:'Save',exact:true}).dispatchEvent('click');
    state.release(); state.hold=null;
    await page.getByText('Fixture settings failed',{exact:true}).waitFor();
    assert.equal(state.writes.length,1);
    assert.equal(await page.locator('#settings-display-name').inputValue(),'Preserved fictional draft');
});

test('network detail and ACL remain usable across 12 language/theme/width combinations', async t => {
    for (const language of ['cn','en']) for (const theme of ['light','dark']) for (const width of [360,768,1440]) {
        const {page,state}=await openDetail(t,{language,theme,width});
        await page.getByRole('button',{name:/^(Add Rule|新增规则)$/}).waitFor();
        assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`${language}/${theme}/${width}`);
        assert.equal(await page.locator('body').innerText().then(text=>/web\.(acl|common|console)\./.test(text)),false);
        await page.getByRole('tab',{name:/^(Settings|网络设置)$/}).focus(); await page.keyboard.press('Enter');
        await page.locator('#settings-display-name').waitFor();
        assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
        assert.equal(state.writes.length,0);
        await page.context().close();
    }
});

test('ACL successful save preserves disabled rules and order; unmounted save cannot toast into next page', async t => {
    const rules=['first','second'].map((id,index)=>({id,name:`Fictional rule ${index}`,enabled:false,action:'deny',sources:[{type:'all'}],destinations:[{type:'all'}],protocols:[{protocol:'icmp',ports:[]}]}));
    const {page,state}=await openDetail(t,{policy:{default_action:'allow',rules}});
    await page.getByRole('button',{name:'Deny by default',exact:true}).click();
    await page.getByRole('button',{name:'Save Policy',exact:true}).click();
    await page.getByText('Access policy saved',{exact:true}).waitFor();
    assert.deepEqual(state.writes[0].rules,rules);
    await page.getByRole('button',{name:'Allow by default',exact:true}).click();
    state.hold=new Promise(resolve=>{state.release=resolve;});
    const request=page.waitForRequest(req=>req.method()==='PUT');
    await page.getByRole('button',{name:'Save Policy',exact:true}).click(); await request;
    await page.locator('.page-heading button').click(); await page.waitForURL('**/#/h/networks');
    // Wait out the earlier successful toast before releasing the old request.
    await page.getByText('Access policy saved',{exact:true}).waitFor({state:'hidden'});
    state.release(); state.hold=null;
    await page.waitForTimeout(250);
    assert.equal(await page.getByText('Access policy saved',{exact:true}).count(),0);
});
