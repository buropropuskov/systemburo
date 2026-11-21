<template>
  <ApplicationAttachmentDetail
    v-bind="$attrs" :application-id="applicationId" :attachment="attachment" :cars="cars" :employees="employees" :changes="visibleChanges"
    :acknowledging-change="busy" @acknowledge-change="acknowledge"
    @assignments-changed="assignmentsChanged"
  />
</template>

<script setup>
import { computed, ref, watch, onBeforeUnmount } from 'vue';
import ApplicationAttachmentDetail from './ApplicationAttachmentDetail.vue';
import { loadApplicationChanges, acknowledgeApplicationChanges } from '@/utils/applicationChanges';
import { useDeletionsStore } from '@/stores/deletions';
defineOptions({ inheritAttrs: false });
const props = defineProps({ applicationId: { type: Number, default: null }, attachment: { type: Object, required: true }, attachments: { type: Array, default: () => [] }, focusAttachment: { type: Function, default: null }, cars: { type: Array, default: () => [] }, employees: { type: Array, default: () => [] } });
const emit = defineEmits(['assignments-changed']);
const changes = ref([]), busy = ref(false);
let request = 0, focusedApplication = null;
const visibleChanges = computed(() => changes.value.filter(change => change.attachment_id === props.attachment.id));
async function refresh() {
  const sequence = ++request, applicationId = props.applicationId;
  if (!applicationId) { changes.value = []; return; }
  try {
    const fresh = await loadApplicationChanges(applicationId);
    if (sequence === request && applicationId === props.applicationId) {
      changes.value = fresh;
      if (focusedApplication !== applicationId && fresh.length) {
        focusedApplication = applicationId;
        if (!fresh.some(change => change.attachment_id === props.attachment.id)) {
          const target = props.attachments.find(attachment => attachment.id === fresh[0].attachment_id);
          if (target) props.focusAttachment?.(target);
        }
      }
    }
  } catch {
    if (sequence === request) useDeletionsStore().notify({ type: 'error', prefix: 'Не удалось загрузить изменения заявки' });
  }
}
watch(() => [props.applicationId, props.attachment.id, props.cars, props.employees], (current, previous) => {
  if (previous && current[0] !== previous[0]) changes.value = [];
  void refresh();
}, { immediate: true });
async function acknowledge(change) {
  if (busy.value) return;
  busy.value = true;
  try { await acknowledgeApplicationChanges(changes.value, change); await refresh(); }
  catch { useDeletionsStore().notify({ type: 'error', prefix: 'Не удалось подтвердить изменение' }); }
  finally { busy.value = false; }
}
function assignmentsChanged(...args) { emit('assignments-changed', ...args); void refresh(); }
onBeforeUnmount(() => { request++; });
</script>
