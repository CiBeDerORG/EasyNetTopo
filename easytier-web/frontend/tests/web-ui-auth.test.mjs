// Actual production authentication pages with mock HTTP, not real identity-provider validation.
import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { startWebUiBrowser } from './web-ui-support/webUiBrowser.mjs';

let app;
before(async () => { app = await startWebUiBrowser(); });
after(async () => { await app?.close(); });

async function authPage(t, path, respond) {
    const context = await app.browser.newContext({ baseURL: app.baseURL, viewport: { width: 360, height: 900 } });
    t.after(() => context.close());
    await context.addInitScript(() => localStorage.setItem('lang', 'en'));
    await context.route('**/api_meta.js', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
    await context.route('**/api/v1/**', route => respond(route, new URL(route.request().url()).pathname.replace('/api/v1', '')));
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    await page.goto(`/#${path}`);
    return page;
}

test('registration stays closed after a policy failure until explicit retry succeeds', async t => {
    let policyRequests = 0;
    const page = await authPage(t, '/auth/register', (route, path) => {
        if (path === '/auth/policy') {
            policyRequests++;
            return policyRequests === 1
                ? route.fulfill({ status: 503, json: { code: 'internal_error' } })
                : route.fulfill({ json: { registration_enabled: true, oidc_enabled: false } });
        }
        return route.fulfill({ status: 404, json: {} });
    });
    const retry = page.getByRole('button', { name: 'Retry', exact: true });
    await retry.waitFor();
    assert.equal(await page.locator('#register-username').count(), 0);
    assert.equal(policyRequests, 1);
    await retry.focus();
    await page.keyboard.press('Enter');
    await page.locator('#register-username').waitFor();
    assert.equal(policyRequests, 2);
    assert.equal(await page.locator('#register-password').count(), 1);
});

test('required password bounds and pending guard prevent invalid or duplicate writes', async t => {
    let writes = 0;
    let release;
    const pending = new Promise(resolve => { release = resolve; });
    t.after(() => release());
    const page = await authPage(t, '/h/password', async (route, path) => {
        if (path === '/auth/me') return route.fulfill({ json: {
            id: 1, username: 'fixture-required', role: 'user', capabilities: [],
            source: 'local', local_password_enabled: true, must_change_password: true, web_login_enabled: true,
        } });
        if (path === '/auth/password') {
            writes++;
            await pending;
            return route.fulfill({ status: 403, json: { code: 'local_password_unavailable' } });
        }
        return route.fulfill({ status: 404, json: {} });
    });
    const input = page.getByLabel('Password', { exact: true });
    const submit = page.getByRole('button', { name: 'Change Password', exact: true });
    await input.waitFor();
    for (const length of [0, 11, 129]) {
        await input.fill('x'.repeat(length));
        assert.equal(await submit.isDisabled(), true);
    }
    await input.fill('x'.repeat(128));
    assert.equal(await submit.isEnabled(), true);
    await input.fill('Fixture-password-123');
    const requested = page.waitForRequest(request => request.url().endsWith('/auth/password'));
    await input.press('Enter');
    await requested;
    assert.equal(await submit.isDisabled(), true);
    assert.equal(await input.isDisabled(), true);
    await page.locator('form').dispatchEvent('submit');
    assert.equal(writes, 1);
    release();
    await input.waitFor({ state: 'detached' });
    await page.getByRole('alert').filter({ hasText: 'Local password changes are unavailable' }).waitFor();
    assert.equal(writes, 1);
    assert.equal(await page.locator('form').count(), 0);
});
