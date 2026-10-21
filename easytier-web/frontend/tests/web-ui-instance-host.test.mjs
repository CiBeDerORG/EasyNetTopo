// Real host/RemoteManagement/HTTP adapter with fictional responses; no configuration reaches a device.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';

let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });
const id = value => `00000000-0000-0000-0000-${value.toString(16).padStart(12, '0')}`;
const uuid = value => ({ part1: 0, part2: 0, part3: 0, part4: value });
const routeFor = (device, instance = 100) => `/h/deviceList/device/${id(device)}/${id(instance)}`;

async function openHost(t, device = 1) {
    const state = { requests: [], writes: [] };
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width: 1440, height: 1000 } });
    t.after(() => context.close());
    await context.addInitScript(() => localStorage.setItem('lang', 'en'));
    await context.route('**/api_meta.js', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
    await context.route('**/api/v1/**', route => {
        const request = route.request();
        const path = new URL(request.url()).pathname.replace('/api/v1', '');
        state.requests.push(path);
        const machineMatch = path.match(/^\/machines\/([^/]+)\/networks(?:\/(.*))?$/);
        if (machineMatch) {
            const [, machineId, suffix] = machineMatch;
            const host = machineId === id(1) ? 'A' : 'B';
            if (!suffix) return route.fulfill({ json: { running_inst_ids: [uuid(100), uuid(101)], disabled_inst_ids: [] } });
            if (suffix === 'metas') return route.fulfill({ json: { metas: Object.fromEntries([100, 101].map(instance => [id(instance), {
                network_name: `Fixture ${host} network ${instance}`, config_permission: 7,
            }])) } });
            if (suffix.startsWith('info/')) return route.fulfill({ json: { info: { map: {
                [suffix.slice(5)]: { running: false, error_msg: `Fixture ${host} runtime not available` },
            } } } });
            if (suffix.startsWith('config/') && request.method() === 'PUT') {
                state.writes.push({ path, config: request.postDataJSON().config });
                return route.fulfill({ json: {} });
            }
        }
        const responses = {
            '/auth/me': { id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'],
                source: 'local', local_password_enabled: true, must_change_password: false, web_login_enabled: true },
            '/console-info': { username: 'fixture-admin', webhook_auth: true, config_server_protocol: 'tcp', config_server_port: 11010 },
            '/machines': { machines: [1, 2].map(value => ({ online: true, client_url: `tcp://192.0.2.${value}:11010`,
                info: { hostname: `Fixture host ${value === 1 ? 'A' : 'B'}`, machine_id: uuid(value), easytier_version: 'fixture',
                    running_network_instances: [uuid(100), uuid(101)] }, networks: [] })) },
        };
        return route.fulfill({ status: path in responses ? 200 : 404, json: responses[path] ?? { code: 'not_found' } });
    });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    await page.goto(`/#${routeFor(device)}`);
    await page.locator('.console-device-drawer').waitFor();
    await page.locator('.console-device-drawer .p-select-label').filter({ hasText: `Fixture ${device === 1 ? 'A' : 'B'} network 100` }).waitFor();
    return { page, state, errors };
}

test('deep-link instance selection updates the same host route and survives reload', async t => {
    const { page, state, errors } = await openHost(t);
    const select = page.locator('.console-device-drawer #dd-inst-id');
    await select.click();
    await page.getByRole('option').filter({ hasText: 'Fixture A network 101' }).click();
    await page.waitForURL(`**/#${routeFor(1, 101)}`);
    await page.reload();
    await page.locator('.console-device-drawer .p-select-label').filter({ hasText: 'Fixture A network 101' }).waitFor();
    assert.ok(state.requests.includes(`/machines/${id(1)}/networks/info/${id(101)}`));
    await page.goBack();
    await page.waitForURL(`**/#${routeFor(1)}`);
    assert.deepEqual(errors, []);
});

test('switching hosts with the same instance UUID cannot retain the previous host metadata', async t => {
    const { page, state, errors } = await openHost(t);
    await page.evaluate(path => { location.hash = `#${path}`; }, routeFor(2));
    await page.locator('.console-device-drawer .drawer-heading h2').filter({ hasText: 'Fixture host B' }).waitFor();
    await page.locator('.console-device-drawer .p-select-label').filter({ hasText: 'Fixture B network 100' }).waitFor();
    assert.ok(state.requests.includes(`/machines/${id(2)}/networks/metas`));
    await page.getByText('Fixture B runtime not available', { exact: true }).waitFor();
    assert.equal(await page.getByText('Fixture A runtime not available', { exact: true }).count(), 0);
    assert.deepEqual(errors, []);
});

test('new configuration uses the selected host client and hostname without running a VPN', async t => {
    const { page, state, errors } = await openHost(t, 2);
    await page.locator('.console-device-drawer .create-button').click();
    await page.waitForURL(url => url.hash.startsWith(`#/h/deviceList/device/${id(2)}/`) && !url.hash.endsWith(id(100)));
    assert.equal(state.writes.length, 1);
    assert.ok(state.writes[0].path.startsWith(`/machines/${id(2)}/networks/config/`));
    assert.equal(state.writes[0].config.hostname, 'Fixture host B');
    assert.equal(state.requests.some(path => path === `/machines/${id(1)}/networks`), false);
    assert.deepEqual(errors, []);
});
