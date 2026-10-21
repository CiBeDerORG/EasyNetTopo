import { mkdtemp, rm, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer as createPortReservation } from 'node:net';
import { createServer } from 'vite';
import vue from '@vitejs/plugin-vue';
import yaml from '@modyfi/vite-plugin-yaml';

export const frontendRoot = fileURLToPath(new URL('../../', import.meta.url));
const fixtureKeys = ['topology', 'nodeDetails', 'resourceDetails', 'access', 'navigation', 'queryNotice', 'capabilityNotice', 'composition'];

async function availableLoopbackPort() {
    const reservation = createPortReservation();
    await new Promise((resolve, reject) => {
        reservation.once('error', reject);
        reservation.listen(0, '127.0.0.1', resolve);
    });
    const port = reservation.address().port;
    await new Promise((resolve, reject) => reservation.close(error => error ? reject(error) : resolve()));
    return port;
}

/** Each preview owns its port/cache; never writes either production dist directory. */
export async function createEcosystemPreview({
    componentPath = 'tests/ecosystem-support/HarnessProbe.vue',
    fixtureKey = 'topology',
    eventNames = ['refresh', 'selectNetwork', 'selectNode'],
} = {}) {
    if (!/^(src\/components\/ecosystem\/|tests\/ecosystem-support\/)[A-Za-z0-9_/-]+\.vue$/.test(componentPath)) {
        throw new Error('Preview component must be an ecosystem component or support probe.');
    }
    if (!fixtureKeys.includes(fixtureKey) || eventNames.some(name => !/^[a-z][A-Za-z]+$/.test(name))) {
        throw new Error('Unknown fixture key or event name.');
    }
    await access(resolve(frontendRoot, componentPath));
    const cacheDirectory = await mkdtemp(join(tmpdir(), 'easytier-ecosystem-vite-'));
    const entry = `
import { createApp, h, reactive } from 'vue';
import { createI18n } from 'vue-i18n';
import PrimeVue from 'primevue/config';
import ConsoleTheme from '/src/theme.ts';
import '/src/console.css';
import '/src/style.css';
import Component from ${JSON.stringify('/' + componentPath)};
import { createEcosystemFixture, fixtureScenarios } from '/tests/ecosystem-support/fixtures.ts';
import { createCompositionFixture } from '/tests/ecosystem-support/compositionFixture.ts';
import en from '/@fs/${frontendRoot}../frontend-lib/src/locales/en.yaml';
import cn from '/@fs/${frontendRoot}../frontend-lib/src/locales/cn.yaml';
const query = new URLSearchParams(location.search);
const scenario = query.get('scenario') || 'central';
if (!fixtureScenarios.includes(scenario)) throw new Error('Unknown fixture scenario: ' + scenario);
const props = reactive(${fixtureKey === 'composition' ? 'createCompositionFixture()' : `createEcosystemFixture(scenario)[${JSON.stringify(fixtureKey)}]`});
const events = [];
const handlers = Object.fromEntries(${JSON.stringify(eventNames)}.map(name => ['on' + name[0].toUpperCase() + name.slice(1), (...args) => events.push({ name, args })]));
window.ecosystemFixture = { props, events, setProps(next) { Object.assign(props, next); }, clearEvents() { events.length = 0; } };
document.documentElement.classList.toggle('app-dark', query.get('theme') === 'dark');
const app = createApp({ render: () => h(Component, { ...props, ...handlers }) })
    .use(PrimeVue, { theme: { preset: ConsoleTheme, options: { darkModeSelector: '.app-dark' } } })
    .use(createI18n({ legacy: false, locale: query.get('lang') === 'cn' ? 'cn' : 'en', messages: { en, cn } }))
    ;
app.mount('#app');
window.ecosystemFixture.unmount = () => app.unmount();`;
    let server;
    async function close() {
        try { await server?.close(); }
        finally { await rm(cacheDirectory, { recursive: true, force: true }); }
    }
    try {
        const port = await availableLoopbackPort();
        server = await createServer({
            root: frontendRoot, configFile: false, cacheDir: cacheDirectory,
            plugins: [vue(), yaml(), {
                name: 'ecosystem-development-fixture',
                resolveId(id) { if (id === '/ecosystem-fixture.js') return id; },
                load(id) { if (id === '/ecosystem-fixture.js') return entry; },
                configureServer(vite) {
                    vite.middlewares.use((req, res, next) => {
                        if (req.url?.split('?')[0] !== '/ecosystem-preview') return next();
                        res.setHeader('Content-Type', 'text/html; charset=utf-8');
                        res.end('<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body><p>Development sample — not live network data</p><main id="app"></main><script type="module" src="/ecosystem-fixture.js"></script></body></html>');
                    });
                },
            }],
            server: { host: '127.0.0.1', port, strictPort: true, fs: { allow: [resolve(frontendRoot, '..')] } },
        });
        await server.listen();
        const base = `http://127.0.0.1:${server.httpServer.address().port}/ecosystem-preview`;
        return {
            url: base,
            close,
        };
    } catch (error) {
        await close();
        throw error;
    }
}

/** Real browser rendering of development props. Unexpected API requests are recorded and blocked. */
export async function openEcosystemPreview(browser, preview, { scenario = 'central', language = 'en', theme = 'light', width = 1440 } = {}) {
    const context = await browser.newContext({ viewport: { width, height: 900 } });
    const page = await context.newPage();
    const requests = [], errors = [];
    await page.route('**/api/**', route => {
        requests.push({ method: route.request().method(), path: new URL(route.request().url()).pathname });
        return route.abort();
    });
    page.on('pageerror', error => errors.push(error.message));
    page.setDefaultTimeout(10000);
    try {
        await page.goto(`${preview.url}?${new URLSearchParams({ scenario, lang: language, theme })}`);
        await page.waitForFunction(() => !!window.ecosystemFixture && document.querySelector('#app')?.children.length > 0);
        return { page, context, requests, errors, close: () => context.close() };
    } catch (error) {
        await context.close();
        throw error;
    }
}
