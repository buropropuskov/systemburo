<template>
  <div
    class="refresh-overlay"
    :class="{ 'refresh-overlay--solid': solid }"
  >
    <LoaderSpinner :label="label" />
  </div>
</template>

<script setup>
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';

/**
 * Плёнка обновления поверх уже показанного списка: строки остаются на месте,
 * высота блока не схлопывается, прокрутка не улетает вверх (#1305). Первой
 * загрузке она не годится - под ней пусто, там нужен лоадер вместо содержимого.
 * Родитель обязан быть position: relative.
 */
defineProps({
  label: { type: String, default: 'Обновление…' },
  // Первой загрузке плёнка нужна непрозрачной: под ней ещё нет содержимого,
  // а пустые заготовки блоков просвечивают и врут о том, что данных нет.
  solid: { type: Boolean, default: false },
});
</script>

<style scoped>
.refresh-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--surface) 75%, transparent);
  backdrop-filter: blur(1px);
  z-index: 2;
  pointer-events: none;
}

.refresh-overlay--solid {
  background: var(--surface);
  backdrop-filter: none;
}
</style>
