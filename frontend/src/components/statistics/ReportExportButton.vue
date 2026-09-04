<template>
  <div
    ref="root"
    class="reb"
  >
    <button
      type="button"
      class="lk-button lk-button--ghost reb__trigger"
      :disabled="disabled || exporting"
      :aria-expanded="open"
      aria-haspopup="menu"
      data-testid="rr-export"
      @click="toggle"
    >
      {{ exporting ? 'Готовим…' : 'Экспорт' }}
      <svg
        class="reb__caret"
        :class="{ 'reb__caret--open': open }"
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M6 9l6 6 6-6"
          stroke="currentColor"
          stroke-width="2.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </button>
    <transition name="reb-menu">
      <div
        v-if="open"
        class="reb__menu"
        role="menu"
      >
        <button
          type="button"
          class="reb__item"
          role="menuitem"
          data-testid="rr-export-excel"
          @click="choose('excel')"
        >
          Excel (.xlsx)
        </button>
        <button
          type="button"
          class="reb__item"
          role="menuitem"
          data-testid="rr-export-pdf"
          @click="choose('pdf')"
        >
          PDF (.pdf)
        </button>
        <button
          v-if="withImage"
          type="button"
          class="reb__item"
          role="menuitem"
          data-testid="rr-export-png"
          @click="choose('png')"
        >
          Картинка (.png)
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue';

defineProps({
  disabled: { type: Boolean, default: false },
  exporting: { type: Boolean, default: false },
  // Картинку предлагаем только когда на экране график: у таблицы сохранять нечего.
  withImage: { type: Boolean, default: false },
});

const emit = defineEmits(['export']);

const root = ref(null);
const open = ref(false);

function toggle() {
  open.value = !open.value;
}

function choose(format) {
  open.value = false;
  emit('export', format);
}

function onDocMousedown(e) {
  if (open.value && root.value && !root.value.contains(e.target)) open.value = false;
}

function onKeydown(e) {
  if (e.key === 'Escape') open.value = false;
}

onMounted(() => {
  document.addEventListener('mousedown', onDocMousedown);
  document.addEventListener('keydown', onKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMousedown);
  document.removeEventListener('keydown', onKeydown);
});
</script>

<style scoped>
.reb {
  position: relative;
  display: inline-flex;
}

/* min-width держит ширину при смене «Экспорт» <-> «Готовим…», чтобы кнопка не дёргалась. */
.reb__trigger {
  min-width: 116px;
  justify-content: center;
  gap: 6px;
}

.reb__caret {
  transition: transform 0.18s ease;
}

.reb__caret--open {
  transform: rotate(180deg);
}

.reb__menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 20;
  min-width: 100%;
  display: flex;
  flex-direction: column;
  padding: 6px;
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
}

.reb__item {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
  text-align: left;
  white-space: nowrap;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.reb__item:hover {
  background: var(--color-primary-tint);
  color: var(--accent-text);
}

.reb-menu-enter-active,
.reb-menu-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.reb-menu-enter-from,
.reb-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
