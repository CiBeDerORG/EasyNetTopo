import { pathToFileURL, fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
const { chromium } = await import(pathToFileURL(resolve('easytier-web/frontend/node_modules/playwright/index.mjs')).href);
const baseUrl = process.env.WUI_EVIDENCE_BASE;
const username = process.env.WUI_EVIDENCE_USERNAME;
const password = process.env.WUI_EVIDENCE_PASSWORD;
if (!baseUrl || !username || !password)
    throw new Error('Set isolated WUI_EVIDENCE_BASE, WUI_EVIDENCE_USERNAME and WUI_EVIDENCE_PASSWORD.');
const evidenceDirectory = fileURLToPath(new URL('../', import.meta.url));
import { writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
const out = evidenceDirectory;
const b = await chromium.launch({ headless: true, executablePath: '/usr/bin/chromium' });
const c = await b.newContext({ baseURL: baseUrl, viewport: { width: 360, height: 900 } });
await c.addInitScript(() => { localStorage.setItem('lang', 'en'); localStorage.setItem('console-theme-mode', 'light'); });
await c.request.post('/api/v1/auth/login', { data: { username, password: createHash('md5').update(password).digest('hex') }, headers: { 'X-Requested-With': 'EasyTier' } });
const p = await c.newPage();
p.setDefaultTimeout(5000);
const results = [];
async function scenario(name, fn) { try {
    results.push({ name, ...await fn() });
}
catch (e) {
    results.push({ name, status: 'incomplete', reason: e.message.split('\n')[0] });
} }
try {
    await scenario('user-create-failure-draft-duplicate-focus', async () => {
        await p.goto('/#/h/users');
        const create = p.getByRole('button', { name: 'Create user', exact: true });
        await create.click();
        const dialog = p.getByRole('dialog');
        await dialog.getByLabel('Username', { exact: true }).fill('fictional-wui-retained');
        let count = 0;
        await p.route('**/api/v1/admin/users', async (r) => { if (r.request().method() === 'POST') {
            count++;
            await new Promise(resolve => setTimeout(resolve, 400));
            await r.abort();
        }
        else
            await r.continue(); });
        const submit = dialog.getByRole('button', { name: 'Create user', exact: true });
        await submit.click();
        const busyDisabled = await submit.isDisabled();
        await dialog.getByText('The server could not be reached.', { exact: false }).waitFor();
        const draft = await dialog.getByLabel('Username', { exact: true }).inputValue();
        await p.screenshot({ path: out + '/user-create-failure-en-light-360.png', fullPage: true });
        await p.keyboard.press('Escape');
        await dialog.waitFor({ state: 'hidden' });
        const focusReturned = await create.evaluate(el => el === document.activeElement);
        await p.unroute('**/api/v1/admin/users');
        return { status: 'observed', faultInjection: 'POST aborted before backend; other API real', writeRequests: count, busyDisabled, draftRetained: draft === 'fictional-wui-retained', focusReturned };
    });
    await scenario('login-failure-draft-and-keyboard', async () => {
        await p.goto('/#/auth');
        await p.locator('#username').fill('fictional-wui-invalid');
        await p.locator('#password').fill('fictional-invalid-password');
        await p.locator('#password').press('Enter');
        await p.getByRole('alert').filter({ hasText: /Invalid username|incorrect|invalid|password/i }).first().waitFor();
        const retained = await p.locator('#username').inputValue();
        await p.locator('#password').fill('');
        await p.screenshot({ path: out + '/login-failure-en-light-360.png', fullPage: true });
        return { status: 'observed', backend: 'real isolated API', usernameRetained: retained === 'fictional-wui-invalid', keyboardSubmitted: true, error: await p.getByRole('alert').allTextContents() };
    });
    await scenario('mobile-menu-keyboard-and-focus', async () => {
        await p.goto('/#/h/users');
        await p.locator('.mobile-nav-toggle').click();
        const drawer = p.locator('.console-mobile-nav');
        await drawer.waitFor();
        await p.keyboard.press('Escape');
        await drawer.waitFor({ state: 'hidden' });
        return { status: 'observed', escapeClosed: true, focusReturned: await p.locator('.mobile-nav-toggle').evaluate(el => el === document.activeElement) };
    });
    await scenario('late-user-list-after-navigation', async () => {
        await p.goto('/#/h/users');
        await p.getByRole('heading', { name: 'User management', exact: true }).waitFor();
        let delayed = false;
        await p.route('**/api/v1/admin/users?**', async (route) => { if (!delayed) {
            delayed = true;
            const response = await route.fetch();
            await new Promise(r => setTimeout(r, 700));
            try {
                await route.fulfill({ response });
            }
            catch { }
        }
        else
            await route.continue(); });
        await p.getByRole('button', { name: 'Search username', exact: true }).click();
        await p.evaluate(() => location.hash = '#/h/password');
        await p.locator('#new-account-password').waitFor();
        await p.waitForTimeout(850);
        await p.unroute('**/api/v1/admin/users?**');
        return { status: 'observed', faultInjection: 'delayed delivery of real GET response', delayApplied: delayed, passwordStillVisible: await p.locator('#new-account-password').isVisible(), userTableAbsent: await p.locator('.user-management table').count() === 0 };
    });
    await writeFile(out + '/interaction-observations.json', JSON.stringify({ baseline: '1e92296b17fc2e85dcd4521b6012232e11abed95', results }, null, 2) + '\n');
    console.log(JSON.stringify(results));
}
finally {
    await c.close();
    await b.close();
}
