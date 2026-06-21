<template>{{ display }}</template>

<script setup>
import { ref, watch, onBeforeUnmount } from 'vue';

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

function toNumber(v) {
  if (v === null || v === undefined || v === '') return null;
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

function format(n) {
  return n == null ? '—' : Math.round(n).toLocaleString('ru-RU');
}

function prefersReducedMotion() {
  try {
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  } catch {
    return false;
  }
}

// current — числовое показанное значение (для продолжения анимации с середины),
// display — строка в DOM.
const current = ref(toNumber(props.value));
const display = ref(format(current.value));

let rafId = null;
let started = false;
let startTs = 0;
let startVal = 0;
let targetVal = 0;

function cancel() {
  if (rafId !== null) {
    cancelAnimationFrame(rafId);
    rafId = null;
  }
}

function applyImmediate(n) {
  cancel();
  current.value = n;
  display.value = format(n);
}

function step(ts) {
  // Якорим стартовый timestamp на первом кадре (он может быть 0).
  if (!started) {
    started = true;
    startTs = ts;
  }
  const p = Math.min(1, (ts - startTs) / props.duration);
  // easeOutCubic — быстрый старт, мягкое торможение к финалу.
  const eased = 1 - Math.pow(1 - p, 3);
  const val = startVal + (targetVal - startVal) * eased;
  current.value = val;
  display.value = format(val);
  if (p < 1) {
    rafId = requestAnimationFrame(step);
  } else {
    applyImmediate(targetVal);
  }
}

watch(
  () => props.value,
  (next) => {
    const n = toNumber(next);
    // Появление из прочерка, уход в прочерк, без изменения или без анимации —
    // моментально, иначе count-up от текущего значения к новому.
    if (
      n == null
      || current.value == null
      || n === current.value
      || props.duration <= 0
      || prefersReducedMotion()
      || typeof requestAnimationFrame !== 'function'
    ) {
      applyImmediate(n);
      return;
    }
    cancel();
    started = false;
    startVal = current.value;
    targetVal = n;
    rafId = requestAnimationFrame(step);
  }
);

onBeforeUnmount(cancel);
</script>
