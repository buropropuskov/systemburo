<template>
  <div
    class="area-chart"
    :style="{ height: height + 'px' }"
  >
    <VueApexCharts
      v-if="hasData"
      type="area"
      :height="height"
      :options="options"
      :series="series"
    />
    <div
      v-else
      class="area-chart__empty"
    >
      Нет данных для отображения
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import VueApexCharts from 'vue3-apexcharts';
import { formatDuration } from '@/utils/datetime';

const props = defineProps({
  /** Точки ряда в форме [{ timestamp, count }] — совместимо с timeline дашборда. */
  data: {
    type: Array,
    default: () => [],
  },
  /**
   * Явные подписи оси X. Нужны рядам без дат (тренд инсайтов — серия по дням
   * без самих дат: движок отдаёт разреженные бины, восстановить даты нельзя).
   * null -> подписи берутся из дат timestamp, как у timeline.
   */
  categories: {
    type: Array,
    default: null,
  },
  height: {
    type: Number,
    default: 300,
  },
  color: {
    type: String,
    default: '#4F5BDF',
  },
  /** Имя ряда (для доступности; легенда скрыта). */
  seriesName: {
    type: String,
    default: 'Значение',
  },
  /** Три формы склонения единицы тултипа [одна, две-четыре, пять+]. */
  unitForms: {
    type: Array,
    default: () => ['значение', 'значения', 'значений'],
  },
  /** Дробная метрика (среднее и т.п.): ось Y и тултип не округляют до целых. */
  isFloat: {
    type: Boolean,
    default: false,
  },
  /**
   * Тип значения ряда. 'duration' — секунды: ось и тултип рисуют «2 ч 15 мин»
   * вместо сырых 8100. Пусто — число с единицей из unitForms.
   */
  valueType: {
    type: String,
    default: '',
  },
});

// Ряд, где значения нет ни у одной точки (этап не прошёл никто), — это тоже «нет
// данных»: рисовать пустую сетку с осями значило бы выдать отсутствие данных за сбой.
const hasData = computed(() => props.data.some((d) => d.count != null));

const isDuration = computed(() => props.valueType === 'duration');

// Дата timeline приходит как 'YYYY-MM-DD'. Парсим вручную, без new Date(), чтобы
// не словить сдвиг таймзоны (date-only -> UTC-полночь -> съезд на -3ч в МСК).
function dateParts(ts) {
  const s = String(ts ?? '').slice(0, 10);
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  return m ? { y: m[1], mo: m[2], d: m[3] } : null;
}

function formatShort(ts) {
  const p = dateParts(ts);
  return p ? `${p.d}.${p.mo}` : String(ts ?? '');
}

function formatFull(ts) {
  const p = dateParts(ts);
  return p ? `${p.d}.${p.mo}.${p.y}` : String(ts ?? '');
}

function pluralize(n) {
  const [one, few, many] = props.unitForms;
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

const categories = computed(() =>
  props.categories ?? props.data.map((d) => formatShort(d.timestamp))
);
const fullLabels = computed(() =>
  props.categories ?? props.data.map((d) => formatFull(d.timestamp))
);
// null/undefined — «нет данных» (производные метрики: этап никто не прошёл), и это
// НЕ ноль: Apex рисует такую точку разрывом, а не значением на дне шкалы, иначе
// «данных нет» читалось бы как «прошло мгновенно» (см. metricValue).
const values = computed(() =>
  props.data.map((d) => (d.count == null ? null : Number(d.count) || 0))
);

const series = computed(() => [{ name: props.seriesName, data: values.value }]);

const options = computed(() => ({
  chart: {
    type: 'area',
    height: props.height,
    fontFamily: 'inherit',
    toolbar: { show: false },
    zoom: { enabled: false },
    animations: { enabled: true, easing: 'easeinout', speed: 400 },
  },
  colors: [props.color],
  dataLabels: { enabled: false },
  stroke: { curve: 'smooth', width: 2, lineCap: 'round' },
  // Мягкий вертикальный градиент в палитре проекта — аналитический стиль без неона.
  fill: {
    type: 'gradient',
    gradient: {
      shadeIntensity: 1,
      opacityFrom: 0.32,
      opacityTo: 0.02,
      stops: [0, 95],
    },
  },
  grid: {
    borderColor: '#eef0f7',
    strokeDashArray: 0,
    xaxis: { lines: { show: false } },
    padding: { top: 0, right: 8, bottom: 0, left: 8 },
  },
  markers: { size: 0, strokeWidth: 2, hover: { size: 5 } },
  xaxis: {
    categories: categories.value,
    tickAmount: Math.min(8, categories.value.length),
    labels: {
      rotate: 0,
      hideOverlappingLabels: true,
      style: { colors: '#a2a2a2', fontSize: '11px' },
    },
    axisBorder: { show: false },
    axisTicks: { show: false },
    tooltip: { enabled: false },
  },
  yaxis: {
    min: 0,
    forceNiceScale: true,
    labels: {
      style: { colors: '#a2a2a2', fontSize: '11px' },
      formatter: (v) => formatAxis(v),
    },
  },
  legend: { show: false },
  tooltip: {
    theme: 'dark',
    x: {
      formatter: (_val, opts) => fullLabels.value[opts?.dataPointIndex] ?? '',
    },
    y: {
      formatter: (v) => formatTooltipValue(v),
      title: { formatter: () => '' },
    },
    marker: { show: true },
  },
}));

function formatAxis(v) {
  if (isDuration.value) return formatDuration(v);
  const num = Number(v) || 0;
  return props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 1 })
    : Math.round(num).toLocaleString('ru-RU');
}

// У длительности единица уже внутри текста («2 ч 15 мин») — склонять нечего.
// Точку-разрыв Apex тултипом не показывает, но при hover по соседней серии
// значение может прийти null — рисуем «—», а не «0».
function formatTooltipValue(v) {
  if (v == null) return '—';
  if (isDuration.value) return formatDuration(v);
  const num = Number(v) || 0;
  const shown = props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
    : Math.round(num).toLocaleString('ru-RU');
  return `${shown} ${pluralize(Math.round(num))}`;
}
</script>

<style scoped>
.area-chart {
  position: relative;
  width: 100%;
}

.area-chart__empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 13px;
}
</style>
