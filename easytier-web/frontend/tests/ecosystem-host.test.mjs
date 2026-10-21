import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { createServer } from 'node:net';
import { preview } from 'vite';
import { chromium } from 'playwright';
let server, browser, base;
before(async () => {
    const reservation = createServer();
    await new Promise(resolve => reservation.listen(0, '127.0.0.1', resolve));
    const port = reservation.address().port;
    await new Promise(resolve => reservation.close(resolve));
    server = await preview({ preview: { host: '127.0.0.1', port, strictPort: true } });
    base = `http://127.0.0.1:${port}`;
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await server?.close(); });
async function open(t, { required = false, webhook = false, language = 'en', width = 1440 } = {}) {
    const context = await browser.newContext({ viewport: { width, height: 900 } });
    t.after(() => context.close());
    await context.addInitScript(language => localStorage.setItem('lang', language), language);
    const requests = [], errors = [];
    await context.route('**/api_meta.js', route => route.fulfill({ body: '', contentType: 'application/javascript' }));
    await context.route('**/api/v1/**', route => {
        const path = new URL(route.request().url()).pathname.replace('/api/v1', '');
        requests.push({ path, method: route.request().method() });
        let json = {};
        if (path === '/auth/me') json = { id: 42, username: 'fixture-user', role: 'user', capabilities: [], source: required ? 'local' : 'oidc', local_password_enabled: required, must_change_password: required, web_login_enabled: true };
        if (path === '/console-info') json = { username: 'fixture-user', webhook_auth: webhook, config_server_protocol: 'tcp', config_server_port: 11010 };
        if (path === '/machines') json = { machines: [] };
        if (path === '/networks') json = { networks: [] };
        if (path === '/blocked-devices') json = { blocked: [] };
        return route.fulfill({ json });
    });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(`${base}/#/h/ecosystem`);
    return { page, requests, errors };
}
test('ordinary deep link renders ecosystem, active navigation and existing account controls', async t => {
    const { page, requests, errors } = await open(t);
    await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
    const link = page.getByRole('link', { name: 'Networks and resources', exact: true });
    assert.equal(await link.getAttribute('aria-current'), 'page');
    assert.equal(await page.locator('.console-page-title').getAttribute('title'), 'Networks and resources');
    assert.equal(await page.getByRole('link', { name: 'User management', exact: true }).count(), 0);
    assert.equal(requests.filter(item => item.path === '/machines').length, 1);
    await page.getByRole('button', { name: 'Account', exact: true }).click();
    assert.equal(await page.getByText('Change Password', { exact: true }).count(), 0);
    assert.equal(requests.every(item => item.method === 'GET'), true);
    assert.deepEqual(errors, []);
});
test('webhook mobile navigation keeps ecosystem available and restores content focus', async t => {
    const { page, requests, errors } = await open(t, { webhook: true, width: 360 });
    await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
    assert.equal(requests.some(item => item.path === '/networks'), false);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    const toggle = page.getByRole('button', { name: 'Navigation', exact: true });
    await toggle.click();
    await page.getByRole('link', { name: 'Dashboard', exact: true }).last().click();
    await page.waitForFunction(() => document.activeElement?.id === 'main-content');
    await toggle.click();
    await page.getByRole('link', { name: 'Networks and resources', exact: true }).last().click();
    await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
    await page.waitForFunction(() => document.activeElement?.id === 'main-content');
    assert.deepEqual(errors, []);
});
test('forced-password deep link cannot mount ecosystem or expose its navigation', async t => {
    const { page, requests } = await open(t, { required: true });
    await page.getByText('Change your temporary password before accessing the workspace.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('link', { name: 'Networks and resources', exact: true }).count(), 0);
    assert.equal(requests.some(item => ['/machines', '/console-info', '/networks'].includes(item.path)), false);
});
test('Chinese production ecosystem labels and unsupported notices are bundled', async t => {
    const { page, errors } = await open(t, { language: 'cn' });
    await page.locator('.ecosystem-access').waitFor();
    const text = await page.locator('main').innerText();
    assert.doesNotMatch(text, /web\.ecosystem|api_missing/);
    assert.match(text, /暂不支持/);
    assert.match(await page.locator('.console-page-title').innerText(), /网络/);
    assert.deepEqual(errors, []);
});
test('ecosystem retains config-server command copy and theme switching', async t => {
    const { page } = await open(t);
    await page.locator('.ecosystem-access').waitFor();
    assert.doesNotMatch(await page.locator('main').innerText(), /web\.ecosystem|api_missing/);
    await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async value => { window.fixtureCopied = value; } } }));
    await page.getByRole('button', { name: 'Device Enrollment', exact: true }).click();
    const command = 'easytier-core --config-server tcp://127.0.0.1:11010/fixture-user';
    await page.getByText(command, { exact: true }).waitFor();
    await page.getByRole('button', { name: 'Copy enrollment command', exact: true }).click();
    assert.equal(await page.evaluate(() => window.fixtureCopied), command);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Theme (light / dark / system)', exact: true }).click();
    assert.equal(await page.evaluate(() => localStorage.getItem('console-theme-mode')), 'light');
});
