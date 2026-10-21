<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import EcosystemCapabilityNotice from '../shared/EcosystemCapabilityNotice.vue';
import type { EcosystemResourceDetailsProps } from '../../../modules/ecosystemTypes';

defineProps<EcosystemResourceDetailsProps>();
const { t } = useI18n();
</script>

<template>
    <section class="ecosystem-resource-details" aria-labelledby="ecosystem-resource-title">
        <h2 id="ecosystem-resource-title">{{ t('web.ecosystem.resource.title') }}</h2>
        <p v-if="!node" role="status">{{ t('web.ecosystem.resource.select_host') }}</p>
        <template v-else>
            <div class="ecosystem-resource-context">
                <h3>{{ t(`web.ecosystem.resource.${kind}`) }}</h3>
                <dl>
                    <dt>{{ t('web.ecosystem.resource.host_context') }}</dt>
                    <dd>{{ node.alias || node.hostname || node.ref.machineId }}</dd>
                    <dt>{{ t('web.ecosystem.node.machine_id') }}</dt>
                    <dd>{{ node.ref.machineId }}</dd>
                </dl>
            </div>
            <EcosystemCapabilityNotice :capability="capability" />
            <p>{{ t('web.ecosystem.resource.availability_note') }}</p>
        </template>
    </section>
</template>

<style scoped>
.ecosystem-resource-details { display: grid; gap: 1rem; min-width: 0; color: var(--p-text-color); }
.ecosystem-resource-details h2, .ecosystem-resource-details h3 { font-weight: 600; margin: 0; overflow-wrap: anywhere; }
.ecosystem-resource-details h2 { font-size: 1.25rem; }
.ecosystem-resource-details p { margin: 0; overflow-wrap: anywhere; }
.ecosystem-resource-context { min-width: 0; padding: 1rem; border: 1px solid var(--p-content-border-color); border-radius: 6px; background: var(--p-content-background); }
.ecosystem-resource-context dl { margin: 0; }
.ecosystem-resource-context dt { margin-top: 0.75rem; color: var(--p-text-muted-color); }
.ecosystem-resource-context dd { margin: 0.25rem 0 0; overflow-wrap: anywhere; }
</style>
