<script setup lang="ts">
import { computed } from 'vue';
import { Button } from 'primevue';
import { useI18n } from 'vue-i18n';
import EcosystemCapabilityNotice from '../shared/EcosystemCapabilityNotice.vue';
import type { EcosystemNodeDetailsEvents, EcosystemNodeDetailsProps, EcosystemResourceKind } from '../../../modules/ecosystemTypes';

const props = defineProps<EcosystemNodeDetailsProps>();
const emit = defineEmits<EcosystemNodeDetailsEvents>();
const { t } = useI18n();
const resourceKinds: readonly EcosystemResourceKind[] = ['service', 'device_group', 'device'];
const hostFields = computed(() => {
    const node = props.node;
    if (!node) return [];
    return [
        { key: 'machine_id', value: node.ref.machineId },
        { key: 'hostname', value: node.hostname },
        { key: 'alias', value: node.alias },
        { key: 'version', value: node.version },
        { key: 'os', value: node.os?.type },
        { key: 'os_version', value: node.os?.version },
        { key: 'distribution', value: node.os?.distribution },
    ];
});
const estimatedLocation = computed(() => {
    const location = props.node?.geoLocation;
    return location ? [location.country, location.region, location.city].filter(Boolean).join(' / ') : '';
});
</script>

<template>
    <section class="ecosystem-node-details" aria-labelledby="ecosystem-node-title">
        <h2 id="ecosystem-node-title">{{ t('web.ecosystem.node.title') }}</h2>
        <p v-if="!node" role="status">{{ t('web.ecosystem.node.select_hint') }}</p>
        <template v-else>
            <div class="ecosystem-node-panel">
                <h3>{{ node.alias || node.hostname || node.ref.machineId }}</h3>
                <p>{{ t(`web.ecosystem.connection.${node.connection}`) }}</p>
                <p class="ecosystem-node-note">{{ t('web.ecosystem.node.health_note') }}</p>
                <dl class="ecosystem-node-fields">
                    <div v-for="field in hostFields" :key="field.key">
                        <dt>{{ t(`web.ecosystem.node.${field.key}`) }}</dt>
                        <dd>{{ field.value || t('web.ecosystem.unknown') }}</dd>
                    </div>
                    <div>
                        <dt>{{ t('web.ecosystem.node.last_seen') }}</dt>
                        <dd><time v-if="node.lastSeenAt" :datetime="node.lastSeenAt">{{ node.lastSeenAt }}</time><span v-else>{{ t('web.ecosystem.unknown') }}</span></dd>
                    </div>
                    <div>
                        <dt>{{ t('web.ecosystem.node.geo_location') }}</dt>
                        <dd>{{ estimatedLocation || t('web.ecosystem.unknown') }}<p class="ecosystem-node-note">{{ t('web.ecosystem.node.geo_note') }}</p></dd>
                    </div>
                </dl>
            </div>

            <section class="ecosystem-node-panel" aria-labelledby="ecosystem-node-metadata">
                <h3 id="ecosystem-node-metadata">{{ t('web.ecosystem.node.additional_metadata') }}</h3>
                <EcosystemCapabilityNotice :capability="capabilities.nodeMetadata" />
                <dl class="ecosystem-node-fields">
                    <div v-for="field in ['deployment_location', 'manufacturer', 'model', 'architecture']" :key="field">
                        <dt>{{ t(`web.ecosystem.node.${field}`) }}</dt><dd>{{ t('web.ecosystem.unknown') }}</dd>
                    </div>
                </dl>
            </section>

            <section class="ecosystem-node-panel" aria-labelledby="ecosystem-node-networks">
                <h3 id="ecosystem-node-networks">{{ t('web.ecosystem.node.networks') }}</h3>
                <EcosystemCapabilityNotice v-if="capabilities.networks.status === 'unsupported'" :capability="capabilities.networks" />
                <ul v-else-if="node.networks.length" class="ecosystem-node-list">
                    <li v-for="network in node.networks" :key="network.networkId">
                        <strong>{{ network.displayName || network.networkName || network.networkId }}</strong>
                        <span>{{ network.networkId }}</span>
                    </li>
                </ul>
                <p v-else>{{ t('web.ecosystem.node.no_networks_reported') }}</p>
            </section>

            <section class="ecosystem-node-panel" aria-labelledby="ecosystem-node-instances">
                <h3 id="ecosystem-node-instances">{{ t('web.ecosystem.node.instances') }}</h3>
                <ul v-if="node.runningInstanceIds.length" class="ecosystem-node-list">
                    <li v-for="instanceId in node.runningInstanceIds" :key="instanceId">{{ instanceId }}</li>
                </ul>
                <p v-else>{{ t('web.ecosystem.node.no_instances_reported') }}</p>
            </section>

            <section class="ecosystem-node-panel" aria-labelledby="ecosystem-node-resources">
                <h3 id="ecosystem-node-resources">{{ t('web.ecosystem.node.resource_types') }}</h3>
                <p>{{ t('web.ecosystem.node.resource_boundary') }}</p>
                <div class="ecosystem-resource-types">
                    <Button v-for="kind in resourceKinds" :key="kind" type="button" :label="t(`web.ecosystem.resource.${kind}`)"
                        @click="emit('selectResourceKind', kind)" />
                </div>
                <EcosystemCapabilityNotice :capability="capabilities.resources" />
            </section>
        </template>
    </section>
</template>

<style scoped>
.ecosystem-node-details { display: grid; gap: 1rem; min-width: 0; color: var(--p-text-color); }
.ecosystem-node-details h2, .ecosystem-node-details h3 { font-weight: 600; margin: 0; overflow-wrap: anywhere; }
.ecosystem-node-details h2 { font-size: 1.25rem; }
.ecosystem-node-details p { margin: 0.5rem 0; overflow-wrap: anywhere; }
.ecosystem-node-panel { display: grid; gap: 0.5rem; padding: 1rem; min-width: 0; border: 1px solid var(--p-content-border-color); border-radius: 6px; background: var(--p-content-background); }
.ecosystem-node-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 15rem), 1fr)); gap: 1rem; margin: 0.5rem 0; }
.ecosystem-node-fields div { min-width: 0; }
.ecosystem-node-fields dt, .ecosystem-node-note { color: var(--p-text-muted-color); }
.ecosystem-node-fields dd { margin: 0.25rem 0 0; overflow-wrap: anywhere; }
.ecosystem-node-list { display: grid; gap: 0.75rem; margin: 0; padding-left: 1.25rem; overflow-wrap: anywhere; }
.ecosystem-node-list span { display: block; color: var(--p-text-muted-color); }
.ecosystem-resource-types { display: flex; flex-wrap: wrap; gap: 0.5rem; margin: 0.5rem 0; }
.ecosystem-resource-types :deep(.p-button-label) { white-space: normal; }
</style>
