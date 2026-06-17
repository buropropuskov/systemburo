<template>
  <div class="rb">
    <!-- Тип отчёта -->
    <div class="rb__type">
      <span class="rb__section-label">Тип отчёта</span>
      <FilterTabs
        v-model="form.mode"
        :tabs="modeTabs"
      />
      <span class="rb__hint">{{ form.mode === 'aggregate'
        ? 'Посчитать показатель в выбранном разрезе.'
        : 'Выгрузить строки сущности столбцами.' }}</span>
    </div>

    <!-- Параметры -->
    <div class="rb__grid">
      <template v-if="form.mode === 'aggregate'">
        <label class="rb__field">
          <span class="rb__field-label">Что считаем</span>
          <BaseDropdown
            v-model="form.metric"
            :options="catalog.metrics"
            label-key="label"
            value-key="key"
          />
        </label>
        <label class="rb__field">
          <span class="rb__field-label">Разрез</span>
          <BaseDropdown
            v-model="form.dimension"
            :options="dimensionOptions"
            label-key="label"
            value-key="key"
          />
        </label>
        <label
          v-if="form.dimension === 'period'"
          class="rb__field"
        >
          <span class="rb__field-label">Шаг</span>
          <BaseDropdown
            v-model="form.granularity"
            :options="catalog.granularities"
            label-key="label"
            value-key="value"
          />
        </label>
      </template>

      <label
        v-else
        class="rb__field"
      >
        <span class="rb__field-label">Что выгружаем</span>
        <BaseDropdown
          v-model="form.entity"
          :options="catalog.list_entities"
          label-key="label"
          value-key="key"
        />
      </label>
    </div>

    <!-- list-фильтры из каталога (date_range = период из шапки, не дублируем) -->
    <div
      v-if="listFilterFields.length"
      class="rb__filters"
    >
      <div
        v-for="f in listFilterFields"
        :key="f.key"
        class="rb__filter"
      >
        <span class="rb__field-label">{{ f.label }}</span>
        <div class="rb__pills">
          <button
            v-for="opt in f.options"
            :key="opt.value"
            type="button"
            class="rb__pill"
            :class="{ 'rb__pill--on': isSelected(f.key, opt.value) }"
            @click="toggleFilter(f.key, opt.value)"
          >
            {{ opt.label }}
          </button>
          <span
            v-if="!f.options.length"
            class="rb__pills-empty"
          >нет значений в справочнике</span>
        </div>
      </div>
    </div>

    <!-- Итог + действия -->
    <div class="rb__footer">
      <p class="rb__summary">
        {{ summary }}
      </p>
      <div class="rb__actions">
        <label class="rb__limit">
          <span>Строк</span>
          <input
            v-model="form.limit"
            type="number"
            min="1"
            max="1000"
            placeholder="100"
            class="lk-input rb__limit-input"
          >
        </label>
        <button
          type="button"
          class="lk-button lk-button--primary"
          :disabled="!canRun || loading"
          @click="run"
        >
          {{ loading ? 'Строим…' : 'Построить отчёт' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, computed, watch } from 'vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { buildReportRequest } from '@/composables/useReportRequest';

const props = defineProps({
  catalog: { type: Object, required: true },
  period: { type: Object, default: () => ({ from: '', to: '' }) },
  loading: { type: Boolean, default: false },
});

const emit = defineEmits(['run']);

const modeTabs = [
  { key: 'aggregate', label: 'Сводка по разрезу' },
  { key: 'list', label: 'Выгрузка строк' },
];

const form = reactive({
  mode: 'aggregate',
  metric: props.catalog.metrics?.[0]?.key || '',
  dimension: '',
  granularity: 'day',
  entity: props.catalog.list_entities?.[0]?.key || '',
  filters: {},
  limit: '',
});

const filterByKey = computed(() => {
  const map = {};
  for (const f of props.catalog.filters || []) map[f.key] = f;
  return map;
});

const selectedMetric = computed(
  () => (props.catalog.metrics || []).find((m) => m.key === form.metric) || null,
);

const dimensionOptions = computed(() => {
  const dims = selectedMetric.value?.dimensions || [];
  return (props.catalog.dimensions || []).filter((d) => dims.includes(d.key));
});

const selectedEntity = computed(
  () => (props.catalog.list_entities || []).find((e) => e.key === form.entity) || null,
);

const listFilterFields = computed(() => {
  if (form.mode !== 'list') return [];
  const keys = selectedEntity.value?.filters || [];
  return keys
    .filter((k) => k !== 'date_range')
    .map((k) => filterByKey.value[k])
    .filter(Boolean)
    .map((f) => ({ ...f, options: f.options || [] }));
});

// Aggregate per-metric фильтры пока не в каталоге -> отдаём только период (срез B3b).
const applicableFilters = computed(() =>
  form.mode === 'list' ? selectedEntity.value?.filters || [] : ['date_range'],
);

// Разрез должен принадлежать выбранной метрике — иначе движок отклонит запрос.
watch(selectedMetric, (m) => {
  if (!m) {
    form.dimension = '';
    return;
  }
  if (!m.dimensions.includes(form.dimension)) {
    form.dimension = m.dimensions[0] || '';
  }
}, { immediate: true });

// Смена выгружаемой сущности обнуляет выбранные фильтры — у разных сущностей
// разный набор применимых фильтров, чужие значения сбивали бы с толку.
watch(() => form.entity, () => {
  form.filters = {};
});

const canRun = computed(() =>
  form.mode === 'aggregate'
    ? Boolean(form.metric && form.dimension)
    : Boolean(form.entity),
);

const periodLabel = computed(() => {
  const { from, to } = props.period || {};
  if (from && to) return `${from} — ${to}`;
  if (from) return `с ${from}`;
  if (to) return `по ${to}`;
  return 'весь период';
});

const summary = computed(() => {
  if (form.mode === 'aggregate') {
    const metric = selectedMetric.value?.label || '—';
    const dim = dimensionOptions.value.find((d) => d.key === form.dimension)?.label || '—';
    return `Считаем «${metric}» в разрезе «${dim}», период: ${periodLabel.value}.`;
  }
  // Период упоминаем только если сущность его поддерживает (машины/люди — нет).
  const supportsPeriod = (selectedEntity.value?.filters || []).includes('date_range');
  const periodPart = supportsPeriod ? `, период: ${periodLabel.value}` : '';
  return `Выгрузка строк: «${selectedEntity.value?.label || '—'}»${periodPart}.`;
});

function isSelected(key, value) {
  return (form.filters[key] || []).includes(value);
}

function toggleFilter(key, value) {
  const current = form.filters[key] ? [...form.filters[key]] : [];
  const idx = current.indexOf(value);
  if (idx >= 0) current.splice(idx, 1);
  else current.push(value);
  form.filters = { ...form.filters, [key]: current };
}

function run() {
  emit('run', buildReportRequest(form, props.period, applicableFilters.value));
}
</script>

<style scoped>
.rb {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.rb__section-label,
.rb__field-label {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: var(--color-text-muted);
  text-transform: uppercase;
}

.rb__type {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rb__hint {
  font-size: 13px;
  color: var(--color-text-muted);
}

.rb__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.rb__field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.rb__filters {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 4px;
}

.rb__filter {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.rb__pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.rb__pill {
  padding: 5px 13px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-pill);
  font-size: 13px;
  font-family: inherit;
  color: var(--color-text);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, border-color 0.18s ease;
}

.rb__pill:hover {
  border-color: var(--color-primary);
}

.rb__pill--on {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.rb__pills-empty {
  font-size: 13px;
  color: var(--color-text-muted);
  font-style: italic;
}

.rb__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding-top: 4px;
  border-top: 1px solid var(--color-border);
}

.rb__summary {
  margin: 0;
  font-size: 14px;
  color: var(--color-text);
  flex: 1;
  min-width: 240px;
}

.rb__actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

.rb__limit {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--color-text-muted);
}

.rb__limit-input {
  width: 84px;
}
</style>
