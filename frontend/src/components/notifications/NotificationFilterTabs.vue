<template>
  <div class="notif-filter-tabs">
    <button
      type="button"
      class="notif-filter-tabs__btn"
      :class="{ active: modelValue === 'all' }"
      data-testid="notif-filter-all"
      @click="emit('update:modelValue', 'all')"
    >
      Все
    </button>
    <button
      type="button"
      class="notif-filter-tabs__btn"
      :class="{ active: modelValue === 'unread' }"
      data-testid="notif-filter-unread"
      @click="emit('update:modelValue', 'unread')"
    >
      Непрочитанные
    </button>
  </div>
</template>

<script setup>
// Фильтр Все/Непрочитанные (#1748 S7) - раньше был только в UserNotificationsInline.vue,
// теперь общий для колокольчика в шапке и блока личного кабинета: разметка и стили
// в одном месте, не дублируются. Фильтр только эмитит выбор - запрос за отфильтрованными
// данными делает родитель (v-model управляет параметром filter в fetchPage).
defineProps({
  modelValue: {
    type: String,
    default: 'all',
    validator: (v) => ['all', 'unread'].includes(v),
  },
});
const emit = defineEmits(['update:modelValue']);
</script>

<style scoped>
.notif-filter-tabs {
  display: flex;
  gap: 4px;
}

.notif-filter-tabs__btn {
  background: none;
  border: none;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px 12px;
  border-radius: 30px;
  transition: all 0.2s ease;
  font-weight: 500;
  font-family: inherit;
}

.notif-filter-tabs__btn.active {
  color: var(--accent-text);
  background: color-mix(in srgb, var(--accent) 8%, var(--surface));
}

.notif-filter-tabs__btn:hover:not(.active) {
  color: var(--text-muted);
  background: var(--surface-2);
}
</style>
