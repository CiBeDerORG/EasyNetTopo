import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';

let browser;
let preview;
before(async () => {
    preview = await createEcosystemPreview({ componentPath: 'src/components/ecosystem/resources/EcosystemResourceDetails.vue', fixtureKey: 'resourceDetails', eventNames: [] });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });
async function open(t, options) {
    const view = await openEcosystemPreview(browser, preview, options);
    t.after(async () => { await view.close(); assert.deepEqual(view.requests, []); assert.deepEqual(view.errors, []); });
    return view.page;
}

test('all three resource categories identify the host without inventing resource records or operations', async t => {
    const page = await open(t);
    for (const [kind, label] of [['service', 'Services'], ['device_group', 'Device groups'], ['device', 'Devices']]) {
        await page.evaluate(kind => window.ecosystemFixture.setProps({ kind }), kind);
        await page.getByRole('heading', { name: label, exact: true }).waitFor();
        await page.getByText('Sample host', { exact: true }).waitFor();
        await page.getByText('Business services and devices have not been connected to this console. Their information and controls are unavailable.', { exact: true }).waitFor();
        await page.getByText('No verified business-resource records or access endpoints are available in this view. The selected host is not a business device.', { exact: true }).waitFor();
        assert.equal(await page.locator('#app button, #app input, #app a, #app table').count(), 0);
        assert.deepEqual(await page.evaluate(() => window.ecosystemFixture.events), []);
    }
});

test('host changes and deselection remove stale resource context without persisting a draft', async t => {
    const page = await open(t);
    await page.evaluate(() => {
        const state = window.ecosystemFixture.props;
        state.node = { ...state.node, hostname: 'Different host', ref: { userId: 43, machineId: '00000000-0000-4000-8000-000000000043' } };
        state.kind = 'device';
    });
    await page.getByText('Different host', { exact: true }).waitFor();
    assert.equal(await page.getByText('Sample host', { exact: true }).count(), 0);
    await page.getByText('00000000-0000-4000-8000-000000000043', { exact: true }).waitFor();
    await page.evaluate(() => window.ecosystemFixture.setProps({ node: null }));
    await page.getByText('Select a host before viewing its resource capabilities.', { exact: true }).waitFor();
    assert.equal(await page.getByText('Different host', { exact: true }).count(), 0);
    assert.equal(await page.getByRole('heading', { name: 'Devices', exact: true }).count(), 0);
    assert.equal(await page.evaluate(() => localStorage.length), 0);
});

test('a capability flag alone never manufactures records, endpoints or a successful action', async t => {
    const page = await open(t);
    await page.evaluate(() => window.ecosystemFixture.setProps({ capability: { status: 'supported' } }));
    assert.equal(await page.getByText('Not supported', { exact: true }).count(), 0);
    await page.getByText('No verified business-resource records or access endpoints are available in this view. The selected host is not a business device.', { exact: true }).waitFor();
    assert.equal(await page.locator('#app button, #app input, #app a, #app table').count(), 0);
    assert.deepEqual(await page.evaluate(() => window.ecosystemFixture.events), []);
});

test('untrusted host text stays plain text across CN/EN, themes and 360/768/1440 layouts', async t => {
    for (const language of ['cn', 'en']) for (const theme of ['light', 'dark']) for (const width of [360, 768, 1440]) {
        const page = await open(t, { language, theme, width });
        await page.evaluate(() => { window.ecosystemFixture.props.node.alias = '<img src=x onerror=alert(1)>' + 'Long-fictional-host-'.repeat(40); });
        assert.equal(await page.locator('#app img').count(), 0);
        assert.match(await page.locator('dd').first().innerText(), /<img src=x/);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
        await page.getByRole('heading', { name: language === 'cn' ? '业务资源' : 'Business resources', exact: true }).waitFor();
        assert.ok(!(await page.locator('#app').innerText()).includes('web.ecosystem.'));
    }
});
