// Real production components and CSS; HTTP responses are explicit test fixtures.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './webUiBrowser.mjs';

let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });

const networkId = '00000000-0000-4000-8000-000000000103';
const networkName = 'fictional-web-ui-long-network-name-for-responsive-layout';
const network = {
    network_id: networkId, display_name: networkName, network_name: 'fixture',
    networking_method: 'standalone', member_count: 0, online_member_count: 0,
};

async function openNetwork(t, { width, language, theme }) {
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width, height: 900 } });
    t.after(() => context.close());
    await context.addInitScript(({ language, theme }) => {
        localStorage.setItem('lang', language);
        localStorage.setItem('console-theme-mode', theme);
    }, { language, theme });
    await context.route('**/api_meta.js', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
    await context.route('**/api/v1/**', route => {
        const path = new URL(route.request().url()).pathname.replace('/api/v1', '');
        const responses = {
            '/auth/me': { id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'],
                local_password_enabled: true, source: 'local', must_change_password: false, web_login_enabled: true },
            '/console-info': { username: 'fixture-admin', webhook_auth: false, config_server_protocol: 'tcp', config_server_port: 11010 },
            '/networks': { networks: [network] },
            '/blocked-devices': { blocked: [] },
            '/networks/gateway-info': {},
            [`/networks/${networkId}`]: network,
            [`/networks/${networkId}/members`]: { members: [] },
        };
        return route.fulfill({ status: path in responses ? 200 : 404, json: responses[path] ?? { code: 'not_found' } });
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(`/#/h/networks/${networkId}`);
    await page.getByRole('heading', { name: networkName, exact: true }).waitFor();
    return { page, errors };
}

for (const language of ['cn', 'en']) {
    for (const theme of ['light', 'dark']) {
        for (const width of [360, 768, 1440]) {
            test(`long network title preserves controls: ${language}/${theme}/${width}`, async t => {
                const { page, errors } = await openNetwork(t, { width, language, theme });
                const topbar = page.locator('.console-topbar');
                const metrics = await topbar.evaluate(element => {
                    const buttons = [...element.querySelectorAll('button')].filter(button => button.getClientRects().length);
                    return { height: element.getBoundingClientRect().height,
                        widths: buttons.map(button => button.getBoundingClientRect().width),
                        overflow: document.documentElement.scrollWidth > innerWidth,
                        title: element.querySelector('.console-page-title').textContent };
                });
                assert.equal(await page.locator('.console-page-title').getAttribute('title'), networkName);
                assert.equal(metrics.title, networkName, 'full title remains available to assistive technology');
                assert.ok(metrics.height <= 80, `title must not inflate topbar: ${metrics.height}`);
                assert.ok(metrics.widths.every(value => value >= 28), `controls must not collapse: ${metrics.widths}`);
                assert.equal(metrics.overflow, false, 'page has no horizontal overflow');
                assert.equal(await page.locator('html').evaluate(element => element.classList.contains('app-dark')), theme === 'dark');
                if (width === 360) {
                    const toggle = page.getByRole('button', { name: language === 'cn' ? '导航' : 'Navigation', exact: true });
                    await toggle.click();
                    await page.locator('.console-mobile-nav').waitFor();
                    await page.keyboard.press('Escape');
                    await page.locator('.console-mobile-nav').waitFor({ state: 'hidden' });
                    await page.waitForFunction(() => document.activeElement?.classList.contains('mobile-nav-toggle'));
                }
                assert.deepEqual(errors, []);
            });
        }
    }
}
