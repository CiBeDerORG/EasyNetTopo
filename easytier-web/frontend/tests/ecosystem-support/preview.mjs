import { createEcosystemPreview } from './browserHarness.mjs';
const [componentPath, fixtureKey] = process.argv.slice(2);
const preview = await createEcosystemPreview({ componentPath, fixtureKey });
console.log(`Development fixture only: ${preview.url}?scenario=central&lang=en&theme=light`);
console.log('Scenarios: central, webhook, empty, unknown, loading, error, idle. Stop with Ctrl+C.');
let closing = false;
async function close() {
    if (closing) return;
    closing = true;
    await preview.close();
}
process.once('SIGINT', close);
process.once('SIGTERM', close);
