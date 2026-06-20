<template>
  <svg
    class="spark"
    :viewBox="`0 0 ${W} ${H}`"
    :width="W"
    :height="H"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <polyline
      :points="points"
      fill="none"
      :stroke="strokeColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({
  /** Числовой ряд значений. */
  series: {
    type: Array,
    default: () => [],
  },
  /** Направление — задаёт цвет линии. */
  direction: {
    type: String,
    default: 'flat',
  },
});

const W = 120;
const H = 32;
const PAD = 3;
const colorByDir = { up: '#28a745', down: '#dc3545', flat: '#8a90a6' };

const strokeColor = computed(() => colorByDir[props.direction] || colorByDir.flat);

const points = computed(() => {
  const s = props.series.map((v) => Number(v) || 0);
  if (s.length === 0) return '';
  if (s.length === 1) {
    const y = H / 2;
    return `${PAD},${y} ${W - PAD},${y}`;
  }
  const min = Math.min(...s);
  const max = Math.max(...s);
  const span = max - min || 1;
  const stepX = (W - PAD * 2) / (s.length - 1);
  return s
    .map((v, i) => {
      const x = PAD + i * stepX;
      const y = H - PAD - ((v - min) / span) * (H - PAD * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
});
</script>
