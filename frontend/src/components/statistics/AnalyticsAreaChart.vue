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

const props = defineProps({
  /** Точки ряда в форме [{ timestamp, count }] — совместимо с timeline дашборда. */
  data: {
    type: Array,
    default: () => [],
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
});

const hasData = computed(() => props.data.length > 0);

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

const categories = computed(() => props.data.map((d) => formatShort(d.timestamp)));
const fullLabels = computed(() => props.data.map((d) => formatFull(d.timestamp)));
const values = computed(() => props.data.map((d) => Number(d.count) || 0));

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
      formatter: (v) => Math.round(v).toLocaleString('ru-RU'),
    },
  },
  legend: { show: false },
  tooltip: {
    theme: 'dark',
    x: {
      formatter: (_val, opts) => fullLabels.value[opts?.dataPointIndex] ?? '',
    },
    y: {
      formatter: (v) => {
        const n = Math.round(Number(v) || 0);
        return `${n.toLocaleString('ru-RU')} ${pluralize(n)}`;
      },
      title: { formatter: () => '' },
    },
    marker: { show: true },
  },
}));
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
  color: #a2a2a2;
  font-size: 13px;
}
</style>
