<script setup lang="ts">
import { computed } from 'vue';
import { Button } from 'primevue';
import { useI18n } from 'vue-i18n';
import type { EcosystemQueryNoticeEvents, EcosystemQueryNoticeProps } from '../../../modules/ecosystemTypes';

const props = defineProps<EcosystemQueryNoticeProps>();
const emit = defineEmits<EcosystemQueryNoticeEvents>();
const { t } = useI18n();
const messageKey = computed(() => props.status === 'error'
    ? `web.ecosystem.errors.${props.error?.kind ?? 'request_failed'}`
    : `web.ecosystem.query.${props.status}`);
</script>

<template>
    <section class="ecosystem-query-notice" :aria-busy="status === 'loading'">
        <div class="ecosystem-query-message" :role="status === 'error' ? 'alert' : 'status'" aria-atomic="true">
            <p>{{ t(messageKey) }}</p>
            <p v-if="queriedAt" class="ecosystem-query-time">
                {{ t('web.ecosystem.query.last_queried') }}
                <time :datetime="queriedAt">{{ queriedAt }}</time>
                <span v-if="status !== 'ready'"> · {{ t('web.ecosystem.query.previous') }}</span>
            </p>
        </div>
        <Button type="button" icon="pi pi-refresh" :label="t('web.ecosystem.refresh')"
            :disabled="status === 'loading'" @click="emit('refresh')" />
    </section>
</template>

<style scoped>
.ecosystem-query-notice {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.75rem;
    min-width: 0;
    padding: 1rem;
    border: 1px solid var(--p-content-border-color);
    border-radius: var(--p-border-radius-md, 6px);
    color: var(--p-text-color);
    background: var(--p-content-background);
}
.ecosystem-query-message { flex: 1 1 15rem; min-width: 0; overflow-wrap: anywhere; }
.ecosystem-query-message p { margin: 0; }
.ecosystem-query-message .ecosystem-query-time { margin-top: 0.35rem; color: var(--p-text-muted-color); font-size: 0.875rem; }
.ecosystem-query-notice :deep(.p-button-label) { white-space: normal; }
</style>
