<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type ApiClient from '../../modules/api';
import { createEcosystemReader, toEcosystemQueryError } from '../../modules/ecosystemApi';
import type { EcosystemMembers, EcosystemNodeRef, EcosystemOverview, EcosystemQueryState, EcosystemReader, EcosystemResourceKind } from '../../modules/ecosystemTypes';
import EcosystemTopology from './topology/EcosystemTopology.vue';
import EcosystemNodeDetails from './nodes/EcosystemNodeDetails.vue';
import EcosystemResourceDetails from './resources/EcosystemResourceDetails.vue';
import EcosystemAccessSettings from './access/EcosystemAccessSettings.vue';
import EcosystemNavigation from './navigation/EcosystemNavigation.vue';

const props = defineProps<{ api: ApiClient; currentUserId: number }>();
const { t } = useI18n();
const overview = shallowRef<EcosystemQueryState<EcosystemOverview>>({ status: 'idle' });
const members = shallowRef<EcosystemQueryState<EcosystemMembers>>({ status: 'idle' });
const selectedNetworkId = ref<string | null>(null);
const selectedNodeRef = ref<EcosystemNodeRef | null>(null);
const resourceKind = ref<EcosystemResourceKind>('service');
const missingHost = ref(false);
const hostSection = ref<HTMLElement | null>(null);
let overviewGeneration = 0;
let membersGeneration = 0;
let reader: EcosystemReader;
let disposed = false;
function snapshot<T>(query: EcosystemQueryState<T>) {
    return query.status === 'ready' ? query.snapshot : query.status === 'loading' || query.status === 'error' ? query.previous : null;
}
const data = computed(() => snapshot(overview.value)?.data ?? null);
const selectedNode = computed(() => data.value?.nodes.find(node => node.ref.userId === selectedNodeRef.value?.userId && node.ref.machineId === selectedNodeRef.value?.machineId) ?? null);

function clearMembers() { ++membersGeneration; members.value = { status: 'idle' }; }
function clearNode() { selectedNodeRef.value = null; missingHost.value = false; resourceKind.value = 'service'; }
async function loadMembers(networkId: string) {
    const generation = ++membersGeneration;
    const previous = snapshot(members.value);
    members.value = { status: 'loading', previous: previous?.data.networkId === networkId ? previous : null };
    try {
        const result = await reader.loadMembers(networkId);
        if (disposed || generation !== membersGeneration) return;
        if (result.networkId !== networkId) throw new Error('Mismatched network');
        members.value = { status: 'ready', snapshot: { data: result, queriedAt: new Date().toISOString() } };
    } catch (error) {
        if (!disposed && generation === membersGeneration) members.value = { status: 'error', error: toEcosystemQueryError(error), previous: previous?.data.networkId === networkId ? previous : null };
    }
}
function selectNetwork(networkId: string | null) {
    if (selectedNetworkId.value === networkId) return;
    if (networkId !== null && !data.value?.networks.some(network => network.networkId === networkId)) return;
    selectedNetworkId.value = networkId;
    clearNode();
    clearMembers();
    if (networkId && overview.value.status === 'ready') void loadMembers(networkId);
}
async function selectNode(node: EcosystemNodeRef) {
    const match = data.value?.nodes.find(item => item.ref.userId === node.userId && item.ref.machineId === node.machineId);
    clearNode();
    if (!match) { missingHost.value = true; return; }
    selectedNodeRef.value = { ...match.ref };
    await nextTick();
    hostSection.value?.focus();
}
async function refresh() {
    if (disposed || overview.value.status === 'loading') return;
    const generation = ++overviewGeneration;
    const previous = snapshot(overview.value);
    ++membersGeneration;
    overview.value = { status: 'loading', previous };
    try {
        const result = await reader.loadOverview();
        if (disposed || generation !== overviewGeneration) return;
        if (result.userId !== props.currentUserId) throw new Error('Mismatched identity');
        overview.value = { status: 'ready', snapshot: { data: result, queriedAt: new Date().toISOString() } };
        if (!selectedNode.value) clearNode();
        const networkId = selectedNetworkId.value;
        if (networkId && result.mode === 'central' && result.networks.some(network => network.networkId === networkId)) void loadMembers(networkId);
        else { selectedNetworkId.value = null; clearMembers(); }
    } catch (error) {
        if (!disposed && generation === overviewGeneration) {
            const safeError = toEcosystemQueryError(error);
            overview.value = { status: 'error', error: safeError, previous };
            if (members.value.status === 'loading') members.value = { status: 'error', error: safeError, previous: members.value.previous };
        }
    }
}
watch(() => [props.api, props.currentUserId] as const, () => {
    ++overviewGeneration;
    clearMembers();
    clearNode();
    selectedNetworkId.value = null;
    overview.value = { status: 'idle' };
    try { reader = createEcosystemReader(props.api, props.currentUserId); void refresh(); }
    catch (error) { overview.value = { status: 'error', error: toEcosystemQueryError(error), previous: null }; }
}, { immediate: true });
onBeforeUnmount(() => { disposed = true; ++overviewGeneration; ++membersGeneration; });
</script>

<template>
    <div class="ecosystem-page">
        <EcosystemTopology :overview="overview" :members="members" :selected-network-id="selectedNetworkId" :selected-node="selectedNodeRef" @refresh="refresh" @select-network="selectNetwork" @select-node="selectNode" />
        <p v-if="missingHost" role="alert">{{ t('web.ecosystem.topology.host_unknown') }}</p>
        <template v-if="data">
            <section ref="hostSection" tabindex="-1" class="ecosystem-host-section" :aria-label="t('web.ecosystem.node.title')">
                <EcosystemNodeDetails :node="selectedNode" :capabilities="data.capabilities" @select-resource-kind="resourceKind = $event" />
            </section>
            <EcosystemResourceDetails :node="selectedNode" :kind="resourceKind" :capability="data.capabilities.resources" />
            <EcosystemAccessSettings :public-access="data.capabilities.publicAccess" :port-mappings="data.capabilities.portMappings" :navigation="data.capabilities.navigation" />
            <EcosystemNavigation :node="selectedNode" :capability="data.capabilities.navigation" @back-to-node="selectNode" />
        </template>
    </div>
</template>

<style scoped>
.ecosystem-page { display: grid; gap: 1.5rem; min-width: 0; }
.ecosystem-host-section { min-width: 0; }
.ecosystem-host-section:focus-visible { outline: 2px solid var(--p-primary-color); outline-offset: 4px; }
</style>
