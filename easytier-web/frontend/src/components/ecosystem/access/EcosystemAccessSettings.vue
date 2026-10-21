<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { EcosystemAccessSettingsProps } from '../../../modules/ecosystemTypes';
import EcosystemCapabilityNotice from '../shared/EcosystemCapabilityNotice.vue';

const props = defineProps<EcosystemAccessSettingsProps>();
const { t } = useI18n();
const capabilities = computed(() => [
    { key: 'public_access', capability: props.publicAccess },
    { key: 'port_mappings', capability: props.portMappings },
    { key: 'navigation', capability: props.navigation },
]);
</script>

<template>
    <section class="ecosystem-access">
        <h2>{{ t('web.ecosystem.access.title') }}</h2>
        <p class="ecosystem-access-description">{{ t('web.ecosystem.access.independent_note') }}</p>
        <div class="ecosystem-access-capabilities">
            <section v-for="item in capabilities" :key="item.key" :data-capability="item.key">
                <h3>{{ t(`web.ecosystem.access.${item.key}`) }}</h3>
                <EcosystemCapabilityNotice :capability="item.capability" />
                <p v-if="item.capability.status === 'supported'" role="status">
                    {{ t('web.ecosystem.access.supported_note') }}
                </p>
            </section>
        </div>
    </section>
</template>

<style scoped>
.ecosystem-access { min-width: 0; color: var(--p-text-color); overflow-wrap: anywhere; }
.ecosystem-access h2 { margin: 0 0 0.5rem; }
.ecosystem-access-description { margin: 0 0 1rem; color: var(--p-text-muted-color); }
.ecosystem-access-capabilities { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); }
.ecosystem-access-capabilities section { min-width: 0; }
.ecosystem-access-capabilities h3 { margin: 0 0 0.5rem; }
</style>
