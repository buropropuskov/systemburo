<template>
  <button
    type="button"
    class="filter-btn"
    :class="{ 'filter-btn--active': active }"
    @click="$emit('click')"
  >
    <svg
      class="filter-btn__icon"
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M22 3H2l8 9.46V19l4 2v-8.54L22 3z" />
    </svg>
    {{ label }}
    <span
      v-if="active"
      class="filter-btn__dot"
      aria-hidden="true"
    />
  </button>
</template>

<script>
/**
 * Кнопка-триггер «Фильтр» для сворачивания фильтров в bottom-sheet на мобилке
 * (эпик mobile-filter-collapse). Извлечена 1:1 из ApplicationsCenter (эталон):
 * та же воронка-иконка, классы и точка-индикатор активных фильтров - чтобы вид не
 * расходился между страницами. Открытие sheet - на стороне страницы (эмит click).
 * `active` = есть активные вторичные фильтры (точка-индикатор).
 */
export default {
  name: 'FilterButton',
  props: {
    active: {
      type: Boolean,
      default: false,
    },
    label: {
      type: String,
      default: 'Фильтр',
    },
  },
  emits: ['click'],
};
</script>

<style scoped>
.filter-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 36px;
  padding: 0 16px;
  border: 1px solid var(--color-border);
  background: var(--surface);
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
  white-space: nowrap;
  flex-shrink: 0;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.filter-btn:hover {
  background: var(--color-bg);
  border-color: var(--accent);
  color: var(--accent-text);
}

.filter-btn--active {
  border-color: var(--accent);
  color: var(--accent-text);
}

.filter-btn__icon {
  flex-shrink: 0;
}

.filter-btn__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-primary);
  flex-shrink: 0;
}
</style>
