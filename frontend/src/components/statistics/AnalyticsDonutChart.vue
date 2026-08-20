<template>
  <div
    class="donut-chart"
    :style="{ height: height + 'px' }"
  >
    <canvas
      v-if="hasData"
      ref="canvas"
      role="img"
      :aria-label="totalLabel"
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
import { computed, ref } from 'vue';
import { centerLabelPlugin, sliceLabelsPlugin } from './donutPlugins';
import { TOOLTIP_STYLE, lighten, themeColor, useChartCanvas } from './useChartCanvas';

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
  /**
   * Рисовать кольцо и без данных: серый ободок с нулём в центре вместо
   * заглушки текстом. Нужно там, где график - постоянная часть раскладки и его
   * исчезновение читается как поломка, а не как «за период ничего не было».
   */
  emptyRing: {
    type: Boolean,
    default: false,
  },
});

/** Высота строки легенды: её место резервирует кольцо без сегментов. */
const LEGEND_RESERVE = 30;

const canvas = ref(null);

// Нулевые сегменты не рисуем — пустые доли искажают кольцо и легенду.
const segments = computed(() =>
  props.data
    .map((d) => ({ label: String(d.label), value: Number(d.value) || 0 }))
    .filter((d) => d.value > 0),
);

const hasData = computed(() => segments.value.length > 0 || props.emptyRing);

// Кольцо без единого сегмента: рисуем ободок-заглушку, чтобы место графика не
// пустело. Заглушка не сегмент данных - у неё нет подсказки, доли и легенды.
const isEmptyRing = computed(() => segments.value.length === 0 && props.emptyRing);

const labels = computed(() => (isEmptyRing.value ? [''] : segments.value.map((d) => d.label)));
// Единица заглушки - не значение, а способ получить у Chart.js замкнутую дугу:
// сегмент нулевой величины он не рисует вовсе.
const series = computed(() => (isEmptyRing.value ? [1] : segments.value.map((d) => d.value)));

// Палитру раскладываем по сегментам сами: сегментов может быть больше, чем
// цветов, и тогда набор идёт по кругу.
const segmentColors = computed(() =>
  segments.value.map((_, i) => props.colors[i % props.colors.length]),
);

// Заглушка красится подложкой темы: ободок виден, но не притворяется данными.
// Цвет отдаётся функцией, а НЕ массивом из одной функции: значения внутри
// массива Chart.js раскладывает по сегментам как есть и вычисляемыми не считает
// - функция уходила в холст цветом и кольцо рисовалось чёрным.
const ringColor = themeColor('--surface-2', '#eef0f7');

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

const config = computed(() => ({
  type: 'doughnut',
  data: {
    labels: labels.value,
    datasets: [
      {
        data: series.value,
        backgroundColor: isEmptyRing.value ? ringColor : segmentColors.value,
        hoverBackgroundColor: isEmptyRing.value
          ? ringColor
          : segmentColors.value.map((c) => lighten(c, 0.06)),
        // Разделитель в цвет карточки: белая обводка на тёмной теме читалась
        // жирным кольцом вокруг диаграммы.
        borderColor: themeColor('--surface', '#ffffff'),
        borderWidth: 2,
        hoverBorderColor: themeColor('--surface', '#ffffff'),
      },
    ],
  },
  options: {
    responsive: true,
    maintainAspectRatio: false,
    // Место снизу вместо скрытой легенды: без него пустое кольцо раздувалось
    // на её высоту и рядом с соседним, у которого легенда есть, выглядело
    // кольцом другого размера.
    layout: { padding: { bottom: isEmptyRing.value ? LEGEND_RESERVE : 0 } },
    // Толщина кольца: та же доля радиуса, что была у прежнего движка.
    cutout: '64%',
    animation: { duration: 400, easing: 'easeInOutQuad' },
    plugins: {
      sliceLabels: { display: !isEmptyRing.value },
      legend: {
        display: !isEmptyRing.value,
        position: 'bottom',
        labels: {
          color: themeColor('--text', '#666'),
          font: { size: 12 },
          usePointStyle: true,
          pointStyle: 'rectRounded',
          boxWidth: 10,
          boxHeight: 10,
          padding: 12,
        },
      },
      tooltip: {
        ...TOOLTIP_STYLE,
        enabled: !isEmptyRing.value,
        callbacks: {
          // Имя сегмента уже стоит в строке значения — отдельный заголовок
          // повторял бы его.
          title: () => '',
          label: (item) => {
            const num = Number(item?.raw) || 0;
            return ` ${item?.label}: ${formatValue(num)} ${pluralize(Math.round(num))}`;
          },
        },
      },
    },
  },
  plugins: [
    sliceLabelsPlugin,
    centerLabelPlugin({
      label: props.totalLabel,
      format: formatValue,
      total: isEmptyRing.value ? 0 : null,
    }),
  ],
}));

useChartCanvas(canvas, config);
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
