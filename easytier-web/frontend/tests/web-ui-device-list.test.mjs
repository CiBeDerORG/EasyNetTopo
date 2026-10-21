// Actual production device pages with fictional HTTP data; no device or VPN is connected.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';

let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });
const uuid = value => ({ part1: 0, part2: 0, part3: 0, part4: value });
const machineId = '00000000-0000-0000-0000-000000000001';
const hostname = `fixture-long-host-${'name-'.repeat(14)}`;

async function openDevices(t, options = {}) {
    const state = { failList: false, failAlias: false, holdAlias: null, writes: [],
        machines: options.empty ? [] : [
            { online: true, client_url: 'tcp://192.0.2.10:11010', alias: 'Alpha fixture',
                info: { hostname, machine_id: uuid(1), easytier_version: '2.4.5-fixture', running_network_instances: [] },
                location: { country: 'Fictional country', region: '', city: 'Fictional city' }, networks: [] },
            { online: false, client_url: 'tcp://192.0.2.11:11010',
                info: { hostname: 'Beta fixture', machine_id: uuid(2), easytier_version: '2.4.5', running_network_instances: [] }, networks: [] },
        ] };
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width: options.width ?? 1440, height: 900 } });
    t.after(() => context.close());
    await context.addInitScript(options => {
        localStorage.setItem('lang', options.language ?? 'en');
        localStorage.setItem('console-theme-mode', options.theme ?? 'light');
        localStorage.setItem('deviceList.viewMode', options.viewMode ?? 'table');
        Object.defineProperty(navigator, 'clipboard', { value: { writeText: async value => { window.copiedEnrollment = value; } } });
    }, options);
    await context.route('**/api_meta.js', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
    await context.route('**/api/v1/**', async route => {
        const request = route.request();
        const path = new URL(request.url()).pathname.replace('/api/v1', '');
        if (path.endsWith('/alias') && request.method() === 'PUT') {
            const input = request.postDataJSON(); state.writes.push({ path, input });
            if (state.holdAlias) await state.holdAlias;
            if (state.failAlias) return route.fulfill({ status: 503, json: { code: 'internal_error' } });
            state.machines[0].alias = input.alias;
            return route.fulfill({ json: {} });
        }
        if (path === '/machines' && state.failList) return route.fulfill({ status: 503, json: { code: 'internal_error' } });
        const responses = {
            '/auth/me': { id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'],
                source: 'local', local_password_enabled: true, must_change_password: false, web_login_enabled: true },
            '/console-info': { username: 'fixture-admin', webhook_auth: false, config_server_protocol: 'tcp', config_server_port: 11010 },
            '/machines': { machines: state.machines }, '/networks': { networks: [] }, '/blocked-devices': { blocked: [] },
            [`/machines/${machineId}/networks`]: { running_inst_ids: [], disabled_inst_ids: [] },
            [`/machines/${machineId}/networks/metas`]: { metas: {} },
        };
        return route.fulfill({ status: path in responses ? 200 : 404, json: responses[path] ?? { code: 'not_found' } });
    });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    await page.goto('/#/h/deviceList');
    await page.locator('.console-page .table-count').waitFor();
    return { page, state, errors };
}

test('filtering, expanded details and selection retain the stable device route', async t => {
    const { page, errors } = await openDevices(t);
    const search = page.getByRole('textbox', { name: 'Search name or address', exact: true });
    await search.fill('192.0.2.10');
    assert.equal(await page.locator('.desktop-list tbody tr').count(), 1);
    await page.locator('.p-datatable-row-toggle-button').first().click();
    await page.locator('.desktop-list .device-details').waitFor();
    assert.equal(await page.locator('.desktop-list .device-details .hostname .detail-value').textContent(), hostname);
    await page.locator('.desktop-list .entity-link').first().click();
    await page.locator('.console-device-drawer').waitFor();
    assert.match(page.url(), new RegExp(`/device/${machineId}$`));
    assert.match(await page.locator('.console-device-drawer .drawer-heading').innerText(), /Alpha fixture/);
    assert.deepEqual(errors, []);
});

test('a saved card preference uses only the compact list on a narrow screen', async t => {
    const { page, errors } = await openDevices(t, { width: 360, language: 'cn', theme: 'dark', viewMode: 'card' });
    await page.locator('.mobile-list-item').first().waitFor();
    assert.equal(await page.locator('.device-card-grid').isVisible(), false, 'desktop card grid must not duplicate mobile rows');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await page.locator('.mobile-list-item details > summary').first().click();
    assert.equal(await page.locator('.mobile-list-item .device-details .hostname .detail-value').first().textContent(), hostname);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    assert.deepEqual(errors, []);
});

test('alias save preserves a failed draft and suppresses duplicate Enter submissions', async t => {
    const { page, state } = await openDevices(t);
    await page.locator('.desktop-list').getByRole('button', { name: 'Set Alias', exact: true }).first().click();
    const dialog = page.getByRole('dialog'); const input = dialog.locator('#device-alias-input');
    await input.fill('Retained fixture alias');
    let release; state.holdAlias = new Promise(resolve => { release = resolve; }); t.after(() => release());
    state.failAlias = true;
    const requested = page.waitForRequest(request => request.url().endsWith('/alias'));
    await input.press('Enter'); await requested;
    await input.dispatchEvent('keydown', { key: 'Enter', code: 'Enter' });
    release(); state.holdAlias = null;
    await page.getByText('Unable to refresh data.', { exact: false }).first().waitFor();
    assert.equal(state.writes.length, 1);
    assert.equal(await input.inputValue(), 'Retained fixture alias');
    state.failAlias = false;
    await dialog.getByRole('button', { name: 'Confirm', exact: true }).click();
    await dialog.waitFor({ state: 'hidden' });
    await page.locator('.desktop-list .entity-link').filter({ hasText: 'Retained fixture alias' }).waitFor();
    assert.equal(state.writes.at(-1).path, `/machines/${machineId}/alias`);
});

test('list failure is distinct from empty and enrollment copy keeps its exact command', async t => {
    const { page, state } = await openDevices(t, { empty: true, width: 768 });
    const code = page.locator('.console-empty-state code'); await code.waitFor();
    const command = 'easytier-core --config-server tcp://127.0.0.1:11010/fixture-admin';
    assert.equal(await code.textContent(), command);
    await page.getByRole('button', { name: 'Copy enrollment command', exact: true }).click();
    assert.equal(await page.evaluate(() => window.copiedEnrollment), command);
    state.failList = true;
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await page.getByRole('heading', { name: 'Unable to refresh data.', exact: false }).waitFor();
    assert.equal(await page.getByRole('heading', { name: 'No devices yet', exact: true }).count(), 0);
    state.failList = false;
    await page.getByRole('button', { name: 'Retry', exact: true }).click();
    await page.getByRole('heading', { name: 'No devices yet', exact: true }).waitFor();
});
