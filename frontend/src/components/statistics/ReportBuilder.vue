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
        ? 'Посчитать один или несколько показателей в выбранном разрезе.'
        : 'Выгрузить строки сущности столбцами.' }}</span>
    </div>

    <template v-if="form.mode === 'aggregate'">
      <!-- Шаг 1: что считаем (мультивыбор метрик карточками, сгруппированы) -->
      <div class="rb__step">
        <div class="rb__step-head">
          <span class="rb__step-num">1</span>
          <span class="rb__step-name">Что считаем</span>
        </div>
        <template
          v-for="grp in metricGroups"
          :key="grp.group"
        >
          <div class="rb__group-title">
            {{ grp.group }}
          </div>
          <div class="rb__metrics">
            <label
              v-for="m in grp.metrics"
              :key="m.key"
              class="rb__metric"
              :class="{ 'rb__metric--on': isMetricOn(m.key) }"
            >
              <input
                type="checkbox"
                class="rb__metric-input"
                :checked="isMetricOn(m.key)"
                @change="toggleMetric(m.key)"
              >
              <span class="rb__metric-box">
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="3"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                ><path d="M20 6 9 17l-5-5" /></svg>
              </span>
              <span class="rb__metric-text">
                <span class="rb__metric-name">{{ m.label }}</span>
                <span
                  v-if="m.unit"
                  class="rb__metric-unit"
                >{{ m.unit }}</span>
              </span>
            </label>
          </div>
        </template>
      </div>

      <!-- Шаг 2: по чему разбиваем (радио-сетка, «без разреза» всегда доступен) -->
      <div class="rb__step">
        <div class="rb__step-head">
          <span class="rb__step-num">2</span>
          <span class="rb__step-name">По чему разбиваем</span>
        </div>
        <div class="rb__dims">
          <label
            v-for="d in availableDimensions"
            :key="d.key"
            class="rb__dim"
            :class="{ 'rb__dim--on': form.dimension === d.key }"
          >
            <input
              v-model="form.dimension"
              type="radio"
              name="rb-dimension"
              class="rb__dim-input"
              :value="d.key"
            >
            <span class="rb__dim-dot" />
            {{ d.label }}
          </label>
        </div>
        <div
          v-if="form.dimension === 'period'"
          class="rb__gran"
        >
          <span class="rb__field-label">Шаг по времени</span>
          <FilterTabs
            v-model="form.granularity"
            :tabs="granularityTabs"
          />
        </div>
      </div>
    </template>

    <!-- list-режим: что выгружаем -->
    <div
      v-else
      class="rb__grid"
    >
      <label class="rb__field">
        <span class="rb__field-label">Что выгружаем</span>
        <BaseDropdown
          v-model="form.entity"
          :options="catalog.list_entities"
          label-key="label"
          value-key="key"
        />
      </label>
    </div>

    <!-- Шаг 3: фильтры (чипсы, применимые к выбранным метрикам/сущности) -->
    <div
      v-if="filterFields.length"
      class="rb__step"
    >
      <div class="rb__step-head">
        <span
          v-if="form.mode === 'aggregate'"
          class="rb__step-num"
        >3</span>
        <span class="rb__step-name">Фильтры</span>
        <span class="rb__step-opt">необязательно</span>
      </div>
      <div class="rb__filters">
        <div
          v-for="f in filterFields"
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
    </div>

    <!-- Шаг 4: период (пресеты + произвольный диапазон, локальный для отчёта) -->
    <div
      v-if="periodApplicable"
      class="rb__step"
    >
      <div class="rb__step-head">
        <span
          v-if="form.mode === 'aggregate'"
          class="rb__step-num"
        >4</span>
        <span class="rb__step-name">Период</span>
      </div>
      <div class="rb__period">
        <div class="rb__period-presets">
          <button
            v-for="p in periodPresets"
            :key="p.key"
            type="button"
            class="rb__pill"
            :class="{ 'rb__pill--on': activePeriodPreset === p.key }"
            @click="applyPeriodPreset(p.key)"
          >
            {{ p.label }}
          </button>
        </div>
        <div class="rb__period-dates">
          <label class="rb__date">
            <span class="rb__field-label">С</span>
            <input
              v-model="form.period.from"
              type="date"
              class="lk-input"
              @change="activePeriodPreset = 'custom'"
            >
          </label>
          <label class="rb__date">
            <span class="rb__field-label">По</span>
            <input
              v-model="form.period.to"
              type="date"
              class="lk-input"
              @change="activePeriodPreset = 'custom'"
            >
          </label>
        </div>
      </div>
    </div>

    <!-- Превью «что построим» -->
    <div class="rb__preview">
      <span class="rb__preview-label">Что построим</span>
      <p class="rb__summary">
        {{ summary }}
      </p>
      <p class="rb__preview-result">
        {{ resultHint }}
      </p>
    </div>

    <!-- Действия -->
    <div class="rb__footer">
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
</template>

<script setup>
import { reactive, ref, computed, watch, nextTick } from 'vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { buildReportRequest } from '@/composables/useReportRequest';

const props = defineProps({
  catalog: { type: Object, required: true },
  // Начальный период (из шапки фильтров). Дальше отчёт ведёт период локально в
  // шаге 4 — дашборд и отчёт могут смотреть на разные диапазоны.
  period: { type: Object, default: () => ({ from: '', to: '' }) },
  loading: { type: Boolean, default: false },
  // Пресет из галереи: { mode, metric?/metrics?, dimension?, granularity?, entity? }.
  // Заполняет конструктор и сразу строит отчёт. null — пресет не выбран.
  preset: { type: Object, default: null },
});

const emit = defineEmits(['run', 'change']);

const modeTabs = [
  { key: 'aggregate', label: 'Сводка по разрезу' },
  { key: 'list', label: 'Выгрузка строк' },
];

const form = reactive({
  mode: 'aggregate',
  metrics: props.catalog.metrics?.[0]?.key ? [props.catalog.metrics[0].key] : [],
  dimension: '',
  granularity: 'day',
  entity: props.catalog.list_entities?.[0]?.key || '',
  filters: {},
  period: { from: props.period?.from || '', to: props.period?.to || '' },
  limit: '',
});

const periodPresets = [
  { key: 'week', label: 'Эта неделя' },
  { key: 'month', label: 'Этот месяц' },
  { key: 'year', label: 'Этот год' },
  { key: 'all', label: 'Весь период' },
];
const activePeriodPreset = ref('');

const filterByKey = computed(() => {
  const map = {};
  for (const f of props.catalog.filters || []) map[f.key] = f;
  return map;
});

// Метрики, разложенные по группам каталога (group), порядок групп — по первому
// появлению. Карточки шага «Что считаем» строятся отсюда, не из мокапа.
const metricGroups = computed(() => {
  const order = [];
  const byGroup = {};
  for (const m of props.catalog.metrics || []) {
    const g = m.group || 'Прочее';
    if (!byGroup[g]) {
      byGroup[g] = [];
      order.push(g);
    }
    byGroup[g].push(m);
  }
  return order.map((g) => ({ group: g, metrics: byGroup[g] }));
});

const selectedMetrics = computed(
  () => (props.catalog.metrics || []).filter((m) => form.metrics.includes(m.key)),
);

// «Без разреза» применим к любой метрике (движок валидирует его отдельной веткой),
// поэтому всегда доступен; берётся из каталога, с фолбэком на случай отсутствия.
const dimNoneOption = computed(
  () => (props.catalog.dimensions || []).find((d) => d.key === 'none') || { key: 'none', label: 'Без разреза' },
);

// Доступные разрезы: «без разреза» + пересечение dimensions всех выбранных метрик
// (общий разрез корректен для каждой), в порядке каталога.
const availableDimensions = computed(() => {
  const sel = selectedMetrics.value;
  if (!sel.length) return [dimNoneOption.value];
  let common = null;
  for (const m of sel) {
    const set = new Set(m.dimensions || []);
    common = common === null ? set : new Set([...common].filter((k) => set.has(k)));
  }
  const realDims = (props.catalog.dimensions || []).filter(
    (d) => d.key !== 'none' && common.has(d.key),
  );
  return [dimNoneOption.value, ...realDims];
});

const selectedEntity = computed(
  () => (props.catalog.list_entities || []).find((e) => e.key === form.entity) || null,
);

// Применимые фильтры текущего среза: list — фильтры сущности, aggregate —
// объединение per-metric фильтров выбранных метрик (каталог B3d, движок применяет
// каждый к тем метрикам, что его поддерживают). Включает date_range.
const applicableFilters = computed(() => {
  if (form.mode === 'list') return selectedEntity.value?.filters || [];
  const set = new Set();
  for (const m of selectedMetrics.value) (m.filters || []).forEach((k) => set.add(k));
  // Порядок — по каталогу фильтров, чтобы чипсы шли стабильно.
  return (props.catalog.filters || []).map((f) => f.key).filter((k) => set.has(k));
});

// Поля-чипсы шага 3: применимые фильтры без date_range (период — отдельный шаг).
const filterFields = computed(() =>
  applicableFilters.value
    .filter((k) => k !== 'date_range')
    .map((k) => filterByKey.value[k])
    .filter(Boolean)
    .map((f) => ({ ...f, options: f.options || [] })));

const periodApplicable = computed(() => applicableFilters.value.includes('date_range'));

const granularityTabs = computed(
  () => (props.catalog.granularities || []).map((g) => ({ key: g.value, label: g.label })),
);

// Разрез должен оставаться валидным для текущего набора метрик. При невалидном
// дефолтим на первый реальный разрез (полезнее «без разреза»), иначе на none.
watch(availableDimensions, (dims) => {
  if (dims.some((d) => d.key === form.dimension)) return;
  const firstReal = dims.find((d) => d.key !== 'none');
  form.dimension = (firstReal || dims[0]).key;
}, { immediate: true });

// Смена выгружаемой сущности обнуляет выбранные фильтры — у разных сущностей
// разный набор применимых фильтров, чужие значения сбивали бы с толку.
watch(() => form.entity, () => {
  form.filters = {};
});

// При смене метрик (aggregate) убираем значения фильтров, ставших неприменимыми,
// чтобы не слать в движок фильтр по метрике, которая его не поддерживает.
watch(applicableFilters, (keys) => {
  const allowed = new Set(keys);
  const next = {};
  let changed = false;
  for (const [k, v] of Object.entries(form.filters)) {
    if (allowed.has(k)) next[k] = v;
    else changed = true;
  }
  if (changed) form.filters = next;
});

const canRun = computed(() =>
  form.mode === 'aggregate'
    ? form.metrics.length > 0 && Boolean(form.dimension)
    : Boolean(form.entity));

const periodLabel = computed(() => {
  const { from, to } = form.period;
  if (from && to) return `${from} — ${to}`;
  if (from) return `с ${from}`;
  if (to) return `по ${to}`;
  return 'весь период';
});

const summary = computed(() => {
  // Период упоминаем только когда срез его поддерживает (есть date_range в applicable).
  const periodPart = periodApplicable.value ? `, период: ${periodLabel.value}` : '';
  if (form.mode === 'aggregate') {
    const names = selectedMetrics.value.map((m) => m.label).join(', ') || '—';
    const dim = availableDimensions.value.find((d) => d.key === form.dimension)?.label || '—';
    return `Считаем: ${names}; разрез «${dim}»${periodPart}.`;
  }
  return `Выгрузка строк: «${selectedEntity.value?.label || '—'}»${periodPart}.`;
});

const resultHint = computed(() => {
  if (form.mode === 'aggregate') {
    const n = selectedMetrics.value.length;
    // «Без разреза» -> один итоговый ряд без столбца разреза, поэтому формулировка
    // отличается от разбивки по реальному разрезу.
    if (form.dimension === 'none') {
      return n > 1
        ? `Результат: итоговая строка с ${n} столбцами-метриками.`
        : 'Результат: одно итоговое значение.';
    }
    return n > 1
      ? `Результат: таблица «разрез + ${n} столбца-метрики» с итогами.`
      : 'Результат: таблица «разрез + значение» с итогом.';
  }
  const cols = (selectedEntity.value?.columns || []).length;
  return cols
    ? `Результат: таблица строк, ${cols} столбцов.`
    : 'Результат: таблица строк.';
});

// Применить пресет из галереи и сразу построить отчёт. Разрез сверяется watch'ем
// availableDimensions (пресеты используют валидные ключи каталога).
watch(() => props.preset, (p) => {
  if (!p) return;
  form.mode = p.mode || 'aggregate';
  if (p.mode === 'list') {
    form.entity = p.entity || form.entity;
  } else {
    form.metrics = p.metrics ? [...p.metrics] : (p.metric ? [p.metric] : []);
    form.dimension = p.dimension || '';
    form.granularity = p.granularity || 'day';
  }
  // Пресеты не несут значений фильтров; watch entity их и так обнулит.
  form.filters = {};
  nextTick(run);
});

function isMetricOn(key) {
  return form.metrics.includes(key);
}

function toggleMetric(key) {
  const idx = form.metrics.indexOf(key);
  if (idx >= 0) form.metrics.splice(idx, 1);
  else form.metrics.push(key);
}

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

/**
 * Диапазон дат для пресета периода. «all» -> пустой диапазон (весь период).
 * @param {'week'|'month'|'year'|'all'} kind
 * @returns {{from: string, to: string}}
 */
function computePeriod(kind) {
  if (kind === 'all') return { from: '', to: '' };
  const now = new Date();
  const fmt = (dt) => `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`;
  const to = fmt(now);
  if (kind === 'week') {
    const monday = new Date(now);
    monday.setDate(now.getDate() - ((now.getDay() + 6) % 7)); // Пн = начало недели
    return { from: fmt(monday), to };
  }
  if (kind === 'month') return { from: fmt(new Date(now.getFullYear(), now.getMonth(), 1)), to };
  if (kind === 'year') return { from: fmt(new Date(now.getFullYear(), 0, 1)), to };
  return { from: '', to: '' };
}

function applyPeriodPreset(kind) {
  const range = computePeriod(kind);
  form.period.from = range.from;
  form.period.to = range.to;
  activePeriodPreset.value = kind;
}

// Снимок состояния формы для индикатора шагов мастера (родитель считает по нему
// прогресс). metric — ключ первой выбранной метрики: шаг «что считаем» закрыт,
// когда выбрана хотя бы одна. periodFilled — задан ли диапазон. immediate —
// чтобы степпер был корректен сразу.
const filterCount = computed(
  () => Object.values(form.filters).reduce((n, vals) => n + (vals?.length || 0), 0),
);
watch(
  () => ({
    mode: form.mode,
    metric: form.metrics[0] || '',
    dimension: form.dimension,
    entity: form.entity,
    filterCount: filterCount.value,
    periodApplicable: periodApplicable.value,
    periodFilled: Boolean(form.period.from && form.period.to),
  }),
  (snapshot) => emit('change', snapshot),
  { immediate: true, deep: true },
);

function run() {
  // Защита инварианта: run() зовётся не только кнопкой (ещё из watch пресета),
  // поэтому проверяем canRun здесь, а не полагаемся на :disabled кнопки.
  if (!canRun.value) return;
  emit('run', buildReportRequest(form, form.period, applicableFilters.value));
}
</script>

<style scoped>
.rb {
  display: flex;
  flex-direction: column;
  gap: 22px;
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

/* Шаги мастера */
.rb__step {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.rb__step-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}

.rb__step-num {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  display: grid;
  place-items: center;
}

.rb__step-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
}

.rb__step-opt {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
}

.rb__group-title {
  margin: 14px 0 9px 36px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.rb__group-title:first-of-type {
  margin-top: 2px;
}

.rb__metrics {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-left: 36px;
}

.rb__metric {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #fff;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease;
}

.rb__metric:hover {
  border-color: #c9cdf0;
  background: #fcfcff;
}

.rb__metric--on {
  border-color: var(--color-primary);
  background: var(--color-primary-tint);
}

.rb__metric-input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.rb__metric-box {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  margin-top: 1px;
  border: 2px solid var(--color-border);
  border-radius: 6px;
  background: #fff;
  display: grid;
  place-items: center;
  color: #fff;
  transition: background 0.18s ease, border-color 0.18s ease;
}

.rb__metric-box svg {
  width: 13px;
  height: 13px;
  opacity: 0;
  transition: opacity 0.18s ease;
}

.rb__metric--on .rb__metric-box {
  background: var(--color-primary);
  border-color: var(--color-primary);
}

.rb__metric--on .rb__metric-box svg {
  opacity: 1;
}

.rb__metric-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rb__metric-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.rb__metric-unit {
  font-size: 11px;
  color: var(--color-text-muted);
}

/* Разрезы */
.rb__dims {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 9px;
  margin-left: 36px;
}

.rb__dim {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #fff;
  font-size: 13px;
  color: var(--color-text);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.rb__dim:hover {
  border-color: #c9cdf0;
}

.rb__dim--on {
  border-color: var(--color-primary);
  background: var(--color-primary-tint);
  color: var(--color-primary);
}

.rb__dim-input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.rb__dim-dot {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  border: 2px solid var(--color-border);
  border-radius: 50%;
  display: grid;
  place-items: center;
}

.rb__dim--on .rb__dim-dot {
  border-color: var(--color-primary);
}

.rb__dim--on .rb__dim-dot::after {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary);
}

.rb__gran {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 14px 0 0 36px;
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
  margin-left: 36px;
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

/* Период */
.rb__period {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-left: 36px;
}

.rb__period-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.rb__period-dates {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.rb__date {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.rb__preview {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px 16px;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.rb__preview-label {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: var(--color-text-muted);
  text-transform: uppercase;
}

.rb__summary {
  margin: 0;
  font-size: 14px;
  color: var(--color-text);
}

.rb__preview-result {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-muted);
}

.rb__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 14px;
  flex-wrap: wrap;
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

@media (max-width: 980px) {
  .rb__metrics {
    grid-template-columns: repeat(2, 1fr);
  }

  .rb__dims {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 620px) {
  .rb__metrics {
    grid-template-columns: 1fr;
    margin-left: 0;
  }

  .rb__dims {
    grid-template-columns: repeat(2, 1fr);
    margin-left: 0;
  }

  .rb__group-title,
  .rb__gran,
  .rb__filters,
  .rb__period {
    margin-left: 0;
  }
}
</style>
