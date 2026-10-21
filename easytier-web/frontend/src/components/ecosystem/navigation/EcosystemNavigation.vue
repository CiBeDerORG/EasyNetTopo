<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import Button from 'primevue/button';
import type { EcosystemNavigationProps, EcosystemNavigationEvents } from '../../../modules/ecosystemTypes';
import EcosystemCapabilityNotice from '../shared/EcosystemCapabilityNotice.vue';

const props = defineProps<EcosystemNavigationProps>();
const emit = defineEmits<EcosystemNavigationEvents>();
const { t } = useI18n();
const hostName = computed(() => props.node?.alias || props.node?.hostname || t('web.ecosystem.unknown'));
function backToNode() {
    if (props.node) emit('backToNode', { ...props.node.ref });
}
</script>

<template>
    <section class="ecosystem-navigation">
        <h2>{{ t('web.ecosystem.navigation.title') }}</h2>
        <p v-if="node">{{ t('web.ecosystem.navigation.host_context', { host: hostName }) }}</p>
        <p v-else>{{ t('web.ecosystem.resource.select_host') }}</p>
        <EcosystemCapabilityNotice :capability="capability" />
        <p v-if="capability.status === 'supported'" role="status">{{ t('web.ecosystem.access.supported_note') }}</p>
        <p>{{ t('web.ecosystem.navigation.no_link') }}</p>
        <Button v-if="node" type="button" :label="t('web.ecosystem.navigation.back_to_node')" @click="backToNode" />
    </section>
</template>

<style scoped>
.ecosystem-navigation { min-width: 0; color: var(--p-text-color); overflow-wrap: anywhere; }
.ecosystem-navigation h2 { margin: 0 0 0.5rem; }
.ecosystem-navigation p { margin: 0.75rem 0; color: var(--p-text-muted-color); }
.ecosystem-navigation :deep(button) { max-width: 100%; white-space: normal; }
</style>
