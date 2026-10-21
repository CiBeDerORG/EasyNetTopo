import assert from 'node:assert/strict';
import { before, after, test } from 'node:test';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
// Real Axios and HTTP, fixture responses. This is not real Web/database acceptance.
const require = createRequire(import.meta.url);
const { build } = createRequire(require.resolve('vite/package.json'))('esbuild');
const root = fileURLToPath(new URL('../', import.meta.url));
const bundle = await build({
    stdin: { contents: "export {ApiClient} from './src/modules/api'; export * from './src/modules/ecosystemApi';", resolveDir: root, loader: 'ts' },
    bundle: true, write: false, platform: 'node', format: 'esm',
    banner: { js: "import {createRequire as bundleRequire} from 'node:module';const require=bundleRequire(process.cwd()+'/package.json');" },
    plugins: [{ name: 'network-ui-boundary', setup(build) {
                build.onResolve({ filter: /^easytier-frontend-lib$/ }, () => ({ path: 'network-ui', namespace: 'test' }));
                build.onLoad({ filter: /.*/, namespace: 'test' }, () => ({
                    contents: `import * as Utils from ${JSON.stringify(root + '../frontend-lib/src/modules/utils.ts')};export {Utils};export const NetworkTypes={};`, resolveDir: root,
                }));
            } }],
});
const { ApiClient, createEcosystemReader, toEcosystemQueryError } = await import(`data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].text).toString('base64')}`);
let server, base, responses, requests, expired;
const machineId = '00000001-0000-0002-0000-000300000004';
const uuid = { part1: 1, part2: 2, part3: 3, part4: 4 };
const network = { network_id: 'network-one', display_name: 'Fixture network', network_name: 'fixture', member_count: 1, online_member_count: 0 };
const secret = 'fictional-sensitive-marker';
before(async () => {
    server = createServer(async (req, res) => {
        requests.push({ method: req.method, path: req.url });
        const selected = responses[req.url];
        const reply = typeof selected === 'function' ? await selected() : selected;
        if (reply?.drop) {
            req.socket.destroy();
            return;
        }
        res.writeHead(reply?.status ?? 200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(reply && 'body' in reply ? reply.body : {}));
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    base = `http://127.0.0.1:${server.address().port}`;
});
after(() => new Promise(resolve => server.close(resolve)));
function fixture(options) {
    requests = [];
    expired = 0;
    // Field shapes read from ListMachineItem/HeartbeatRequest, CentralNetworkSummary/member DTOs.
    responses = {
        '/api/v1/console-info': { body: { webhook_auth: false, username: 'fixture-user' } },
        '/api/v1/machines': { body: { machines: [{ info: { machine_id: uuid, hostname: 'Fixture host', running_network_instances: [uuid], user_token: secret, device_os: { os_type: 'Linux', version: 'fixture', distribution: 'sample' } }, online: false, alias: 'Fixture alias', location: { country: 'Sample' }, networks: [network], client_url: secret }] } },
        '/api/v1/networks': { body: { networks: [{ ...network, network_secret: secret }] } },
        '/api/v1/networks/network-one/members': { body: { members: [{ member_id: 'member-one', device_id: machineId, hostname: 'Fixture host', running: false, credential_secret: secret }], temporary_peers: [{ peer_id: 7, hostname: 'Fixture peer', credential_secret: secret }] } },
    };
    const api = new ApiClient(base, () => expired++);
    return { api, reader: createEcosystemReader(api, 42, options) };
}
async function rejectsKind(operation, kind, status = null) {
    await assert.rejects(operation, error => {
        assert.deepEqual(toEcosystemQueryError(error), { kind, status });
        assert.equal(JSON.stringify(error).includes(secret), false);
        assert.equal(String(error).includes(secret), false);
        assert.equal('response' in error, false);
        assert.equal('cause' in error, false);
        return true;
    });
}
test('central exact paths, real UUID utility, scoped identity, unknown values and secret omission', async () => {
    const { reader } = fixture();
    const overview = await reader.loadOverview();
    assert.deepEqual(requests.map(x => x.path), ['/api/v1/console-info', '/api/v1/machines', '/api/v1/networks']);
    const node = overview.nodes[0];
    assert.deepEqual(node.ref, { userId: 42, machineId });
    assert.deepEqual(node.runningInstanceIds, [machineId]);
    assert.equal(node.connection, 'offline');
    assert.equal(node.version, null);
    assert.equal(node.geoLocation.city, null);
    assert.equal(node.os.type, 'Linux');
    assert.equal(overview.networks[0].onlineMemberCount, 0);
    assert.equal(overview.capabilities.resources.reason, 'resource_api_missing');
    const members = await reader.loadMembers('network-one');
    assert.equal(members.members[0].connection, 'unknown');
    assert.equal(members.members[0].running, false);
    assert.equal('node' in members.temporaryPeers[0], false);
    assert.equal(JSON.stringify({ overview, members }).includes(secret), false);
    assert.ok(requests.every(x => x.method === 'GET' && !x.path.includes('user_id')));
});
test('webhook and uninitialized/guessed selections never issue central reads', async () => {
    const { reader } = fixture();
    await rejectsKind(() => reader.loadMembers('network-one'), 'request_failed');
    assert.equal(requests.length, 0);
    responses['/api/v1/console-info'].body.webhook_auth = true;
    const result = await reader.loadOverview();
    assert.equal(result.mode, 'webhook');
    assert.deepEqual(result.networks, []);
    assert.equal(result.capabilities.networks.reason, 'external_console');
    await rejectsKind(() => reader.loadMembers('network-one'), 'forbidden');
    assert.equal(requests.length, 2);
    const central = fixture().reader;
    await central.loadOverview();
    await rejectsKind(() => central.loadMembers('../credentials'), 'not_found');
    assert.equal(requests.length, 3);
});
test('missing booleans/counts remain unknown while explicit empty arrays succeed', async () => {
    const { reader } = fixture();
    delete responses['/api/v1/machines'].body.machines[0].online;
    delete responses['/api/v1/networks'].body.networks[0].member_count;
    const overview = await reader.loadOverview();
    assert.equal(overview.nodes[0].connection, 'unknown');
    assert.equal(overview.networks[0].memberCount, null);
    responses['/api/v1/machines'].body.machines = [];
    responses['/api/v1/networks'].body.networks = [];
    const empty = await reader.loadOverview();
    assert.deepEqual(empty.nodes, []);
    assert.deepEqual(empty.networks, []);
});
test('malformed arrays/IDs/booleans/counts and null heartbeat fail without inventing nodes', async () => {
    const cases = [
        () => { responses['/api/v1/console-info'].body = {}; },
        () => { responses['/api/v1/machines'].body = { machines: {} }; },
        () => { responses['/api/v1/machines'].body = null; },
        () => { responses['/api/v1/machines'].body.machines[0].info = null; },
        () => { responses['/api/v1/machines'].body.machines[0].info.machine_id = {}; },
        () => { responses['/api/v1/machines'].body.machines[0].info.machine_id = { ...uuid, part1: -1 }; },
        () => { responses['/api/v1/machines'].body.machines[0].online = 'true'; },
        () => { responses['/api/v1/networks'].body = {}; },
        () => { responses['/api/v1/networks'].body.networks[0].member_count = -1; },
        () => { responses['/api/v1/networks'].body.networks[0].network_id = '../../credentials'; },
    ];
    for (const corrupt of cases) {
        const { reader } = fixture();
        corrupt();
        await rejectsKind(() => reader.loadOverview(), 'invalid_response');
    }
});
test('members require registered UUID and safe temporary peer number', async () => {
    const { reader } = fixture();
    await reader.loadOverview();
    responses['/api/v1/networks/network-one/members'].body.members[0].device_id = 'hostname';
    await rejectsKind(() => reader.loadMembers('network-one'), 'invalid_response');
    responses['/api/v1/networks/network-one/members'].body.members = [];
    responses['/api/v1/networks/network-one/members'].body.temporary_peers[0].peer_id = Number.MAX_SAFE_INTEGER + 1;
    await rejectsKind(() => reader.loadMembers('network-one'), 'invalid_response');
});
test('safe HTTP categories preserve existing 401 callback and never retry', async () => {
    for (const [status, code, kind] of [[401, null, 'unauthenticated'], [403, 'password_change_required', 'password_change_required'], [403, null, 'forbidden'], [404, null, 'not_found'], [408, null, 'timeout'], [504, null, 'timeout'], [500, null, 'request_failed']]) {
        const { reader } = fixture();
        responses['/api/v1/console-info'] = { status, body: { code, message: secret } };
        await rejectsKind(() => reader.loadOverview(), kind, status);
        assert.equal(expired, status === 401 ? 1 : 0);
        assert.equal(requests.length, 1);
    }
    const { reader } = fixture();
    responses['/api/v1/console-info'] = { drop: true };
    await rejectsKind(() => reader.loadOverview(), 'network');
    assert.equal(requests.length, 1);
});
test('actual Axios timeout ends read with no retry', async () => {
    const { reader } = fixture({ timeoutMs: 25 });
    responses['/api/v1/console-info'] = async () => { await new Promise(r => setTimeout(r, 100)); return { body: { webhook_auth: false } }; };
    await rejectsKind(() => reader.loadOverview(), 'timeout');
    await new Promise(r => setTimeout(r, 130));
    assert.equal(requests.length, 1);
});
test('late overview cannot overwrite new mode; concurrent member snapshots retain network identity', async () => {
    const { reader } = fixture();
    let release, firstEntered;
    const entered = new Promise(r => firstEntered = r);
    let calls = 0;
    responses['/api/v1/console-info'] = async () => { if (++calls === 1) {
        firstEntered();
        await new Promise(r => release = r);
        return { body: { webhook_auth: false } };
    } return { body: { webhook_auth: true } }; };
    const older = reader.loadOverview();
    await entered;
    assert.equal((await reader.loadOverview()).mode, 'webhook');
    release();
    await rejectsKind(() => older, 'request_failed');
    assert.equal(requests.some(request => request.path === '/api/v1/networks'), false);
    await rejectsKind(() => reader.loadMembers('network-one'), 'forbidden');
    const central = fixture().reader;
    responses['/api/v1/networks'].body.networks.push({ ...network, network_id: 'network-two' });
    responses['/api/v1/networks/network-two/members'] = { body: { members: [], temporary_peers: [] } };
    await central.loadOverview();
    const [one, two] = await Promise.all([central.loadMembers('network-one'), central.loadMembers('network-two')]);
    assert.equal(one.networkId, 'network-one');
    assert.equal(two.networkId, 'network-two');
});
test('existing no-options consumers remain compatible and factory input is validated', async () => {
    const { api } = fixture();
    assert.equal((await api.get_console_info()).webhook_auth, false);
    assert.equal((await api.list_machines()).length, 1);
    assert.equal((await api.list_networks()).length, 1);
    assert.equal((await api.list_network_members('network-one')).members.length, 1);
    const before = requests.length;
    assert.throws(() => createEcosystemReader(api, 0));
    assert.throws(() => createEcosystemReader(api, 42, { timeoutMs: Infinity }));
    assert.equal(requests.length, before);
});
