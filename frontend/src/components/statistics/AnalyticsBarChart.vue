<template>
  <div
    class="bar-chart"
    :style="{ height: height + 'px' }"
  >
    <VueApexCharts
      v-if="hasData"
      type="bar"
      :height="height"
      :options="options"
      :series="series"
    />
    <div
      v-else
      class="bar-chart__empty"
    >
      Нет данных для отображения
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import VueApexCharts from 'vue3-apexcharts';

const props = defineProps({
  /** Столбцы в форме [{ label, value }]; label — подпись оси X (час суток и т.п.). */
  data: {
    type: Array,
    default: () => [],
  },
  height: {
    type: Number,
    default: 240,
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
});

const hasData = computed(() => props.data.length > 0);

const categories = computed(() => props.data.map((d) => String(d.label)));
const values = computed(() => props.data.map((d) => Number(d.value) || 0));
const series = computed(() => [{ name: props.seriesName, data: values.value }]);

function pluralize(n) {
  const [one, few, many] = props.unitForms;
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

const options = computed(() => ({
  chart: {
    type: 'bar',
    height: props.height,
    fontFamily: 'inherit',
    toolbar: { show: false },
    zoom: { enabled: false },
    animations: { enabled: true, easing: 'easeinout', speed: 400 },
  },
  colors: [props.color],
  dataLabels: { enabled: false },
  plotOptions: {
    bar: { columnWidth: '62%', borderRadius: 4, borderRadiusApplication: 'end' },
  },
  states: { hover: { filter: { type: 'lighten', value: 0.08 } } },
  grid: {
    borderColor: '#eef0f7',
    strokeDashArray: 0,
    xaxis: { lines: { show: false } },
    padding: { top: 0, right: 8, bottom: 0, left: 8 },
  },
  xaxis: {
    categories: categories.value,
    tickAmount: Math.min(12, categories.value.length),
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
    y: {
      formatter: (v) => {
        const num = Number(v) || 0;
        const shown = props.isFloat
          ? num.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
          : Math.round(num).toLocaleString('ru-RU');
        return `${shown} ${pluralize(Math.round(num))}`;
      },
      title: { formatter: () => '' },
    },
  },
}));

function formatAxis(v) {
  const num = Number(v) || 0;
  return props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 1 })
    : Math.round(num).toLocaleString('ru-RU');
}
</script>

<style scoped>
.bar-chart {
  position: relative;
  width: 100%;
}

.bar-chart__empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-size: 13px;
}
</style>
