<template>
  <svg
    width="14"
    height="14"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="path" />
  </svg>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({
  /** Направление изменения метрики. */
  direction: {
    type: String,
    default: 'flat',
  },
});

// Настоящая стрелка: вертикальное (или горизонтальное для flat) древко +
// наконечник. Отличается от шеврона '<>' наличием древка — владелец просил
// нормальную стрелку, а не галочку (#632).
const paths = {
  up: 'M8 13V4M4.5 7.5L8 4L11.5 7.5',
  down: 'M8 3V12M4.5 8.5L8 12L11.5 8.5',
  flat: 'M3 8H12M8.5 4.5L12 8L8.5 11.5',
};

const path = computed(() => paths[props.direction] || paths.flat);
</script>
