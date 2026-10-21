import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './browserHarness.mjs';
let browser, preview;
before(async () => {
    preview = await createEcosystemPreview();
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => { await browser?.close(); await preview?.close(); });
test('actual Vue contract consumer renders every fixture state without API calls', async () => {
    for (const scenario of ['central', 'webhook', 'empty', 'unknown', 'loading', 'error', 'idle']) {
        const view = await openEcosystemPreview(browser, preview, { scenario, language: 'cn', theme: 'dark', width: 360 });
        try {
            assert.equal(await view.page.getByTestId('status').innerText(), ['loading', 'error', 'idle'].includes(scenario) ? scenario : 'ready');
            if (scenario === 'webhook') assert.equal(await view.page.getByTestId('mode').innerText(), 'webhook');
            if (scenario === 'empty') assert.equal(await view.page.getByRole('button', { name: 'Select sample host' }).count(), 0);
            if (scenario === 'unknown') assert.equal(await view.page.evaluate(() => window.ecosystemFixture.props.overview.snapshot.data.nodes[0].connection), 'unknown');
            assert.equal(await view.page.evaluate(() => document.documentElement.classList.contains('app-dark')), true);
            assert.deepEqual(view.errors, []);
            assert.deepEqual(view.requests, []);
        } finally { await view.close(); }
    }
});
test('fixture captures typed selection events and reactive prop transitions', async () => {
    const view = await openEcosystemPreview(browser, preview);
    try {
        await view.page.getByRole('button', { name: 'Refresh sample' }).click();
        await view.page.getByRole('button', { name: 'Select sample host' }).click();
        const events = await view.page.evaluate(() => window.ecosystemFixture.events);
        assert.deepEqual(events.map(event => event.name), ['refresh', 'selectNode']);
        assert.equal(events[1].args[0].userId, 42);
        await view.page.evaluate(() => window.ecosystemFixture.setProps({ overview: { status: 'loading', previous: null } }));
        await view.page.getByTestId('status').filter({ hasText: 'loading' }).waitFor();
        assert.deepEqual(view.errors, []);
        assert.deepEqual(view.requests, []);
    } finally { await view.close(); }
});
test('fixture refuses arbitrary production paths and invalid fixture names', async () => {
    await assert.rejects(createEcosystemPreview({ componentPath: 'src/components/MainPage.vue' }), /ecosystem component/);
    await assert.rejects(createEcosystemPreview({ fixtureKey: 'missing' }), /Unknown fixture key/);
});

test('simultaneous previews use distinct explicitly reserved loopback ports', async () => {
    const second = await createEcosystemPreview();
    try {
        assert.notEqual(new URL(second.url).port, new URL(preview.url).port);
        assert.notEqual(new URL(second.url).port, '5173');
        const response = await fetch(second.url);
        assert.equal(response.status, 200);
        assert.match(await response.text(), /Development sample/);
    } finally { await second.close(); }
});
