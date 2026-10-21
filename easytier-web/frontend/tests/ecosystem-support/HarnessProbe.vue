<script setup lang="ts">
import type { EcosystemTopologyProps, EcosystemTopologyEvents } from '../../src/modules/ecosystemTypes';
defineProps<EcosystemTopologyProps>();
const emit = defineEmits<EcosystemTopologyEvents>();
</script>
<template>
    <section aria-label="Harness probe">
        <p data-testid="status">{{ overview.status }}</p>
        <p v-if="overview.status === 'ready'" data-testid="mode">{{ overview.snapshot.data.mode }}</p>
        <button type="button" @click="emit('refresh')">Refresh sample</button>
        <button v-if="selectedNode" type="button" @click="emit('selectNode', selectedNode)">Select sample host</button>
    </section>
</template>
