<template>
  <div class="modal-header">
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
.modal-title { margin: 0; font-size: 16px; font-weight: 600; color: var(--text); }
:deep(.modal-close) { background: none; border: none; cursor: pointer; padding: 6px; border-radius: 6px; display: flex; align-items: center; justify-content: center; transition: background-color .2s ease; }
:deep(.modal-close:hover) { background-color: var(--surface-2); }
</style>
