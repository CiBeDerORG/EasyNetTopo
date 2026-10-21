import assert from 'node:assert/strict';
import { test } from 'node:test';
import { chmod, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, relative } from 'node:path';
import { resolveWebTestBinary } from './helpers/webTestBinary.mjs';

const executableName = process.platform === 'win32' ? 'easytier-web.exe' : 'easytier-web';
async function executable(path) {
    await mkdir(dirname(path), { recursive: true });
    // Path discovery only: no fake backend is launched by these tests.
    await writeFile(path, 'fictional executable fixture');
    await chmod(path, 0o755);
    return path;
}
async function fixture(t) {
    const root = await mkdtemp(join(tmpdir(), 'easytier binary path '));
    t.after(() => rm(root, { recursive: true, force: true }));
    const cwd = join(root, 'frontend');
    await mkdir(cwd);
    await mkdir(join(root, 'src'));
    await writeFile(join(root, 'src/main.rs'), 'fn main() {}\n');
    await writeFile(join(root, 'Cargo.toml'), '[package]\nname = "fictional-binary-path"\nversion = "0.0.0"\nedition = "2021"\n');
    const env = { ...process.env };
    delete env.WUM_TEST_WEB_BINARY;
    delete env.CARGO_TARGET_DIR;
    delete env.CARGO_BUILD_TARGET_DIR;
    delete env.CARGO_BUILD_TARGET;
    return { root, cwd, env };
}

test('explicit binary paths with spaces take precedence without requiring Cargo', async t => {
    const { root, cwd, env } = await fixture(t);
    const binary = await executable(join(root, 'custom build', executableName));
    for (const path of [binary, relative(cwd, binary)]) {
        assert.equal(resolveWebTestBinary({ cwd, env: { ...env, PATH: '', WUM_TEST_WEB_BINARY: path } }), binary);
    }
});

test('Cargo default target directory is found from the frontend build directory', async t => {
    const { root, cwd, env } = await fixture(t);
    const binary = await executable(join(root, 'target/debug', executableName));
    assert.equal(resolveWebTestBinary({ cwd, env }), binary);
});

test('Cargo absolute and relative CARGO_TARGET_DIR override a stale default binary', async t => {
    const { root, cwd, env } = await fixture(t);
    await executable(join(root, 'target/debug', executableName));
    const absoluteTarget = join(root, 'external output');
    const absoluteBinary = await executable(join(absoluteTarget, 'debug', executableName));
    assert.equal(resolveWebTestBinary({ cwd, env: { ...env, CARGO_TARGET_DIR: absoluteTarget } }), absoluteBinary);
    const relativeTarget = 'relative output';
    const relativeBinary = await executable(join(cwd, relativeTarget, 'debug', executableName));
    assert.equal(resolveWebTestBinary({ cwd, env: { ...env, CARGO_TARGET_DIR: relativeTarget } }), relativeBinary);
});

test('Cargo configuration in the frontend build directory is honored', async t => {
    const { root, cwd, env } = await fixture(t);
    await executable(join(root, 'target/debug', executableName));
    await mkdir(join(cwd, '.cargo'));
    await writeFile(join(cwd, '.cargo/config.toml'), '[build]\ntarget-dir = "configured output"\n');
    const binary = await executable(join(cwd, 'configured output/debug', executableName));
    assert.equal(resolveWebTestBinary({ cwd, env }), binary);
});

test('missing or invalid overrides fail explicitly instead of using a default binary', async t => {
    const { root, cwd, env } = await fixture(t);
    await executable(join(root, 'target/debug', executableName));
    assert.throws(() => resolveWebTestBinary({ cwd, env: { ...env, WUM_TEST_WEB_BINARY: 'missing file' } }), /missing or not executable.*missing file/);
    assert.throws(() => resolveWebTestBinary({ cwd, env: { ...env, WUM_TEST_WEB_BINARY: '' } }), /nonempty executable path/);
    assert.throws(() => resolveWebTestBinary({ cwd, env: { ...env, WUM_TEST_WEB_BINARY: root } }), /missing or not executable/);
    assert.throws(() => resolveWebTestBinary({ cwd, env: { ...env, CARGO_TARGET_DIR: join(root, 'missing target') } }), /missing or not executable.*missing target/);
});

test('Cargo discovery failure explains how to select an executable', async t => {
    const { cwd, env } = await fixture(t);
    assert.throws(() => resolveWebTestBinary({ cwd, env: { ...env, PATH: '' } }), /cargo metadata failed.*WUM_TEST_WEB_BINARY/);
});
