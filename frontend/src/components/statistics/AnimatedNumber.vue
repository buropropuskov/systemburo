<template>{{ display }}</template>

<script setup>
import { useAnimatedNumber } from '@/composables/useAnimatedNumber';

const props = defineProps({
  /** Число для показа. null/пусто -> прочерк. */
  value: {
    type: [Number, String],
    default: null,
  },
  /** Длительность счётчика, мс. <=0 -> моментально (reduced-motion/тесты). */
  duration: {
    type: Number,
    default: 600,
  },
});

function format(n) {
  return n == null ? '—' : Math.round(n).toLocaleString('ru-RU');
}

const { display } = useAnimatedNumber(() => props.value, {
  duration: () => props.duration,
  format,
});
</script>
