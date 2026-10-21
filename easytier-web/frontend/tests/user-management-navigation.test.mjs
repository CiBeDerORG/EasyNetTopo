import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer, preview } from 'vite';
import { chromium } from 'playwright';
let server, browser, base;
// Default to built assets: development mode can hide production i18n failures.
const usePreview = process.env.WEB_TEST_DEV !== '1';
before(async () => {
    server = await (usePreview ? preview : createServer)({ root: fileURLToPath(new URL('../', import.meta.url)), server: { host: '127.0.0.1', port: 0 }, preview: { host: '127.0.0.1', port: 0 } });
    if (!usePreview) await server.listen(); base = `http://127.0.0.1:${server.httpServer.address().port}`;
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); if (server?.close) await server.close(); else server?.httpServer.close(); });
const summary = { id: 2, username: 'fixture-user', role: 'user', web_login_enabled: true, source: 'unknown', local_password_enabled: true, must_change_password: false, created_at: null, last_login_at: null };
async function open(t, path = '/h/users', setup = () => {}) {
    const state = { language: 'en', me: { ...summary, id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'], default_password_warning: false }, policy: { registration_enabled: true, oidc_enabled: false }, users: [summary], requests: [], fail: null, listDelay: 0, meStatus: 200 };
    setup(state);
    const context = await browser.newContext(); t.after(() => context.close());
    await context.addInitScript(language => localStorage.setItem('lang', language), state.language);
    await context.route('**/api_meta.js', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
    await context.route('**/api/v1/**', async route => {
        const req = route.request(), url = new URL(req.url()); const path = url.pathname.replace('/api/v1', '');
        state.requests.push({ path, method: req.method(), body: req.postDataJSON() });
        const json = (body, status = 200) => route.fulfill({ status, json: body });
        if (state.fail && path === state.fail.path) {
            if (state.fail.delay) await new Promise(resolve => setTimeout(resolve, state.fail.delay));
            if (state.fail.network) return route.abort();
            return json({ code: state.fail.code, message: 'untrusted' }, state.fail.status);
        }
        if (path === '/auth/me') return json(state.me, state.meStatus);
        if (path === '/auth/policy') return json(state.policy);
        if (path === '/auth/login' || path === '/auth/password' || path === '/auth/logout') return json({});
        if (path === '/console-info') return json({ username: state.me.username, webhook_auth: true, config_server_protocol: 'tcp', config_server_port: 11010 });
        if (path === '/admin/users' && req.method() === 'GET') {
            const items = structuredClone(state.users), delay = state.listDelay;
            if (delay) await new Promise(r => setTimeout(r, delay));
            return json({ items, total: items.length, page: 1, page_size: 20 });
        }
        if (path === '/admin/users' && req.method() === 'POST') return json({ user: { ...summary, username: req.postDataJSON().username }, temporary_password: 'fixture-only-temporary-password', expires_at: '2026-10-22T00:00:00Z' }, 201);
        if (path === '/admin/users/2') return json(summary);
        if (path === '/admin/user-events') return json({ items: [], total: 0, page: 1, page_size: 20 });
        if (path === '/summary') return json({ device_count: 0 });
        if (path === '/sessions') return json([]);
        return json({});
    });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    await page.goto(`${base}/#${path}`);
    return { page, state };
}
test('administrator navigation loads real client and components; credential clears on navigation', async t => {
    const { page: p, state } = await open(t);
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    await p.getByRole('cell', { name: 'fixture-user', exact: true }).waitFor();
    await p.getByRole('button', { name: 'Create user', exact: true }).click();
    const dialog = p.getByRole('dialog'); await dialog.getByLabel('Username', { exact: true }).fill('fixture-new');
    assert.match(await dialog.innerText(), /dots, @ or hyphens/);
    await dialog.getByRole('button', { name: 'Create user', exact: true }).click();
    await p.locator('input#temporary-password').waitFor();
    assert.equal(await p.locator('input#temporary-password').getAttribute('type'), 'password');
    assert.equal(state.requests.filter(x => x.path === '/admin/users' && x.method === 'POST').length, 1);
    await p.evaluate(() => location.hash = '#/h/password');
    await p.getByLabel('Password', { exact: true }).waitFor();
    assert.equal(await p.locator('input#temporary-password').count(), 0);
    await p.evaluate(() => location.hash = '#/h/users');
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    assert.equal(await p.locator('input#temporary-password').count(), 0);
});
test('ordinary direct navigation is forbidden and never fetches management data', async t => {
    const { page: p, state } = await open(t, '/h/users', s => { s.me.role = 'user'; s.me.capabilities = []; });
    await p.getByText('You do not have permission for this operation.', { exact: true }).waitFor();
    assert.equal(state.requests.some(x => x.path.startsWith('/admin/')), false);
    assert.equal(await p.getByRole('link', { name: 'User management', exact: true }).count(), 0);
});
test('registration policy disables direct registration without rendering a usable form', async t => {
    const { page: p, state } = await open(t, '/auth/register', s => s.policy.registration_enabled = false);
    await p.getByText('Public registration is disabled. Contact your administrator for an account.').waitFor();
    assert.equal(await p.locator('#register-username').count(), 0);
    assert.equal(state.requests.some(x => x.path === '/auth/register'), false);
});
test('restricted login allows password change only and signs out afterwards', async t => {
    const { page: p, state } = await open(t, '/h/users', s => s.me.must_change_password = true);
    await p.getByText('Change your temporary password before accessing the workspace.', { exact: true }).waitFor();
    assert.equal(state.requests.some(x => x.path === '/console-info' || x.path.startsWith('/admin/')), false);
    await p.getByLabel('Password', { exact: true }).fill('Fixture-New-Pass-123');
    await p.getByRole('button', { name: 'Change Password', exact: true }).click();
    await p.getByText('Password changed. Sign in again with your new password.').waitFor();
    assert.match(state.requests.find(x => x.path === '/auth/password').body.new_password, /^[0-9a-f]{32}$/);
});
test('legacy default-password notice remains nonblocking', async t => {
    const { page: p } = await open(t, '/h/users', s => s.me.default_password_warning = true);
    await p.getByText('This account still uses a publicly known default password. Change it immediately.').waitFor();
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    assert.equal(await p.getByRole('button', { name: 'Create user', exact: true }).isEnabled(), true);
});
test('403 refreshes capabilities; network failure preserves a create draft', async t => {
    const { page: p, state } = await open(t);
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    await p.getByRole('button', { name: 'Create user', exact: true }).click();
    const dialog = p.getByRole('dialog'); await dialog.getByLabel('Username', { exact: true }).fill('retained-draft');
    state.fail = { path: '/admin/users', network: true };
    await dialog.getByRole('button', { name: 'Create user', exact: true }).click();
    await dialog.getByText('The server could not be reached. Check your connection and retry.').waitFor();
    assert.equal(await dialog.getByLabel('Username', { exact: true }).inputValue(), 'retained-draft');
    state.fail = { path: '/admin/users', status: 403, code: 'forbidden' }; state.me.capabilities = [];
    await dialog.getByRole('button', { name: 'Create user', exact: true }).click();
    await dialog.waitFor({ state: 'hidden' });
    await p.getByText('You do not have permission for this operation.', { exact: true }).waitFor();
    assert.equal(await p.getByRole('button', { name: 'Create user', exact: true }).count(), 0);
});
test('401 exits and a delayed management response cannot restore data after navigation', async t => {
    const { page: p, state } = await open(t);
    await p.getByRole('cell', { name: 'fixture-user', exact: true }).waitFor();
    state.listDelay = 300;
    await p.getByRole('button', { name: 'Search username', exact: true }).click();
    await p.evaluate(() => location.hash = '#/h/password');
    await p.getByLabel('Password', { exact: true }).waitFor();
    await p.waitForTimeout(400);
    assert.equal(await p.getByRole('table').count(), 0);
    state.listDelay = 0; state.fail = { path: '/admin/users', status: 401, code: 'unauthenticated' };
    await p.evaluate(() => location.hash = '#/h/users');
    await p.getByText('Your session expired. Sign in again.', { exact: true }).waitFor();
    assert.match(p.url(), /auth/);
});
test('login distinguishes invalid credentials from network failure while retaining username', async t => {
    const { page: p, state } = await open(t, '/auth');
    await p.getByLabel('Username', { exact: true }).fill('login-fixture');
    await p.getByLabel('Password', { exact: true }).fill('fixture-only-password');
    state.fail = { path: '/auth/login', network: true };
    await p.getByRole('button', { name: 'Login', exact: true }).click();
    await p.getByText('The server could not be reached. Check your connection and retry.').waitFor();
    assert.equal(await p.getByLabel('Username', { exact: true }).inputValue(), 'login-fixture');
    state.fail = { path: '/auth/login', status: 401, code: 'invalid_credentials' };
    await p.getByRole('button', { name: 'Login', exact: true }).click();
    await p.getByText('Invalid username or password. Please try again.').waitFor();
});
test('login sends a temporary account directly to password change without mounting dashboard', async t => {
    const { page: p, state } = await open(t, '/auth', s => s.me.must_change_password = true);
    await p.getByLabel('Username', { exact: true }).fill('temporary-fixture');
    await p.getByLabel('Password', { exact: true }).fill('fixture-only-password');
    await p.getByRole('button', { name: 'Login', exact: true }).click();
    await p.getByText('Change your temporary password before accessing the workspace.', { exact: true }).waitFor();
    assert.equal(state.requests.some(x => x.path === '/console-info' || x.path === '/summary'), false);
});

test('production Chinese create dialog renders the literal username character guidance', async t => {
    const { page: p } = await open(t, '/h/users', state => state.language = 'cn');
    await p.getByRole('button', { name: '创建用户', exact: true }).click();
    const dialog = p.getByRole('dialog');
    await dialog.getByLabel('用户名', { exact: true }).waitFor();
    assert.match(await dialog.innerText(), /点、@ 或连字符/);
});

for (const language of ['en', 'cn']) {
    test(`OIDC-only password route explains external management without a form or menu action (${language})`, async t => {
        const { page: p, state } = await open(t, '/h/password', s => {
            s.language = language; s.me.source = 'oidc'; s.me.local_password_enabled = false;
        });
        await p.setViewportSize({ width: 375, height: 812 });
        const message = language === 'cn'
            ? '此账号使用外部身份认证平台，请前往该平台修改密码；当前无法在此控制台修改本地密码。'
            : 'This account uses an external identity provider. Change your password there; a local password cannot be changed in this console.';
        await p.getByRole('alert').filter({ hasText: message }).waitFor();
        assert.equal(await p.locator('#new-account-password').count(), 0);
        const account = p.locator('[aria-controls="user-menu"]');
        await account.focus(); await p.keyboard.press('Enter');
        await p.locator('#user-menu').waitFor();
        assert.equal(await p.getByRole('menuitem', { name: language === 'cn' ? '修改密码' : 'Change Password', exact: true }).count(), 0);
        assert.equal(await p.getByRole('menuitem').count(), 1, 'logout remains available');
        assert.equal(state.requests.some(x => x.path === '/auth/password'), false);
        assert.equal(await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    });
}

test('unknown local-password capability shows guidance without assuming an identity provider', async t => {
    const { page: p, state } = await open(t, '/h/password', s => s.me.local_password_enabled = null);
    await p.getByText('Local password changes are unavailable for this account. Contact your administrator to confirm your sign-in method.', { exact: true }).waitFor();
    assert.equal(await p.locator('#new-account-password').count(), 0);
    assert.equal(state.requests.some(x => x.path === '/auth/password'), false);
});

test('OIDC source with local credentials keeps the menu and can change its local password', async t => {
    const { page: p, state } = await open(t, '/h/users', s => s.me.source = 'oidc');
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    await p.locator('[aria-controls="user-menu"]').click();
    await p.getByRole('menuitem', { name: 'Change Password', exact: true }).click();
    await p.getByLabel('Password', { exact: true }).fill('Fixture-New-Local-Password-123');
    await p.getByRole('button', { name: 'Change Password', exact: true }).click();
    await p.getByText('Password changed. Sign in again with your new password.').waitFor();
    assert.equal(state.requests.filter(x => x.path === '/auth/password').length, 1);
});

test('server local-password rejection explains the capability and stops further submissions', async t => {
    const { page: p, state } = await open(t, '/h/password', s => {
        s.me.source = 'oidc';
        s.fail = { path: '/auth/password', status: 403, code: 'local_password_unavailable' };
    });
    await p.getByLabel('Password', { exact: true }).fill('Fixture-Rejected-Password-123');
    await p.getByRole('button', { name: 'Change Password', exact: true }).click();
    await p.getByRole('alert').filter({ hasText: 'This account uses an external identity provider.' }).waitFor();
    assert.equal(await p.locator('#new-account-password').count(), 0);
    assert.equal(await p.getByText('You do not have permission for this operation.', { exact: true }).count(), 0);
    assert.equal(state.requests.filter(x => x.path === '/auth/password').length, 1);
});

test('mobile navigation dismissal restores toggle repeatedly; navigation closes onto content', async t => {
    const { page: p } = await open(t);
    await p.setViewportSize({ width: 360, height: 900 });
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    const toggle = p.locator('.mobile-nav-toggle');
    const drawer = p.locator('.console-mobile-nav');
    for (let attempt = 0; attempt < 3; attempt++) {
        await toggle.click(); await drawer.waitFor(); await p.keyboard.press('Escape');
        await drawer.waitFor({ state: 'hidden' });
        await p.waitForFunction(() => document.activeElement?.classList.contains('mobile-nav-toggle'));
    }
    await toggle.click(); await drawer.waitFor();
    await drawer.getByRole('link', { name: 'Dashboard', exact: true }).click();
    await p.waitForURL('**/#/h'); await drawer.waitFor({ state: 'hidden' });
    await p.waitForFunction(() => document.activeElement?.id === 'main-content');
    await toggle.click(); await drawer.waitFor();
    await p.evaluate(() => { location.hash = '#/h/users'; });
    await drawer.waitFor({ state: 'hidden' });
    await p.waitForFunction(() => document.activeElement?.id === 'main-content');
});

test('forced-password identity arriving while mobile navigation is open focuses available content', async t => {
    const { page: p, state } = await open(t);
    await p.setViewportSize({ width: 360, height: 900 });
    await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
    state.fail = { path: '/admin/users', status: 403, code: 'forbidden', delay: 500 };
    state.me.must_change_password = true;
    await p.locator('form.filters').dispatchEvent('submit');
    await p.locator('.mobile-nav-toggle').click();
    await p.locator('.console-mobile-nav').waitFor();
    await p.waitForURL('**/#/h/password');
    await p.locator('.console-mobile-nav').waitFor({ state: 'hidden' });
    assert.equal(await p.locator('.mobile-nav-toggle').count(), 0);
    await p.waitForFunction(() => document.activeElement?.id === 'main-content');
});
