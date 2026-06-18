<template>
  <div class="rr">
    <!-- Загрузка -->
    <div
      v-if="loading"
      class="rr__state"
    >
      <LoaderSpinner />
      <span>Строим отчёт…</span>
    </div>

    <!-- Ошибка -->
    <div
      v-else-if="error"
      class="rr__state rr__state--error"
    >
      {{ error }}
    </div>

    <!-- Пусто (отчёт ещё не построен) -->
    <div
      v-else-if="!result"
      class="rr__state rr__empty"
    >
      <svg
        width="44"
        height="44"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M3 3v18h18" />
        <path d="M7 14l3-4 3 3 4-6" />
      </svg>
      <p>Здесь появится ваш отчёт</p>
      <span>Задайте параметры выше и нажмите «Построить отчёт».</span>
    </div>

    <!-- Агрегатный отчёт -->
    <template v-else-if="result.mode === 'aggregate'">
      <div class="rr__toolbar">
        <div class="rr__seg">
          <button
            type="button"
            class="rr__seg-btn"
            :class="{ 'rr__seg-btn--active': view === 'table' }"
            data-testid="rr-view-table"
            @click="view = 'table'"
          >
            Таблица
          </button>
          <button
            type="button"
            class="rr__seg-btn"
            :class="{ 'rr__seg-btn--active': view === 'chart' }"
            :disabled="!hasRows"
            data-testid="rr-view-chart"
            @click="view = 'chart'"
          >
            График
          </button>
        </div>
        <div
          v-if="view === 'chart' && hasRows"
          class="rr__seg"
        >
          <button
            v-for="opt in chartTypeOptions"
            :key="opt.value"
            type="button"
            class="rr__seg-btn"
            :class="{ 'rr__seg-btn--active': chartType === opt.value }"
            :data-testid="`rr-chart-${opt.value}`"
            @click="chartType = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>
        <!-- Выбор метрики для графика, если их несколько (график рисует одну серию) -->
        <div
          v-if="view === 'chart' && hasRows && aggColumns.length > 1"
          class="rr__seg rr__seg--metrics"
        >
          <button
            v-for="col in aggColumns"
            :key="col.key"
            type="button"
            class="rr__seg-btn"
            :class="{ 'rr__seg-btn--active': chartMetric === col.key }"
            :data-testid="`rr-metric-${col.key}`"
            @click="chartMetric = col.key"
          >
            {{ col.label }}
          </button>
        </div>
      </div>

      <ReportChart
        v-if="view === 'chart' && hasRows"
        :rows="chartRows"
        :type="chartType"
        :unit="chartUnit"
        :label="chartLabel"
      />

      <div
        v-else
        class="rr__table-wrap"
      >
        <table class="rr__table">
          <thead>
            <tr>
              <th>{{ dimensionHeader }}</th>
              <th
                v-for="col in aggColumns"
                :key="col.key"
                class="rr__num"
              >
                {{ col.label }}{{ col.unit ? `, ${col.unit}` : '' }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in aggRows"
              :key="i"
            >
              <td>{{ row.label }}</td>
              <td
                v-for="col in aggColumns"
                :key="col.key"
                class="rr__num"
              >
                {{ formatNumber(cellValue(row, col.key)) }}
              </td>
            </tr>
            <tr v-if="!aggRows.length">
              <td
                :colspan="aggColumns.length + 1"
                class="rr__norows"
              >
                Нет данных за выбранный период
              </td>
            </tr>
          </tbody>
          <tfoot v-if="aggRows.length && showTotals">
            <tr>
              <td>Итого</td>
              <td
                v-for="col in aggColumns"
                :key="col.key"
                class="rr__num"
              >
                <b>{{ formatNumber(aggTotals[col.key] ?? 0) }}</b>
              </td>
            </tr>
          </tfoot>
        </table>
      </div>
      <div class="rr__footer">
        строк: {{ aggRows.length }}
      </div>
    </template>

    <!-- Выгрузка строк (list) -->
    <template v-else>
      <div class="rr__table-wrap">
        <table class="rr__table">
          <thead>
            <tr>
              <th
                v-for="col in (result.columns || [])"
                :key="col.key"
              >
                {{ col.label }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in result.rows"
              :key="i"
            >
              <td
                v-for="col in (result.columns || [])"
                :key="col.key"
              >
                {{ formatCell(row[col.key]) }}
              </td>
            </tr>
            <tr v-if="!result.rows.length">
              <td
                :colspan="(result.columns || []).length || 1"
                class="rr__norows"
              >
                Нет данных за выбранный период
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="rr__footer">
        Всего: <b>{{ result.total }}</b>
        <span class="rr__footer-sep">·</span> показано строк: {{ result.rows.length }}
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import ReportChart from './ReportChart.vue';

const props = defineProps({
  result: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
});

const view = ref('table'); // 'table' | 'chart'
const chartType = ref('bar'); // 'bar' | 'pie' | 'line'
const chartMetric = ref(''); // ключ колонки-метрики, отображаемой на графике

// Мультиметрика и одиночная сводка приведены к единому виду: колонки-метрики +
// строки {label, values:{колонка->число}} + итоги по колонкам. Legacy-форма
// (rows[{label,value}]/total/unit) синтезируется в одну колонку «value», поэтому
// таблица и график не зависят от того, в каком формате пришёл ответ.
const aggColumns = computed(() => {
  const r = props.result;
  // mode-guard: у list-результата свои columns — они не метрики, сюда не берём.
  if (r?.mode === 'aggregate' && r?.columns?.length) return r.columns;
  return [{ key: 'value', label: 'Количество', unit: r?.unit || '' }];
});

const aggRows = computed(() => {
  const r = props.result;
  if (r?.metric_rows) return r.metric_rows;
  return (r?.rows || []).map((row) => ({ label: row.label, values: { value: row.value } }));
});

const aggTotals = computed(() => {
  const r = props.result;
  if (r?.totals) return r.totals;
  return { value: r?.total ?? 0 };
});

const hasRows = computed(() => aggRows.value.length > 0);

// «Без разреза» -> единственная строка уже итоговая, отдельный футер итогов лишний.
const showTotals = computed(() => props.result?.dimension !== 'none');

const dimensionHeader = computed(() => (props.result?.dimension === 'none' ? 'Итог' : 'Значение разреза'));

// График рисует одну серию — берём выбранную метрику-колонку.
const chartColumn = computed(
  () => aggColumns.value.find((c) => c.key === chartMetric.value) || aggColumns.value[0] || null,
);
const chartRows = computed(() => {
  const key = chartColumn.value?.key;
  return aggRows.value.map((row) => ({ label: row.label, value: cellValue(row, key) }));
});
const chartUnit = computed(() => chartColumn.value?.unit || '');
const chartLabel = computed(() => (aggColumns.value.length > 1 ? chartColumn.value?.label || '' : ''));

// Временной разрез рисуем линией, остальные — столбцы/круговая.
const chartTypeOptions = computed(() => (
  props.result?.dimension === 'period'
    ? [{ value: 'line', label: 'Линия' }, { value: 'bar', label: 'Столбцы' }]
    : [{ value: 'bar', label: 'Столбцы' }, { value: 'pie', label: 'Круговая' }]
));

// Новый результат: list/пусто — только таблица; aggregate — сбрасываем тип
// графика на дефолт по разрезу (period -> линия, иначе столбцы) и метрику графика
// на первую колонку. Вид оставляем как выбрал пользователь, чтобы повторный отчёт
// не сбрасывал его на таблицу.
watch(() => props.result, (r) => {
  if (r?.mode !== 'aggregate') {
    view.value = 'table';
    return;
  }
  chartType.value = r.dimension === 'period' ? 'line' : 'bar';
  chartMetric.value = aggColumns.value[0]?.key || '';
}, { immediate: true });

function cellValue(row, key) {
  return row?.values?.[key] ?? 0;
}

function formatNumber(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n.toLocaleString('ru-RU') : value;
}

function formatCell(value) {
  if (value === null || value === undefined || value === '') return '—';
  return value;
}
</script>

<style scoped>
.rr {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rr__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 220px;
  text-align: center;
  color: var(--color-text-muted);
}

.rr__state--error {
  color: #c0392b;
}

.rr__empty svg {
  opacity: 0.35;
}

.rr__empty p {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text);
}

.rr__empty span {
  font-size: 13px;
  max-width: 320px;
}

.rr__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.rr__seg {
  display: inline-flex;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  padding: 3px;
  gap: 2px;
}

/* Метрик может быть много -> перенос на несколько строк; pill-радиус на
   многострочном блоке смотрится криво, поэтому скругление поменьше. */
.rr__seg--metrics {
  flex-wrap: wrap;
  border-radius: var(--radius-md);
}

.rr__seg-btn {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
  padding: 5px 12px;
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.rr__seg-btn:hover:not(:disabled) {
  color: var(--color-primary);
}

.rr__seg-btn--active {
  background: var(--color-primary);
  color: #fff;
}

.rr__seg-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.rr__table-wrap {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.rr__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}

.rr__table thead th {
  position: sticky;
  top: 0;
  background: var(--color-bg);
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 11px 14px;
  white-space: nowrap;
  border-bottom: 1px solid var(--color-border);
}

.rr__table tbody td {
  padding: 10px 14px;
  color: var(--color-text);
  border-bottom: 1px solid var(--color-border);
  vertical-align: top;
}

.rr__table tbody tr:last-child td {
  border-bottom: none;
}

.rr__table tbody tr:hover td {
  background: var(--color-bg);
}

.rr__table tfoot td {
  padding: 11px 14px;
  color: var(--color-text);
  border-top: 2px solid var(--color-border);
  background: var(--color-bg);
  white-space: nowrap;
}

.rr__num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.rr__norows {
  text-align: center;
  color: var(--color-text-muted);
  padding: 24px 14px;
}

.rr__footer {
  font-size: 14px;
  color: var(--color-text);
}

.rr__footer-sep {
  color: var(--color-text-muted);
  margin: 0 4px;
}
</style>
