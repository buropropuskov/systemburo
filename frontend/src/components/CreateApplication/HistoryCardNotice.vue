<template>
  <p class="history-card-notice">Показаны текущие назначения.<span v-if="expired"> Срок действия истёк.</span></p>
</template>
<script setup>
import { computed } from 'vue';
import { historicalPeriodExpired } from '@/utils/passageProjection';
const props = defineProps({ entity: { type: Object, default: null } });
const expired = computed(() => historicalPeriodExpired(props.entity));
</script>
<style scoped>
.history-card-notice { margin: 0 0 12px; color: var(--text-muted); font-size: 12px; line-height: 1.5; }
</style>
