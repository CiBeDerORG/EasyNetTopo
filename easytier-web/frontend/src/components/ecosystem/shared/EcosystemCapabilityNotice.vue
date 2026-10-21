<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import type { EcosystemCapabilityNoticeProps } from '../../../modules/ecosystemTypes';

defineProps<EcosystemCapabilityNoticeProps>();
const { t } = useI18n();
</script>

<template>
    <div v-if="capability.status === 'unsupported'" class="ecosystem-capability-notice" role="status">
        <strong>{{ t('web.ecosystem.unsupported_title') }}</strong>
        <p>{{ t(`web.ecosystem.unsupported.${capability.reason}`) }}</p>
    </div>
</template>

<style scoped>
.ecosystem-capability-notice {
    min-width: 0;
    padding: 1rem;
    border: 1px solid var(--p-content-border-color);
    border-radius: var(--p-border-radius-md, 6px);
    background: var(--p-content-background);
    color: var(--p-text-color);
    overflow-wrap: anywhere;
}
.ecosystem-capability-notice p { margin: 0.35rem 0 0; color: var(--p-text-muted-color); }
</style>
