import assert from 'node:assert/strict';
import { readFile, mkdir } from 'node:fs/promises';
import { before, after, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';
let browser, preview;
const componentPath = 'src/components/ecosystem/navigation/EcosystemNavigation.vue';
const evidenceDirectory = new URL('../../../process/easytier-ecosystem-integration-done/plan/design/navigation/', import.meta.url);
before(async () => {
    preview = await createEcosystemPreview({ componentPath, fixtureKey: 'navigation', eventNames: ['backToNode'] });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });
for (const language of ['en', 'cn']) for (const theme of ['light', 'dark']) for (const width of [360, 768, 1440]) {
    test(`navigation: ${language}/${theme}/${width}`, async () => {
        const view = await openEcosystemPreview(browser, preview, { language, theme, width });
        try {
            const text = await view.page.locator('#app').innerText();
            assert.doesNotMatch(text, /web\.ecosystem|API|api_missing/);
            assert.match(text, /Sample host/);
            assert.equal(await view.page.locator('#app a, #app input').count(), 0);
            assert.equal(await view.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
            const button = view.page.getByRole('button');
            await view.page.keyboard.press('Tab');
            assert.equal(await button.evaluate(el => el === document.activeElement), true);
            await view.page.keyboard.press('Enter');
            await view.page.keyboard.press('Space');
            assert.deepEqual(await view.page.evaluate(() => window.ecosystemFixture.events), [
                { name: 'backToNode', args: [{ userId: 42, machineId: '00000000-0000-4000-8000-000000000042' }] },
                { name: 'backToNode', args: [{ userId: 42, machineId: '00000000-0000-4000-8000-000000000042' }] },
            ]);
            assert.deepEqual(view.errors, []);
            assert.deepEqual(view.requests, []);
            if (process.env.ECO_NAVIGATION_SCREENSHOTS === '1' && width === 360) {
                await mkdir(evidenceDirectory, { recursive: true });
                await view.page.screenshot({ path: new URL(`navigation-${language}-${theme}-360.png`, evidenceDirectory).pathname, fullPage: true });
            }
        } finally { await view.close(); }
    });
}
test('null, unknown and replaced hosts cannot retain a stale return target; supported never invents links', async () => {
    const view = await openEcosystemPreview(browser, preview, { scenario: 'empty' });
    try {
        assert.equal(await view.page.getByRole('button').count(), 0);
        assert.doesNotMatch(await view.page.locator('#app').innerText(), /web\.ecosystem/);
        await view.page.evaluate(() => window.ecosystemFixture.setProps({
            node: { ref: { userId: 7, machineId: '00000000-0000-4000-8000-000000000007' }, alias: null, hostname: null },
            capability: { status: 'supported' },
        }));
        await view.page.getByText('Current host: Unknown', { exact: true }).waitFor();
        await view.page.getByRole('button').click();
        assert.deepEqual(await view.page.evaluate(() => window.ecosystemFixture.events[0].args[0]), { userId: 7, machineId: '00000000-0000-4000-8000-000000000007' });
        assert.equal(await view.page.locator('#app a').count(), 0);
        await view.page.evaluate(() => window.ecosystemFixture.setProps({ node: null }));
        await view.page.getByRole('button').waitFor({ state: 'detached' });
        assert.deepEqual(view.errors, []);
        assert.deepEqual(view.requests, []);
    } finally { await view.close(); }
});
test('navigation owns no requests, timer, persistence or external URL construction', async () => {
    const source = await readFile(new URL(`../${componentPath}`, import.meta.url), 'utf8');
    assert.doesNotMatch(source, /setInterval|setTimeout|fetch\(|axios|localStorage|sessionStorage|window\.open|href=|new URL/);
});
