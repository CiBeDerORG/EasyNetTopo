import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';

let browser;
let preview;
before(async () => {
    preview = await createEcosystemPreview({ componentPath: 'src/components/ecosystem/topology/EcosystemTopology.vue' });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });
async function open(t, options) {
    const view = await openEcosystemPreview(browser, preview, options);
    t.after(async () => { await view.close(); assert.deepEqual(view.requests, []); assert.deepEqual(view.errors, []); });
    await view.page.getByRole('heading', { name: options?.language === 'cn' ? '网络归属' : 'Network membership', exact: true }).waitFor();
    return view.page;
}

test('membership and temporary peers stay distinct; keyboard selects the stable host identity', async t => {
    const page = await open(t);
    await page.getByText('Membership shows network assignment, not measured direct or relay connections.', { exact: true }).waitFor();
    await page.getByRole('heading', { name: 'Sample temporary peer', exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).count(), 1);
    const button = page.getByRole('button', { name: /^View host:/ });
    await button.focus();
    await page.keyboard.press('Enter');
    assert.deepEqual(await page.evaluate(() => window.ecosystemFixture.events.at(-1)), {
        name: 'selectNode', args: [{ userId: 42, machineId: '00000000-0000-4000-8000-000000000042' }],
    });
    await page.getByLabel('Select a network', { exact: true }).selectOption('');
    assert.deepEqual(await page.evaluate(() => window.ecosystemFixture.events.at(-1)), { name: 'selectNetwork', args: [null] });
    assert.equal(await page.evaluate(() => window.ecosystemFixture.props.selectedNetworkId), 'fixture-network');
});

test('multi-network hosts remain one record and location filters only use supplied GeoIP fields', async t => {
    const page = await open(t);
    await page.evaluate(() => {
        const state = window.ecosystemFixture.props;
        state.selectedNetworkId = null;
        const host = state.overview.snapshot.data.nodes[0];
        host.geoLocation = { country: 'Fictional country', region: null, city: 'Sample city' };
        host.networks.push({ networkId: 'second-network', displayName: 'Second network', networkName: 'second' });
        state.overview.snapshot.data.nodes.push({ ...host, ref: { userId: 42, machineId: '00000000-0000-4000-8000-000000000043' }, geoLocation: null, networks: [] });
    });
    assert.equal(await page.getByRole('heading', { name: 'Sample host', exact: true }).count(), 2);
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).count(), 2);
    await page.getByLabel('Estimated connection IP location', { exact: true }).selectOption(JSON.stringify('Fictional country / Sample city'));
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).count(), 1);
    await page.getByText('Sample membership network, Second network', { exact: true }).waitFor();
    await page.getByLabel('Estimated connection IP location', { exact: true }).selectOption('unknown');
    await page.getByRole('button', { name: /000000000043$/ }).click();
    assert.equal(await page.evaluate(() => window.ecosystemFixture.events.at(-1).args[0].machineId), '00000000-0000-4000-8000-000000000043');
});

test('webhook mode shows unavailable membership without an empty central-network claim', async t => {
    const page = await open(t, { scenario: 'webhook' });
    await page.getByText('Network membership is managed by the external console and is unavailable here.', { exact: true }).waitFor();
    assert.equal(await page.getByLabel('Select a network', { exact: true }).count(), 0);
    assert.equal(await page.getByText('No managed networks found.', { exact: true }).count(), 0);
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).count(), 1);
});

test('initial loading and failures do not masquerade as empty or offline', async t => {
    const page = await open(t, { scenario: 'idle' });
    await page.evaluate(() => window.ecosystemFixture.setProps({ overview: { status: 'loading', previous: null } }));
    await page.getByText('Loading…', { exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: 'Refresh', exact: true }).isDisabled(), true);
    assert.equal(await page.getByText('No hosts found in this result.', { exact: true }).count(), 0);
    await page.evaluate(() => window.ecosystemFixture.setProps({ overview: { status: 'error', error: { kind: 'forbidden', status: 403 }, previous: null } }));
    await page.getByRole('alert').getByText('Access was denied.', { exact: true }).waitFor();
    assert.equal(await page.getByText('Control session offline', { exact: true }).count(), 0);
    assert.equal(await page.getByText('No managed networks found.', { exact: true }).count(), 0);
});

test('unknown data is explicit; unrelated tenant and stale-network members cannot become selected hosts', async t => {
    const page = await open(t, { scenario: 'unknown' });
    await page.getByText('Control session unknown', { exact: true }).waitFor();
    assert.equal(await page.getByText('Unknown', { exact: true }).count() >= 2, true);
    await page.evaluate(() => { const member = window.ecosystemFixture.props.members.snapshot.data.members[0]; member.node = { ...member.node, userId: 99 }; });
    await page.getByText('Host details are unavailable.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).isDisabled(), true);
    await page.evaluate(() => window.ecosystemFixture.setProps({ selectedNetworkId: 'another-network' }));
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).count(), 0);
    assert.equal(await page.getByText('Control session unknown', { exact: true }).count(), 0);
    await page.getByText('Not queried yet.', { exact: true }).waitFor();
    await page.evaluate(() => {
        const state = window.ecosystemFixture.props;
        state.members = { status: 'error', error: { kind: 'not_found', status: 404 }, previous: state.members.snapshot };
    });
    assert.equal(await page.getByRole('alert').count(), 0);
    assert.equal(await page.getByRole('button', { name: /^View host:/ }).count(), 0);
    await page.getByText('Not queried yet.', { exact: true }).waitFor();
});

test('successful empty data and retained failed snapshots are separately described', async t => {
    const empty = await open(t, { scenario: 'empty' });
    await empty.getByText('No hosts found in this result.', { exact: true }).waitFor();
    await empty.getByText('No managed networks found.', { exact: true }).waitFor();
    const failed = await open(t, { scenario: 'error' });
    assert.equal(await failed.getByRole('alert').count(), 2);
    assert.equal(await failed.getByRole('heading', { name: 'Sample host', exact: true }).count(), 1);
    assert.equal(await failed.getByText('Last successful result', { exact: false }).count(), 2);
});

test('CN/EN, themes and 360/768/1440 layouts retain controls with long host and network names', async t => {
    for (const language of ['cn', 'en']) for (const theme of ['light', 'dark']) for (const width of [360, 768, 1440]) {
        const page = await open(t, { language, theme, width });
        await page.evaluate(() => {
            const state = window.ecosystemFixture.props;
            state.overview.snapshot.data.nodes[0].hostname = 'Fictional-long-host-'.repeat(30);
            state.overview.snapshot.data.networks[0].displayName = 'Long-network-'.repeat(30);
        });
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
        const selector = page.getByLabel(language === 'cn' ? '选择网络' : 'Select a network', { exact: true });
        assert.equal(await selector.isVisible(), true);
        await selector.focus();
        assert.equal(await selector.evaluate(element => element === document.activeElement), true);
        assert.ok(!(await page.locator('#app').innerText()).includes('web.ecosystem.'));
    }
});
