<template>
  <div
    class="area-chart"
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
      class="area-chart__empty"
    >
      Нет данных для отображения
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { formatDuration } from '@/utils/datetime';
import { crosshairPlugin } from './linePlugins';
import {
  AXIS_LABEL,
  GRID_COLOR,
  TOOLTIP_STYLE,
  hoverPointStyle,
  useChartCanvas,
  verticalGradient,
} from './useChartCanvas';

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

const canvas = ref(null);

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
// НЕ ноль: линия рвётся на такой точке, а не проседает на дно шкалы, иначе
// «данных нет» читалось бы как «прошло мгновенно» (см. metricValue).
const values = computed(() =>
  props.data.map((d) => (d.count == null ? null : Number(d.count) || 0))
);

function formatAxis(v) {
  if (isDuration.value) return formatDuration(v);
  const num = Number(v) || 0;
  return props.isFloat
    ? num.toLocaleString('ru-RU', { maximumFractionDigits: 1 })
    : Math.round(num).toLocaleString('ru-RU');
}

// У длительности единица уже внутри текста («2 ч 15 мин») — склонять нечего.
// Точку-разрыв Chart.js подсказкой не показывает, но при наведении рядом
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

const config = computed(() => ({
  type: 'line',
  data: {
    labels: categories.value,
    datasets: [
      {
        label: props.seriesName,
        data: values.value,
        borderColor: props.color,
        borderWidth: 2,
        // Мягкий вертикальный градиент в палитре проекта — аналитический стиль без неона.
        backgroundColor: verticalGradient(props.color),
        fill: true,
        // Сглаживание кривой; 0.4 - привычный вид, который был у прежнего движка.
        tension: 0.4,
        pointRadius: 0,
        ...hoverPointStyle(props.color),
        // Разрыв на null не затягиваем: соединив соседей прямой, график
        // показал бы значение там, где данных нет.
        spanGaps: false,
        // Запас сверху: ряд, упирающийся в потолок шкалы, рисует точку прямо на
        // границе области, а Chart.js режет набор данных по этой границе - у
        // точки срезало верх. layout.padding это не лечит, он двигает саму
        // область, а не клип.
        clip: { top: 12, right: 0, bottom: 0, left: 0 },
      },
    ],
  },
  plugins: [crosshairPlugin],
  options: {
    responsive: true,
    maintainAspectRatio: false,
    // Точка ряда на самом верху шкалы выходит за область графика и срезается
    // краем холста - место под её радиус с обводкой.
    layout: { padding: { top: 10 } },
    animation: { duration: 400, easing: 'easeInOutQuad' },
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { display: false },
      tooltip: {
        ...TOOLTIP_STYLE,
        // Рядом с точкой, а не по середине ряда: у края области подсказка
        // разворачивается на другую сторону и точку не закрывает.
        position: 'nearest',
        callbacks: {
          title: (items) => fullLabels.value[items?.[0]?.dataIndex] ?? '',
          label: (item) => formatTooltipValue(item?.raw),
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        border: { display: false },
        ticks: { ...AXIS_LABEL, maxRotation: 0, autoSkip: true, maxTicksLimit: 8 },
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
