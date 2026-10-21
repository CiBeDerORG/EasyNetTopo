// Actual backend/API + current production dist. No VPN clients or HTTP mocks.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createServer } from 'node:http';
import { createHash } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { preview } from 'vite';
import { chromium } from 'playwright';
import { resolveWebTestBinary } from './helpers/webTestBinary.mjs';
const binary = resolveWebTestBinary();
const password = 'Fictional-ecosystem-password-2026';
const hash = value => createHash('md5').update(value).digest('hex');
const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !/^(ET_|OIDC_|RUST_LOG$|(?:HTTP|HTTPS|ALL)_PROXY$)/i.test(key)));
let browser;
const services = [];
before(async () => { browser = await chromium.launch({ executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium', headless: true }); });
after(async () => {
    await browser?.close();
    for (const service of services.reverse()) {
        await service.preview?.close();
        if (service.process && service.process.exitCode === null) {
            await new Promise(resolve => { service.process.once('exit', resolve); service.process.kill('SIGTERM'); });
        }
        await new Promise(resolve => service.webhook.close(resolve));
        await rm(service.directory, { recursive: true, force: true });
    }
});
async function freePort() {
    const server = createServer(); await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const port = server.address().port; await new Promise(resolve => server.close(resolve)); return port;
}
async function start(mode) {
    const directory = await mkdtemp(join(tmpdir(), `eco114-${mode}-`));
    const webhook = createServer((req, res) => { res.writeHead(404); res.end(); });
    await new Promise(resolve => webhook.listen(0, '127.0.0.1', resolve));
    const service = { directory, webhook }; services.push(service);
    const database = join(directory, 'test.sqlite');
    const recovery = spawnSync(binary, ['--db', database, '--recover-admin', 'fictional-admin'], { input: `${password}\n`, encoding: 'utf8', env });
    assert.ifError(recovery.error); assert.equal(recovery.status, 0, 'isolated administrator recovery');
    const port = await freePort(), backend = `http://127.0.0.1:${port}`;
    const args = ['--db', database, '--api-server-addr', '127.0.0.1', '--api-server-port', String(port), '--config-server-protocol', 'tcp', '--config-server-port', '0', '--disable-registration', '--console-log-level', 'warn'];
    if (mode === 'webhook') args.push('--webhook-url', `http://127.0.0.1:${webhook.address().port}`, '--webhook-secret', 'fictional-webhook-secret');
    service.process = spawn(binary, args, { env, stdio: 'ignore' });
    let ready = false;
    for (let attempt = 0; attempt < 120; attempt++) {
        if (await fetch(`${backend}/api/v1/auth/policy`).then(response => response.ok).catch(() => false)) { ready = true; break; }
        if (service.process.exitCode !== null) break;
        await delay(250);
    }
    assert.ok(ready, 'isolated API starts');
    service.preview = await preview({ configFile: false, preview: { host: '127.0.0.1', port: await freePort(), strictPort: true, proxy: { '/api': { target: backend }, '/api_meta.js': { target: backend } } } });
    service.base = `http://127.0.0.1:${service.preview.httpServer.address().port}`;
    return service;
}
async function request(context, base, path, method = 'GET', data, expected = 200) {
    const response = await context.request.fetch(`${base}/api/v1${path}`, { method, data, headers: { 'X-Requested-With': 'EasyTier' } });
    assert.equal(response.status(), expected, `${method} ${path}`);
    const body = await response.text(); return body ? JSON.parse(body) : null;
}
for (const mode of ['central', 'webhook']) {
    test(`real ${mode} identity, network boundaries and 22s idle request ownership`, { timeout: 60000 }, async t => {
        const service = await start(mode), { base } = service;
        const context = await browser.newContext({ baseURL: base }); t.after(() => context.close());
        await context.addInitScript(() => localStorage.setItem('lang', 'en'));
        await request(context, base, '/auth/login', 'POST', { username: 'fictional-admin', password: hash(password) });
        const identity = await request(context, base, '/auth/me'); assert.equal(identity.role, 'admin');
        const info = await request(context, base, '/console-info'); assert.equal(info.webhook_auth, mode === 'webhook');
        let network;
        if (mode === 'central') network = await request(context, base, '/networks', 'POST', { settings: { display_name: 'Fictional ECO114 network', network_name: null, networking_method: 'Standalone', public_server_url: null, peer_urls: [], virtual_cidr: null, secure_mode: false } });
        const page = await context.newPage(); page.setDefaultTimeout(10000);
        const requests = [], errors = [];
        page.on('request', req => { if (req.url().includes('/api/v1/')) requests.push(new URL(req.url()).pathname.replace('/api/v1', '')); });
        page.on('pageerror', error => errors.push(error.message));
        await page.goto('/#/h/ecosystem');
        await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
        assert.equal(await page.locator('.console-page-title').innerText(), 'Networks and resources');
        if (network) {
            await page.locator('.ecosystem-topology select').first().selectOption(network.network_id);
            await page.locator('.ecosystem-members').waitFor();
            assert.ok(requests.includes(`/networks/${network.network_id}/members`));
        }
        const before = requests.length;
        await delay(22000);
        const idle = requests.slice(before);
        assert.equal(idle.includes('/machines'), false, 'ecosystem overview is not polled');
        assert.equal(idle.some(path => path.endsWith('/members')), false, 'selected members are not polled');
        assert.ok(idle.filter(path => path === '/auth/me').length >= 2, 'old host identity timer remains observable');
        if (mode === 'webhook') assert.equal(requests.some(path => path.startsWith('/networks')), false);
        console.log(JSON.stringify({ mode, idleMilliseconds: 22000, idlePaths: idle, initialMachines: requests.filter(path => path === '/machines').length }));
        assert.deepEqual(errors, []);
        await request(context, base, '/auth/logout', 'GET');
        await page.reload(); await page.waitForURL('**/#/auth*');
    });
}

test('real ordinary identity cannot read another user network and forced identity cannot mount ecosystem', { timeout: 30000 }, async t => {
    const { base } = services[0];
    const admin = await browser.newContext({ baseURL: base }); t.after(() => admin.close());
    await request(admin, base, '/auth/login', 'POST', { username: 'fictional-admin', password: hash(password) });
    const created = await request(admin, base, '/admin/users', 'POST', { username: 'fictional-eco-reader' }, 201);
    const context = await browser.newContext({ baseURL: base }); t.after(() => context.close());
    await context.addInitScript(() => localStorage.setItem('lang', 'en'));
    await request(context, base, '/auth/login', 'POST', { username: created.user.username, password: hash(created.temporary_password) });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    const requests = [];
    page.on('request', req => requests.push(new URL(req.url()).pathname));
    await page.goto('/#/h/ecosystem'); await page.waitForURL('**/#/h/password');
    assert.equal(requests.some(path => path === '/api/v1/machines'), false);
    await request(context, base, '/auth/password', 'PUT', { new_password: hash(password) });
    await request(context, base, '/auth/login', 'POST', { username: created.user.username, password: hash(password) });
    const networks = await request(admin, base, '/networks');
    const denied = await context.request.get(`${base}/api/v1/networks/${networks.networks[0].network_id}/members`);
    assert.ok([403, 404].includes(denied.status()), 'foreign network must not disclose members');
    await request(context, base, '/admin/users', 'GET', undefined, 403);
    await page.goto('/#/h/ecosystem');
    await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('link', { name: 'User management', exact: true }).count(), 0);
    assert.equal(await page.getByText('Fictional ECO114 network', { exact: true }).count(), 0);
});

test('real deleted network reports not-found and explicit refresh removes stale selection', async t => {
    const { base } = await start('central');
    const context = await browser.newContext({ baseURL: base }); t.after(() => context.close());
    await context.addInitScript(() => localStorage.setItem('lang', 'en'));
    await request(context, base, '/auth/login', 'POST', { username: 'fictional-admin', password: hash(password) });
    const network = await request(context, base, '/networks', 'POST', { settings: { display_name: 'Fictional disposable network', network_name: null, networking_method: 'Standalone', public_server_url: null, peer_urls: [], virtual_cidr: null, secure_mode: false } });
    const networkId = network.network_id;
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    await page.goto('/#/h/ecosystem');
    await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
    await request(context, base, `/networks/${networkId}`, 'DELETE', undefined, 204);
    await page.locator('#ecosystem-network').selectOption(networkId);
    await page.getByText('The selected object was not found.', { exact: true }).waitFor();
    await page.locator('.ecosystem-members').getByRole('button', { name: 'Refresh', exact: true }).click();
    await page.locator('.ecosystem-members').waitFor({ state: 'hidden' });
    assert.equal(await page.locator('#ecosystem-network').inputValue(), '');
});
