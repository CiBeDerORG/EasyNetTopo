import { spawnSync } from 'node:child_process';
import { access } from 'node:fs/promises';
import { frontendRoot } from './browserHarness.mjs';
import { resolve } from 'node:path';
// Explicit registration prevents newly named suites from silently disappearing.
const suites = {
    support: 'tests/ecosystem-support/harness.test.mjs',
    shared: 'tests/ecosystem-shared.test.mjs',
    topology: 'tests/ecosystem-topology.test.mjs',
    nodes: 'tests/ecosystem-nodes.test.mjs',
    resources: 'tests/ecosystem-resources.test.mjs',
    access: 'tests/ecosystem-access.test.mjs',
    navigation: 'tests/ecosystem-navigation.test.mjs',
    api: 'tests/ecosystem-api.test.mjs',
    composition: 'tests/ecosystem-composition.test.mjs',
    host: 'tests/ecosystem-host.test.mjs',
    integration: 'tests/ecosystem-integration.test.mjs',
};
const requested = process.argv.slice(2);
const selected = requested.length ? requested : ['support'];
const names = selected.includes('all') ? Object.keys(suites) : [...new Set(selected)];
try {
    for (const name of names) {
        if (!Object.hasOwn(suites, name)) throw new Error(`Unknown suite ${name}. Choose: ${Object.keys(suites).join(', ')}, all.`);
        await access(resolve(frontendRoot, suites[name])).catch(() => { throw new Error(`Suite ${name} is not delivered: ${suites[name]}. No tests were run.`); });
    }
    console.log(`Ecosystem suites: ${names.join(', ')}. Development fixtures are not real API acceptance.`);
    const result = spawnSync(process.execPath, ['--test', '--test-concurrency=1', ...names.map(name => suites[name])], { cwd: frontendRoot, stdio: 'inherit', env: process.env });
    if (result.error) throw result.error;
    process.exitCode = result.status ?? 1;
} catch (error) {
    console.error(error.message);
    process.exitCode = 1;
}
