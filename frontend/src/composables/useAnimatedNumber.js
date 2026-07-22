import { ref, watch, onBeforeUnmount } from 'vue';

/**
 * Общий движок «числа, доезжающего до нового значения»: rAF-tween с easeOutCubic,
 * продолжением с середины при смене цели и мгновенным применением при
 * prefers-reduced-motion. Потребители - AnimatedNumber (аналитика, локализованный
 * формат с прочерком) и AnimatedCounter (живые счётчики шапки таблиц).
 */

function toNumber(v) {
  if (v === null || v === undefined || v === '') return null;
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

function prefersReducedMotion() {
  try {
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  } catch {
    return false;
  }
}

function defaultFormat(n) {
  return n == null ? '—' : String(Math.round(n));
}

/**
 * @param {() => number|string|null} source - getter отслеживаемого значения
 * @param {{ duration?: () => number, format?: (n: number|null) => string }} options
 * @returns {{ display: import('vue').Ref<string> }}
 */
export function useAnimatedNumber(source, { duration = () => 600, format = defaultFormat } = {}) {
  // current - числовое показанное значение (для продолжения анимации с середины),
  // display - строка в DOM.
  const current = ref(toNumber(source()));
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
    const p = Math.min(1, (ts - startTs) / duration());
    // easeOutCubic - быстрый старт, мягкое торможение к финалу.
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

  watch(source, (next) => {
    const n = toNumber(next);
    // Появление из прочерка, уход в прочерк, без изменения или без анимации -
    // моментально, иначе count-up от текущего значения к новому.
    if (
      n == null
      || current.value == null
      || n === current.value
      || duration() <= 0
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
  });

  onBeforeUnmount(cancel);

  return { display };
}
