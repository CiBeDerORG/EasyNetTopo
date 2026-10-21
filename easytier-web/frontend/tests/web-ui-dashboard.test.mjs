// Production Dashboard + HTTP fixtures. Counts describe fixture data, not live VPN devices.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';

let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });

const uuid = value => ({ part1: 0, part2: 0, part3: 0, part4: value });
const machine = (value, online) => ({
    client_url: `tcp://192.0.2.${value}:11010`, online,
    info: { hostname: `fixture-host-${value}`, machine_id: uuid(value), easytier_version: 'fixture',
        running_network_instances: online ? [uuid(100)] : [] },
    networks: [],
});

async function openDashboard(t, configure = () => {}) {
    const state = { external: false, failures: new Set(), requests: [], holdSummary: null,
        machines: [machine(1, true), machine(2, false)],
        networks: [{ network_id: 'fixture-network', display_name: 'Fixture network', network_name: 'fixture',
            networking_method: 'Standalone', member_count: 2, online_member_count: 1 }] };
    configure(state);
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width: 360, height: 900 } });
    t.after(() => context.close());
    await context.addInitScript(() => localStorage.setItem('lang', 'en'));
    await context.route('**/api_meta.js', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
    await context.route('**/api/v1/**', async route => {
        const path = new URL(route.request().url()).pathname.replace('/api/v1', '');
        state.requests.push(path);
        if (path === '/summary' && state.holdSummary) await state.holdSummary;
        if (state.failures.has(path)) return route.fulfill({ status: 503, json: { code: 'internal_error' } });
        const responses = {
            '/auth/me': { id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'],
                source: 'local', local_password_enabled: true, must_change_password: false, web_login_enabled: true },
            '/console-info': { username: 'fixture-admin', webhook_auth: state.external, config_server_protocol: 'tcp', config_server_port: 11010 },
            '/summary': { device_count: state.machines.length },
            '/machines': { machines: state.machines },
            '/networks': { networks: state.networks },
            '/blocked-devices': { blocked: [] },
        };
        return route.fulfill({ status: path in responses ? 200 : 404, json: responses[path] ?? { code: 'not_found' } });
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto('/#/h');
    await page.getByRole('heading', { name: 'Dashboard', exact: true }).waitFor();
    return { page, state, errors };
}

async function waitForCounts(page, expected) {
    await page.waitForFunction(values => JSON.stringify([...document.querySelectorAll('.summary-value')]
        .map(element => element.textContent.trim())) === JSON.stringify(values), expected);
}

test('dashboard retains failed source data, updates successful counts and retries manually', async t => {
    const { page, state, errors } = await openDashboard(t);
    await waitForCounts(page, ['2', '1', '1', '1']);
    assert.match(await page.locator('.overview-columns a.preview-row').first().getAttribute('href'), /deviceList\/device\//);
    assert.equal(await page.locator('.overview-columns a[href="#\/h\/networks\/fixture-network"]').count(), 1);
    state.failures.add('/networks');
    state.machines.pop();
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await page.getByText('Unable to refresh data.', { exact: false }).waitFor();
    await waitForCounts(page, ['1', '1', '1', '1']);
    state.failures.clear();
    state.networks = [];
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await waitForCounts(page, ['1', '1', '0', '1']);
    await page.getByText('Unable to refresh data.', { exact: false }).waitFor({ state: 'hidden' });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    assert.deepEqual(errors, []);
});

test('first source failure is unknown, not a false zero or empty result', async t => {
    const { page, errors } = await openDashboard(t, state => {
        state.failures.add('/summary'); state.failures.add('/machines');
    });
    await waitForCounts(page, ['—', '—', '1', '—']);
    await page.getByText('Unable to refresh data.', { exact: false }).waitFor();
    assert.equal(await page.getByRole('heading', { name: 'No devices yet', exact: true }).count(), 0);
    assert.deepEqual(errors, []);
});

test('external Console has no central requests and repeated refresh does not overlap', async t => {
    const { page, state, errors } = await openDashboard(t, state => { state.external = true; });
    await waitForCounts(page, ['2', '1', '1']);
    let release;
    state.holdSummary = new Promise(resolve => { release = resolve; });
    t.after(() => release());
    const before = state.requests.filter(path => path === '/summary').length;
    const requested = page.waitForRequest(request => request.url().endsWith('/summary'));
    const refresh = page.getByRole('button', { name: 'Refresh', exact: true });
    await refresh.click();
    await requested;
    await refresh.dispatchEvent('click');
    assert.equal(state.requests.filter(path => path === '/summary').length, before + 1);
    release(); state.holdSummary = null;
    await page.waitForFunction(() => !document.querySelector('.page-heading button').classList.contains('p-button-loading'));
    assert.equal(state.requests.some(path => path.startsWith('/networks')), false);
    assert.equal(await page.locator('a[href*="/networks"]').count(), 0);
    assert.deepEqual(errors, []);
});
