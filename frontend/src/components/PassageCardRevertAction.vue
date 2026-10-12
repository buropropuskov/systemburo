<template>
  <div v-if="available" class="card-revert-action">
    <button type="button" class="lk-button lk-button--ghost lk-button--sm" data-testid="passage-card-revert" @click="ask">Отменить последнюю отметку</button>
  </div>
</template>
<script setup>
import { computed } from 'vue';
import { usePermissionsStore } from '@/stores/permissions';
import { usePassageRevertStore } from '@/stores/passageRevert';
import { usePassageClock } from '@/composables/usePassageClock';
import { canRevertMark, canQuickRevertMark, lastMarkDirection } from '@/utils/passageMarks';
const props = defineProps({ status: { type: Object, default: null }, kind: { type: String, required: true },
  entityID: { type: Number, default: null }, tableID: { type: Number, default: null },
  subject: { type: String, default: '' }, active: { type: Boolean, default: false } });
const emit = defineEmits(['changed']);
const permissions = usePermissionsStore();
const clock = usePassageClock(computed(() => props.status?.server_now), { active: computed(() => props.active) });
const available = computed(() => props.active && permissions.hasPermission('page.admin')
  && Number.isSafeInteger(props.entityID) && Number.isSafeInteger(props.tableID)
  && props.status?.last_mark_table_id === props.tableID
  && Number.isSafeInteger(props.status?.passage_state?.last_event_id)
  && canRevertMark(props.status, props.tableID) && lastMarkDirection(props.status)
  && clock.now.value !== null && !canQuickRevertMark(props.status, props.tableID, clock.now.value));
function ask() {
  if (!available.value) return;
  usePassageRevertStore().ask({ kind: props.kind === 'car' ? 'cars' : 'employees', id: props.entityID,
    direction: lastMarkDirection(props.status), tableId: props.tableID, subject: props.subject,
    expectedLastEventID: props.status?.passage_state?.last_event_id,
    onDone: () => emit('changed'), onRefresh: () => emit('changed') });
}
</script>
<style scoped>
.card-revert-action { margin-bottom: 12px; }
</style>
