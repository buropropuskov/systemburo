<template>
  <BaseModal
    :show="show"
    title="Фильтры"
    width="600px"
    radius="30px"
    content-class="filter-modal"
    sheet-swipe
    @close="$emit('close')"
  >
    <div class="filter-modal__body">
      <!-- Справочники мультивыбором (#1398): те же конфиги, что в шапке десктопа,
           поэтому набор фильтров не расходится между раскладками. -->
      <div
        v-for="filter in directoryFilters"
        :key="filter.field"
        class="filter-section"
      >
        <div class="filter-section__header">
          <span class="filter-label">{{ filter.summaryLabel }}</span>
        </div>
        <BaseDropdown
          :model-value="filter.values"
          :options="filter.options"
          :placeholder="filter.allLabel"
          :summary-label="filter.summaryLabel"
          :data-testid="filter.testid"
          multiple
          searchable
          teleport
          @update:model-value="values => $emit('set-filter', filter.field, values)"
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
        <!-- «Обновления» здесь больше нет: переключатель переехал в шапку Центра, где
             он виден и на мобилке. Оставлять копию значило держать два контрола на один
             флаг в одном вьюпорте и два узла с data-testid="center-button-updates". -->
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

      <div
        v-for="filter in stateFilters"
        :key="filter.field"
        class="filter-section"
      >
        <div class="filter-section__header">
          <span class="filter-label">{{ filter.summaryLabel }}</span>
        </div>
        <BaseDropdown
          :model-value="filter.values"
          :options="filter.options"
          :placeholder="filter.allLabel"
          :summary-label="filter.summaryLabel"
          label-key="label"
          value-key="value"
          :data-testid="filter.testid"
          multiple
          teleport
          @update:model-value="values => $emit('set-filter', filter.field, values)"
        />
      </div>

      <!-- Сортировка: на мобилке шапка-таблицы скрыта (rt-head-row display:none),
           поэтому выбор поля/направления вынесен сюда. Клик по активному полю
           переключает направление (та же логика, что sortBy по клику колонки). -->
      <div class="filter-section">
        <div class="filter-section__header">
          <span class="filter-label">Сортировка</span>
        </div>
        <div class="status-buttons">
          <button
            v-for="option in sortOptions"
            :key="option.value"
            class="status-btn sort-btn"
            :class="{ 'status-btn--active': sortField === option.value }"
            :data-testid="`center-button-sort-${option.value}`"
            @click="$emit('sort-by', option.value)"
          >
            {{ option.label }}
            <span
              v-if="sortField === option.value"
              class="sort-btn__dir"
              aria-hidden="true"
            >{{ sortDirection === 'asc' ? '↑' : '↓' }}</span>
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
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

/**
 * Вторичные фильтры Центра заявок, вынесенные из шапки в модалку (#1097 W3.6).
 * Dumb view: только рендер и проброс событий - вся логика фильтрации/сортировки
 * остаётся в ApplicationsCenter (обработчики привязаны к эмитам этой модалки).
 */
export default {
  name: 'ApplicationsFilterModal',
  components: { BaseModal, DateFilter, BaseDropdown },
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    // Конфиги мультивыборных фильтров приходят готовыми из ApplicationsCenter (#1398):
    // {field, values, options, allLabel, summaryLabel, testid}. Модалка остаётся dumb view
    // и не знает, что именно фильтруется - меняется только в одном месте.
    directoryFilters: {
      type: Array,
      default: () => [],
    },
    stateFilters: {
      type: Array,
      default: () => [],
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
    sortField: {
      type: String,
      default: null,
    },
    sortDirection: {
      type: String,
      default: 'desc',
    },
    hasActiveFilters: {
      type: Boolean,
      default: false,
    },
  },
  emits: [
    'close',
    'set-filter',
    'update:selected-date',
    'update:date-range-start',
    'update:date-range-end',
    'apply-date',
    'clear-date',
    'toggle-today',
    'sort-by',
    'reset-sort',
    'reset-filters',
  ],
  data() {
    return {
      // Поля сортировки = кликабельные колонки таблицы Центра (десктоп), скрытой на мобилке.
      sortOptions: [
        { value: 'date', label: 'Дата' },
        { value: 'number', label: 'Номер' },
        { value: 'organization', label: 'Организация' },
        { value: 'sender', label: 'Отправитель' },
        { value: 'status', label: 'Статус' },
        { value: 'confirmation', label: 'Подтверждение' },
      ],
    };
  },
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

.sort-btn__dir {
  margin-left: 4px;
  font-weight: 700;
}

.filter-label {
  font-size: 12px;
  color: var(--text-muted);
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
  background: var(--surface);
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
  border-color: var(--accent);
  color: var(--accent-text);
}

.status-btn--active {
  background: var(--color-primary);
  color: var(--accent-contrast);
  border-color: var(--accent);
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
  background: var(--surface);
  border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
  color: var(--danger-text);
}

.reset-filters-btn:hover:not(:disabled) {
  background: var(--color-danger);
  border-color: var(--danger);
  color: var(--fill-text);
}
</style>
