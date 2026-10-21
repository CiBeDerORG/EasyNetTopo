import { mkdtemp, writeFile, rm, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { frontendRoot } from './browserHarness.mjs';
const inputs = process.argv.slice(2);
if (!inputs.length) inputs.push('tests/ecosystem-support/HarnessProbe.vue');
const directory = await mkdtemp(join(tmpdir(), 'easytier-ecosystem-types-'));
try {
    for (const input of inputs) {
        if (!/^(src\/|tests\/ecosystem-support\/)[A-Za-z0-9_/-]+\.(vue|ts)$/.test(input)) throw new Error(`Invalid component/type input: ${input}`);
        await access(resolve(frontendRoot, input));
    }
    const config = join(directory, 'tsconfig.json');
    await writeFile(config, JSON.stringify({
        extends: resolve(frontendRoot, 'tsconfig.app.json'),
        compilerOptions: { incremental: false, composite: false, tsBuildInfoFile: undefined, noEmit: true },
        include: inputs.map(input => resolve(frontendRoot, input)).concat(resolve(frontendRoot, 'tests/ecosystem-support/fixtures.ts')),
        exclude: [],
    }));
    const result = spawnSync(process.execPath, [resolve(frontendRoot, 'node_modules/vue-tsc/bin/vue-tsc.js'), '--project', config, '--noEmit'], { cwd: frontendRoot, stdio: 'inherit' });
    if (result.error) throw result.error;
    process.exitCode = result.status ?? 1;
} finally {
    await rm(directory, { recursive: true, force: true });
}
