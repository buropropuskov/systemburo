<template>
  <div
    class="empty-state"
    data-testid="aa-empty"
  >
    <p>{{ hasActiveFilters ? 'Ничего не найдено' : 'Доступных вложений нет' }}</p>
    <span class="empty-state__hint">
      {{ hasActiveFilters
        ? 'Измените условия поиска или сбросьте фильтры.'
        : 'Здесь появятся вложения подтверждённых заявок по вашим местам доступа.' }}
    </span>
    <button
      v-if="hasActiveFilters"
      type="button"
      class="lk-button lk-button--secondary"
      data-testid="aa-empty-reset"
      @click="$emit('reset')"
    >
      Сбросить фильтры
    </button>
  </div>
</template>

<script setup>
defineProps({ hasActiveFilters: { type: Boolean, required: true } });
defineEmits(['reset']);
</script>

<style scoped>
.empty-state {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 20px;
  text-align: center;
}

.empty-state p {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.empty-state__hint {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 320px;
}
</style>
