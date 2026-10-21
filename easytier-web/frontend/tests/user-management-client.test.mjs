import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

// Compile the actual client. Only unrelated network UI exports are stubbed;
// Axios, session configuration, interceptors and requests run unchanged.
const require = createRequire(import.meta.url);
const { build } = createRequire(require.resolve('vite/package.json'))('esbuild');
const bundle = await build({
    entryPoints: [fileURLToPath(new URL('../src/modules/api.ts', import.meta.url))],
    bundle: true, write: false, format: 'esm', platform: 'node',
    banner: { js: "import { createRequire as bundleRequire } from 'node:module'; const require = bundleRequire(process.cwd() + '/package.json');" },
    plugins: [{ name: 'network-ui-boundary', setup(build) {
        build.onResolve({ filter: /^easytier-frontend-lib$/ }, () => ({ path: 'network-ui', namespace: 'test' }));
        build.onLoad({ filter: /.*/, namespace: 'test' }, () => ({ contents: 'export const NetworkTypes = {}; export const Utils = {};' }));
    } }],
});
const { ApiClient } = await import(`data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].text).toString('base64')}`);
let server;
let base;
let requests = [];
let responseBody;
let responseStatus;
const user = { id: 7, username: 'test-user', role: 'user', web_login_enabled: true, source: 'unknown', local_password_enabled: null, must_change_password: false, created_at: null, last_login_at: null };

before(async () => {
    server = createServer(async (req, res) => {
        let body = '';
        for await (const chunk of req) body += chunk;
        requests.push({ method: req.method, url: req.url, headers: req.headers, body: body ? JSON.parse(body) : undefined });
        if (responseStatus === 0) { req.socket.destroy(); return; }
        res.writeHead(responseStatus, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(responseBody));
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    base = `http://127.0.0.1:${server.address().port}`;
});
after(() => new Promise(resolve => server.close(resolve)));

function fixture(body = user, status = 200) {
    requests = [];
    responseBody = body;
    responseStatus = status;
    let expired = 0;
    const api = new ApiClient(base + '/', () => expired++);
    return { api, client: api.userManagement, expired: () => expired };
}

test('all contract methods use the shared transport, exact DTOs and protected write headers', async () => {
    const { client } = fixture();
    const calls = [
        [() => client.getMe(), 'GET', '/auth/me'],
        [() => client.getPolicy(), 'GET', '/auth/policy'],
        [() => client.getUser(7), 'GET', '/admin/users/7'],
        [() => client.createUser('new.test'), 'POST', '/admin/users', { username: 'new.test' }],
        [() => client.setAccess(7, false), 'PUT', '/admin/users/7/access', { web_login_enabled: false }],
        [() => client.setRole(7, 'admin'), 'PUT', '/admin/users/7/role', { role: 'admin' }],
        [() => client.resetPassword(7), 'POST', '/admin/users/7/password-reset', {}],
        [() => client.revokeSessions(7), 'POST', '/admin/users/7/revoke-sessions', {}],
    ];
    for (const [call, method, path, body] of calls) {
        assert.deepEqual(await call(), user);
        const request = requests.at(-1);
        assert.equal(request.method, method);
        assert.equal(request.url, '/api/v1' + path);
        assert.deepEqual(request.body, body);
        if (method !== 'GET') {
            assert.equal(request.headers['x-requested-with'], 'EasyTier');
            assert.match(request.headers['content-type'], /application\/json/);
        }
    }
});

test('pagination preserves literal search, false filters, unknown values and empty results', async () => {
    const page = { items: [user], total: 1, page: 2, page_size: 10 };
    const { client } = fixture(page);
    assert.deepEqual(await client.listUsers({ search: 'a%_ +', role: 'user', web_login_enabled: false, page: 2, page_size: 10 }), page);
    const query = new URL(requests[0].url, base).searchParams;
    assert.equal(query.get('search'), 'a%_ +');
    assert.equal(query.get('web_login_enabled'), 'false');
    assert.equal(query.get('role'), 'user');
    assert.equal(query.get('page'), '2');
    assert.equal(query.get('page_size'), '10');
    responseBody = { items: [], total: 0, page: 1, page_size: 20 };
    assert.deepEqual(await client.listEvents({ target_id: 7 }), responseBody);
    const eventQuery = new URL(requests.at(-1).url, base).searchParams;
    assert.equal(eventQuery.get('target_id'), '7');
    assert.equal(eventQuery.get('page'), '1');
    assert.equal(eventQuery.get('page_size'), '20');
});

test('401, 403, conflict and network errors remain distinct without sensitive Axios context', async () => {
    for (const [status, code] of [[401, 'unauthenticated'], [403, 'password_change_required'], [403, 'forbidden'], [409, 'username_conflict'], [409, 'last_admin']]) {
        const { client, expired } = fixture({ code, message: 'DO_NOT_EXPOSE_TEST_SECRET', password: 'DO_NOT_EXPOSE_TEST_SECRET' }, status);
        await assert.rejects(client.createUser('test-user'), error => {
            assert.equal(error.status, status);
            assert.equal(error.code, code);
            assert.equal(error.response, undefined);
            assert.equal(error.config, undefined);
            assert.ok(!String(error).includes('DO_NOT_EXPOSE'));
            assert.ok(!JSON.stringify(error).includes('DO_NOT_EXPOSE'));
            return true;
        });
        assert.equal(expired(), status === 401 ? 1 : 0);
        assert.equal(requests.length, 1, 'writes must not retry');
    }
    const { client } = fixture();
    // The isolated mock server drops the connection without an HTTP response.
    responseStatus = 0;
    await assert.rejects(client.getMe(), error => error.status === null && error.code === 'network_error');
});

test('credential responses and failures never log payloads; old MD5 input stays compatible', async () => {
    const delivery = { user, temporary_password: 'fictional-temporary-password', expires_at: '2026-10-22T00:00:00Z' };
    const { api, client } = fixture(delivery, 201);
    const logs = [];
    const originals = Object.fromEntries(['log', 'debug', 'error'].map(key => [key, console[key]]));
    try {
        for (const key of Object.keys(originals)) console[key] = (...args) => logs.push(args);
        assert.deepEqual(await client.createUser('test-user'), delivery);
        assert.deepEqual(await client.resetPassword(7), delivery);
        const password = 'fictional-test-password';
        await api.login({ username: 'test-user', password });
        await api.change_password(password);
        const { createHash } = await import('node:crypto');
        const digest = createHash('md5').update(password).digest('hex');
        assert.equal(requests.at(-2).body.password, digest);
        assert.equal(requests.at(-1).body.new_password, digest);
        responseStatus = 403;
        responseBody = { code: 'forbidden', message: delivery.temporary_password };
        await assert.rejects(client.resetPassword(7));
        assert.deepEqual(logs, []);
        assert.ok(requests.every(request => !request.url.includes(delivery.temporary_password)));
    } finally {
        Object.assign(console, originals);
    }
});

test('current identity exposes the compatibility warning without altering unknown account fields', async () => {
    const me = { ...user, capabilities: ['users.manage'], default_password_warning: true };
    const { client } = fixture(me);
    assert.deepEqual(await client.getMe(), me);
});


test('login and registration preserve safe error codes and retryable unmodified drafts', async () => {
    const credentials = { username: 'test-account', password: 'fictional-password-for-retry' };
    const registration = { credentials: { ...credentials }, captcha: 'test-captcha' };
    const { createHash } = await import('node:crypto');
    const digest = createHash('md5').update(credentials.password).digest('hex');
    for (const [status, code] of [[401, 'invalid_credentials'], [403, 'forbidden'], [409, 'username_conflict'], [0, 'network_error']]) {
        const { api } = fixture({ message: 'DO_NOT_EXPOSE_TEST_SECRET' }, status);
        for (const operation of ['login', 'register']) {
            const input = operation === 'login' ? credentials : registration;
            const result = await api[operation](input);
            assert.equal(result.success, false);
            assert.equal(result.status, status || null);
            assert.equal(result.code, operation === 'register' && status === 401 ? 'unauthenticated' : status === 409 ? 'request_failed' : code);
            assert.ok(!result.message.includes('DO_NOT_EXPOSE'));
            await api[operation](input);
            const submitted = requests.at(-1).body;
            assert.equal(operation === 'login' ? submitted.password : submitted.credentials.password, digest);
        }
    }
    assert.equal(registration.credentials.password, credentials.password);
    const { api } = fixture({ code: 'username_conflict', message: 'DO_NOT_EXPOSE_TEST_SECRET' }, 409);
    assert.equal((await api.register(registration)).code, 'username_conflict');
});
