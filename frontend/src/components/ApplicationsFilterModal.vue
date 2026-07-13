<template>
  <BaseModal
    :show="show"
    title="Фильтры"
    width="600px"
    radius="30px"
    content-class="filter-modal"
    @close="$emit('close')"
  >
    <div class="filter-modal__body">
      <div class="filter-section">
        <div class="filter-section__header">
          <span class="filter-label">Организация</span>
        </div>
        <OrganizationFilter
          :value="selectedOrganizationId"
          :organizations="organizations"
          @change="$emit('organization-change', $event)"
        />
      </div>

      <div class="filter-section filter-section--date">
        <div class="filter-section__header">
          <span class="filter-label">Дата</span>
        </div>
        <DateFilter
          ref="dateFilter"
          mode="range"
          :selected-date="selectedDate"
          :date-range-start="dateRangeStart"
          :date-range-end="dateRangeEnd"
          @update:selected-date="$emit('update:selected-date', $event)"
          @update:date-range-start="$emit('update:date-range-start', $event)"
          @update:date-range-end="$emit('update:date-range-end', $event)"
          @apply="$emit('apply-date')"
          @clear="$emit('clear-date')"
        />
      </div>

      <div class="filter-section">
        <div class="filter-section__header">
          <span class="filter-label">Заявки</span>
        </div>
        <div class="status-buttons">
          <button
            class="status-btn"
            :class="{ 'status-btn--active': activeToday }"
            data-testid="center-button-today"
            @click="$emit('toggle-today')"
          >
            Заявки на сегодня
          </button>
        </div>
      </div>

      <div class="filter-section">
        <div class="filter-section__header">
          <span class="filter-label">Подтверждение</span>
        </div>
        <div class="status-buttons">
          <button
            v-for="confirmation in confirmations"
            :key="confirmation.value"
            class="status-btn"
            :class="{ 'status-btn--active': selectedConfirmations.includes(confirmation.value) }"
            :data-testid="`center-button-confirmation-${confirmation.value}`"
            @click="$emit('toggle-confirmation', confirmation.value)"
          >
            {{ confirmation.label }}
          </button>
        </div>
      </div>

      <div class="filter-section">
        <div class="filter-section__header">
          <span class="filter-label">Статус заявки</span>
        </div>
        <div class="status-buttons">
          <button
            v-for="status in applicationStatuses"
            :key="status.value"
            class="status-btn"
            :class="{ 'status-btn--active': selectedApplicationStatuses.includes(status.value) }"
            :data-testid="`center-button-status-${status.value}`"
            @click="$emit('toggle-status', status.value)"
          >
            {{ status.label }}
          </button>
        </div>
      </div>

      <div class="filter-section">
        <div class="filter-section__header">
          <span class="filter-label">Теги</span>
        </div>
        <div class="status-buttons">
          <button
            v-for="tag in tags"
            :key="tag.value"
            class="status-btn"
            :class="{ 'status-btn--active': selectedTags.includes(tag.value) }"
            :data-testid="`center-button-tag-${tag.value}`"
            @click="$emit('toggle-tag', tag.value)"
          >
            {{ tag.label }}
          </button>
        </div>
      </div>
    </div>

    <template #actions>
      <button
        class="reset-sort-btn"
        :disabled="!sortField"
        @click="$emit('reset-sort')"
      >
        Сбросить сортировку
      </button>
      <button
        class="reset-filters-btn"
        data-testid="center-button-reset-filters"
        :disabled="!hasActiveFilters"
        @click="$emit('reset-filters')"
      >
        Сбросить фильтры
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import DateFilter from '@/components/DateFilter.vue';
import OrganizationFilter from '@/components/OrganizationFilter.vue';

/**
 * Вторичные фильтры Центра заявок, вынесенные из шапки в модалку (#1097 W3.6).
 * Dumb view: только рендер и проброс событий - вся логика фильтрации/сортировки
 * остаётся в ApplicationsCenter (обработчики привязаны к эмитам этой модалки).
 */
export default {
  name: 'ApplicationsFilterModal',
  components: { BaseModal, DateFilter, OrganizationFilter },
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    organizations: {
      type: Array,
      default: () => [],
    },
    selectedOrganizationId: {
      type: [Number, String],
      default: null,
    },
    selectedDate: {
      type: Date,
      default: null,
    },
    dateRangeStart: {
      type: Date,
      default: null,
    },
    dateRangeEnd: {
      type: Date,
      default: null,
    },
    activeToday: {
      type: Boolean,
      default: false,
    },
    confirmations: {
      type: Array,
      default: () => [],
    },
    selectedConfirmations: {
      type: Array,
      default: () => [],
    },
    applicationStatuses: {
      type: Array,
      default: () => [],
    },
    selectedApplicationStatuses: {
      type: Array,
      default: () => [],
    },
    tags: {
      type: Array,
      default: () => [],
    },
    selectedTags: {
      type: Array,
      default: () => [],
    },
    sortField: {
      type: String,
      default: null,
    },
    hasActiveFilters: {
      type: Boolean,
      default: false,
    },
  },
  emits: [
    'close',
    'organization-change',
    'update:selected-date',
    'update:date-range-start',
    'update:date-range-end',
    'apply-date',
    'clear-date',
    'toggle-today',
    'toggle-confirmation',
    'toggle-status',
    'toggle-tag',
    'reset-sort',
    'reset-filters',
  ],
  methods: {
    /**
     * Полный сброс внутреннего состояния DateFilter (в т.ч. подсветки быстрых периодов).
     * Зовётся родителем из resetFilters: сброс date-пропсов в null уже гасит отображение
     * через immediate-watcher'ы DateFilter, но activeQuickDate обнуляет только clearSelection.
     */
    clearDateFilter() {
      if (this.$refs.dateFilter && this.$refs.dateFilter.clearSelection) {
        this.$refs.dateFilter.clearSelection();
      }
    },
  },
};
</script>

<style scoped>
.filter-modal__body {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
}

.filter-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.filter-section__header {
  display: flex;
  align-items: center;
}

.filter-label {
  font-size: 12px;
  color: #666;
  font-weight: 500;
  white-space: nowrap;
}

.status-buttons {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.status-btn,
.reset-sort-btn,
.reset-filters-btn {
  padding: 7px 14px;
  border: 1px solid var(--color-border);
  background: white;
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
  height: 32px;
  color: var(--color-text);
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
}

.status-btn:hover:not(.status-btn--active),
.reset-sort-btn:hover:not(:disabled) {
  background: var(--color-bg);
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.status-btn--active {
  background: var(--color-primary);
  color: white;
  border-color: var(--color-primary);
}

.status-btn--active:hover {
  background: var(--color-primary-hover);
  border-color: var(--color-primary-hover);
}

.reset-sort-btn:disabled,
.reset-filters-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.reset-filters-btn {
  background: #fff;
  border-color: #fecaca;
  color: var(--color-danger);
}

.reset-filters-btn:hover:not(:disabled) {
  background: var(--color-danger);
  border-color: var(--color-danger);
  color: #fff;
}
</style>
