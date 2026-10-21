// Actual production NetworkList with fictional HTTP responses; no real network writes.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';
let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });
async function openList(t, gateway = true, language = 'en') {
    const state = { writes: [], fail: true, release: null, hold: null };
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width: 360, height: 900 } });
    t.after(async () => { state.release?.(); await context.close(); });
    await context.addInitScript(language => { localStorage.setItem('lang', language); localStorage.setItem('console-theme-mode', language === 'cn' ? 'dark' : 'light'); }, language);
    await context.route('**/api_meta.js', route => route.fulfill({ body: '' }));
    await context.route('**/api/v1/**', async route => {
        const path = new URL(route.request().url()).pathname.replace('/api/v1', '');
        if (path === '/networks' && route.request().method() === 'POST') {
            state.writes.push(route.request().postDataJSON()); if (state.hold) await state.hold;
            return route.fulfill({ status: state.fail ? 503 : 200, json: state.fail ? { message: 'Fixture create failed' } : { network_id: 'fixture-created' } });
        }
        const data = {
            '/auth/me': { id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'], source: 'local', local_password_enabled: true, web_login_enabled: true, must_change_password: false },
            '/console-info': { username: 'fixture-admin', webhook_auth: false },
            '/networks': { networks: [] }, '/machines': { machines: [] },
            '/networks/gateway-info': { enabled: gateway, relay_data: false, peer_url: 'tcp://192.0.2.1:11010' },
        };
        return route.fulfill({ status: path in data ? 200 : 404, json: data[path] ?? { code: 'not_found' } });
    });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    await page.goto('/#/h/networks'); await page.locator('.page-heading button').click();
    const dialog = page.getByRole('dialog'); await dialog.waitFor();
    await dialog.locator('#network-display-name').fill('Fictional network long draft');
    await dialog.locator('#network-cidr').fill('10.123.4.0');
    return { page, dialog, state };
}
for (const mode of ['Gateway', 'Manual', 'Standalone']) {
    test(`${mode} payload, failure draft, explicit retry and returned networkId route`, async t => {
        const { page, dialog, state } = await openList(t, mode !== 'Standalone');
        if (mode === 'Manual') await dialog.getByRole('button', { name: 'Advanced: specify initial nodes manually' }).click();
        await dialog.getByRole('button', { name: /^(Confirm|确认)$/ }).click();
        await page.getByText('Fixture create failed', { exact: true }).waitFor();
        assert.equal(await dialog.locator('#network-display-name').inputValue(), 'Fictional network long draft');
        const settings = state.writes[0].settings;
        assert.equal(settings.networking_method, mode);
        assert.equal(settings.virtual_cidr, '10.123.4.0/24');
        assert.equal(settings.secure_mode, mode === 'Gateway');
        assert.deepEqual(settings.peer_urls, mode === 'Manual' ? ['tcp://192.0.2.1:11010'] : []);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
        state.fail = false; await dialog.getByRole('button', { name: /^(Confirm|确认)$/ }).click();
        await page.waitForURL('**/#/h/networks/fixture-created'); assert.equal(state.writes.length, 2);
    });
}
test('pending creation blocks cancel and duplicate submission, failure restores editing', async t => {
    const { page, dialog, state } = await openList(t, true, 'cn');
    state.hold = new Promise(resolve => { state.release = resolve; });
    const requested = page.waitForRequest(req => req.method() === 'POST' && req.url().endsWith('/networks'));
    await dialog.getByRole('button', { name: /^(Confirm|确认)$/ }).click(); await requested;
    assert.equal(await dialog.getByRole('button', { name: /^(Cancel|取消)$/ }).isDisabled(), true);
    await page.keyboard.press('Escape'); assert.equal(await dialog.isVisible(), true);
    assert.equal(await dialog.locator('#network-display-name').isDisabled(), true);
    await dialog.getByRole('button', { name: /^(Confirm|确认)$/ }).dispatchEvent('click');
    state.release(); state.hold = null;
    await page.getByText('Fixture create failed', { exact: true }).first().waitFor();
    assert.equal(state.writes.length, 1); assert.equal(await dialog.locator('#network-display-name').isEnabled(), true);
    await dialog.getByRole('button', { name: /^(Cancel|取消)$/ }).click(); await dialog.waitFor({ state: 'hidden' });
});
