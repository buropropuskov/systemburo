<template>
  <div
    class="daily-chart"
    :style="{ height: height + 'px' }"
  >
    <canvas
      ref="canvas"
      role="img"
      :aria-label="summary"
    />
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { formatDay, formatNum } from '@/utils/requestLogsFormat';
import {
  AXIS_LABEL, GRID_COLOR, TOOLTIP_STYLE, cssVariable, lighten, themeColor, useChartCanvas
} from '@/components/statistics/useChartCanvas';

/**
 * Обращения по суткам: столбик - все запросы дня, красная шапка - ответы с
 * ошибкой. Один ряд с долей ошибок отдельной осью здесь не годится: две шкалы
 * в одном поле читаются как одна, и всплеск ошибок кажется всплеском нагрузки.
 *
 * Оформление (сетка, подписи, подсказка) берётся из общего модуля аналитики,
 * а собственный компонент нужен из-за контракта: `AnalyticsBarChart` рисует
 * один ряд с единицей измерения, здесь же два составных ряда и доля ошибок в
 * подсказке.
 */
const props = defineProps({
  /** Ряд суток подряд; `requests: null` - день без записей (см. dailyChartPoints). */
  points: {
    type: Array,
    default: () => [],
  },
  height: {
    type: Number,
    default: 220,
  },
});

const canvas = ref(null);
const { isNarrow } = useNarrowScreen();

const SUCCESS_LABEL = 'Успешных';
const ERROR_LABEL = 'С ошибкой';
const TOP_CORNERS = { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 };

/**
 * Цвет ряда под курсором - тот же цвет темы, подмешанный к белому. Оставив его
 * прежним, подсказку не с чем связать: над каким столбиком она всплыла, по
 * самому графику не видно.
 * @param {string} name имя переменной темы
 * @param {string} fallback цвет для окружения без темы
 * @returns {(ctx: object) => string}
 */
function themeHover(name, fallback) {
  return (ctx) => lighten(cssVariable(ctx?.chart?.canvas, name, fallback), 0.12);
}

/** Подпись оси: сутки без года, год стоит в подсказке и в охвате периода. */
function axisDay(day) {
  return formatDay(day).slice(0, 5);
}

function pointAt(index) {
  return props.points[index] || {};
}

function errorsAt(index) {
  return Number(pointAt(index).errors) || 0;
}

/** Доля ответов с ошибкой за сутки, той же точности, что и в карточках сводки. */
function errorRateAt(index) {
  const requests = Number(pointAt(index).requests) || 0;
  if (!requests) return '0.00';
  return ((errorsAt(index) / requests) * 100).toFixed(2);
}

// Успешные и ошибочные складываются в высоту столбика, поэтому нижний ряд несёт
// разность. Число дня целиком человек читает в подсказке и в подписи охвата.
const successValues = computed(() => props.points.map((p) => (
  p.requests == null ? null : Math.max((Number(p.requests) || 0) - (Number(p.errors) || 0), 0)
)));

// Сутки без ошибок остаются пустыми, а не нулевыми: с minBarLength нулевое
// значение нарисовало бы ту же полоску, что и единственная ошибка за день.
const errorValues = computed(() => props.points.map((p) => (
  p.requests == null || !Number(p.errors) ? null : Number(p.errors)
)));

const summary = computed(() => {
  const days = props.points.filter((p) => p.requests != null).length;
  const requests = props.points.reduce((sum, p) => sum + (Number(p.requests) || 0), 0);
  const errors = props.points.reduce((sum, p) => sum + (Number(p.errors) || 0), 0);
  return `Обращения по суткам: дней с записями ${days}, запросов ${formatNum(requests)}, `
    + `из них с ошибкой ${formatNum(errors)}`;
});

// Подписи дат на телефоне сливаются в полосу: ниже мобильного порога их шесть,
// на широком экране двенадцать. Сами столбики остаются все.
const maxTicks = computed(() => (isNarrow.value ? 6 : 12));

const config = computed(() => ({
  type: 'bar',
  data: {
    labels: props.points.map((p) => axisDay(p.day)),
    datasets: [
      {
        label: SUCCESS_LABEL,
        data: successValues.value,
        backgroundColor: themeColor('--accent', '#4F5BDF'),
        hoverBackgroundColor: themeHover('--accent', '#4F5BDF'),
        // Верх столбика скругляется у того ряда, который в этот день сверху:
        // иначе под красной шапкой остаётся ямка от скруглённого синего.
        borderRadius: (ctx) => (errorsAt(ctx.dataIndex) ? 0 : TOP_CORNERS),
        borderSkipped: false,
        categoryPercentage: 0.9,
        barPercentage: 0.72,
      },
      {
        label: ERROR_LABEL,
        data: errorValues.value,
        backgroundColor: themeColor('--danger', '#dc3545'),
        hoverBackgroundColor: themeHover('--danger', '#dc3545'),
        borderRadius: TOP_CORNERS,
        borderSkipped: false,
        // Доля ошибок обычно меньше процента, и честная высота сегмента уходит
        // в доли пикселя - день с ошибками выглядел бы как день без них.
        minBarLength: 3,
        categoryPercentage: 0.9,
        barPercentage: 0.72,
      },
    ],
  },
  options: {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 400, easing: 'easeInOutQuad' },
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: {
        position: 'top',
        align: 'end',
        labels: {
          ...AXIS_LABEL,
          boxWidth: 10,
          boxHeight: 10,
          usePointStyle: true,
          pointStyle: 'rectRounded',
        },
      },
      tooltip: {
        ...TOOLTIP_STYLE,
        position: 'nearest',
        // Ряд ошибок в подсказке появляется, только когда ошибки были: «С
        // ошибкой: 0» повторяет долю из подвала. Ряд успешных остаётся всегда -
        // на сутках без записей он несёт объяснение вместо чисел. Отбор идёт по
        // самим суткам, а не по `raw`: столбик высотой null у Chart.js не
        // помечен пропущенным (в отличие от точки линии), и в подсказку он
        // приходит наравне с остальными.
        filter: (item) => item.datasetIndex === 0 || errorsAt(item.dataIndex) > 0,
        callbacks: {
          title: (items) => formatDay(pointAt(items[0]?.dataIndex).day),
          label: (item) => {
            const point = pointAt(item.dataIndex);
            if (point.requests == null) return 'Записей за эти сутки нет';
            const value = item.datasetIndex === 0
              ? Math.max((Number(point.requests) || 0) - errorsAt(item.dataIndex), 0)
              : errorsAt(item.dataIndex);
            return `${item.dataset.label}: ${formatNum(value)}`;
          },
          footer: (items) => {
            const index = items[0]?.dataIndex;
            const requests = pointAt(index).requests;
            if (requests == null) return '';
            return `Всего ${formatNum(Number(requests) || 0)}, доля ошибок ${errorRateAt(index)}%`;
          },
        },
      },
    },
    scales: {
      x: {
        stacked: true,
        grid: { display: false },
        border: { display: false },
        ticks: { ...AXIS_LABEL, maxRotation: 0, autoSkip: true, maxTicksLimit: maxTicks.value },
      },
      y: {
        stacked: true,
        beginAtZero: true,
        grid: { color: GRID_COLOR },
        border: { display: false },
        ticks: { ...AXIS_LABEL, maxTicksLimit: 6, callback: (v) => formatNum(Math.round(v)) },
      },
    },
  },
}));

useChartCanvas(canvas, config);
</script>

<style scoped>
.daily-chart {
  position: relative;
  width: 100%;
}
</style>
