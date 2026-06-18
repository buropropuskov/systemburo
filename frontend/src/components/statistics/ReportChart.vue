<template>
  <div class="rc">
    <div
      v-if="libError"
      class="rc__msg rc__msg--error"
    >
      Не удалось загрузить библиотеку графиков
    </div>
    <canvas
      v-else
      ref="canvasEl"
    />
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue';

const props = defineProps({
  // Строки агрегатного отчёта: [{ label, value }].
  rows: { type: Array, default: () => [] },
  // Тип графика: 'bar' | 'pie' | 'line'.
  type: { type: String, default: 'bar' },
  // Единица измерения метрики (для подписи набора данных).
  unit: { type: String, default: '' },
  // Подпись серии (имя метрики); по умолчанию — «Количество».
  label: { type: String, default: '' },
});

const canvasEl = ref(null);
const libError = ref(false);

let chart = null;
let ChartCtor = null;
// Гонка ленивой загрузки: пока import резолвится, props могут смениться и watch
// запустит второй render. Токен гарантирует, что устаревший render не создаст
// второй Chart поверх актуального (иначе два инстанса на одном canvas).
let renderSeq = 0;

const PALETTE = [
  '#4F5BDF', '#28a745', '#ffc107', '#dc3545', '#17a2b8',
  '#6f42c1', '#fd7e14', '#20c997', '#e83e8c', '#6610f2',
];

async function loadLib() {
  if (ChartCtor) return ChartCtor;
  try {
    const mod = await import('chart.js/auto');
    ChartCtor = mod.Chart || mod.default;
    return ChartCtor;
  } catch {
    libError.value = true;
    return null;
  }
}

function lineBarOptions() {
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      y: { beginAtZero: true, ticks: { precision: 0 } },
      x: { ticks: { autoSkip: true, maxRotation: 0 } },
    },
  };
}

function buildConfig() {
  const labels = props.rows.map((r) => r.label);
  const data = props.rows.map((r) => Number(r.value) || 0);
  const base = props.label || 'Количество';
  const datasetLabel = `${base}${props.unit ? `, ${props.unit}` : ''}`;

  if (props.type === 'pie') {
    return {
      type: 'pie',
      data: {
        labels,
        datasets: [{
          data,
          backgroundColor: labels.map((_, i) => PALETTE[i % PALETTE.length]),
          borderColor: '#fff',
          borderWidth: 2,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: { legend: { position: 'right' } },
      },
    };
  }

  if (props.type === 'line') {
    return {
      type: 'line',
      data: {
        labels,
        datasets: [{
          label: datasetLabel,
          data,
          borderColor: '#4F5BDF',
          backgroundColor: 'rgba(79, 91, 223, 0.12)',
          fill: true,
          tension: 0.3,
          pointRadius: 3,
        }],
      },
      options: lineBarOptions(),
    };
  }

  return {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        label: datasetLabel,
        data,
        backgroundColor: labels.map((_, i) => PALETTE[i % PALETTE.length]),
        borderRadius: 6,
        maxBarThickness: 48,
      }],
    },
    options: lineBarOptions(),
  };
}

async function render() {
  const seq = ++renderSeq;
  const Ctor = await loadLib();
  if (seq !== renderSeq || !Ctor || !canvasEl.value) return;
  if (chart) { chart.destroy(); chart = null; }
  chart = new Ctor(canvasEl.value, buildConfig());
}

onMounted(render);
// rows/unit приходят из API новой ссылкой при каждом отчёте, type меняется кликом —
// глубокое наблюдение не нужно, перерисовываем по смене любой из ссылок.
watch(() => [props.rows, props.type, props.unit, props.label], render);
onBeforeUnmount(() => {
  if (chart) { chart.destroy(); chart = null; }
});
</script>

<style scoped>
.rc {
  position: relative;
  height: 360px;
  width: 100%;
}

.rc__msg {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--color-text-muted);
}

.rc__msg--error {
  color: #c0392b;
}
</style>
