import { fileURLToPath } from 'node:url';
import { preview } from 'vite';
import { chromium } from 'playwright';

/** Own a production preview and browser; callers own isolated contexts and API fixtures. */
export async function startWebUiBrowser() {
    const server = await preview({
        root: fileURLToPath(new URL('../../', import.meta.url)),
        preview: { host: '127.0.0.1', port: 0 },
    });
    const closeServer = () => new Promise((resolve, reject) => {
        server.httpServer.close(error => error ? reject(error) : resolve());
    });
    let browser;
    try {
        browser = await chromium.launch({
            headless: true,
            executablePath: process.env.WEB_TEST_CHROMIUM || '/usr/bin/chromium',
        });
    } catch (error) {
        await closeServer();
        throw error;
    }
    return {
        browser,
        baseURL: `http://127.0.0.1:${server.httpServer.address().port}`,
        async close() {
            try { await browser.close(); }
            finally { await closeServer(); }
        },
    };
}
