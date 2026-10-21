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
const c = await b.newContext({ baseURL: baseUrl, viewport: { width: 360, height: 900 }, permissions: ['clipboard-read', 'clipboard-write'] });
await c.addInitScript(() => { localStorage.setItem('lang', 'cn'); localStorage.setItem('console-theme-mode', 'dark'); });
await c.request.post('/api/v1/auth/login', { data: { username, password: createHash('md5').update(password).digest('hex') }, headers: { 'X-Requested-With': 'EasyTier' } });
const p = await c.newPage();
const results = {};
try {
    await p.goto('/#/h/users');
    await p.locator('.user-management tbody tr').last().waitFor();
    results.userTable = await p.evaluate(() => ({ rows: [...document.querySelectorAll('.user-management tbody tr')].map(x => ({ height: x.getBoundingClientRect().height, usernameCellWidth: x.cells[0].getBoundingClientRect().width, usernameLength: x.cells[0].textContent.length })), containerWidth: document.querySelector('.table-scroll').clientWidth, tableWidth: document.querySelector('.table-scroll table').getBoundingClientRect().width }));
    await p.goto('/#/h/networks/' + process.env.WUI_EVIDENCE_NETWORK_ID);
    await p.getByRole('heading', { name: 'fictional-wui-long-network-name-for-responsive-layout', exact: true }).waitFor();
    results.longTitle = await p.evaluate(() => ({ topbarHeight: document.querySelector('.console-topbar').getBoundingClientRect().height, menuButtonWidth: document.querySelector('.mobile-nav-toggle').getBoundingClientRect().width, titleWidth: document.querySelector('.console-topbar>div>span:last-child').getBoundingClientRect().width }));
    await p.goto('/#/h/deviceList');
    await p.locator('.console-empty-state code').waitFor();
    const expected = await p.locator('.console-empty-state code').innerText();
    await p.locator('.console-empty-state button').click();
    results.enrollmentCopy = { matchesRendered: await p.evaluate(() => navigator.clipboard.readText()) === expected, expectedStructure: /^easytier-core --config-server tcp:\/\/127\.0\.0\.1:\d+\/[^/]+$/.test(expected) };
    await writeFile(evidenceDirectory + 'layout-observations.json', JSON.stringify(results, null, 2) + '\n');
    console.log(JSON.stringify(results));
}
finally {
    await c.close();
    await b.close();
}
