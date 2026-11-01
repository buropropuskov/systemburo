<script setup>
import { computed } from 'vue';
import AppIcon from '@/components/icons/AppIcon.vue';
const props = defineProps({
  field: { type: String, required: true },
  label: { type: String, required: true },
  sortField: { type: [String, null], required: true },
  sortDirection: { type: String, required: true },
});
const emit = defineEmits(['sort']);
const active = computed(() => props.sortField === props.field);
const description = computed(() => props.label + ': ' + (active.value
  ? (props.sortDirection === 'asc' ? 'по возрастанию' : 'по убыванию') : 'сортировка не выбрана'));
</script>
<template>
  <button
    type="button"
    class="header-col"
    :data-field="field"
    :aria-label="description"
    @click="emit('sort', field)"
  >
    <span :class="{ 'active-sort': active }">{{ label }}</span>
    <AppIcon
      name="sort"
      class="sort-icon"
      :class="{ sorted: active, desc: active && sortDirection === 'desc' }"
    />
  </button>
</template>
<style scoped>
button { border: 0; background: none; font: inherit; }
button:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
.sort-icon { color: var(--text-muted); width: 12px; height: 12px; transition: .2s; }
</style>
