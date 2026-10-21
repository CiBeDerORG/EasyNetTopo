<script setup lang="ts">
import { computed, ref } from 'vue';
import { Button } from 'primevue';
import { useI18n } from 'vue-i18n';
import EcosystemQueryNotice from '../shared/EcosystemQueryNotice.vue';
import EcosystemCapabilityNotice from '../shared/EcosystemCapabilityNotice.vue';
import type {
    EcosystemNode, EcosystemNodeRef, EcosystemQueryState,
    EcosystemTopologyEvents, EcosystemTopologyProps,
} from '../../../modules/ecosystemTypes';

const props = defineProps<EcosystemTopologyProps>();
const emit = defineEmits<EcosystemTopologyEvents>();
const { t } = useI18n();
const locationFilter = ref('');

function snapshot<T>(query: EcosystemQueryState<T>) {
    return query.status === 'ready' ? query.snapshot
        : query.status === 'loading' || query.status === 'error' ? query.previous : null;
}
const overviewSnapshot = computed(() => snapshot(props.overview));
const overviewData = computed(() => overviewSnapshot.value?.data ?? null);
const membersSnapshot = computed(() => {
    const value = snapshot(props.members);
    return value?.data.networkId === props.selectedNetworkId ? value : null;
});
const memberStatus = computed(() => {
    const previous = snapshot(props.members);
    if (previous && previous.data.networkId !== props.selectedNetworkId) {
        return props.members.status === 'loading' ? 'loading' : 'idle';
    }
    return props.members.status;
});
const nodesById = computed(() => new Map(overviewData.value?.nodes.map(node => [nodeKey(node.ref), node]) ?? []));
const memberRows = computed(() => membersSnapshot.value?.data.members ?? []);
const temporaryPeers = computed(() => membersSnapshot.value?.data.temporaryPeers ?? []);
const networkSupported = computed(() => overviewData.value?.capabilities.networks.status === 'supported');
const selectedNetwork = computed(() => overviewData.value?.networks.find(network => network.networkId === props.selectedNetworkId));

function nodeKey(node: EcosystemNodeRef) { return `${node.userId}:${node.machineId}`; }
function nodeName(node: EcosystemNode) { return node.alias || node.hostname || node.ref.machineId; }
function memberName(node: EcosystemNodeRef, hostname: string | null) {
    const host = nodesById.value.get(nodeKey(node));
    return host ? nodeName(host) : hostname || node.machineId;
}
function locationLabel(node: EcosystemNode) {
    const location = node.geoLocation;
    return location ? [location.country, location.region, location.city].filter(Boolean).join(' / ') : '';
}
const locations = computed(() => [...new Set(overviewData.value?.nodes.map(locationLabel).filter(Boolean) ?? [])].sort());
function matchesLocation(node: EcosystemNode | undefined) {
    if (!locationFilter.value) return true;
    const label = node ? locationLabel(node) : '';
    return locationFilter.value === 'unknown' ? !label : JSON.stringify(label) === locationFilter.value;
}
const visibleNodes = computed(() => overviewData.value?.nodes.filter(matchesLocation) ?? []);
const visibleMembers = computed(() => memberRows.value.filter(member => matchesLocation(nodesById.value.get(nodeKey(member.node)))));
function isSelected(node: EcosystemNodeRef) {
    return props.selectedNode !== null && nodeKey(node) === nodeKey(props.selectedNode);
}
function selectNetwork(event: Event) {
    const target = event.target;
    if (target instanceof HTMLSelectElement) emit('selectNetwork', target.value || null);
}
</script>

<template>
    <section class="ecosystem-topology" aria-labelledby="ecosystem-topology-title">
        <header>
            <h2 id="ecosystem-topology-title">{{ t('web.ecosystem.topology.title') }}</h2>
            <p>{{ t('web.ecosystem.topology.membership_note') }}</p>
        </header>
        <EcosystemQueryNotice :status="overview.status"
            :error="overview.status === 'error' ? overview.error : null"
            :queried-at="overviewSnapshot?.queriedAt ?? null" @refresh="emit('refresh')" />

        <template v-if="overviewSnapshot">
            <EcosystemCapabilityNotice :capability="overviewSnapshot.data.capabilities.networks" />
            <div class="ecosystem-filters">
                <div v-if="networkSupported" class="ecosystem-filter">
                    <label for="ecosystem-network">
                    {{ t('web.ecosystem.topology.network') }}
                    </label>
                    <select id="ecosystem-network" :value="selectedNetworkId ?? ''"
                        :disabled="overview.status === 'loading'" @change="selectNetwork">
                        <option value="">{{ t('web.ecosystem.topology.all_hosts') }}</option>
                        <option v-for="network in overviewSnapshot.data.networks" :key="network.networkId" :value="network.networkId">
                            {{ network.displayName || network.networkName || network.networkId }}
                        </option>
                    </select>
                </div>
                <div class="ecosystem-filter">
                    <label for="ecosystem-location">
                    {{ t('web.ecosystem.node.geo_location') }}
                    </label>
                    <select id="ecosystem-location" v-model="locationFilter">
                        <option value="">{{ t('web.ecosystem.topology.all_locations') }}</option>
                        <option value="unknown">{{ t('web.ecosystem.unknown') }}</option>
                        <option v-for="location in locations" :key="location" :value="JSON.stringify(location)">{{ location }}</option>
                    </select>
                </div>
            </div>
            <p v-if="networkSupported && overviewSnapshot.data.networks.length === 0">{{ t('web.ecosystem.topology.no_networks') }}</p>

            <section v-if="selectedNetworkId && networkSupported" class="ecosystem-members" aria-labelledby="ecosystem-members-title">
                <h3 id="ecosystem-members-title">{{ selectedNetwork?.displayName || selectedNetworkId }}</h3>
                <EcosystemQueryNotice :status="memberStatus"
                    :error="members.status === 'error' ? members.error : null"
                    :queried-at="membersSnapshot?.queriedAt ?? null" @refresh="emit('refresh')" />
                <template v-if="membersSnapshot">
                    <p v-if="memberRows.length === 0">{{ t('web.ecosystem.topology.no_members') }}</p>
                    <p v-else-if="visibleMembers.length === 0">{{ t('web.ecosystem.topology.no_matching_hosts') }}</p>
                    <ul v-else class="ecosystem-host-list">
                        <li v-for="member in visibleMembers" :key="member.memberId" class="ecosystem-host-card">
                            <h4>{{ memberName(member.node, member.hostname) }}</h4>
                            <p>{{ t(`web.ecosystem.connection.${member.connection}`) }}</p>
                            <dl>
                                <dt>{{ t('web.ecosystem.topology.member_id') }}</dt><dd>{{ member.memberId }}</dd>
                                <dt>{{ t('web.ecosystem.node.machine_id') }}</dt><dd>{{ member.node.machineId }}</dd>
                                <dt>{{ t('web.ecosystem.topology.virtual_ip') }}</dt><dd>{{ member.virtualIpv4 ?? t('web.ecosystem.unknown') }}</dd>
                                <dt>{{ t('web.ecosystem.topology.runtime_ip') }}</dt><dd>{{ member.runtimeVirtualIpv4 ?? t('web.ecosystem.unknown') }}</dd>
                            </dl>
                            <p v-if="!nodesById.has(nodeKey(member.node))">{{ t('web.ecosystem.topology.host_unknown') }}</p>
                            <Button type="button" :label="t('web.ecosystem.topology.select_host')"
                                :aria-label="`${t('web.ecosystem.topology.select_host')}: ${member.node.machineId}`"
                                :aria-pressed="isSelected(member.node)" :disabled="!nodesById.has(nodeKey(member.node))"
                                @click="emit('selectNode', member.node)" />
                        </li>
                    </ul>
                    <section v-if="temporaryPeers.length" aria-labelledby="ecosystem-temporary-title">
                        <h3 id="ecosystem-temporary-title">{{ t('web.ecosystem.topology.temporary_peers') }}</h3>
                        <p>{{ t('web.ecosystem.topology.temporary_note') }}</p>
                        <ul class="ecosystem-host-list">
                            <li v-for="peer in temporaryPeers" :key="`${peer.networkId}:${peer.peerId}`" class="ecosystem-host-card">
                                <h4>{{ peer.hostname ?? t('web.ecosystem.unknown') }}</h4>
                                <p>{{ t('web.ecosystem.topology.peer_id') }}: {{ peer.peerId }}</p>
                                <p>{{ peer.ipv4 ?? t('web.ecosystem.unknown') }}</p>
                            </li>
                        </ul>
                    </section>
                </template>
            </section>
            <template v-else>
                <p v-if="overviewSnapshot.data.nodes.length === 0">{{ t('web.ecosystem.topology.no_hosts') }}</p>
                <p v-else-if="visibleNodes.length === 0">{{ t('web.ecosystem.topology.no_matching_hosts') }}</p>
                <ul v-else class="ecosystem-host-list">
                    <li v-for="node in visibleNodes" :key="nodeKey(node.ref)" class="ecosystem-host-card">
                        <h3>{{ nodeName(node) }}</h3>
                        <p>{{ t(`web.ecosystem.connection.${node.connection}`) }}</p>
                        <dl>
                            <dt>{{ t('web.ecosystem.node.machine_id') }}</dt><dd>{{ node.ref.machineId }}</dd>
                            <dt>{{ t('web.ecosystem.node.geo_location') }}</dt><dd>{{ locationLabel(node) || t('web.ecosystem.unknown') }}</dd>
                            <dt>{{ t('web.ecosystem.node.networks') }}</dt>
                            <dd>{{ node.networks.map(network => network.displayName || network.networkId).join(', ') || t('web.ecosystem.unknown') }}</dd>
                        </dl>
                        <Button type="button" :label="t('web.ecosystem.topology.select_host')"
                            :aria-label="`${t('web.ecosystem.topology.select_host')}: ${node.ref.machineId}`"
                            :aria-pressed="isSelected(node.ref)" @click="emit('selectNode', node.ref)" />
                    </li>
                </ul>
            </template>
        </template>
    </section>
</template>

<style scoped>
.ecosystem-topology { display: grid; gap: 1rem; min-width: 0; color: var(--p-text-color); }
.ecosystem-topology h2, .ecosystem-topology h3, .ecosystem-topology h4 { font-weight: 600; margin: 0; overflow-wrap: anywhere; }
.ecosystem-topology h2 { font-size: 1.25rem; }
.ecosystem-topology p { margin: 0.5rem 0; overflow-wrap: anywhere; }
.ecosystem-filters { display: flex; flex-wrap: wrap; gap: 1rem; }
.ecosystem-filter { display: grid; gap: 0.35rem; flex: 1 1 16rem; min-width: 0; }
.ecosystem-filters select { max-width: 100%; min-width: 0; padding: 0.65rem; background: var(--p-content-background); color: var(--p-text-color); border: 1px solid var(--p-content-border-color); border-radius: 6px; }
.ecosystem-filters select:focus-visible { outline: 2px solid var(--p-primary-color); outline-offset: 2px; }
.ecosystem-members { display: grid; gap: 1rem; min-width: 0; }
.ecosystem-host-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: 1rem; margin: 0; padding: 0; list-style: none; }
.ecosystem-host-card { padding: 1rem; min-width: 0; border: 1px solid var(--p-content-border-color); border-radius: 6px; background: var(--p-content-background); overflow-wrap: anywhere; }
.ecosystem-host-card dl { margin: 0.75rem 0; }
.ecosystem-host-card dt { margin-top: 0.5rem; color: var(--p-text-muted-color); }
.ecosystem-host-card dd { margin: 0; }
</style>
