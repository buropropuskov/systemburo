<template>
  <button v-if="available" type="button" class="lk-button lk-button--ghost lk-button--sm" data-testid="open-passages-button" @click="show = true">
    {{ kind === 'employee' ? 'Незакрытые проходы' : 'Незакрытые проезды' }}
  </button>
  <OpenPassagesModal v-if="available" :show="show" :kind="kind" :source="source" :table-i-d="tableID" :organizations="organizations"
    @close="show = false" @changed="$emit('refresh')" @open-entity="$emit('open-entity', $event)" />
</template>
<script setup>
import { computed, ref, watch, defineAsyncComponent } from 'vue';
const OpenPassagesModal = defineAsyncComponent(() => import('./OpenPassagesModal.vue'));
import { usePermissionsStore } from '@/stores/permissions';
import { usePassageClock } from '@/composables/usePassageClock';
import { passageDeadlines } from '@/utils/passageProjection';
const props = defineProps({ kind: { type: String, required: true }, tableID: { type: Number, default: null },
  source: { type: String, default: 'table', validator: value => ['table', 'admin_summary'].includes(value) }, rows: { type: Array, default: () => [] } });
const emit = defineEmits(['refresh', 'open-entity', 'clock', 'available']);
const permissions = usePermissionsStore(), show = ref(false);
const canCorrect = computed(() => permissions.hasPermission('detail.passage.correct'));
const administrative = computed(() => ['admin', 'super'].includes(permissions.mode));
const validTable = computed(() => props.source === 'table' && Number.isSafeInteger(props.tableID) && props.tableID > 0);
const available = computed(() => administrative.value && (props.source === 'admin_summary' ? canCorrect.value : validTable.value));
watch(available, value => emit('available', value), { immediate: true });
const organizations = computed(() => [...new Map(props.rows.filter(row => row.organization_id).map(row =>
  [row.organization_id, { id: row.organization_id, name: row.organization_name || row.organization }])).values()]);
const serverNow = computed(() => props.rows.find(row => row.server_now)?.server_now ?? null);
// Сроки обычного поста обновляются и без административного инструмента.
const active = validTable;
const deadlines = computed(() => passageDeadlines(props.rows));
const clock = usePassageClock(serverNow, { active, deadlines, onBoundary: () => emit('refresh') });
watch(clock.now, value => emit('clock', value), { immediate: true });
watch(() => [props.kind, props.tableID, props.source, available.value], () => { show.value = false; });
</script>
