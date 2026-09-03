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
          v-for="(sec, secIndex) in visibleMetricSections"
          :key="sec.key"
        >
          <div class="rb__group-title">
            {{ sec.title }}
          </div>
          <div class="rb__metrics">
            <label
              v-for="m in sec.metrics"
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
              <HintTooltip
                v-if="metricHint(m.key)"
                class="rb__metric-hint"
                :text="metricHint(m.key)"
              />
            </label>
          </div>

          <button
            v-if="secIndex === 0 && hasPrimarySection && extraMetricsCount"
            type="button"
            class="rb__more"
            :aria-expanded="String(showAllMetrics)"
            @click="showAllMetrics = !showAllMetrics"
          >
            <span
              class="rb__more-caret"
              :class="{ 'rb__more-caret--open': showAllMetrics }"
              aria-hidden="true"
            />
            <span class="rb__more-text">Показатели обработки</span>
            <span class="rb__more-count">{{ extraMetricsCount }}</span>
            <span class="rb__more-note">времена, медианы, перцентили, доли</span>
          </button>
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
          <template v-if="availablePivots.length">
            <span class="rb__field-label rb__pivot-label">Развернуть в колонки</span>
            <div class="rb__pills">
              <button
                type="button"
                class="rb__pill"
                :class="{ 'rb__pill--on': !form.pivot }"
                @click="form.pivot = ''"
              >
                Без разворота
              </button>
              <button
                v-for="p in availablePivots"
                :key="p.key"
                type="button"
                class="rb__pill"
                :class="{ 'rb__pill--on': form.pivot === p.key }"
                @click="form.pivot = p.key"
              >
                {{ p.label }}
              </button>
            </div>
            <span class="rb__hint">Каждое значение оси станет отдельным столбцом-счётчиком рядом с периодом.</span>
          </template>
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
      <button
        type="button"
        class="rb__step-head rb__step-head--toggle"
        :aria-expanded="String(filtersOpen)"
        @click="filtersOpen = !filtersOpen"
      >
        <span
          v-if="form.mode === 'aggregate'"
          class="rb__step-num"
        >3</span>
        <span class="rb__step-name">Фильтры</span>
        <span class="rb__step-opt">необязательно</span>
        <span class="rb__step-sum">{{ filtersSummary }}</span>
        <span
          class="rb__more-caret rb__step-caret"
          :class="{ 'rb__more-caret--open': filtersOpen }"
          aria-hidden="true"
        />
      </button>
      <div
        v-show="filtersOpen"
        class="rb__filters"
      >
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
      <button
        type="button"
        class="rb__step-head rb__step-head--toggle"
        :aria-expanded="String(periodOpen)"
        @click="periodOpen = !periodOpen"
      >
        <span
          v-if="form.mode === 'aggregate'"
          class="rb__step-num"
        >4</span>
        <span class="rb__step-name">Период</span>
        <span class="rb__step-sum">{{ periodSummary }}</span>
        <span
          class="rb__more-caret rb__step-caret"
          :class="{ 'rb__more-caret--open': periodOpen }"
          aria-hidden="true"
        />
      </button>
      <div
        v-show="periodOpen"
        class="rb__period"
      >
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
          <DateFilter
            mode="range"
            :date-range-start="periodStartDate"
            :date-range-end="periodEndDate"
            @update:date-range-start="onPeriodRangeStart"
            @update:date-range-end="onPeriodRangeEnd"
            @clear="onPeriodClear"
          />
        </div>
      </div>
    </div>

    <!-- Превью «что построим»: описание + визуальный скелет колонок результата -->
    <div class="rb__preview">
      <span class="rb__preview-label">Что построим</span>
      <p class="rb__summary">
        {{ summary }}
      </p>
      <div
        v-if="previewColumns.length"
        class="rb__skeleton"
        aria-hidden="true"
      >
        <div
          v-for="(col, i) in previewColumns"
          :key="i"
          class="rb__skel-col"
          :class="`rb__skel-col--${col.kind}`"
        >
          <span class="rb__skel-head">
            {{ col.label }}<span
              v-if="col.kind === 'pivot'"
              class="rb__skel-more"
            >, …</span>
          </span>
          <span
            v-if="col.sub"
            class="rb__skel-sub"
          >{{ col.sub }}</span>
          <span class="rb__skel-cell" />
          <span class="rb__skel-cell" />
        </div>
      </div>
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
          :max="MAX_REPORT_LIMIT"
          :placeholder="String(limitPlaceholder)"
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
import HintTooltip from '@/components/ui/HintTooltip.vue';
import DateFilter from '@/components/DateFilter.vue';
import { buildReportRequest, defaultReportLimit, MAX_REPORT_LIMIT } from '@/composables/useReportRequest';
import { formatDateRu } from '@/utils/datetime';

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
  // Ось cross-tab: значения разворачиваются в колонки. '' -> без разворота.
  // Применима лишь при dimension='period' и метрике из pivot.metrics (см. availablePivots).
  pivot: '',
  entity: props.catalog.list_entities?.[0]?.key || '',
  filters: {},
  period: { from: props.period?.from || '', to: props.period?.to || '' },
  limit: '',
});

// Ходовые показатели: с них начинают почти любой отчёт. Остальные 22 - времена,
// медианы, перцентили и доли - нужны точечно и прячутся под раскрывашку.
const PRIMARY_METRIC_KEYS = ['applications_count', 'items_sum', 'car_entries_count', 'people_entries_count'];

// Подсказки к неочевидным показателям. Формулировки сверены с выражениями в
// report_quality_metrics.go и report_duration_metrics.go, а не написаны по памяти.
const METRIC_HINTS = {
  refusal_rate: 'Обе ветки отказа вместе: отказал принимающий или не согласовали согласующие. Считается от заявок, поданных за период.',
  rejected_rate: 'Только заявки со статусом «Отказано» - решение принимающего.',
  not_approved_rate: 'Только заявки, которые не согласовали согласующие. С отказом принимающего может совпасть, поэтому две доли не складываются в общую.',
};

function metricHint(key) {
  if (METRIC_HINTS[key]) return METRIC_HINTS[key];
  if (key.startsWith('p50_')) return 'Медиана: половина заявок укладывается в это время, половина идёт дольше.';
  if (key.startsWith('p90_')) return 'Девять заявок из десяти укладываются в это время. Показывает длинный хвост, который среднее сглаживает.';
  return '';
}

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

// Оси cross-tab, применимые к текущему срезу: только при разрезе «период» и когда
// ось поддерживает КАЖДУЮ выбранную метрику (бэк требует metrics ⊆ pivot.metrics,
// иначе 400). Пустой набор метрик -> разворачивать нечего.
const availablePivots = computed(() => {
  if (form.dimension !== 'period') return [];
  const sel = selectedMetrics.value;
  if (!sel.length) return [];
  return (props.catalog.pivots || []).filter(
    (p) => sel.every((m) => (p.metrics || []).includes(m.key)),
  );
});

const activePivot = computed(
  () => availablePivots.value.find((p) => p.key === form.pivot) || null,
);

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
// Секции шага «Что считаем»: первая - «Основное», дальше исходные группы каталога
// без вошедших в неё метрик. Пустые группы отбрасываем, иначе останется заголовок
// без карточек.
const metricSections = computed(() => {
  const primary = [];
  for (const key of PRIMARY_METRIC_KEYS) {
    const m = (props.catalog.metrics || []).find((x) => x.key === key);
    if (m) primary.push(m);
  }
  const primaryKeys = new Set(primary.map((m) => m.key));
  const rest = metricGroups.value
    .map((g) => ({ key: g.group, title: g.group, metrics: g.metrics.filter((m) => !primaryKeys.has(m.key)) }))
    .filter((g) => g.metrics.length);
  if (!primary.length) return rest;
  return [{ key: 'primary', title: 'Основное', metrics: primary }, ...rest];
});

// Раскрывашка нужна, только когда «Основное» реально что-то скрывает.
const hasPrimarySection = computed(() => metricSections.value[0]?.key === 'primary');
const extraMetricsCount = computed(
  () => (hasPrimarySection.value ? metricSections.value.slice(1) : []).reduce((n, g) => n + g.metrics.length, 0),
);
const showAllMetrics = ref(false);
const visibleMetricSections = computed(
  () => (hasPrimarySection.value && !showAllMetrics.value ? metricSections.value.slice(0, 1) : metricSections.value),
);

// Пресет или шаблон могут включить показатель из свёрнутой части - тогда блок надо
// открыть, иначе галочка стоит там, где её не видно.
watch(() => form.metrics.slice(), (keys) => {
  if (showAllMetrics.value || !hasPrimarySection.value) return;
  if (keys.some((k) => !PRIMARY_METRIC_KEYS.includes(k))) showAllMetrics.value = true;
}, { immediate: true });

// Шаги «Фильтры» и «Период» свёрнуты по умолчанию: в раскрытом виде они занимали
// больше двух экранов и отодвигали кнопку построения. Состояние выносится в
// заголовок, чтобы свёрнутый шаг не прятал того, что уже задано.
const filtersOpen = ref(false);
const periodOpen = ref(false);

const filtersSummary = computed(() => {
  const parts = [];
  for (const f of filterFields.value) {
    const count = (form.filters[f.key] || []).filter((v) => v != null && String(v).trim() !== '').length;
    if (count) parts.push(`${f.label}: ${count}`);
  }
  return parts.length ? parts.join(' · ') : 'не заданы';
});

const periodSummary = computed(() => {
  const { from, to } = form.period;
  if (!from && !to) return 'весь период';
  if (from && to) return `${formatDateRu(from)} - ${formatDateRu(to)}`;
  return from ? `с ${formatDateRu(from)}` : `по ${formatDateRu(to)}`;
});

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

// Ось разворота держим валидной: сменили разрез с «период» или метрику на
// неподдерживаемую -> сбрасываем pivot, иначе ушлём в движок невалидную ось (400).
watch(availablePivots, (pivots) => {
  if (form.pivot && !pivots.some((p) => p.key === form.pivot)) form.pivot = '';
});

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
  if (from && to) return `${formatDateRu(from)} — ${formatDateRu(to)}`;
  if (from) return `с ${formatDateRu(from)}`;
  if (to) return `по ${formatDateRu(to)}`;
  return 'весь период';
});

const summary = computed(() => {
  // Период упоминаем только когда срез его поддерживает (есть date_range в applicable).
  const periodPart = periodApplicable.value ? `, период: ${periodLabel.value}` : '';
  if (form.mode === 'aggregate') {
    const names = selectedMetrics.value.map((m) => m.label).join(', ') || '—';
    const dim = availableDimensions.value.find((d) => d.key === form.dimension)?.label || '—';
    const pivotPart = activePivot.value
      ? `, развёрнуто по «${activePivot.value.label}» в колонки`
      : '';
    return `Считаем: ${names}; разрез «${dim}»${pivotPart}${periodPart}.`;
  }
  return `Выгрузка строк: «${selectedEntity.value?.label || '—'}»${periodPart}.`;
});

const resultHint = computed(() => {
  if (form.mode === 'aggregate') {
    const n = selectedMetrics.value.length;
    // Cross-tab: период в строках, каждое значение оси -> своя колонка-счётчик.
    if (activePivot.value) {
      return `Результат: строки по периоду, отдельная колонка-счётчик на каждое значение «${activePivot.value.label}», с итогами.`;
    }
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

// Скелет колонок результата для превью «Что построим»: первый столбец — разрез
// (или «Итог» без разреза), затем по столбцу на метрику, затем ось разворота
// (pivot) — её значения станут динамическими колонками, поэтому показываем одну
// «призрачную» колонку-маркер. list -> столбцы выгружаемой сущности.
const previewColumns = computed(() => {
  if (form.mode === 'list') {
    return (selectedEntity.value?.columns || []).map((c) => ({ label: c.label, kind: 'data' }));
  }
  const cols = [];
  const dimLabel = form.dimension === 'none'
    ? 'Итог'
    : (availableDimensions.value.find((d) => d.key === form.dimension)?.label || 'Разрез');
  cols.push({ label: dimLabel, kind: 'dim' });
  for (const m of selectedMetrics.value) {
    cols.push({ label: m.label, sub: m.unit || '', kind: 'metric' });
  }
  if (activePivot.value) {
    cols.push({ label: activePivot.value.label, sub: 'колонки по значениям', kind: 'pivot' });
  }
  return cols;
});

// Применить пресет/шаблон и сразу построить отчёт. Галерея-пресеты несут только
// метрику/разрез; шаблоны (G2) дополнительно несут filters/period/period_preset.
// Разрез сверяется watch'ем availableDimensions (ключи каталога валидны).
watch(() => props.preset, (p) => {
  if (!p) return;
  form.mode = p.mode || 'aggregate';
  if (p.mode === 'list') {
    form.entity = p.entity || form.entity;
  } else {
    form.metrics = p.metrics ? [...p.metrics] : (p.metric ? [p.metric] : []);
    form.dimension = p.dimension || '';
    form.granularity = p.granularity || 'day';
    // Ось разворота из шаблона; невалидную (не period / чужая метрика) сбросит
    // watch availablePivots до построения отчёта.
    form.pivot = p.pivot || '';
  }
  // Период — синхронно (watch'и его не сбрасывают). Именованный пресет (неделя/
  // месяц/год/весь) пересчитываем на текущую дату; «custom»/произвольный — берём
  // явные даты шаблона, иначе computePeriod стёр бы их.
  const namedPreset = ['week', 'month', 'year', 'all'].includes(p.period_preset);
  if (namedPreset) {
    applyPeriodPreset(p.period_preset);
  } else if (p.period) {
    form.period.from = p.period.from || '';
    form.period.to = p.period.to || '';
    activePeriodPreset.value = 'custom';
  }
  // Фильтры применяем в nextTick: watch entity/metrics успевает синхронно сбросить
  // form.filters, иначе он затёр бы значения шаблона. Затем строим отчёт.
  nextTick(() => {
    form.filters = p.filters ? { ...p.filters } : {};
    nextTick(run);
  });
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
  const to = dateToIso(now);
  if (kind === 'week') {
    const monday = new Date(now);
    monday.setDate(now.getDate() - ((now.getDay() + 6) % 7)); // Пн = начало недели
    return { from: dateToIso(monday), to };
  }
  if (kind === 'month') return { from: dateToIso(new Date(now.getFullYear(), now.getMonth(), 1)), to };
  if (kind === 'year') return { from: dateToIso(new Date(now.getFullYear(), 0, 1)), to };
  return { from: '', to: '' };
}

function applyPeriodPreset(kind) {
  const range = computePeriod(kind);
  form.period.from = range.from;
  form.period.to = range.to;
  activePeriodPreset.value = kind;
}

// Период хранится в form как ISO YYYY-MM-DD (формат бэка), а DateFilter работает с
// Date-объектами в локальной зоне. Разбираем/собираем по календарным частям, не через
// toISOString, чтобы не словить сдвиг даты на границе суток.
function dateToIso(dt) {
  return `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`;
}

function isoToDate(iso) {
  if (!iso) return null;
  const [y, m, d] = iso.split('-').map(Number);
  if (!y || !m || !d) return null;
  return new Date(y, m - 1, d);
}

const periodStartDate = computed(() => isoToDate(form.period.from));
const periodEndDate = computed(() => isoToDate(form.period.to));

// DateFilter эмитит update:* при каждом «Применить», даже когда даты не менялись.
// Уводим период в 'custom' только при РЕАЛЬНОЙ смене границы — иначе «Применить»
// на выставленном пресете без правок сбивал бы его подсветку (нативный input
// раньше тоже молчал, пока значение не изменится).
function onPeriodRangeStart(date) {
  const iso = date ? dateToIso(date) : '';
  if (iso === form.period.from) return;
  form.period.from = iso;
  activePeriodPreset.value = 'custom';
}

function onPeriodRangeEnd(date) {
  const iso = date ? dateToIso(date) : '';
  if (iso === form.period.to) return;
  form.period.to = iso;
  activePeriodPreset.value = 'custom';
}

function onPeriodClear() {
  form.period.from = '';
  form.period.to = '';
  activePeriodPreset.value = 'custom';
}

// Снимок состояния формы для индикатора шагов мастера (родитель считает по нему
// прогресс). metric — ключ первой выбранной метрики: шаг «что считаем» закрыт,
// когда выбрана хотя бы одна. periodFilled — задан ли диапазон. immediate —
// чтобы степпер был корректен сразу.
const filterCount = computed(
  () => Object.values(form.filters).reduce((n, vals) => n + (vals?.length || 0), 0),
);

// Плейсхолдер поля «Строк» показывает, какой лимит уйдёт в запрос при пустом
// поле: у разреза «период» это потолок (иначе хвост периода обрезался бы), у
// остальных — дефолт движка.
const limitPlaceholder = computed(
  () => defaultReportLimit({ mode: form.mode, dimension: form.dimension }),
);

// Полный снимок состояния гида для сохранения в шаблон. Тот же формат принимает
// preset-watch при применении шаблона. period_preset сохраняем, чтобы при
// восстановлении подсветить ту же кнопку периода. Бэк хранит config как непрозрачный.
const reportConfig = computed(() => ({
  mode: form.mode,
  metrics: [...form.metrics],
  dimension: form.dimension,
  granularity: form.granularity,
  pivot: form.pivot,
  entity: form.entity,
  filters: { ...form.filters },
  period: { from: form.period.from, to: form.period.to },
  period_preset: activePeriodPreset.value,
}));

watch(
  () => ({
    mode: form.mode,
    metric: form.metrics[0] || '',
    dimension: form.dimension,
    entity: form.entity,
    filterCount: filterCount.value,
    periodApplicable: periodApplicable.value,
    periodFilled: Boolean(form.period.from && form.period.to),
    config: reportConfig.value,
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

// Пустой результат предлагает расширить период до «весь». Отдаём наружу метод, а не
// повторяем вычисление диапазона в ReportsTab: границы периода живут здесь.
function expandPeriodToAll() {
  applyPeriodPreset('all');
  run();
}

defineExpose({ expandPeriodToAll });
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
  color: var(--accent-contrast);
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

/* Заголовок-переключатель шага: обычная строка заголовка, но кликабельная целиком. */
.rb__step-head--toggle {
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.rb__step-sum {
  margin-left: auto;
  font-size: 13px;
  color: var(--color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rb__step-caret {
  flex: none;
  margin-left: 4px;
}
.rb__more-caret {
  width: 0;
  height: 0;
  border-left: 5px solid currentColor;
  border-top: 4px solid transparent;
  border-bottom: 4px solid transparent;
  transition: transform 0.18s ease;
}
.rb__more-caret--open {
  transform: rotate(90deg);
}

/* Раскрывашка остальных показателей: тон служебной строки, не вторая главная кнопка. */
.rb__more {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  margin-top: 4px;
  padding: 10px 14px;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  background: none;
  color: var(--color-text-muted);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, color 0.18s ease;
}
.rb__more:hover {
  border-color: var(--color-primary);
  color: var(--color-text);
}
.rb__more-text {
  font-weight: 600;
}
.rb__more-count {
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  background: var(--color-border);
  font-size: 12px;
}
.rb__more-note {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rb__metric-hint {
  flex: none;
  margin-left: auto;
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
  background: var(--surface);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease;
}

.rb__metric:hover {
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
  background: var(--accent-tint);
}

.rb__metric--on {
  border-color: var(--accent);
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
  background: var(--surface);
  display: grid;
  place-items: center;
  color: var(--accent-contrast);
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
  border-color: var(--accent);
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
  background: var(--surface);
  font-size: 13px;
  color: var(--color-text);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.rb__dim:hover {
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.rb__dim--on {
  border-color: var(--accent);
  background: var(--color-primary-tint);
  color: var(--accent-text);
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
  border-color: var(--accent);
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
  background: var(--surface);
  border-radius: var(--radius-pill);
  font-size: 13px;
  font-family: inherit;
  color: var(--color-text);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, border-color 0.18s ease;
}

.rb__pill:hover {
  border-color: var(--accent);
}

.rb__pill--on {
  background: var(--color-primary);
  color: var(--accent-contrast);
  border-color: var(--accent);
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

/* Скелет колонок результата: горизонтальный набор «столбцов», каждый — заголовок
   + пара призрачных ячеек. Даёт почувствовать форму таблицы до построения. */
.rb__skeleton {
  display: flex;
  gap: 8px;
  margin: 4px 0 2px;
  padding-bottom: 4px;
  overflow-x: auto;
}

.rb__skel-col {
  flex: 1 1 0;
  min-width: 96px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm, 10px);
  background: var(--surface);
}

.rb__skel-col--dim {
  background: var(--color-bg);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.rb__skel-col--metric {
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
  background: var(--color-primary-tint);
}

/* Pivot-колонки динамические (значения оси) -> пунктир + многоточие-маркер. */
.rb__skel-col--pivot {
  border-style: dashed;
  border-color: var(--accent);
  background: var(--surface);
}

.rb__skel-head {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
  /* Перенос вместо обрезки: при выгрузке строк колонок много (7+), длинные
     подписи («Организация/Компания») не должны теряться под многоточием. */
  overflow-wrap: anywhere;
}

.rb__skel-more {
  color: var(--accent-text);
  font-weight: 700;
}

.rb__skel-sub {
  font-size: 10px;
  color: var(--color-text-muted);
  overflow-wrap: anywhere;
}

.rb__skel-cell {
  height: 7px;
  border-radius: 4px;
  background: var(--color-border);
}

.rb__skel-col--metric .rb__skel-cell {
  background: var(--accent);
}

.rb__pivot-label {
  margin-top: 6px;
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

/* Планшет: свёрнутый сайдбар делает конструктор узким, 3-4 колонки не тянет. */
@media (max-width: 980px) {
  .rb__metrics {
    grid-template-columns: repeat(2, 1fr);
  }

  .rb__dims {
    grid-template-columns: repeat(3, 1fr);
  }
}

/* Мобилка (#1097, канонический брейкпоинт 768): контролы стопкой, левый отступ
   под номер шага убираем (отъедает ширину), тач-зоны пилюль/карточек крупнее. */
@media (max-width: 768px) {
  .rb {
    gap: 18px;
  }

  .rb__dims {
    grid-template-columns: repeat(2, 1fr);
  }

  .rb__metrics,
  .rb__dims,
  .rb__group-title,
  .rb__gran,
  .rb__filters,
  .rb__period {
    margin-left: 0;
  }

  .rb__pill {
    padding: 8px 14px;
  }

  .rb__dim {
    padding: 12px;
  }

  .rb__metric {
    padding: 13px 14px;
  }

  /* «Построить отчёт» на всю ширину под полем «Строк» — крупная зона нажатия. */
  .rb__footer {
    justify-content: flex-start;
  }

  .rb__footer .lk-button--primary {
    flex: 1 1 100%;
  }
}

@media (max-width: 480px) {
  .rb__metrics {
    grid-template-columns: 1fr;
  }
}
</style>
