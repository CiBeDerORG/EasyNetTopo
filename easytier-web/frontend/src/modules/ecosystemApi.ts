import axios from 'axios';
import { Utils } from 'easytier-frontend-lib';
import type ApiClient from './api';
import type {
    EcosystemCapabilities, EcosystemConnectionState, EcosystemMembers, EcosystemNetwork,
    EcosystemNetworkRef, EcosystemNode, EcosystemOverview, EcosystemQueryError, EcosystemReader,
} from './ecosystemTypes';

type EcosystemTransport = Pick<ApiClient, 'get_console_info' | 'list_machines' | 'list_networks' | 'list_network_members'>;
const defaultTimeoutMs = 15_000;

/** Safe error metadata only. Never retain a request, response, credentials or cause. */
export class EcosystemReadError extends Error implements EcosystemQueryError {
    constructor(public readonly kind: EcosystemQueryError['kind'], public readonly status: number | null = null) {
        super('Ecosystem query failed.');
        this.name = 'EcosystemReadError';
    }
}

export function toEcosystemQueryError(error: unknown): EcosystemQueryError {
    if (error instanceof EcosystemReadError) return { kind: error.kind, status: error.status };
    if (!axios.isAxiosError(error)) return { kind: 'request_failed', status: null };
    const status = error.response?.status ?? null;
    if (status === 401) return { kind: 'unauthenticated', status };
    if (status === 403) {
        const body: unknown = error.response?.data;
        const code = typeof body === 'object' && body !== null && 'code' in body ? body.code : null;
        return { kind: code === 'password_change_required' ? 'password_change_required' : 'forbidden', status };
    }
    if (status === 404) return { kind: 'not_found', status };
    if (status === 408 || status === 504 || error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') return { kind: 'timeout', status };
    return { kind: status === null ? 'network' : 'request_failed', status };
}

function invalidResponse(): never { throw new EcosystemReadError('invalid_response'); }
function record(value: unknown): Record<string, unknown> {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) return invalidResponse();
    return value as Record<string, unknown>;
}
function list(value: unknown): unknown[] {
    if (!Array.isArray(value)) return invalidResponse();
    return value;
}
function text(value: unknown): string | null { return typeof value === 'string' && value.trim() ? value : null; }
function identifier(value: unknown): string {
    if (typeof value !== 'string' || !/^[a-zA-Z0-9_-]+$/.test(value)) return invalidResponse();
    return value;
}
function optionalBoolean(value: unknown): boolean | null {
    if (value === undefined || value === null) return null;
    if (typeof value !== 'boolean') return invalidResponse();
    return value;
}
function count(value: unknown): number | null {
    if (value === undefined || value === null) return null;
    if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) return invalidResponse();
    return value;
}
function connection(value: unknown): EcosystemConnectionState {
    const online = optionalBoolean(value);
    return online === null ? 'unknown' : online ? 'online' : 'offline';
}
function machineId(value: unknown): string {
    if (typeof value === 'string') {
        if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)) return invalidResponse();
        return value.toLowerCase();
    }
    const uuid = record(value);
    const parts = ['part1', 'part2', 'part3', 'part4'].map(key => uuid[key]);
    if (!parts.every(part => typeof part === 'number' && Number.isInteger(part) && part >= 0 && part <= 0xffffffff)) return invalidResponse();
    return Utils.UuidToStr({ part1: parts[0] as number, part2: parts[1] as number, part3: parts[2] as number, part4: parts[3] as number });
}
function parseNetworkRef(value: unknown): EcosystemNetworkRef {
    const row = record(value);
    const displayName = text(row.display_name), networkName = text(row.network_name);
    if (displayName === null || networkName === null) return invalidResponse();
    return { networkId: identifier(row.network_id), displayName, networkName };
}
function parseNetworkSummary(value: unknown): EcosystemNetwork {
    const row = record(value);
    return { ...parseNetworkRef(row), memberCount: count(row.member_count), onlineMemberCount: count(row.online_member_count) };
}
function parseHostNode(value: unknown, userId: number): EcosystemNode {
    const row = record(value), info = record(row.info);
    const os = info.device_os == null ? null : record(info.device_os);
    const location = row.location == null ? null : record(row.location);
    return {
        ref: { userId, machineId: machineId(info.machine_id) },
        hostname: text(info.hostname), alias: text(row.alias), connection: connection(row.online),
        lastSeenAt: text(row.last_seen), version: text(info.easytier_version),
        os: os ? { type: text(os.os_type), version: text(os.version), distribution: text(os.distribution) } : null,
        geoLocation: location ? { country: text(location.country), region: text(location.region), city: text(location.city) } : null,
        networks: list(row.networks).map(parseNetworkRef),
        runningInstanceIds: list(info.running_network_instances).map(machineId),
    };
}
function capabilities(central: boolean): EcosystemCapabilities {
    return {
        networks: central ? { status: 'supported' } : { status: 'unsupported', reason: 'external_console' },
        nodeMetadata: { status: 'unsupported', reason: 'node_metadata_api_missing' },
        resources: { status: 'unsupported', reason: 'resource_api_missing' },
        publicAccess: { status: 'unsupported', reason: 'public_access_api_missing' },
        portMappings: { status: 'unsupported', reason: 'port_mapping_api_missing' },
        navigation: { status: 'unsupported', reason: 'navigation_api_missing' },
    };
}
function parseMembers(value: unknown, networkId: string, userId: number): EcosystemMembers {
    const response = record(value);
    return {
        networkId,
        members: list(response.members).map(value => {
            const row = record(value);
            return {
                networkId, memberId: identifier(row.member_id), node: { userId, machineId: machineId(row.device_id) },
                hostname: text(row.hostname_override) ?? text(row.hostname), connection: connection(row.online),
                running: optionalBoolean(row.running), virtualIpv4: text(row.virtual_ipv4), runtimeVirtualIpv4: text(row.runtime_virtual_ipv4),
            };
        }),
        temporaryPeers: list(response.temporary_peers).map(value => {
            const row = record(value), peerId = count(row.peer_id);
            if (peerId === null) return invalidResponse();
            return { networkId, peerId, hostname: text(row.hostname), ipv4: text(row.ipv4) };
        }),
    };
}

/** Read-only adapter; selection/query generations and snapshots belong to EcosystemPage. */
export function createEcosystemReader(api: EcosystemTransport, userId: number, options: { timeoutMs?: number } = {}): EcosystemReader {
    const timeout = options.timeoutMs ?? defaultTimeoutMs;
    if (!Number.isSafeInteger(userId) || userId <= 0 || !Number.isFinite(timeout) || timeout <= 0) throw new EcosystemReadError('request_failed');
    let scope: { central: boolean; networks: Set<string> } | null = null;
    let overviewGeneration = 0;
    // Legacy API methods unwrap envelopes before returning. Preserve their old
    // throwing behavior, but classify a malformed envelope safely for this reader.
    async function readDto<T>(operation: () => Promise<T>): Promise<T> {
        try { return await operation(); }
        catch (error) {
            if (error instanceof TypeError) return invalidResponse();
            throw error;
        }
    }
    async function read<T>(operation: () => Promise<T>): Promise<T> {
        try { return await operation(); }
        catch (error) { const safe = toEcosystemQueryError(error); throw new EcosystemReadError(safe.kind, safe.status); }
    }
    return {
        loadOverview: () => read(async () => {
            const generation = ++overviewGeneration;
            scope = null;
            const ensureCurrent = () => {
                if (generation !== overviewGeneration) throw new EcosystemReadError('request_failed');
            };
            const consoleInfo = record(await readDto(() => api.get_console_info({ timeout })));
            ensureCurrent();
            if (typeof consoleInfo.webhook_auth !== 'boolean') return invalidResponse();
            const central = !consoleInfo.webhook_auth;
            const nodes = list(await readDto(() => api.list_machines({ timeout }))).map(value => parseHostNode(value, userId));
            ensureCurrent();
            const networks = central ? list(await readDto(() => api.list_networks({ timeout }))).map(parseNetworkSummary) : [];
            ensureCurrent();
            scope = { central, networks: new Set(networks.map(item => item.networkId)) };
            const overview: EcosystemOverview = { userId, mode: central ? 'central' : 'webhook', capabilities: capabilities(central), nodes, networks };
            return overview;
        }),
        loadMembers: networkId => read(async () => {
            if (!scope) throw new EcosystemReadError('request_failed');
            if (!scope.central) throw new EcosystemReadError('forbidden');
            if (!scope.networks.has(networkId)) throw new EcosystemReadError('not_found');
            return parseMembers(await readDto(() => api.list_network_members(networkId, { timeout })), networkId, userId);
        }),
    };
}
