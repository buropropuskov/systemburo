<template>
  <button v-if="available" type="button" class="lk-button lk-button--ghost lk-button--sm" data-testid="open-passages-button" @click="show = true">
    {{ kind === 'employee' ? 'Незакрытые проходы' : 'Незакрытые проезды' }}
  </button>
  <OpenPassagesModal v-if="available" :show="show" :kind="kind" :source="source" :table-i-d="tableID" :organizations="organizations"
    @close="show = false" @changed="$emit('refresh')" @open-entity="$emit('open-entity', $event)" />
</template>
<script setup>
import { computed, ref, watch } from 'vue';
import OpenPassagesModal from './OpenPassagesModal.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { usePassageClock } from '@/composables/usePassageClock';
import { passageDeadlines } from '@/utils/passageProjection';
const props = defineProps({ kind: { type: String, required: true }, tableID: { type: Number, default: null },
  source: { type: String, default: 'table', validator: value => ['table', 'admin_summary'].includes(value) }, rows: { type: Array, default: () => [] } });
const emit = defineEmits(['refresh', 'open-entity']);
const permissions = usePermissionsStore(), show = ref(false);
const available = computed(() => props.source === 'admin_summary'
  ? permissions.hasPermission('detail.passage.correct') : Number.isSafeInteger(props.tableID) && props.tableID > 0);
const organizations = computed(() => [...new Map(props.rows.filter(row => row.organization_id).map(row =>
  [row.organization_id, { id: row.organization_id, name: row.organization_name || row.organization }])).values()]);
const serverNow = computed(() => props.rows.find(row => row.server_now)?.server_now ?? null);
const active = computed(() => available.value && props.source === 'table');
const deadlines = computed(() => passageDeadlines(props.rows));
usePassageClock(serverNow, { active, deadlines, onBoundary: () => emit('refresh') });
watch(() => [props.kind, props.tableID, props.source, available.value], () => { show.value = false; });
</script>
