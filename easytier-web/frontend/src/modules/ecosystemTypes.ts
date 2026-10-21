/** ECO contract v1: Web view models, never wire DTOs or persisted resource records. */
export type EcosystemMode = 'central' | 'webhook';
export type EcosystemConnectionState = 'online' | 'offline' | 'unknown';
export type EcosystemResourceKind = 'service' | 'device_group' | 'device';

export type EcosystemUnsupportedReason =
    | 'external_console'
    | 'node_metadata_api_missing'
    | 'resource_api_missing'
    | 'public_access_api_missing'
    | 'port_mapping_api_missing'
    | 'navigation_api_missing';

/** Unsupported means no usable API; it must not be presented as denied or empty. */
export type EcosystemCapability =
    | { status: 'supported' }
    | { status: 'unsupported'; reason: EcosystemUnsupportedReason };

export interface EcosystemCapabilities {
    networks: EcosystemCapability;
    nodeMetadata: EcosystemCapability;
    resources: EcosystemCapability;
    publicAccess: EcosystemCapability;
    portMappings: EcosystemCapability;
    navigation: EcosystemCapability;
}

export type EcosystemErrorKind =
    | 'unauthenticated'
    | 'forbidden'
    | 'password_change_required'
    | 'not_found'
    | 'timeout'
    | 'network'
    | 'invalid_response'
    | 'request_failed';

/** Safe presentation metadata only: never retain Axios objects or server bodies. */
export interface EcosystemQueryError {
    kind: EcosystemErrorKind;
    status: number | null;
}

/** queriedAt is the time of this successful read, not a device health timestamp. */
export interface EcosystemSnapshot<T> {
    data: T;
    queriedAt: string;
}

export type EcosystemQueryState<T> =
    | { status: 'idle' }
    | { status: 'loading'; previous: EcosystemSnapshot<T> | null }
    | { status: 'ready'; snapshot: EcosystemSnapshot<T> }
    | { status: 'error'; error: EcosystemQueryError; previous: EcosystemSnapshot<T> | null };

/** Current browser-session tenant scope; never send this userId as an API override. */
export interface EcosystemNodeRef {
    userId: number;
    machineId: string;
}

export interface EcosystemNetworkRef {
    networkId: string;
    displayName: string;
    networkName: string;
}

export interface EcosystemNetwork extends EcosystemNetworkRef {
    memberCount: number | null;
    onlineMemberCount: number | null;
}

export interface EcosystemNode {
    ref: EcosystemNodeRef;
    hostname: string | null;
    alias: string | null;
    connection: EcosystemConnectionState;
    lastSeenAt: string | null;
    version: string | null;
    os: { type: string | null; version: string | null; distribution: string | null } | null;
    geoLocation: { country: string | null; region: string | null; city: string | null } | null;
    networks: readonly EcosystemNetworkRef[];
    runningInstanceIds: readonly string[];
}

/** Membership is an intent relationship, not evidence of a direct/relay edge. */
export interface EcosystemMembership {
    networkId: string;
    memberId: string;
    node: EcosystemNodeRef;
    hostname: string | null;
    connection: EcosystemConnectionState;
    running: boolean | null;
    virtualIpv4: string | null;
    runtimeVirtualIpv4: string | null;
}

/** Temporary peers have no inferred registered host or business-device identity. */
export interface EcosystemTemporaryPeer {
    networkId: string;
    peerId: number;
    hostname: string | null;
    ipv4: string | null;
}

export interface EcosystemMembers {
    networkId: string;
    members: readonly EcosystemMembership[];
    temporaryPeers: readonly EcosystemTemporaryPeer[];
}

export interface EcosystemOverview {
    userId: number;
    mode: EcosystemMode;
    capabilities: EcosystemCapabilities;
    nodes: readonly EcosystemNode[];
    networks: readonly EcosystemNetwork[];
}

/** Only supported reads. Missing resource capabilities have no fake API methods. */
export interface EcosystemReader {
    loadOverview(): Promise<EcosystemOverview>;
    loadMembers(networkId: string): Promise<EcosystemMembers>;
}

export interface EcosystemTopologyProps {
    overview: EcosystemQueryState<EcosystemOverview>;
    members: EcosystemQueryState<EcosystemMembers>;
    selectedNetworkId: string | null;
    selectedNode: EcosystemNodeRef | null;
}

export interface EcosystemTopologyEvents {
    refresh: [];
    selectNetwork: [networkId: string | null];
    selectNode: [node: EcosystemNodeRef];
}

export interface EcosystemNodeDetailsProps {
    node: EcosystemNode | null;
    capabilities: EcosystemCapabilities;
}

export interface EcosystemNodeDetailsEvents {
    selectResourceKind: [kind: EcosystemResourceKind];
}

/** There are no verified business-resource records or endpoints in contract v1. */
export interface EcosystemResourceDetailsProps {
    node: EcosystemNode | null;
    kind: EcosystemResourceKind;
    capability: EcosystemCapability;
}

/** These independent capabilities do not imply each other's permission/support. */
export interface EcosystemAccessSettingsProps {
    publicAccess: EcosystemCapability;
    portMappings: EcosystemCapability;
    navigation: EcosystemCapability;
}

export interface EcosystemNavigationProps {
    capability: EcosystemCapability;
    node: EcosystemNode | null;
}

export interface EcosystemNavigationEvents {
    backToNode: [node: EcosystemNodeRef];
}

export interface EcosystemQueryNoticeProps {
    status: EcosystemQueryState<unknown>['status'];
    error: EcosystemQueryError | null;
    queriedAt: string | null;
}

export interface EcosystemQueryNoticeEvents {
    refresh: [];
}

export interface EcosystemCapabilityNoticeProps {
    capability: EcosystemCapability;
}
