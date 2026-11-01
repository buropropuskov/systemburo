<script setup>
import { shallowRef, ref, watch, nextTick, onBeforeUnmount } from 'vue';
import { loadFeedbackModal } from '@/utils/feedbackModalLoader';
import { useDeletionsStore } from '@/stores/deletions';

defineOptions({ inheritAttrs: false });
const props = defineProps({ show: Boolean });
const emit = defineEmits(['update:show']);
const notifications = useDeletionsStore();
const modal = shallowRef(null);
const visible = ref(false);
let generation = 0;

watch(() => props.show, async show => {
  const current = ++generation;
  visible.value = false;
  if (!show) return;
  try {
    const loaded = modal.value || (await loadFeedbackModal()).default;
    if (current !== generation) return;
    modal.value = loaded;
    // Mount closed first: the existing window owns its entrance animation and draft.
    await nextTick();
    if (current === generation) visible.value = true;
  } catch {
    if (current !== generation) return;
    notifications.notify({ type: 'error', prefix: 'Не удалось открыть форму. Обновите страницу и попробуйте ещё раз.' });
    emit('update:show', false);
  }
}, { immediate: true });
onBeforeUnmount(() => { generation++; });
</script>
<template>
  <component
    :is="modal"
    v-if="modal"
    v-bind="$attrs"
    :show="visible"
    @update:show="emit('update:show', $event)"
  />
</template>
