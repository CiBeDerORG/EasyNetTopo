// Real loopback services and disposable SQLite only. No core/VPN process is started.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createServer } from 'node:http';
import { createHash, generateKeyPairSync, randomUUID, sign } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from 'playwright';
import { preview } from 'vite';
import { resolveWebTestBinary } from './helpers/webTestBinary.mjs';

const binary = resolveWebTestBinary();
// Never inherit deployment credentials, endpoints, proxies or file-log locations.
const testEnvironment = Object.fromEntries(Object.entries(process.env).filter(([key]) => !/^(ET_|OIDC_|RUST_LOG$|(?:HTTP|HTTPS|ALL)_PROXY$)/i.test(key)));
const adminPassword = 'Fictional-admin-password-2026';
const userPassword = 'Fictional-user-password-2026';
const md5 = value => createHash('md5').update(value).digest('hex');
let frontendPreview;
let directory, web, browser, base, admin, idp, issuer, logs = '', identity, tokenMode;
const errorMarker = 'FICTIONAL-OIDC-SECRET-MARKER';
const errorMarkerBytes = Array.from(Buffer.from(errorMarker)).join(', ');
const secrets = [adminPassword, userPassword, md5(adminPassword), md5(userPassword)];
const oidcHits = new Set();
const browserMessages = [];
const contexts = [];
async function listen(server) { await new Promise(r => server.listen(0, '127.0.0.1', r)); return server.address().port; }
async function port() { const s = createServer(); const p = await listen(s); await new Promise(r => s.close(r)); return p; }
async function context() { const c = await browser.newContext({ baseURL: base }); contexts.push(c); c.on('page', page => page.on('console', message => browserMessages.push(message.text()))); c.setDefaultTimeout(10000); c.setDefaultNavigationTimeout(10000); await c.addInitScript(() => localStorage.setItem('lang', 'en')); return c; }
async function api(c, path, method = 'GET', data, expected = 200, headers = {}) {
    const response = await c.request.fetch(`${base}/api/v1${path}`, { method, data, headers: { 'X-Requested-With': 'EasyTier', ...headers } });
    assert.equal(response.status(), expected, `${method} ${path}: unexpected HTTP status`);
    const text = await response.text(); return text ? JSON.parse(text) : null;
}
async function login(c, name, password, expected = 200) { return api(c, '/auth/login', 'POST', { username: name, password: md5(password) }, expected); }
async function create(name) { const result = await api(admin, '/admin/users', 'POST', { username: name }, 201); secrets.push(result.temporary_password, md5(result.temporary_password)); return result; }
async function readyUser(name) { const result = await create(name); const c = await context(); await login(c, name, result.temporary_password); await api(c, '/auth/password', 'PUT', { new_password: md5(userPassword) }); await login(c, name, userPassword); return { c, user: result.user }; }

before(async () => {
    directory = await mkdtemp(join(tmpdir(), 'easytier-wum-test-'));
    const { privateKey, publicKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
    const jwk = { ...publicKey.export({ format: 'jwk' }), kid: 'fictional-test-key', use: 'sig', alg: 'RS256' };
    const codes = new Map();
    identity = { sub: 'fictional-subject-1', preferred_username: 'fictional-oidc' };
    idp = createServer(async (req, res) => {
        const url = new URL(req.url, issuer); oidcHits.add(url.pathname);
        const json = (body, status = 200) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(body)); };
        if (url.pathname === '/.well-known/openid-configuration') return json({ issuer, authorization_endpoint: `${issuer}/authorize`, token_endpoint: `${issuer}/token`, jwks_uri: `${issuer}/jwks`, response_types_supported: ['code'], subject_types_supported: ['public'], id_token_signing_alg_values_supported: ['RS256'], token_endpoint_auth_methods_supported: ['client_secret_basic', 'none'] });
        if (url.pathname === '/jwks') return json({ keys: [jwk] });
        if (url.pathname === '/authorize') {
            const q = url.searchParams; const code = randomUUID(); secrets.push(code, q.get('state'), q.get('nonce'));
            if (tokenMode === 'authorization-error') { const callback = new URL(q.get('redirect_uri')); callback.searchParams.set('error', 'access_denied'); callback.searchParams.set('error_description', errorMarker); callback.searchParams.set('state', q.get('state')); res.writeHead(302, { Location: callback.href }); return res.end(); }
            if (!q.get('state') || !q.get('nonce') || q.get('code_challenge_method') !== 'S256') return json({ error: 'invalid_request' }, 400);
            codes.set(code, { nonce: q.get('nonce'), challenge: q.get('code_challenge'), redirect: q.get('redirect_uri'), identity: { ...identity } });
            const callback = new URL(q.get('redirect_uri')); callback.searchParams.set('code', code); callback.searchParams.set('state', tokenMode === 'wrong-state' ? 'fictional-wrong-state' : q.get('state'));
            res.writeHead(302, { Location: callback.href }); return res.end();
        }
        if (url.pathname === '/token') {
            let body = ''; for await (const chunk of req) body += chunk;
            const q = new URLSearchParams(body), entry = codes.get(q.get('code')); codes.delete(q.get('code'));
            if (!entry || q.get('grant_type') !== 'authorization_code' || q.get('redirect_uri') !== entry.redirect || createHash('sha256').update(q.get('code_verifier') || '').digest('base64url') !== entry.challenge) return json({ error: 'invalid_grant' }, 400);
            if (tokenMode === 'oauth-error') return json({ error: 'invalid_grant', error_description: errorMarker }, 400);
            if (tokenMode?.startsWith('malformed-')) { res.writeHead(tokenMode === 'malformed-success' ? 200 : 400, { 'Content-Type': 'application/json' }); return res.end(errorMarker); }
            const encode = value => Buffer.from(JSON.stringify(value)).toString('base64url');
            const payload = `${encode({ alg: 'RS256', kid: jwk.kid })}.${encode({ iss: issuer, aud: 'fictional-client', iat: Math.floor(Date.now() / 1000), exp: Math.floor(Date.now() / 1000) + 300, nonce: tokenMode === 'wrong-nonce' ? 'fictional-wrong-nonce' : entry.nonce, ...entry.identity })}`;
            const token = `${payload}.${sign('RSA-SHA256', Buffer.from(payload), privateKey).toString('base64url')}`;
            secrets.push(token);
            return json({ access_token: 'fictional-access-token', token_type: 'Bearer', expires_in: 300, id_token: token });
        }
        json({ error: 'not_found' }, 404);
    });
    issuer = `http://127.0.0.1:${await listen(idp)}`;
    const database = join(directory, 'isolated.sqlite');
    const recovery = spawnSync(binary, ['--db', database, '--recover-admin', 'fictional-admin'], { input: `${adminPassword}\n`, encoding: 'utf8', env: testEnvironment });
    assert.ifError(recovery.error);
    assert.equal(recovery.status, 0, `offline recovery succeeds using stdin (backend: ${binary})`);
    logs += recovery.stdout + recovery.stderr;
    const backendBase = `http://127.0.0.1:${await port()}`;
    base = backendBase;
    if (process.env.WEB_TEST_DIST) {
        frontendPreview = await preview({ configFile: false, build: { outDir: process.env.WEB_TEST_DIST }, preview: { host: '127.0.0.1', port: 0, cors: false, proxy: { '/api/': { target: backendBase, changeOrigin: false }, '/api_meta.js': { target: backendBase, changeOrigin: false } } } });
        base = `http://127.0.0.1:${frontendPreview.httpServer.address().port}`;
    }
    web = spawn(binary, ['--db', database, '--api-server-addr', '127.0.0.1', '--api-server-port', new URL(backendBase).port, '--config-server-protocol', 'tcp', '--config-server-port', '0', '--disable-registration', '--console-log-level', 'warn', '--oidc-issuer-url', issuer, '--oidc-client-id', 'fictional-client', '--oidc-redirect-url', `${base}/api/v1/auth/oidc/callback`], { stdio: ['ignore', 'pipe', 'pipe'], env: testEnvironment });
    let startError;
    web.once('error', error => { startError = error; });
    for (const stream of [web.stdout, web.stderr]) stream.on('data', data => { logs += data; });
    let ready = false;
    for (let i = 0; i < 120; i++) { if (await fetch(`${backendBase}/api/v1/auth/policy`).then(r => r.ok).catch(() => false)) { ready = true; break; } if (startError || web.exitCode !== null) break; await delay(250); }
    assert.ifError(startError);
    assert.ok(ready, `isolated backend starts (backend: ${binary})`);
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
    admin = await context(); await login(admin, 'fictional-admin', adminPassword);
}, { timeout: 45000 });
after(async () => {
    for (const c of contexts) { secrets.push(...(await c.cookies()).map(cookie => cookie.value)); await c.close(); } await browser?.close();
    if (web && web.exitCode === null) { web.kill('SIGTERM'); await Promise.race([new Promise(r => web.once('exit', r)), delay(3000)]); if (web.exitCode === null) web.kill('SIGKILL'); }
    await new Promise(r => idp ? idp.close(r) : r());
    await new Promise(r => frontendPreview ? frontendPreview.httpServer.close(r) : r());
    await rm(directory, { recursive: true, force: true });
    const output = logs + browserMessages.join('\n');
    for (const secret of [...secrets, errorMarker, errorMarkerBytes, 'fictional-access-token', '$argon2']) if (secret) assert.ok(!output.includes(secret), 'captured logs exclude credentials and provider response bodies');
});

test('UA-01/02/03/05: authorization, duplicate races, registration and first password', async () => {
    const anonymous = await context(); await api(anonymous, '/admin/users', 'GET', undefined, 401);
    assert.deepEqual(await api(anonymous, '/auth/policy'), { registration_enabled: false, oidc_enabled: true });
    const registration = await anonymous.request.post(`${base}/api/v1/auth/register`, { data: { credentials: { username: 'fictional-registration', password: md5(userPassword) }, captcha: 'fictional' } }); assert.equal(registration.status(), 403);
    const created = await create('fictional-user'); const user = await context();
    await login(user, 'fictional-user', created.temporary_password);
    assert.equal((await api(user, '/auth/me')).must_change_password, true);
    await api(user, '/admin/users', 'GET', undefined, 403);
    await api(user, '/summary', 'GET', undefined, 403);
    await api(user, '/auth/password', 'PUT', { new_password: md5(userPassword) });
    await api(user, '/auth/me', 'GET', undefined, 401); await login(user, 'fictional-user', userPassword);
    await api(user, '/admin/users', 'GET', undefined, 403);
    const race = await Promise.all([0, 1].map(() => admin.request.post(`${base}/api/v1/admin/users`, { headers: { 'X-Requested-With': 'EasyTier' }, data: { username: 'fictional-race' } })));
    assert.deepEqual(race.map(r => r.status()).sort(), [201, 409]);
    const reset = await api(admin, `/admin/users/${created.user.id}/password-reset`, 'POST', {}); secrets.push(reset.temporary_password);
    await api(user, '/auth/me', 'GET', undefined, 401); await login(user, 'fictional-user', userPassword, 401);
    await login(user, 'fictional-user', reset.temporary_password); await api(user, '/summary', 'GET', undefined, 403);
});

test('UM-02: literal search, AND filters, stable pages and empty results', async () => {
    await create('fictional-page-a'); await create('fictional-page-b'); await create('fictional-page-c');
    const history = (await api(admin, '/admin/users?search=user')).items.find(user => user.username === 'user');
    assert.equal(history.source, 'unknown'); assert.equal(history.created_at, null); assert.equal(history.last_login_at, null); assert.equal(history.local_password_enabled, null);
    await api(admin, `/admin/users/${history.id}/password-reset`, 'POST', {}, 403);
    const all = await api(admin, '/admin/users?search=FICTIONAL-PAGE&role=user&web_login_enabled=true&page_size=2');
    const second = await api(admin, '/admin/users?search=fictional-page&role=user&web_login_enabled=true&page_size=2&page=2');
    assert.equal(all.total, 3); assert.equal(all.items.length, 2); assert.equal(second.items.length, 1);
    assert.ok(all.items[1].id < second.items[0].id);
    assert.deepEqual((await api(admin, '/admin/users?search=fictional-page&role=admin')).items, []);
    assert.deepEqual((await api(admin, '/admin/users?search=%25')).items, []);
    assert.deepEqual((await api(admin, '/admin/users?search=fictional-page&web_login_enabled=false')).items, []);
});

test('UA-04 and UM-07: independent revoke, disable and restore never revive old sessions', async () => {
    const { c, user } = await readyUser('fictional-session'); const other = await context(); await login(other, user.username, userPassword);
    const before = await api(admin, `/admin/users/${user.id}`);
    await api(admin, `/admin/users/${user.id}/revoke-sessions`, 'POST', {});
    const after = await api(admin, `/admin/users/${user.id}`);
    for (const key of ['role', 'web_login_enabled', 'must_change_password']) assert.equal(after[key], before[key]);
    for (const old of [c, other]) await api(old, '/auth/me', 'GET', undefined, 401);
    await login(c, user.username, userPassword); await login(other, user.username, userPassword);
    await api(admin, `/admin/users/${user.id}/access`, 'PUT', { web_login_enabled: false });
    for (const old of [c, other]) await api(old, '/auth/me', 'GET', undefined, 401);
    await login(await context(), user.username, userPassword, 401);
    await api(admin, `/admin/users/${user.id}/access`, 'PUT', { web_login_enabled: true });
    for (const old of [c, other]) await api(old, '/auth/me', 'GET', undefined, 401);
    await login(c, user.username, userPassword);
});

test('UA-06/15: real OIDC login, password capability, stable binding and disabled login', async () => {
    async function oidcLogin() { const c = await context(); const p = await c.newPage(); const response = await p.goto(`${base}/api/v1/auth/oidc/login`); return { c, p, response }; }
    const first = await oidcLogin(); assert.equal(first.response.status(), 200);
    const me = await api(first.c, '/auth/me'); assert.equal(me.role, 'user'); assert.equal(me.source, 'oidc'); assert.equal(me.local_password_enabled, false);
    const passwordRequests = [];
    first.p.on('request', request => {
        if (new URL(request.url()).pathname === '/api/v1/auth/password') passwordRequests.push(request.method());
    });
    await first.p.goto(`${base}/#/h/password`);
    await first.p.getByRole('alert').filter({ hasText: 'This account uses an external identity provider.' }).waitFor();
    assert.equal(await first.p.locator('#new-account-password').count(), 0);
    await first.p.locator('[aria-controls="user-menu"]').click();
    await first.p.locator('#user-menu').waitFor();
    assert.equal(await first.p.getByRole('menuitem', { name: 'Change Password', exact: true }).count(), 0);
    assert.deepEqual(passwordRequests, [], 'OIDC-only password page never submits a local password');
    const unavailable = await api(first.c, '/auth/password', 'PUT', { new_password: md5(userPassword) }, 403);
    assert.equal(unavailable.code, 'local_password_unavailable', 'server capability check remains authoritative');
    await api(first.c, '/admin/users', 'GET', undefined, 403);
    await api(admin, `/admin/users/${me.id}/password-reset`, 'POST', {}, 403);
    identity.preferred_username = 'fictional-renamed';
    const next = await oidcLogin(); const stable = await api(next.c, '/auth/me'); assert.equal(stable.id, me.id); assert.equal(stable.username, me.username);
    await api(admin, `/admin/users/${me.id}/access`, 'PUT', { web_login_enabled: false });
    const disabled = await oidcLogin(); assert.equal(disabled.response.status(), 401);
    identity = { sub: 'fictional-conflict', preferred_username: 'fictional-admin' };
    const conflict = await oidcLogin(); assert.equal(conflict.response.status(), 409); await api(conflict.c, '/auth/me', 'GET', undefined, 401);
    for (const [mode, status] of [['malformed-success', 500], ['malformed-error', 500], ['oauth-error', 500], ['authorization-error', 400], ['wrong-state', 400], ['wrong-nonce', 401]]) { tokenMode = mode; const failed = await oidcLogin(); assert.equal(failed.response.status(), status, `${mode} fails at the expected verification stage`); const responseText = await failed.response.text(); assert.ok(!responseText.includes(errorMarker)); assert.ok(!responseText.includes(errorMarkerBytes)); }
    tokenMode = undefined; assert.ok(!logs.includes(errorMarker), 'provider failure never logs response bodies');
    assert.ok(!logs.includes(errorMarkerBytes), 'provider failure never logs Rust Debug byte vectors');
    for (const endpoint of ['/.well-known/openid-configuration', '/jwks', '/authorize', '/token']) assert.ok(oidcHits.has(endpoint));
});

test('UA-14: cross-origin and missing write intent rejected; cookie and no-store', async () => {
    await api(admin, '/admin/users', 'POST', { username: 'fictional-csrf' }, 403, { Origin: 'https://fictional.invalid' });
    const response = await admin.request.post(`${base}/api/v1/admin/users`, { data: { username: 'fictional-csrf' } }); assert.equal(response.status(), 403);
    await api(admin, '/admin/users', 'OPTIONS', undefined, 403, { Origin: 'https://fictional.invalid' });
    const read = await admin.request.get(`${base}/api/v1/admin/users`); assert.match(read.headers()['cache-control'], /no-store/);
    const cookies = await admin.cookies(); assert.ok(cookies.some(c => c.httpOnly && c.sameSite === 'Lax'));
});

test('UA-01/02/03/13: real browser creates and inspects account; audit contains no credentials', async () => {
    const page = await admin.newPage(); const browserLogs = []; page.on('console', message => browserLogs.push(message.text()));
    await page.goto(`${base}/#/h/users`);
    await page.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    await page.getByRole('cell', { name: 'fictional-admin', exact: true }).waitFor();
    await page.getByRole('button', { name: 'Create user', exact: true }).click();
    const dialog = page.getByRole('dialog'); await dialog.getByLabel('Username', { exact: true }).fill('fictional-browser'); await dialog.getByRole('button', { name: 'Create user', exact: true }).click();
    await page.locator('#temporary-password').waitFor(); assert.equal(await page.locator('#temporary-password').getAttribute('type'), 'password');
    secrets.push(await page.locator('#temporary-password').inputValue());
    await page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).last().click();
    await page.getByRole('cell', { name: 'fictional-browser', exact: true }).waitFor();
    const managed = (await api(admin, '/admin/users?search=fictional-browser')).items[0];
    await page.getByRole('button', { name: 'Details: fictional-browser', exact: true }).click();
    async function operation(label) {
        await page.getByRole('button', { name: label, exact: true }).click();
        await page.getByRole('dialog').getByRole('button', { name: 'Confirm operation', exact: true }).click();
        await page.getByRole('dialog').waitFor({ state: 'hidden' });
    }
    await operation('Disable console access'); assert.equal((await api(admin, `/admin/users/${managed.id}`)).web_login_enabled, false);
    await operation('Restore console access'); assert.equal((await api(admin, `/admin/users/${managed.id}`)).web_login_enabled, true);
    await operation('Grant administrator role'); assert.equal((await api(admin, `/admin/users/${managed.id}`)).role, 'admin');
    await operation('Revoke all Web sessions');
    await page.getByRole('button', { name: 'Reset local password', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Confirm operation', exact: true }).click();
    await page.locator('#temporary-password').waitFor(); secrets.push(await page.locator('#temporary-password').inputValue());
    await page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).last().click();
    await page.getByRole('heading', { name: 'Management audit', exact: true }).waitFor();
    await page.getByRole('button', { name: 'Create user', exact: true }).click();
    await page.getByRole('dialog').getByLabel('Username', { exact: true }).fill('fictional-offline-draft');
    await admin.setOffline(true);
    await page.getByRole('dialog').getByRole('button', { name: 'Create user', exact: true }).click();
    await page.getByRole('dialog').getByRole('alert').waitFor();
    assert.equal(await page.getByRole('dialog').getByLabel('Username', { exact: true }).inputValue(), 'fictional-offline-draft');
    await admin.setOffline(false);
    assert.equal((await api(admin, '/admin/users?search=fictional-offline-draft')).total, 0);
    const audit = await api(admin, '/admin/user-events?page_size=100'); assert.ok(audit.items.some(e => e.action === 'create' && e.actor_id));
    const output = JSON.stringify(audit) + logs + browserLogs.join('\n');
    for (const secret of [...secrets, 'fictional-access-token']) assert.ok(!output.includes(secret), 'logs/audit exclude fixture credential');
    assert.ok(!output.includes('$argon2'));
});

test('UA-07/10: concurrent administrators and stale authority reject writes', async () => {
    const a = await readyUser('fictional-role-a'); const b = await readyUser('fictional-role-b');
    for (const x of [a, b]) { await api(admin, `/admin/users/${x.user.id}/role`, 'PUT', { role: 'admin' }); await login(x.c, x.user.username, userPassword); }
    const mine = await api(admin, '/auth/me');
    assert.equal((await api(admin, `/admin/users/${mine.id}/role`, 'PUT', { role: 'user' }, 403)).code, 'self_action_forbidden');
    const page = await a.c.newPage(); await page.goto(`${base}/#/h/users`); await page.getByRole('cell', { name: a.user.username, exact: true }).waitFor();
    await api(admin, `/admin/users/${a.user.id}/role`, 'PUT', { role: 'user' });
    await page.getByRole('button', { name: 'Create user', exact: true }).click();
    await page.getByRole('dialog').getByLabel('Username', { exact: true }).fill('fictional-stale-denied');
    await page.getByRole('dialog').getByRole('button', { name: 'Create user', exact: true }).click();
    await page.waitForURL(url => url.hash.startsWith('#/auth')); await page.locator('#username').waitFor();
    assert.equal((await api(admin, '/admin/users?search=fictional-stale-denied')).total, 0);
    await login(a.c, a.user.username, userPassword); await api(a.c, '/admin/users', 'GET', undefined, 403);
    await api(admin, `/admin/users/${a.user.id}/role`, 'PUT', { role: 'admin' }); await login(a.c, a.user.username, userPassword);
    // Leave exactly these two active administrators in this disposable database.
    const active = await api(admin, '/admin/users?role=admin&web_login_enabled=true');
    for (const item of active.items) if (![a.user.id, b.user.id].includes(item.id)) await api(b.c, `/admin/users/${item.id}/access`, 'PUT', { web_login_enabled: false });
    assert.equal((await api(b.c, '/admin/users?role=admin&web_login_enabled=true')).total, 2);
    const results = await Promise.all([[a, b], [b, a]].map(([actor, target]) => actor.c.request.put(`${base}/api/v1/admin/users/${target.user.id}/role`, { headers: { 'X-Requested-With': 'EasyTier' }, data: { role: 'user' } })));
    assert.ok(results.some(r => r.status() === 200)); assert.ok(results.every(r => [200, 401, 403, 409].includes(r.status())));
    let survivor;
    for (const candidate of [a, b]) { await login(candidate.c, candidate.user.username, userPassword); if ((await api(candidate.c, '/auth/me')).role === 'admin') survivor = candidate.c; }
    assert.ok(survivor, 'a valid administrator survives concurrent demotions');
    assert.equal((await api(survivor, '/admin/users?role=admin&web_login_enabled=true')).total, 1);
    await api(survivor, `/admin/users/${mine.id}/access`, 'PUT', { web_login_enabled: true }); await login(admin, 'fictional-admin', adminPassword);
});

test('UA-01/03: ordinary browser login has no administration menu; closed registration', async () => {
    const { user } = await readyUser('fictional-browser-user');
    const c = await context(), page = await c.newPage();
    await page.goto(base);
    await page.locator('#username').fill(user.username);
    await page.getByLabel('Password', { exact: true }).fill(userPassword);
    await page.getByRole('button', { name: 'Login', exact: true }).click();
    await page.locator('.web-console').waitFor();
    assert.equal(await page.getByRole('link', { name: 'User management', exact: true }).count(), 0);
    await page.goto(`${base}/#/h/users`);
    await page.getByText('You do not have permission for this operation.', { exact: true }).waitFor();
    await api(c, '/admin/users', 'GET', undefined, 403);
    const anonymous = await context(), loginPage = await anonymous.newPage(); await loginPage.goto(base);
    await loginPage.locator('#username').waitFor();
    assert.equal(await loginPage.getByRole('button', { name: 'Register', exact: true }).count(), 0);
});

test('UA-05: temporary password browser is restricted until change and fresh login', async () => {
    const created = await create('fictional-browser-temporary'); const c = await context(), page = await c.newPage();
    await page.goto(base); await page.getByLabel('Username', { exact: true }).fill(created.user.username);
    await page.getByLabel('Password', { exact: true }).fill(created.temporary_password);
    await page.getByRole('button', { name: 'Login', exact: true }).click();
    await page.getByText('Change your temporary password before accessing the workspace.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('link', { name: 'User management', exact: true }).count(), 0);
    await page.getByLabel('Password', { exact: true }).fill(userPassword);
    await page.getByRole('button', { name: 'Change Password', exact: true }).click();
    await page.getByText('Password changed. Sign in again with your new password.').waitFor();
    await api(c, '/auth/me', 'GET', undefined, 401);
    await login(c, created.user.username, userPassword); assert.equal((await api(c, '/auth/me')).must_change_password, false);
});
