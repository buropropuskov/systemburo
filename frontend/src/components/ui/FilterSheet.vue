<template>
  <BaseModal
    :show="show"
    :title="title"
    width="600px"
    radius="30px"
    content-class="filter-sheet"
    sheet-swipe
    @close="$emit('close')"
  >
    <div class="filter-sheet__body">
      <slot />
    </div>

    <template #actions>
      <slot name="actions">
        <button
          class="filter-sheet__reset"
          data-testid="filter-sheet-reset"
          :disabled="!hasActiveFilters"
          @click="$emit('reset')"
        >
          Сбросить фильтры
        </button>
      </slot>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';

/**
 * Общий bottom-sheet для сворачивания фильтров в кнопку «Фильтр» на мобилке
 * (эпик mobile-filter-collapse). Тонкая обёртка над BaseModal (контракт закрытия -
 * крестик/overlay/Escape/свайп/блокировка фона - берётся из BaseModal, sheet-swipe).
 * Страница кладёт свои фильтры в default-слот и биндит их к своему состоянию;
 * компонент остаётся dumb view (только рендер + эмиты close/reset). Кнопку «Фильтр»
 * с точкой-индикатором активных фильтров рисует сама страница в шапке.
 *
 * Слот `#actions` переопределяет нижнюю панель, если нужна не только кнопка сброса.
 */
export default {
  name: 'FilterSheet',
  components: { BaseModal },
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: 'Фильтры',
    },
    // Гейтит доступность кнопки «Сбросить фильтры» (страница считает, что активно).
    hasActiveFilters: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['close', 'reset'],
};
</script>

<style scoped>
.filter-sheet__body {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
}

/* Секция-обёртка для подписи + контрола фильтра (страница использует по желанию). */
.filter-sheet__body :deep(.filter-section) {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.filter-sheet__body :deep(.filter-label) {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
  white-space: nowrap;
}

.filter-sheet__reset {
  padding: 7px 14px;
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  background: var(--surface);
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  height: 32px;
  color: var(--danger-text);
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.filter-sheet__reset:hover:not(:disabled) {
  background: var(--color-danger);
  border-color: var(--danger);
  color: var(--fill-text);
}

.filter-sheet__reset:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
