import { pathToFileURL, fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
const { chromium } = await import(pathToFileURL(resolve('easytier-web/frontend/node_modules/playwright/index.mjs')).href);
const baseUrl = process.env.WUI_EVIDENCE_BASE;
const username = process.env.WUI_EVIDENCE_USERNAME;
const password = process.env.WUI_EVIDENCE_PASSWORD;
if (!baseUrl || !username || !password)
    throw new Error('Set isolated WUI_EVIDENCE_BASE, WUI_EVIDENCE_USERNAME and WUI_EVIDENCE_PASSWORD.');
const evidenceDirectory = fileURLToPath(new URL('../', import.meta.url));
import { mkdir, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
const base = baseUrl;
const out = evidenceDirectory;
await mkdir(out, { recursive: true });
const browser = await chromium.launch({ headless: true, executablePath: '/usr/bin/chromium' });
const admin = await browser.newContext({ baseURL: base });
const headers = { 'X-Requested-With': 'EasyTier' };
async function api(ctx, path, method = 'GET', data) { const r = await ctx.request.fetch('/api/v1' + path, { method, data, headers }); if (!r.ok())
    throw new Error(`${method} ${path}: ${r.status()}`); return r.status() === 204 ? null : r.json(); }
await api(admin, '/auth/login', 'POST', { username, password: createHash('md5').update(password).digest('hex') });
const networks = await api(admin, '/networks');
let network = networks.networks.find(n => n.display_name.startsWith('fictional-wui'));
if (!network)
    network = await api(admin, '/networks', 'POST', { settings: { display_name: 'fictional-wui-long-network-name-for-responsive-layout', network_name: null, networking_method: 'Standalone', public_server_url: null, peer_urls: [], virtual_cidr: null, secure_mode: false } });
const matrix = [], faults = [];
const routes = [['login', '/auth'], ['register', '/auth/register'], ['password', '/h/password'], ['users', '/h/users'], ['dashboard', '/h'], ['devices', '/h/deviceList'], ['networks', '/h/networks'], ['network-detail', '/h/networks/' + network.network_id]];
try {
    for (const lang of ['en', 'cn'])
        for (const theme of ['light', 'dark'])
            for (const width of [360, 768, 1440]) {
                const ctx = await browser.newContext({ baseURL: base, viewport: { width, height: 900 }, storageState: await admin.storageState() });
                await ctx.addInitScript(({ lang, theme }) => { localStorage.setItem('lang', lang); localStorage.setItem('console-theme-mode', theme); }, { lang, theme });
                const p = await ctx.newPage();
                p.setDefaultTimeout(5000);
                const errors = [];
                p.on('pageerror', e => errors.push(e.message));
                for (const [name, path] of routes) {
                    const start = errors.length;
                    await p.goto('/#' + path);
                    await p.waitForTimeout(500);
                    const visible = await p.locator('body').innerText();
                    const measure = await p.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth, height: innerHeight, headings: [...document.querySelectorAll('h1,h2')].map(x => x.textContent.trim()), inputCount: document.querySelectorAll('input').length }));
                    const screenshot = (width === 360 && theme === 'dark' && lang === 'cn') || (width === 1440 && theme === 'light' && lang === 'en');
                    if (screenshot)
                        await p.screenshot({ path: `${out}/${name}-${lang}-${theme}-${width}.png`, fullPage: true, animations: 'disabled' });
                    matrix.push({ page: name, path, lang, theme, width, ...measure, empty: visible.trim().length === 0, errorCount: errors.length - start, errors: errors.slice(start), screenshot: screenshot ? `${name}-${lang}-${theme}-${width}.png` : null });
                }
                await ctx.close();
            }
    await writeFile(`${out}/page-matrix.json`, JSON.stringify({ baseline: '1e92296b17fc2e85dcd4521b6012232e11abed95', mode: 'real isolated Web API via Vite; no mocked responses; no Core/VPN/device', matrix }, null, 2) + '\n');
    console.log(JSON.stringify({ entries: matrix.length, overflow: matrix.filter(r => r.scrollWidth > r.width).map(r => ({ page: r.page, lang: r.lang, theme: r.theme, width: r.width, scrollWidth: r.scrollWidth })), errors: matrix.filter(r => r.errorCount).map(r => ({ page: r.page, errors: r.errors })), networkId: network.network_id }));
}
finally {
    await admin.close();
    await browser.close();
}
