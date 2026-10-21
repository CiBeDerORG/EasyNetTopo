import { spawnSync } from 'node:child_process';
import { accessSync, constants, statSync } from 'node:fs';
import { isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendDirectory = fileURLToPath(new URL('../../', import.meta.url));
const buildHint = 'Run cargo build --locked -p easytier-web --features embed from easytier-web/frontend, or set WUM_TEST_WEB_BINARY to the intended executable.';

/** Resolve from the same working directory as the package's Cargo build. */
export function resolveWebTestBinary({ cwd = frontendDirectory, env = process.env } = {}) {
    let binary;
    if (env.WUM_TEST_WEB_BINARY !== undefined) {
        if (!env.WUM_TEST_WEB_BINARY.trim()) throw new Error('WUM_TEST_WEB_BINARY must be a nonempty executable path.');
        binary = resolve(cwd, env.WUM_TEST_WEB_BINARY);
    } else {
        // Cargo owns target-dir resolution, including relative environment paths
        // and .cargo/config.toml. Do not fall back to a possibly stale target/.
        const metadata = spawnSync('cargo', ['metadata', '--format-version', '1', '--no-deps', '--locked'], {
            cwd, env, encoding: 'utf8', timeout: 30000, maxBuffer: 16 * 1024 * 1024,
        });
        if (metadata.error || metadata.status !== 0) {
            const reason = metadata.error?.code ?? metadata.signal ?? `exit ${metadata.status}`;
            throw new Error(`Cannot resolve the E2E backend: cargo metadata failed (${reason}) in ${cwd}. ${buildHint}`);
        }
        let targetDirectory;
        try { targetDirectory = JSON.parse(metadata.stdout).target_directory; }
        catch { throw new Error(`Cannot resolve the E2E backend: cargo metadata returned invalid JSON. ${buildHint}`); }
        if (typeof targetDirectory !== 'string' || !isAbsolute(targetDirectory)) {
            throw new Error(`Cannot resolve the E2E backend: cargo metadata did not return an absolute target_directory. ${buildHint}`);
        }
        // Package scripts build the host debug profile. Other profiles/targets
        // can use an explicit WUM_TEST_WEB_BINARY rather than guessing a path.
        binary = join(targetDirectory, 'debug', process.platform === 'win32' ? 'easytier-web.exe' : 'easytier-web');
    }
    try {
        if (!statSync(binary).isFile()) throw new Error('not a file');
        accessSync(binary, process.platform === 'win32' ? constants.F_OK : constants.X_OK);
    } catch {
        throw new Error(`The E2E backend is missing or not executable at ${binary}. ${buildHint}`);
    }
    return binary;
}
