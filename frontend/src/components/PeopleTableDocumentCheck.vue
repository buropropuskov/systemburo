<script setup>
import AppIcon from '@/components/icons/AppIcon.vue';

defineProps({ employee: { type: Object, required: true } });
defineEmits(['open']);
const fullName = (employee) => [employee.last_name, employee.first_name, employee.middle_name].filter(Boolean).join(' ');
</script>

<template>
  <div class="post-document-name">
    <span class="post-document-surname" :title="employee.last_name">{{ employee.last_name }}</span>
    <span class="post-document-full-name" :title="fullName(employee)">{{ fullName(employee) }}</span>
    <button class="post-document-button" type="button" aria-label="Сверить документ" title="Сверить документ" @mousedown.stop @click.stop="$emit('open')">
      <AppIcon name="clipboard" :size="15" />
    </button>
  </div>
</template>

<style scoped>
.post-document-name {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  min-width: 0;
  white-space: nowrap;
}
.post-document-name > span {
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
.post-document-full-name { display: none; }
.post-document-button {
  display: inline-grid;
  place-items: center;
  flex: none;
  width: 28px;
  height: 28px;
  padding: 0;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--warning) 45%, var(--border));
  background: color-mix(in srgb, var(--warning) 8%, var(--surface));
  color: color-mix(in srgb, var(--warning) 48%, var(--text));
  cursor: pointer;
}
.post-document-button:focus-visible { outline: 2px solid var(--accent); }
@media (max-width: 899.98px) {
  .post-document-surname { display: none; }
  .post-document-full-name { display: inline; }
  .post-document-button { width: 32px; height: 32px; }
}
</style>
