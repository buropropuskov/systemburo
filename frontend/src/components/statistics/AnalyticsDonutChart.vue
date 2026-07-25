<template>
  <div
    class="donut-chart"
    :style="{ height: height + 'px' }"
  >
    <VueApexCharts
      v-if="hasData"
      type="donut"
      :height="height"
      :options="options"
      :series="series"
    />
    <div
      v-else
      class="donut-chart__empty"
    >
      Нет данных для отображения
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import VueApexCharts from 'vue3-apexcharts';

const props = defineProps({
  /** Сегменты в форме [{ label, value }]; label — подпись доли (тип вложения, статус). */
  data: {
    type: Array,
    default: () => [],
  },
  height: {
    type: Number,
    default: 280,
  },
  /**
   * Палитра сегментов. Приглушённый аналитический набор от primary проекта,
   * без неона; токена-палитры в проекте нет, поэтому держим дефолт здесь.
   */
  colors: {
    type: Array,
    default: () => [
      '#4F5BDF', '#6E8BE8', '#34A0A4', '#E0A458',
      '#D1697F', '#7B6FCB', '#5BA88A', '#C2823C',
    ],
  },
  /** Подпись по центру кольца (итог по всем сегментам). */
  totalLabel: {
    type: String,
    default: 'Всего',
  },
  /** Три формы склонения единицы тултипа [одна, две-четыре, пять+]. */
  unitForms: {
    type: Array,
    default: () => ['значение', 'значения', 'значений'],
  },
  /** Дробная метрика: тултип/итог не округляют до целых. */
  isFloat: {
    type: Boolean,
    default: false,
  },
});

// Нулевые сегменты не рисуем — пустые доли искажают кольцо и легенду.
const segments = computed(() =>
  props.data
    .map((d) => ({ label: String(d.label), value: Number(d.value) || 0 }))
    .filter((d) => d.value > 0),
);

const hasData = computed(() => segments.value.length > 0);

const labels = computed(() => segments.value.map((d) => d.label));
const series = computed(() => segments.value.map((d) => d.value));

function pluralize(n) {
  const [one, few, many] = props.unitForms;
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

function formatValue(v) {
  const num = Number(v) || 0;
  return props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
    : Math.round(num).toLocaleString('ru-RU');
}

const options = computed(() => ({
  chart: {
    type: 'donut',
    height: props.height,
    fontFamily: 'inherit',
    toolbar: { show: false },
    animations: { enabled: true, easing: 'easeinout', speed: 400 },
  },
  colors: props.colors,
  labels: labels.value,
  stroke: { width: 2, colors: ['#fff'] },
  dataLabels: {
    enabled: true,
    // Внутри сегмента — доля в процентах (val уже процент для donut).
    formatter: (val) => `${Math.round(val)}%`,
    style: { fontSize: '12px', fontWeight: 600 },
    dropShadow: { enabled: false },
  },
  plotOptions: {
    pie: {
      donut: {
        size: '64%',
        labels: {
          show: true,
          value: {
            color: '#333',
            fontSize: '20px',
            fontWeight: 700,
            formatter: (v) => formatValue(v),
          },
          total: {
            show: true,
            label: props.totalLabel,
            color: '#a2a2a2',
            fontSize: '12px',
            formatter: (w) => {
              const sum = w.globals.seriesTotals.reduce((a, b) => a + b, 0);
              return formatValue(sum);
            },
          },
        },
      },
    },
  },
  states: { hover: { filter: { type: 'lighten', value: 0.06 } } },
  legend: {
    position: 'bottom',
    fontSize: '12px',
    labels: { colors: '#666' },
    markers: { width: 10, height: 10, radius: 4 },
    itemMargin: { horizontal: 8, vertical: 2 },
  },
  tooltip: {
    theme: 'dark',
    y: {
      formatter: (v) => `${formatValue(v)} ${pluralize(Math.round(Number(v) || 0))}`,
      title: { formatter: (name) => `${name}:` },
    },
  },
}));
</script>

<style scoped>
.donut-chart {
  position: relative;
  width: 100%;
}

.donut-chart__empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 13px;
}
</style>
