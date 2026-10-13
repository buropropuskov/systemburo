<template>
  <div class="modal-header detail-card-header">
    <h3 class="modal-title">{{ title }}</h3>
    <slot />
    <DetailCloseButton @click="$emit('close')" />
  </div>
</template>
<script setup>
import DetailCloseButton from '@/components/ui/DetailCloseButton.vue';
defineProps({ title: { type: String, default: '' } });
defineEmits(['close']);
</script>
<style scoped>
.modal-header.detail-card-header { height: 60px; min-height: 60px; flex-wrap: nowrap; }
.detail-card-header .modal-title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.detail-card-header :deep(.modal-close) { flex-shrink: 0; }
@media (max-width: 767.98px) {
  .modal-header.detail-card-header { padding: 6px 10px !important; gap: 6px; }
}

.modal-title { margin: 0; font-size: 16px; font-weight: 600; color: var(--text); }
:deep(.modal-close) { background: none; border: none; cursor: pointer; padding: 6px; border-radius: 6px; display: flex; align-items: center; justify-content: center; transition: background-color .2s ease; }
:deep(.modal-close:hover) { background-color: var(--surface-2); }
</style>
