// Production page + mock account HTTP: layout and component interactions, not authorization E2E.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';

let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });

const user = {
    id: 2, username: `fixture-${'long'.repeat(14)}`, role: 'user', web_login_enabled: true,
    source: 'local', local_password_enabled: true, must_change_password: false,
    created_at: '2026-10-21T00:00:00Z', last_login_at: null,
};

async function openUsers(t, width, language, theme) {
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
            '/auth/me': { ...user, id: 1, username: 'fixture-admin', role: 'admin', capabilities: ['users.manage'] },
            '/console-info': { username: 'fixture-admin', webhook_auth: true, config_server_protocol: 'tcp', config_server_port: 11010 },
            '/admin/users': { items: [user], total: 1, page: 1, page_size: 20 },
            '/admin/users/2': user,
            '/admin/user-events': { items: [], total: 0, page: 1, page_size: 20 },
        };
        return route.fulfill({ status: path in responses ? 200 : 404, json: responses[path] ?? { code: 'not_found' } });
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto('/#/h/users');
    await page.getByRole('cell', { name: user.username, exact: true }).waitFor();
    return { page, errors };
}

for (const language of ['cn', 'en']) {
    for (const theme of ['light', 'dark']) {
        for (const width of [360, 768, 1440]) {
            test(`user table remains readable and operable: ${language}/${theme}/${width}`, async t => {
                const { page, errors } = await openUsers(t, width, language, theme);
                const tableRegion = page.getByRole('region', { name: language === 'cn' ? '用户管理' : 'User management', exact: true });
                const metrics = await tableRegion.evaluate(element => {
                    const cell = element.querySelector('td.username');
                    return {
                        columnWidth: cell.getBoundingClientRect().width,
                        rowHeight: cell.parentElement.getBoundingClientRect().height,
                        username: cell.textContent,
                        scrollable: element.scrollWidth > element.clientWidth,
                        pageOverflow: document.documentElement.scrollWidth > innerWidth,
                    };
                });
                assert.ok(metrics.columnWidth >= 160, `username column readable: ${metrics.columnWidth}`);
                assert.ok(metrics.rowHeight < 160, `long username does not become a vertical column: ${metrics.rowHeight}`);
                assert.equal(metrics.username, user.username, 'no truncation of account identity');
                assert.equal(metrics.pageOverflow, false, 'overflow belongs to table, not whole page');
                if (width === 360) assert.equal(metrics.scrollable, true);
                if (metrics.scrollable) {
                    await tableRegion.focus();
                    await page.keyboard.press('ArrowRight');
                    await page.waitForFunction(() => document.querySelector('.table-scroll').scrollLeft > 0);
                }
                const detail = tableRegion.getByRole('button', { name: `${language === 'cn' ? '详情' : 'Details'}: ${user.username}`, exact: true });
                await detail.scrollIntoViewIfNeeded();
                await detail.click();
                await page.getByRole('heading', { name: user.username, exact: true }).waitFor();
                assert.equal(await page.locator('html').evaluate(element => element.classList.contains('app-dark')), theme === 'dark');
                assert.deepEqual(errors, []);
            });
        }
    }
}
