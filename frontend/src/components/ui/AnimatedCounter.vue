<template>
  <span
    class="animated-counter"
    data-testid="animated-counter"
  ><span
    class="animated-counter__reserve"
    aria-hidden="true"
  >{{ reserve }}</span><span class="animated-counter__value">{{ display }}</span></span>
</template>

<script setup>
/**
 * Живой счётчик шапки таблиц: число доезжает до нового значения за несколько
 * кадров вместо мгновенной подмены - скачок «7 -> 12» без промежуточных значений
 * читается как сбой, а не как приход машины. Движок анимации общий с
 * AnimatedNumber (аналитика) - useAnimatedNumber.
 *
 * Ширина слота фиксирована, поэтому смена 9 -> 10 не двигает соседей по шапке:
 * место держит скрытая строка цифр той же разрядности (реальные цифры, а не
 * min-width в ch - у Montserrat ch уже цифры, и слот на 2ch двузначное число
 * не вмещал).
 */
import { computed } from 'vue';
import { useAnimatedNumber } from '@/composables/useAnimatedNumber';

const props = defineProps({
  value: { type: Number, required: true },
  /** Длительность пересчёта, мс. */
  duration: { type: Number, default: 300 },
  /** Разрядность, под которую резервируется ширина (чтобы 9 -> 10 не двигало соседей). */
  minDigits: { type: Number, default: 2 },
});

const { display } = useAnimatedNumber(() => props.value, {
  duration: () => props.duration,
  // Без локализации: счётчик небольшой, а разделители тысяч сломали бы резерв ширины.
  format: (n) => String(Math.round(n ?? 0)),
});

const reserve = computed(() => '0'.repeat(Math.max(props.minDigits, display.value.length)));
</script>

<style scoped>
.animated-counter {
  position: relative;
  display: inline-block;
  /* Цифры одной ширины: 7 -> 8 не меняет длину строки на пропорциональном шрифте. */
  font-variant-numeric: tabular-nums;
  font-feature-settings: 'tnum';
}

.animated-counter__reserve {
  visibility: hidden;
}

.animated-counter__value {
  position: absolute;
  inset: 0;
  text-align: right;
}
</style>
