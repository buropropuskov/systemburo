<template>
  <div
    class="bar-chart"
    :style="{ height: height + 'px' }"
  >
    <canvas
      v-if="hasData"
      ref="canvas"
      role="img"
      :aria-label="seriesName"
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
import { computed, ref } from 'vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { formatDuration } from '@/utils/datetime';
import { AXIS_LABEL, GRID_COLOR, TOOLTIP_STYLE, lighten, useChartCanvas } from './useChartCanvas';

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
  /**
   * Тип значения ряда. 'duration' — секунды: ось и тултип рисуют «2 ч 15 мин»
   * вместо сырых 8100. Пусто — число с единицей из unitForms.
   */
  valueType: {
    type: String,
    default: '',
  },
});

const canvas = ref(null);
const { isNarrow } = useNarrowScreen();

// Ряд, где значения нет ни у одного столбца (этап не прошёл никто), — это тоже «нет
// данных»: рисовать пустую сетку с осями значило бы выдать отсутствие данных за сбой.
const hasData = computed(() => props.data.some((d) => d.value != null));

const isDuration = computed(() => props.valueType === 'duration');

const categories = computed(() => props.data.map((d) => String(d.label)));
// null/undefined — «нет данных» (производные метрики: этап никто не прошёл), и это
// НЕ ноль: столбца просто не будет, иначе нулевая высота читалась бы как реальное
// значение «прошло мгновенно» (см. metricValue).
const values = computed(() =>
  props.data.map((d) => (d.value == null ? null : Number(d.value) || 0))
);

function pluralize(n) {
  const [one, few, many] = props.unitForms;
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

// Длина, при которой подпись оси X ещё помещается в слот на телефоне; длиннее -
// усекаем с многоточием. «00:00»/статусы короче и проходят как есть.
const LABEL_MAX = 14;
function truncateLabel(value) {
  const s = String(value ?? '');
  return s.length > LABEL_MAX ? `${s.slice(0, LABEL_MAX - 1)}…` : s;
}

function formatAxis(v) {
  if (isDuration.value) return formatDuration(v);
  const num = Number(v) || 0;
  return props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 1 })
    : Math.round(num).toLocaleString('ru-RU');
}

// У длительности единица уже внутри текста («2 ч 15 мин») — склонять нечего.
// Пропущенный столбец подсказкой не показывается, но значение может прийти
// null — рисуем «—», а не «0».
function formatTooltipValue(v) {
  if (v == null) return '—';
  if (isDuration.value) return formatDuration(v);
  const num = Number(v) || 0;
  const shown = props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
    : Math.round(num).toLocaleString('ru-RU');
  return `${shown} ${pluralize(Math.round(num))}`;
}

// На узком экране 12 подписей «пика по часам» (24 бара) сливаются в нечитаемую
// полосу «00:0002:00...». Ниже мобильного брейкпоинта (--bp-mobile 768) режем
// число подписей оси X до 6, бары остаются все; длинные категориальные подписи
// разреза отчёта (организация, место) усекаем по числу символов - короткие
// числовые и статусы проходят целыми.
const maxTicks = computed(() => (isNarrow.value ? 6 : 12));

// Подпись берётся у самой шкалы: на категориальной оси в обработчик приходит
// не текст, а номер деления, и после autoSkip номера идут с пропусками -
// обращение к массиву по порядковому номеру давало бы чужую подпись.
function narrowLabel(value) {
  const label = typeof this?.getLabelForValue === 'function'
    ? this.getLabelForValue(value)
    : categories.value[value] ?? value;
  return truncateLabel(label);
}

// Ключ callback добавляется ТОЛЬКО на узком экране. Записать его со значением
// undefined нельзя: Chart.js накладывает переданные настройки на свои по наличию
// ключа, а не по значению, поэтому явный undefined затирает штатный обработчик
// категориальной оси - и вместо «00:00» на оси появляются номера делений
// (проверено в браузере, юнит-тест на undefined этого не отличал).
const xTicks = computed(() => ({
  ...AXIS_LABEL,
  maxRotation: 0,
  autoSkip: true,
  maxTicksLimit: maxTicks.value,
  ...(isNarrow.value ? { callback: narrowLabel } : {}),
}));

const config = computed(() => ({
  type: 'bar',
  data: {
    labels: categories.value,
    datasets: [
      {
        label: props.seriesName,
        data: values.value,
        backgroundColor: props.color,
        // Столбец под курсором светлеет - тот же отклик, что давал прежний
        // движок. Оставив цвет прежним, подсказку не с чем связать: над каким
        // из столбцов она всплыла, по самому графику не видно.
        hoverBackgroundColor: lighten(props.color, 0.08),
        borderRadius: { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 },
        borderSkipped: false,
        // 62% ширины слота под столбец - остальное зазор, как было раньше.
        categoryPercentage: 0.9,
        barPercentage: 0.69,
      },
    ],
  },
  options: {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 400, easing: 'easeInOutQuad' },
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { display: false },
      tooltip: {
        ...TOOLTIP_STYLE,
        position: 'nearest',
        callbacks: {
          label: (item) => formatTooltipValue(item?.raw),
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        border: { display: false },
        ticks: xTicks.value,
      },
      y: {
        beginAtZero: true,
        grid: { color: GRID_COLOR },
        border: { display: false },
        ticks: { ...AXIS_LABEL, callback: (v) => formatAxis(v) },
      },
    },
  },
}));

useChartCanvas(canvas, config);
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
  color: var(--text-muted);
  font-size: 13px;
}
</style>
