import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';

let browser;
let preview;
before(async () => {
    preview = await createEcosystemPreview({ componentPath: 'src/components/ecosystem/nodes/EcosystemNodeDetails.vue', fixtureKey: 'nodeDetails', eventNames: ['selectResourceKind'] });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });
async function open(t, options) {
    const view = await openEcosystemPreview(browser, preview, options);
    t.after(async () => { await view.close(); assert.deepEqual(view.requests, []); assert.deepEqual(view.errors, []); });
    return view.page;
}
function field(page, label) { return page.locator('dl > div').filter({ has: page.getByText(label, { exact: true }) }).locator('dd'); }

test('unknown host fields remain unknown and unsupported metadata has no fake save controls', async t => {
    const page = await open(t, { scenario: 'unknown' });
    for (const label of ['Hostname', 'Console alias', 'EasyTier version', 'Operating system', 'Manufacturer', 'Model', 'Architecture', 'Deployment location']) {
        assert.equal(await field(page, label).innerText(), 'Unknown');
    }
    await page.getByText('Control session unknown', { exact: true }).waitFor();
    await page.getByText('Saving a deployment location, manufacturer or model is not available here. The information below comes from the host.', { exact: true }).waitFor();
    assert.equal(await page.locator('input, textarea, form').count(), 0);
    assert.equal(await page.getByRole('button').count(), 3);
    await page.getByText('No running instances were reported. This does not confirm that every instance has stopped.', { exact: true }).waitFor();
});

test('reported OS, GeoIP, membership and instance IDs stay distinct and switch with the selected host', async t => {
    const page = await open(t);
    await page.evaluate(() => Object.assign(window.ecosystemFixture.props.node, {
        hostname: 'fictional-linux-arm-host', alias: 'Fictional console alias', version: 'fixture-version',
        os: { type: 'Fixture OS', version: '1.2', distribution: 'Fixture Distribution' },
        geoLocation: { country: 'Fixture country', region: null, city: 'Fixture city' },
        lastSeenAt: '2026-10-21T10:00:00Z', runningInstanceIds: ['fixture-instance-a', 'fixture-instance-b'],
    }));
    assert.equal(await field(page, 'Operating system').innerText(), 'Fixture OS');
    assert.equal(await field(page, 'Architecture').innerText(), 'Unknown');
    assert.equal(await field(page, 'Deployment location').innerText(), 'Unknown');
    assert.match(await field(page, 'Estimated connection IP location').innerText(), /Fixture country \/ Fixture city/);
    await page.getByText('This is not a saved deployment location.', { exact: true }).waitFor();
    assert.equal(await page.locator('time').getAttribute('datetime'), '2026-10-21T10:00:00Z');
    await page.getByText('fixture-instance-a', { exact: true }).waitFor();
    await page.getByText('fixture-network', { exact: true }).waitFor();
    await page.evaluate(() => window.ecosystemFixture.setProps({ node: null }));
    await page.getByText('Select a host to view its details.', { exact: true }).waitFor();
    assert.equal(await page.getByText('Fixture OS', { exact: true }).count(), 0);
    assert.equal(await page.getByRole('button').count(), 0);
});

test('resource category buttons emit typed navigation intent through keyboard without changing the host', async t => {
    const page = await open(t);
    const original = await page.evaluate(() => JSON.stringify(window.ecosystemFixture.props.node));
    for (const [label, kind] of [['Services', 'service'], ['Device groups', 'device_group'], ['Devices', 'device']]) {
        const button = page.getByRole('button', { name: label, exact: true });
        await button.focus();
        await page.keyboard.press('Enter');
        assert.deepEqual(await page.evaluate(() => window.ecosystemFixture.events.at(-1)), { name: 'selectResourceKind', args: [kind] });
    }
    assert.equal(await page.evaluate(() => JSON.stringify(window.ecosystemFixture.props.node)), original);
    assert.equal(await page.evaluate(() => window.ecosystemFixture.events.length), 3);
});

test('webhook membership is unsupported rather than a false empty network success', async t => {
    const page = await open(t, { scenario: 'webhook' });
    await page.getByText('Network membership is managed by the external console and is unavailable here.', { exact: true }).waitFor();
    assert.equal(await page.getByText('No network assignments were reported.', { exact: true }).count(), 0);
    await page.getByText('A control session does not confirm application health or public reachability.', { exact: true }).waitFor();
});

test('CN/EN and light/dark at 360/768/1440 preserve long untrusted text, keyboard controls and readonly semantics', async t => {
    for (const language of ['cn', 'en']) for (const theme of ['light', 'dark']) for (const width of [360, 768, 1440]) {
        const page = await open(t, { language, theme, width });
        await page.evaluate(() => {
            const node = window.ecosystemFixture.props.node;
            node.alias = '<img src=x onerror=alert(1)>' + 'Fictional-host-'.repeat(40);
            node.networks[0].displayName = 'Long-network-'.repeat(40);
            node.runningInstanceIds = ['instance-'.repeat(40)];
        });
        assert.equal(await page.locator('#app img, #app input').count(), 0);
        assert.match(await page.locator('#app h3').first().innerText(), /<img src=x/);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
        const button = page.getByRole('button', { name: language === 'cn' ? '设备组' : 'Device groups', exact: true });
        await button.focus();
        assert.equal(await button.evaluate(element => element === document.activeElement), true);
        assert.ok(!(await page.locator('#app').innerText()).includes('web.ecosystem.'));
    }
});
