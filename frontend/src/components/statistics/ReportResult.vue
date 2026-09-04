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
        width="40"
        height="40"
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
        <!-- Выбор метрики для графика, если их несколько (график рисует одну серию). -->
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
        <!-- Тип графика для категориального разреза: столбцы или доли (кольцо).
             Период — динамика во времени, доли не имеют смысла -> тоггл скрыт. -->
        <div
          v-if="view === 'chart' && hasRows && canDonut"
          class="rr__seg rr__seg--kind"
        >
          <button
            type="button"
            class="rr__seg-btn"
            :class="{ 'rr__seg-btn--active': chartKind === 'bar' }"
            data-testid="rr-kind-bar"
            @click="chartKind = 'bar'"
          >
            Столбцы
          </button>
          <button
            type="button"
            class="rr__seg-btn"
            :class="{ 'rr__seg-btn--active': chartKind === 'donut' }"
            data-testid="rr-kind-donut"
            @click="chartKind = 'donut'"
          >
            Кольцо
          </button>
        </div>
        <ReportExportButton
          class="rr__export"
          :disabled="!canExport"
          :exporting="exporting"
          :with-image="view === 'chart' && hasRows"
          @export="onExport"
        />
      </div>

      <!-- Период -> area (динамика во времени); категориальный разрез -> столбцы
           или кольцо (доли) по выбору.
           Keyed Transition даёт мягкий fade при смене вида/типа. -->
      <Transition
        name="rr-fade"
        mode="out-in"
      >
        <div
          :key="chartType"
          ref="chartBox"
          class="rr__slot"
        >
          <AnalyticsAreaChart
            v-if="chartType === 'area'"
            :data="chartAreaData"
            :height="chartHeight"
            :tension="0"
            :series-name="chartSeriesName"
            :unit-forms="chartUnitForms"
            :is-float="chartIsFloat"
            :value-type="chartValueType"
            data-testid="rr-chart-area"
          />
          <AnalyticsBarChart
            v-else-if="chartType === 'bar'"
            :data="chartBarData"
            :height="chartHeight"
            :series-name="chartSeriesName"
            :unit-forms="chartUnitForms"
            :is-float="chartIsFloat"
            :value-type="chartValueType"
            data-testid="rr-chart-bar"
          />
          <AnalyticsDonutChart
            v-else-if="chartType === 'donut'"
            :data="chartBarData"
            :height="chartHeight"
            :total-label="chartSeriesName"
            :unit-forms="chartUnitForms"
            :is-float="chartIsFloat"
            data-testid="rr-chart-donut"
          />

          <div
            v-else
            class="rr__table-wrap"
          >
            <table class="rr__table">
          <thead>
            <tr>
              <th
                class="rr__th--sortable"
                :aria-sort="ariaSort(LABEL_KEY)"
                @click="toggleSort(LABEL_KEY)"
              >
                {{ dimensionHeader }}<span class="rr__sort">{{ sortMark(LABEL_KEY) }}</span>
              </th>
              <th
                v-for="col in aggColumns"
                :key="col.key"
                class="rr__num rr__th--sortable"
                :aria-sort="ariaSort(col.key)"
                @click="toggleSort(col.key)"
              >
                {{ col.label }}{{ col.unit ? `, ${col.unit}` : '' }}<span class="rr__sort">{{ sortMark(col.key) }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in sortedAggRows"
              :key="i"
            >
              <td>{{ rowLabel(row.label) }}</td>
              <td
                v-for="col in aggColumns"
                :key="col.key"
                class="rr__num"
              >
                {{ formatMetricValue(cellValue(row, col), col) }}
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
                <b>{{ formatMetricValue(totalValue(col), col) }}</b>
              </td>
            </tr>
          </tfoot>
        </table>
          </div>
        </div>
      </Transition>
      <div class="rr__footer">
        строк: {{ aggRows.length }}
        <span
          v-if="truncated"
          class="rr__truncated"
          data-testid="rr-truncated"
        >Достигнут лимит {{ limit }} строк - показана только часть результата.</span>
      </div>
    </template>

    <!-- Выгрузка строк (list) -->
    <template v-else>
      <div class="rr__toolbar rr__toolbar--end">
        <ReportExportButton
          class="rr__export"
          :disabled="!canExport"
          :exporting="exporting"
          @export="onExport"
        />
      </div>
      <div class="rr__table-wrap">
        <table class="rr__table">
          <thead>
            <tr>
              <th
                v-for="col in (result.columns || [])"
                :key="col.key"
                class="rr__th--sortable"
                :class="{ 'rr__num': isNumCol(col) }"
                :aria-sort="ariaSort(col.key)"
                @click="toggleSort(col.key)"
              >
                {{ col.label }}<span class="rr__sort">{{ sortMark(col.key) }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in sortedListRows"
              :key="i"
            >
              <td
                v-for="col in (result.columns || [])"
                :key="col.key"
                :class="{ 'rr__num': isNumCol(col) }"
              >
                {{ formatCell(row[col.key], col.type) }}
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
        <span
          v-if="truncated"
          class="rr__truncated"
          data-testid="rr-truncated"
        >Достигнут лимит {{ limit }} строк - показана только часть результата.</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import AnalyticsAreaChart from './AnalyticsAreaChart.vue';
import AnalyticsBarChart from './AnalyticsBarChart.vue';
import AnalyticsDonutChart from './AnalyticsDonutChart.vue';
import ReportExportButton from './ReportExportButton.vue';
import { useReportExport, exportChartPng } from '@/composables/useReportExport';
import { formatDateRu, formatReportCell } from '@/utils/datetime';
import { isDurationColumn, metricValue } from '@/utils/reportColumns';

const props = defineProps({
  result: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  // Подпись выгрузки в Excel: { title, period:{from,to}, author }.
  meta: { type: Object, default: () => ({}) },
  // Лимит строк, с которым построен этот результат. Нужен, чтобы отличить
  // «данных ровно столько» от «результат упёрся в лимит»: движок признака
  // обрезки не отдаёт, а для разреза «период» обрезка съедает его хвост.
  limit: { type: Number, default: 0 },
});

const emit = defineEmits(['export-error']);

const view = ref('table'); // 'table' | 'chart'
const chartKind = ref('bar'); // 'bar' | 'donut' — тип категориального графика
const chartMetric = ref(''); // ключ колонки-метрики, отображаемой на графике

// Мультиметрика и одиночная сводка приведены к единому виду: колонки-метрики +
// строки {label, values, float_values} + итоги (totals/float_totals). Legacy-форма
// (rows[{label,value}]/total/unit) синтезируется в одну колонку «value», поэтому
// таблица и график не зависят от того, в каком формате пришёл ответ. Колонки могут
// быть обычными метриками, дробными (float -> float_values) и cross-tab pivot
// (kind='pivot', значение в values).
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

const aggFloatTotals = computed(() => props.result?.float_totals || {});

const hasRows = computed(() => aggRows.value.length > 0);

/*
 * Сортировка по клику на заголовок. Отчёт на полсотни строк иначе разбирали глазами
 * или выгружали в Excel ради одного упорядочивания (#2309). Сортируем то, что уже
 * пришло с сервера: движок отдаёт результат целиком, второй запрос не нужен.
 */
const LABEL_KEY = '__label';
const sort = ref({ key: '', dir: 1 });

function toggleSort(key) {
  sort.value = sort.value.key === key ? { key, dir: -sort.value.dir } : { key, dir: 1 };
}

function sortMark(key) {
  if (sort.value.key !== key) return '';
  return sort.value.dir > 0 ? ' ↑' : ' ↓';
}

function ariaSort(key) {
  if (sort.value.key !== key) return 'none';
  return sort.value.dir > 0 ? 'ascending' : 'descending';
}

// Пустые значения всегда внизу: «нет данных» - не самое маленькое число, а отсутствие.
function compareValues(a, b, dir) {
  const aEmpty = a === null || a === undefined || a === '';
  const bEmpty = b === null || b === undefined || b === '';
  if (aEmpty || bEmpty) return aEmpty && bEmpty ? 0 : (aEmpty ? 1 : -1);
  if (typeof a === 'number' && typeof b === 'number') return (a - b) * dir;
  return String(a).localeCompare(String(b), 'ru', { numeric: true }) * dir;
}

// Новый результат приходит со своим порядком (движок уже отсортировал) - сбрасываем.
watch(() => props.result, () => { sort.value = { key: '', dir: 1 }; });

const sortedAggRows = computed(() => {
  const { key, dir } = sort.value;
  if (!key) return aggRows.value;
  const col = aggColumns.value.find((c) => c.key === key);
  const valueOf = (row) => (key === LABEL_KEY ? rowLabel(row.label) : cellValue(row, col));
  return [...aggRows.value].sort((a, b) => compareValues(valueOf(a), valueOf(b), dir));
});

const sortedListRows = computed(() => {
  const { key, dir } = sort.value;
  const rows = props.result?.rows || [];
  if (!key) return rows;
  return [...rows].sort((a, b) => compareValues(a[key], b[key], dir));
});

// Строк ровно столько, сколько разрешал лимит -> движок почти наверняка отрезал
// хвост (точного признака в ответе нет). Ложное срабатывание «данных ровно
// столько» безобидно: подпись лишь предлагает сузить период.
const truncated = computed(() => {
  const r = props.result;
  if (!r || props.limit <= 0) return false;
  const rows = r.mode === 'list' ? (r.rows?.length || 0) : aggRows.value.length;
  return rows >= props.limit;
});

// «Без разреза» -> единственная строка уже итоговая, отдельный футер итогов лишний.
const showTotals = computed(() => props.result?.dimension !== 'none');

const isPeriod = computed(() => props.result?.dimension === 'period');

// Кольцо осмысленно для категориального распределения с >=2 долями: период —
// динамика во времени (доли бессмысленны), один разрез — доля 100% сама по себе.
// Длительность в кольцо не годится: оно рисует доли от суммы, а сумма средних и
// перцентилей смысла не имеет — по той же причине движок считает итог такой
// метрики отдельным запросом, а не сложением строк.
const canDonut = computed(
  () => !isPeriod.value && aggRows.value.length >= 2 && !isDurationColumn(chartColumn.value),
);

// Что показываем в слоте графика: area (период), кольцо (доли по выбору) или
// столбцы; вне режима графика/без строк — таблица. Один источник для v-if и :key.
const chartType = computed(() => {
  if (view.value !== 'chart' || !hasRows.value) return 'table';
  if (isPeriod.value) return 'area';
  if (chartKind.value === 'donut' && canDonut.value) return 'donut';
  return 'bar';
});

const dimensionHeader = computed(() => (props.result?.dimension === 'none' ? 'Итог' : 'Значение разреза'));

// Числовые колонки list-таблицы выравниваем вправо (tabular-nums). Тип берём от
// JSON-значения API (number), а не из col.type (бэк типизирует только date/time)
// и не из содержимого regex'ом: счётчики (attachments_count, people_count) приходят
// числами, идентификаторы (номер заявки) — строками и остаются слева. Выравнивание
// не трансформирует значение, поэтому проверка по typeof здесь безопасна.
const numericListColumns = computed(() => {
  const r = props.result;
  if (r?.mode === 'aggregate' || !Array.isArray(r?.rows) || !r.rows.length) return new Set();
  const set = new Set();
  for (const col of (r.columns || [])) {
    let sawValue = false;
    let allNumeric = true;
    for (const row of r.rows) {
      const v = row[col.key];
      if (v === null || v === undefined || v === '') continue;
      sawValue = true;
      if (typeof v !== 'number') { allNumeric = false; break; }
    }
    if (sawValue && allNumeric) set.add(col.key);
  }
  return set;
});

function isNumCol(col) {
  return numericListColumns.value.has(col.key);
}

// График рисует одну серию — берём выбранную метрику-колонку.
const chartColumn = computed(
  () => aggColumns.value.find((c) => c.key === chartMetric.value) || aggColumns.value[0] || null,
);
const chartSeriesName = computed(() => chartColumn.value?.label || 'Количество');
// Дробная метрика (среднее/день) -> график не округляет до целых, как и таблица.
const chartIsFloat = computed(() => chartColumn.value?.float === true);
// Длительность -> ось и тултип графика рисуют «2 ч 15 мин», а не сырые секунды
// (тот же тип колонки, по которому форматируется таблица).
const chartValueType = computed(() => (isDurationColumn(chartColumn.value) ? 'duration' : ''));
// Единица метрики ("шт", "шт/день") инвариантна по числу -> одна форма на все.
const chartUnitForms = computed(() => {
  const u = chartColumn.value?.unit;
  return u ? [u, u, u] : undefined;
});

// area: period-подпись остаётся ISO (timestamp) — компонент сам форматит дд.мм; bar:
// подпись разреза (статус/организация) — как есть.
const chartAreaData = computed(() =>
  aggRows.value.map((row) => ({ timestamp: row.label, count: cellValue(row, chartColumn.value) })),
);
const chartBarData = computed(() =>
  aggRows.value.map((row) => ({ label: row.label, value: cellValue(row, chartColumn.value) })),
);

// Период -> ровная высота; категориальный bar растёт с числом разрезов, но в рамках.
const chartHeight = computed(() => {
  if (isPeriod.value) return 300;
  return Math.min(380, Math.max(220, aggRows.value.length * 40));
});

// Новый результат: list/пусто — только таблица; aggregate — метрику графика на первую
// колонку. Вид оставляем как выбрал пользователь, чтобы повторный отчёт не сбрасывал
// его на таблицу, но тип категориального графика сбрасываем на столбцы: выбор кольца
// относился к прошлому отчёту, для нового он не должен залипать.
watch(() => props.result, (r) => {
  chartKind.value = 'bar';
  if (r?.mode !== 'aggregate') {
    view.value = 'table';
    return;
  }
  chartMetric.value = aggColumns.value[0]?.key || '';
}, { immediate: true });

const { exporting, exportReport } = useReportExport();

// Холст текущего графика: с него снимается картинка при выгрузке в png.
const chartBox = ref(null);

// list считаем по его собственным строкам, aggregate — по нормализованным.
const canExport = computed(() => {
  const r = props.result;
  if (!r) return false;
  return r.mode === 'list' ? (r.rows?.length || 0) > 0 : hasRows.value;
});

async function onExport(format) {
  try {
    if (format === 'png') {
      await exportChartPng(chartBox.value?.querySelector('canvas'), props.meta);
      return;
    }
    await exportReport(props.result, props.meta, format);
  } catch (e) {
    emit('export-error', e?.message || 'Не удалось выгрузить отчёт');
  }
}

// Значение колонки (транспорт values/float_values и «нет данных» -> null) читаем по
// общему контракту колонок — тому же, по которому строится выгрузка.
function cellValue(row, col) {
  return metricValue(row?.values, row?.float_values, col);
}

function totalValue(col) {
  return metricValue(aggTotals.value, aggFloatTotals.value, col);
}

// Длительность форматируем по ТИПУ колонки (секунды -> «2 ч 15 мин»), «нет данных»
// -> «—»; прочие метрики — число с локальным разделителем.
function formatMetricValue(value, col) {
  if (value === null || value === undefined) return '—';
  if (isDurationColumn(col)) return formatReportCell(value, 'duration');
  return formatNumber(value, col?.float);
}

// Период-разрез отдаёт подписи строк как YYYY-MM-DD -> человекочитаемое дд.мм.гггг.
function rowLabel(label) {
  return isPeriod.value ? formatDateRu(label) : label;
}

function formatNumber(value, float) {
  const n = Number(value);
  if (!Number.isFinite(n)) return value;
  return float
    ? n.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
    : n.toLocaleString('ru-RU');
}

function formatCell(value, type) {
  if (value === null || value === undefined || value === '') return '—';
  return formatReportCell(value, type);
}
</script>

<style scoped>
.rr {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rr__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 180px;
  text-align: center;
  color: var(--color-text-muted);
}

.rr__state--error {
  color: var(--danger-text);
}

.rr__empty svg {
  opacity: 0.35;
}

.rr__empty p {
  margin: 0;
  font-size: 15px;
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

.rr__toolbar--end {
  justify-content: flex-end;
}

/* Меню выгрузки (Excel/PDF) прижато вправо в панели инструментов. */
.rr__export {
  margin-left: auto;
}

.rr__toolbar--end .rr__export {
  margin-left: 0;
}

.rr__seg {
  display: inline-flex;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  padding: 3px;
  gap: 2px;
}

/* Метрик может быть много (cross-tab pivot) -> перенос на несколько строк;
   pill-радиус на многострочном блоке смотрится криво, поэтому скругление поменьше. */
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
  color: var(--accent-text);
}

.rr__seg-btn--active {
  background: var(--color-primary);
  color: var(--accent-contrast);
}

.rr__seg-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.rr__slot {
  width: 100%;
}

/* Мягкий fade при смене вида/типа графика (transform+opacity, без layout-анимаций). */
.rr-fade-enter-active,
.rr-fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.rr-fade-enter-from,
.rr-fade-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

.rr__table-wrap {
  overflow-x: auto;
  /* Таблица не растёт бесконечно — длинный разрез скроллится внутри панели. */
  max-height: 460px;
  overflow-y: auto;
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
  z-index: 1;
  background: var(--color-bg);
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 10px 14px;
  white-space: nowrap;
  border-bottom: 1px solid var(--color-border);
}

/* Числовой заголовок выравниваем вправо вслед за ячейками: у `.rr__table thead th`
   выше специфичность, чем у `.rr__num`, поэтому без явного правила заголовок
   оставался слева, а числа справа (рассогласование). */
.rr__table thead th.rr__num {
  text-align: right;
}

.rr__table tbody td {
  padding: 9px 14px;
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
  position: sticky;
  bottom: 0;
  padding: 10px 14px;
  color: var(--color-text);
  border-top: 2px solid var(--color-border);
  background: var(--color-bg);
  white-space: nowrap;
}

.rr__num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.rr__th--sortable {
  cursor: pointer;
  user-select: none;
}
.rr__th--sortable:hover {
  color: var(--color-primary);
}
.rr__sort {
  font-size: 11px;
  opacity: 0.8;
}

.rr__norows {
  text-align: center;
  color: var(--color-text-muted);
  padding: 24px 14px;
}

.rr__footer {
  font-size: 13px;
  color: var(--color-text);
}

.rr__footer-sep {
  color: var(--color-text-muted);
  margin: 0 4px;
}

.rr__truncated {
  margin-left: 8px;
  color: var(--color-text-muted);
}

/* Мобилка (#1097). Таблица результата - дамп произвольного числа колонок (list -
   сырые строки, aggregate - разрез + N метрик), поэтому не карточки, а честный
   горизонтальный скролл: обёртка .rr__table-wrap (overflow-x:auto выше) держит
   широкую таблицу внутри панели, страница не разъезжается. На телефоне уплотняем
   ячейки, чтобы до включения скролла помещалось больше колонок. Тулбар уже
   переносит сегменты и «Экспорт» строками (flex-wrap выше), а сегмент метрик
   ужимается и переносит свои кнопки внутри - трогать его flex-shrink нельзя. */
@media (max-width: 768px) {
  .rr__table {
    font-size: 13px;
  }

  .rr__table thead th {
    padding: 8px 10px;
  }

  .rr__table tbody td,
  .rr__table tfoot td {
    padding: 8px 10px;
  }
}
</style>
