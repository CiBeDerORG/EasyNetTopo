import type {
    EcosystemCapabilities, EcosystemMembers, EcosystemNode, EcosystemOverview,
    EcosystemQueryState, EcosystemTopologyProps,
    EcosystemNodeDetailsProps, EcosystemResourceDetailsProps, EcosystemAccessSettingsProps,
    EcosystemNavigationProps, EcosystemQueryNoticeProps, EcosystemCapabilityNoticeProps,
} from '../../src/modules/ecosystemTypes';

export const fixtureScenarios = ['central', 'webhook', 'empty', 'unknown', 'loading', 'error', 'idle'] as const;
export type FixtureScenario = typeof fixtureScenarios[number];
const queriedAt = '2026-10-21T00:00:00.000Z';

/** Development samples only: no wire DTOs, credentials, external URLs or real devices. */
export function createEcosystemFixture(scenario: FixtureScenario = 'central') {
    const mode = scenario === 'webhook' ? 'webhook' : 'central';
    const capabilities: EcosystemCapabilities = {
        networks: mode === 'central' ? { status: 'supported' } : { status: 'unsupported', reason: 'external_console' },
        nodeMetadata: { status: 'unsupported', reason: 'node_metadata_api_missing' },
        resources: { status: 'unsupported', reason: 'resource_api_missing' },
        publicAccess: { status: 'unsupported', reason: 'public_access_api_missing' },
        portMappings: { status: 'unsupported', reason: 'port_mapping_api_missing' },
        navigation: { status: 'unsupported', reason: 'navigation_api_missing' },
    };
    const network = { networkId: 'fixture-network', displayName: 'Sample membership network', networkName: 'fixture-network', memberCount: 1, onlineMemberCount: null };
    const node: EcosystemNode = {
        ref: { userId: 42, machineId: '00000000-0000-4000-8000-000000000042' },
        hostname: scenario === 'unknown' ? null : 'Sample host', alias: null,
        connection: scenario === 'unknown' ? 'unknown' : 'online', lastSeenAt: null, version: null,
        os: null, geoLocation: null, networks: mode === 'central' ? [network] : [], runningInstanceIds: [],
    };
    const overview: EcosystemOverview = {
        userId: 42, mode, capabilities,
        nodes: scenario === 'empty' ? [] : [node],
        networks: mode === 'central' && scenario !== 'empty' ? [network] : [],
    };
    const members: EcosystemMembers = {
        networkId: network.networkId,
        members: scenario === 'empty' || mode === 'webhook' ? [] : [{
            networkId: network.networkId, memberId: 'fixture-member', node: node.ref,
            hostname: node.hostname, connection: node.connection, running: null,
            virtualIpv4: null, runtimeVirtualIpv4: null,
        }],
        temporaryPeers: scenario === 'central' ? [{ networkId: network.networkId, peerId: 7, hostname: 'Sample temporary peer', ipv4: null }] : [],
    };
    function query<T>(data: T): EcosystemQueryState<T> {
        const snapshot = { data, queriedAt };
        if (scenario === 'idle') return { status: 'idle' };
        if (scenario === 'loading') return { status: 'loading', previous: snapshot };
        if (scenario === 'error') return { status: 'error', error: { kind: 'network', status: null }, previous: snapshot };
        return { status: 'ready', snapshot };
    }
    const selectedNode = scenario === 'empty' ? null : node;
    const topology: EcosystemTopologyProps = {
        overview: query(overview), members: mode === 'webhook' ? { status: 'idle' } : query(members),
        selectedNetworkId: mode === 'central' && scenario !== 'empty' ? network.networkId : null,
        selectedNode: selectedNode?.ref ?? null,
    };
    return {
        topology,
        nodeDetails: { node: selectedNode, capabilities },
        resourceDetails: { node: selectedNode, kind: 'service' as const, capability: capabilities.resources },
        access: { publicAccess: capabilities.publicAccess, portMappings: capabilities.portMappings, navigation: capabilities.navigation },
        navigation: { node: selectedNode, capability: capabilities.navigation },
        queryNotice: { status: topology.overview.status, error: scenario === 'error' ? { kind: 'network' as const, status: null } : null, queriedAt: scenario === 'idle' ? null : queriedAt },
        capabilityNotice: { capability: capabilities.resources },
    } satisfies {
        topology: EcosystemTopologyProps;
        nodeDetails: EcosystemNodeDetailsProps;
        resourceDetails: EcosystemResourceDetailsProps;
        access: EcosystemAccessSettingsProps;
        navigation: EcosystemNavigationProps;
        queryNotice: EcosystemQueryNoticeProps;
        capabilityNotice: EcosystemCapabilityNoticeProps;
    };
}
