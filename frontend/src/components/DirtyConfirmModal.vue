<template>
  <ConfirmationModal
    :show="confirmState.show"
    title="Несохранённые изменения"
    :message="confirmState.message"
    confirm-text="Уйти без сохранения"
    cancel-text="Остаться"
    @confirm="onConfirm"
    @cancel="onCancel"
  />
</template>

<script>
import ConfirmationModal from './ConfirmationModal.vue';
import { confirmState, resolveDirtyConfirm } from '@/utils/dirtyTracker';

export default {
  name: 'DirtyConfirmModal',
  components: { ConfirmationModal },
  data() {
    return { confirmState };
  },
  methods: {
    onConfirm() {
      resolveDirtyConfirm(true);
    },
    onCancel() {
      resolveDirtyConfirm(false);
    },
  },
};
</script>
