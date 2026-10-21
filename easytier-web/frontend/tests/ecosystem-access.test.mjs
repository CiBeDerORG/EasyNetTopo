import assert from 'node:assert/strict';
import { readFile, mkdir } from 'node:fs/promises';
import { before, after, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';

let browser, preview;
const componentPath = 'src/components/ecosystem/access/EcosystemAccessSettings.vue';
const evidenceDirectory = new URL('../../../process/easytier-ecosystem-integration-done/plan/design/access/', import.meta.url);
before(async () => {
    preview = await createEcosystemPreview({ componentPath, fixtureKey: 'access' });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });

for (const language of ['en', 'cn']) {
    for (const theme of ['light', 'dark']) {
        for (const width of [360, 768, 1440]) {
            test(`access capabilities: ${language}/${theme}/${width}`, async () => {
                const view = await openEcosystemPreview(browser, preview, { language, theme, width });
                try {
                    assert.equal(await view.page.locator('#app h3').count(), 3);
                    assert.equal(await view.page.locator('#app [role="status"]').count(), 3);
                    const text = await view.page.locator('#app').innerText();
                    assert.doesNotMatch(text, /web\.ecosystem|API|api_missing/);
                    assert.match(text, language === 'en' ? /separate capabilities/ : /独立/);
                    assert.equal(await view.page.locator('#app input, #app button, #app a, #app select').count(), 0);
                    assert.equal(await view.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
                    await view.page.keyboard.press('Tab');
                    await view.page.keyboard.press('Enter');
                    await view.page.keyboard.press('Escape');
                    assert.deepEqual(await view.page.evaluate(() => window.ecosystemFixture.events), []);
                    assert.deepEqual(view.errors, []);
                    assert.deepEqual(view.requests, []);
                    if (process.env.ECO_ACCESS_SCREENSHOTS === '1' && width === 360) {
                        await mkdir(evidenceDirectory, { recursive: true });
                        await view.page.screenshot({ path: new URL(`access-${language}-${theme}-360.png`, evidenceDirectory).pathname, fullPage: true });
                    }
                } finally { await view.close(); }
            });
        }
    }
}

test('changing one capability never changes the other two or creates an action', async () => {
    const view = await openEcosystemPreview(browser, preview);
    try {
        for (const [prop, key] of [['publicAccess', 'public_access'], ['portMappings', 'port_mappings'], ['navigation', 'navigation']]) {
            const before = await view.page.locator('[data-capability]').allTextContents();
            await view.page.evaluate(prop => window.ecosystemFixture.setProps({ [prop]: { status: 'supported' } }), prop);
            await view.page.locator(`[data-capability="${key}"]`).getByText('This capability is available.', { exact: false }).waitFor();
            const after = await view.page.locator('[data-capability]').allTextContents();
            for (const [index, sectionKey] of ['public_access', 'port_mappings', 'navigation'].entries()) {
                if (sectionKey !== key) assert.equal(after[index], before[index]);
            }
        }
        assert.equal(await view.page.locator('#app button, #app input, #app a').count(), 0);
        assert.deepEqual(view.requests, []);
        assert.deepEqual(view.errors, []);
    } finally { await view.close(); }
});

test('access component owns no request, timer, persistence or write event', async () => {
    const source = await readFile(new URL(`../${componentPath}`, import.meta.url), 'utf8');
    assert.doesNotMatch(source, /setInterval|setTimeout|fetch\(|axios|localStorage|sessionStorage|defineEmits|onMounted|watch\(/);
});
