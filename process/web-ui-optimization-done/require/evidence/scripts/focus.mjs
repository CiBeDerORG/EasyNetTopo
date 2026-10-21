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
const b = await chromium.launch({ headless: true, executablePath: '/usr/bin/chromium' });
const c = await b.newContext({ baseURL: baseUrl, viewport: { width: 360, height: 900 } });
await c.addInitScript(() => localStorage.setItem('lang', 'en'));
await c.request.post('/api/v1/auth/login', { data: { username, password: createHash('md5').update(password).digest('hex') }, headers: { 'X-Requested-With': 'EasyTier' } });
const p = await c.newPage();
const rows = [];
try {
    await p.goto('/#/h/users');
    const trigger = p.locator('.mobile-nav-toggle');
    await trigger.waitFor();
    for (let i = 0; i < 3; i++) {
        await trigger.focus();
        await p.keyboard.press('Enter');
        await p.locator('.console-mobile-nav').waitFor();
        await p.keyboard.press('Escape');
        await p.locator('.console-mobile-nav').waitFor({ state: 'hidden' });
        await p.waitForTimeout(500);
        rows.push(await p.evaluate(() => ({ tag: document.activeElement.tagName, ariaLabel: document.activeElement.getAttribute('aria-label'), className: document.activeElement.className, returned: document.activeElement === document.querySelector('.mobile-nav-toggle') })));
    }
    await writeFile(evidenceDirectory + 'mobile-focus-observations.json', JSON.stringify({ viewport: 360, mode: 'real isolated API; keyboard Enter then Escape; 500ms after hidden', rows }, null, 2) + '\n');
    console.log(JSON.stringify(rows));
}
finally {
    await c.close();
    await b.close();
}
