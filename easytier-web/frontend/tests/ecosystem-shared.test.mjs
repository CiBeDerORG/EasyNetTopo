import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { chromium } from 'playwright';
import { createEcosystemPreview, openEcosystemPreview } from './ecosystem-support/browserHarness.mjs';

// Real Vue/locales in Chromium; development props are not backend evidence.
const require = createRequire(import.meta.url);
const { load } = createRequire(require.resolve('@modyfi/vite-plugin-yaml'))('js-yaml');
const catalogs = Object.fromEntries(await Promise.all(['en', 'cn'].map(async lang => [lang,
    load(await readFile(new URL(`../../frontend-lib/src/locales/${lang}.yaml`, import.meta.url), 'utf8')).web.ecosystem,
])));
let browser;
let queryPreview;
let capabilityPreview;
before(async () => {
    queryPreview = await createEcosystemPreview({
        componentPath: 'src/components/ecosystem/shared/EcosystemQueryNotice.vue',
        fixtureKey: 'queryNotice', eventNames: ['refresh'],
    });
    capabilityPreview = await createEcosystemPreview({
        componentPath: 'src/components/ecosystem/shared/EcosystemCapabilityNotice.vue',
        fixtureKey: 'capabilityNotice', eventNames: [],
    });
    browser = await chromium.launch({ headless: true, executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium' });
});
after(async () => {
    await browser?.close();
    await queryPreview?.close();
    await capabilityPreview?.close();
});
async function open(t, { lang = 'en', theme = 'light', width = 768, capability = false } = {}) {
    const view = await openEcosystemPreview(browser, capability ? capabilityPreview : queryPreview,
        { scenario: 'idle', language: lang, theme, width });
    t.after(async () => { await view.close(); assert.deepEqual(view.errors, []); assert.deepEqual(view.requests, []); });
    return view.page;
}

test('refresh is an explicit keyboard event; loading prevents duplicates', async t => {
    const page = await open(t);
    const button = page.getByRole('button', { name: 'Refresh' });
    await button.focus();
    await page.keyboard.press('Enter');
    await page.evaluate(() => window.ecosystemFixture.setProps({ status: 'loading' }));
    assert.equal(await button.isDisabled(), true);
    await page.keyboard.press('Enter');
    assert.deepEqual(await page.evaluate(() => window.ecosystemFixture.events), [{ name: 'refresh', args: [] }]);
    await page.getByText('Loading…', { exact: true }).waitFor();
    assert.equal(await page.locator('[aria-busy="true"]').count(), 1);
});

test('failed reads preserve a visible prior observation and never show raw diagnostics', async t => {
    const page = await open(t);
    await page.evaluate(() => Object.assign(window.ecosystemFixture.props, {
        status: 'error', error: { kind: 'network', status: null, message: 'fictional-sensitive-response' }, queriedAt: '2026-10-21T10:00:00Z',
    }));
    assert.match(await page.getByRole('alert').innerText(), /could not be reached/);
    assert.equal(await page.locator('time').getAttribute('datetime'), '2026-10-21T10:00:00Z');
    await page.getByText('Last successful result', { exact: false }).waitFor();
    assert.equal(await page.getByText('fictional-sensitive-response').count(), 0);
    assert.equal(await page.getByRole('button').isEnabled(), true);
    await page.evaluate(() => Object.assign(window.ecosystemFixture.props, { status: 'ready', error: null }));
    await page.getByText('Loaded on request.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('alert').count(), 0);
    assert.equal(await page.getByText('Last successful result', { exact: false }).count(), 0);
});

test('permission, missing object, timeout and unknown error remain distinct', async t => {
    const page = await open(t);
    for (const [kind, text] of [
        ['forbidden', 'Access was denied.'],
        ['password_change_required', 'Change your password before continuing.'],
        ['not_found', 'The selected object was not found.'],
        ['timeout', 'The request timed out. Refresh manually to try again.'],
        ['invalid_response', 'The response could not be read safely.'],
        ['request_failed', 'The query failed. Refresh manually to try again.'],
    ]) {
        await page.evaluate(kind => Object.assign(window.ecosystemFixture.props, { status: 'error', error: { kind, status: null } }), kind);
        await page.getByRole('alert').getByText(text, { exact: true }).waitFor();
    }
});

test('unsupported capability is explicit and supported capability does not invent success', async t => {
    const page = await open(t, { capability: true });
    for (const reason of ['external_console', 'node_metadata_api_missing', 'resource_api_missing', 'public_access_api_missing', 'port_mapping_api_missing', 'navigation_api_missing']) {
        await page.evaluate(reason => { window.ecosystemFixture.props.capability = { status: 'unsupported', reason }; }, reason);
        const expected = catalogs.en.unsupported[reason];
        await page.getByText(expected, { exact: true }).waitFor();
        await page.getByText('Not supported', { exact: true }).waitFor();
        const text = await page.locator('.ecosystem-capability-notice').innerText();
        assert.ok(text.length > 30);
        assert.ok(!text.includes('web.ecosystem.'));
        assert.equal(await page.locator('.ecosystem-capability-notice button, .ecosystem-capability-notice input').count(), 0);
    }
    await page.evaluate(() => { window.ecosystemFixture.props.capability = { status: 'supported' }; });
    await page.locator('.ecosystem-capability-notice').waitFor({ state: 'detached' });
});

test('CN/EN, light/dark and 360/768/1440 layouts use real locale keys without overflow', async t => {
    for (const lang of ['cn', 'en']) {
        for (const theme of ['light', 'dark']) {
            for (const width of [360, 768, 1440]) {
                for (const capability of [false, true]) {
                    const page = await open(t, { lang, theme, width, capability });
                    if (capability) {
                        await page.getByText(lang === 'cn' ? '暂不支持' : 'Not supported', { exact: true }).waitFor();
                    } else {
                        await page.getByRole('button', { name: lang === 'cn' ? '刷新' : 'Refresh', exact: true }).waitFor();
                        await page.evaluate(() => { window.ecosystemFixture.props.queriedAt = '2026-10-21T10:00:00Z'; });
                    }
                    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
                    assert.ok(!(await page.locator('main').innerText()).includes('web.ecosystem.'));
                    assert.equal(await page.evaluate(() => document.documentElement.classList.contains('app-dark')), theme === 'dark');
                }
            }
        }
    }
});


test('ecosystem CN/EN leaf keys match and all translated values are nonempty', async t => {
    const page = await open(t);
    assert.equal(await page.getByRole('button').count(), 1);
    function flatten(value, prefix = '') {
        return Object.entries(value).flatMap(([key, item]) => typeof item === 'string'
            ? [[`${prefix}${key}`, item]] : flatten(item, `${prefix}${key}.`));
    }
    const en = flatten(catalogs.en);
    const cn = flatten(catalogs.cn);
    assert.deepEqual(en.map(([key]) => key).sort(), cn.map(([key]) => key).sort());
    assert.ok([...en, ...cn].every(([, value]) => value.trim().length > 0));
});
