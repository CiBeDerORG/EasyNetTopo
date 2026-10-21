import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';
let browser, preview;
const uuid = '00000000-0000-4000-8000-000000000042';
const nodes = [{ info: { machine_id: uuid, hostname: 'Fixture host', running_network_instances: [] }, networks: [], online: true }];
const networks = ['alpha', 'beta'].map(id => ({ network_id: id, display_name: id, network_name: id, member_count: 0, online_member_count: 0 }));
const empty = { members: [], temporary_peers: [] };
before(async () => {
    preview = await createEcosystemPreview({ componentPath: 'src/components/ecosystem/EcosystemPage.vue', fixtureKey: 'composition', eventNames: [] });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });
const calls = page => page.evaluate(() => window.compositionTransport.calls);
async function settle(page, index, value) {
    await page.waitForFunction(index => window.compositionTransport.calls.length > index, index);
    await page.evaluate(({ index, value }) => window.compositionTransport.resolve(index, value), { index, value });
}
async function overview(page, start = 0) {
    await settle(page, start, { webhook_auth: false });
    await settle(page, start + 1, nodes);
    await settle(page, start + 2, networks);
    await page.locator('#ecosystem-network').waitFor();
}
const refresh = page => page.getByRole('button', { name: 'Refresh', exact: true }).first().click();
test('initial read, member races, focus, errors preserve snapshots and manual recovery', async () => {
    const view = await openEcosystemPreview(browser, preview), page = view.page;
    try {
        assert.equal((await calls(page)).length, 1);
        assert.equal(await page.getByRole('button', { name: 'Refresh', exact: true }).isDisabled(), true);
        await overview(page);
        await page.locator('#ecosystem-network').selectOption('alpha');
        await page.waitForFunction(() => window.compositionTransport.calls.length === 4);
        await page.locator('#ecosystem-network').selectOption('beta');
        await settle(page, 4, { members: [{ member_id: 'fixture-member', device_id: uuid, hostname: 'Fixture host', online: true }], temporary_peers: [{ peer_id: 9, hostname: 'beta peer' }] });
        await page.getByText('beta peer', { exact: true }).waitFor();
        await settle(page, 3, { members: [], temporary_peers: [{ peer_id: 8, hostname: 'stale alpha' }] });
        assert.equal(await page.getByText('stale alpha').count(), 0);
        await page.getByRole('button', { name: `View host: ${uuid}`, exact: true }).click();
        await page.getByRole('button', { name: 'Back to host', exact: true }).click();
        assert.equal(await page.locator('.ecosystem-host-section').evaluate(el => el === document.activeElement), true);
        assert.equal((await calls(page)).length, 5);
        await refresh(page);
        await overview(page, 5);
        await page.waitForFunction(() => window.compositionTransport.calls.length === 9);
        await page.evaluate(() => window.compositionTransport.reject(8));
        await page.getByText('The query failed. Refresh manually to try again.', { exact: true }).waitFor();
        assert.equal(await page.getByText('beta peer', { exact: true }).count(), 1);
        await refresh(page);
        await page.evaluate(() => window.compositionTransport.reject(9));
        await page.waitForFunction(() => document.querySelectorAll('[role="alert"]').length === 2);
        assert.equal(await page.getByText('beta peer', { exact: true }).count(), 1);
        assert.equal((await calls(page)).length, 10);
        await refresh(page);
        await overview(page, 10);
        await settle(page, 13, empty);
        await page.getByText('This network has no registered members.', { exact: true }).waitFor();
        assert.equal((await calls(page)).length, 14);
        assert.deepEqual(view.requests, []); assert.deepEqual(view.errors, []);
    } finally { await view.close(); }
});
test('identity clears old results; late overview and unmounted responses cannot update page', async () => {
    const view = await openEcosystemPreview(browser, preview), page = view.page;
    try {
        await page.evaluate(() => window.ecosystemFixture.setProps({ currentUserId: 43 }));
        await settle(page, 1, { webhook_auth: true });
        await settle(page, 2, nodes);
        await page.getByText('Fixture host', { exact: true }).waitFor();
        await settle(page, 0, { webhook_auth: false });
        await settle(page, 3, nodes); await settle(page, 4, networks);
        assert.equal(await page.locator('#ecosystem-network').count(), 0);
        await refresh(page);
        await page.evaluate(() => window.ecosystemFixture.unmount());
        await settle(page, 5, { webhook_auth: true }); await settle(page, 6, nodes);
        assert.equal(await page.locator('#app').innerText(), '');
        assert.deepEqual(view.errors, []); assert.deepEqual(view.requests, []);
    } finally { await view.close(); }
});
test('clearing selection drops pending members and identity resets visible host and resource choices', async () => {
    const view = await openEcosystemPreview(browser, preview), page = view.page;
    try {
        await overview(page);
        await page.getByRole('button', { name: `View host: ${uuid}`, exact: true }).click();
        await page.getByRole('button', { name: 'Devices', exact: true }).click();
        await page.locator('#ecosystem-network').selectOption('alpha');
        await page.waitForFunction(() => window.compositionTransport.calls.length === 4);
        await page.locator('#ecosystem-network').selectOption('');
        await settle(page, 3, { members: [], temporary_peers: [{ peer_id: 99, hostname: 'cleared peer' }] });
        assert.equal(await page.getByText('cleared peer').count(), 0);
        await page.getByRole('button', { name: `View host: ${uuid}`, exact: true }).click();
        await page.evaluate(() => window.ecosystemFixture.setProps({ currentUserId: 44 }));
        await page.waitForFunction(() => window.compositionTransport.calls.length === 5);
        assert.equal(await page.getByRole('button', { name: 'Back to host', exact: true }).count(), 0);
        assert.equal(await page.getByText('Fixture host', { exact: true }).count(), 0);
        await settle(page, 4, { webhook_auth: true }); await settle(page, 5, []);
        await page.getByText('No hosts found in this result.', { exact: true }).waitFor();
        const before = await calls(page);
        await page.evaluate(() => { window.dispatchEvent(new Event('online')); document.dispatchEvent(new Event('visibilitychange')); });
        assert.deepEqual(await calls(page), before);
        assert.deepEqual(view.errors, []); assert.deepEqual(view.requests, []);
    } finally { await view.close(); }
});
test('pending members exit loading after overview failure and late response cannot replace previous data', async () => {
    const view = await openEcosystemPreview(browser, preview), page = view.page;
    try {
        await overview(page);
        await page.locator('#ecosystem-network').selectOption('alpha');
        await settle(page, 3, { members: [], temporary_peers: [{ peer_id: 1, hostname: 'previous peer' }] });
        await page.getByText('previous peer', { exact: true }).waitFor();
        await refresh(page);
        await overview(page, 4);
        await page.waitForFunction(() => window.compositionTransport.calls.length === 8);
        await page.locator('.ecosystem-members [aria-busy="true"]').waitFor();
        // Member call 7 remains pending while overview call 8 invalidates its generation.
        await refresh(page);
        await page.waitForFunction(() => window.compositionTransport.calls.length === 9);
        await page.evaluate(() => window.compositionTransport.reject(8));
        await page.waitForFunction(() => document.querySelectorAll('[role="alert"]').length === 2);
        assert.equal(await page.locator('[aria-busy="true"]').count(), 0);
        assert.equal(await page.getByText('Loading…', { exact: true }).count(), 0);
        assert.equal(await page.getByText('previous peer', { exact: true }).count(), 1);
        for (const button of await page.getByRole('button', { name: 'Refresh', exact: true }).all()) assert.equal(await button.isEnabled(), true);
        await settle(page, 7, { members: [], temporary_peers: [{ peer_id: 2, hostname: 'late peer' }] });
        assert.equal(await page.getByText('late peer', { exact: true }).count(), 0);
        assert.equal(await page.getByText('previous peer', { exact: true }).count(), 1);
        assert.equal(await page.locator('[role="alert"]').count(), 2);
        await refresh(page);
        await overview(page, 9);
        await settle(page, 12, empty);
        await page.getByText('This network has no registered members.', { exact: true }).waitFor();
        assert.equal(await page.locator('[role="alert"], [aria-busy="true"]').count(), 0);
        assert.equal((await calls(page)).length, 13);
        assert.deepEqual(view.errors, []); assert.deepEqual(view.requests, []);
    } finally { await view.close(); }
});
