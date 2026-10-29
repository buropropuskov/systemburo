<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import PassageTableTools from './PassageTableTools.vue';
import RefreshButton from './RefreshButton.vue';

const props = defineProps({
  title: { type: String, required: true },
  compactTitle: { type: String, required: true },
  kind: { type: String, required: true },
  tableId: { type: Number, default: null },
  rows: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  preview: { type: Boolean, default: false },
  hasSettings: { type: Boolean, default: false },
});
defineEmits(['refresh', 'boundary-refresh', 'clock']);
const header = ref(null), menu = ref(null), compact = ref(true), passageAvailable = ref(false);
const more = ref(null), commands = ref(null);
const tight = ref(false);
const hasCommands = computed(() => props.hasSettings || passageAvailable.value);
let observer;
// Измеряем ширину шапки, а не команды, меняющие свою компоновку.
const measure = () => {
  const width = header.value?.clientWidth;
  if (width > 0) { compact.value = width < 1300; tight.value = width < 310; }
};
watch(hasCommands, available => { if (!available) menu.value?.close(); });
watch(compact, value => {
  if (value && commands.value?.contains(document.activeElement)) {
    nextTick(() => more.value?.focus({ preventScroll: true }));
  }
});
watch(() => menu.value?.isOpen, open => {
  if (!open && compact.value) nextTick(() => {
    if (commands.value?.contains(document.activeElement)) more.value?.focus({ preventScroll: true });
  });
});
function onCommand(event) {
  if (event.target.closest('button')) menu.value?.close();
}
onMounted(() => {
  measure();
  if (typeof ResizeObserver === 'function') {
    observer = new ResizeObserver(measure);
    observer.observe(header.value);
  } else window.addEventListener('resize', measure);
});
onBeforeUnmount(() => {
  observer?.disconnect();
  window.removeEventListener('resize', measure);
});
</script>

<template>
  <div ref="header" class="card-header passage-table-header" :class="{ 'passage-table-header--compact': compact && !preview, 'passage-table-header--tight': tight && !preview }">
    <div class="passage-table-header__title" :title="title">
      <div class="passage-table-header__full-title"><slot name="title" /></div>
      <h3 class="passage-table-header__short-title" :aria-label="title">{{ compactTitle }}</h3>
    </div>
    <slot name="extra" />
    <div v-if="!preview" class="passage-table-header__counter"><slot name="counter" /></div>
    <RefreshButton v-if="!preview" class="passage-table-header__refresh" aria-label="Обновить таблицу" title="Обновить таблицу" :loading="loading" @refresh="$emit('refresh')" />
    <BaseDropdown v-if="!preview" ref="menu" class="passage-table-header__menu" :class="{ 'passage-table-header__menu--empty': !hasCommands }"
      :options="[]" :inline="!compact" keep-mounted teleport menu-align="end" :menu-min-width="260" :menu-z-index="10020">
      <template #trigger="{ toggle, isOpen }">
        <button ref="more" type="button" class="lk-button lk-button--ghost passage-table-header__more" aria-label="Действия таблицы" title="Действия таблицы"
          :aria-expanded="isOpen" @click="toggle">⋯</button>
      </template>
      <template #content>
        <div ref="commands" class="passage-table-commands" :class="{ 'passage-table-commands--popup': compact }" @click="onCommand">
          <slot name="commands" />
          <PassageTableTools :kind="kind" :table-i-d="tableId" :rows="rows" @available="passageAvailable = $event"
            @refresh="$emit('boundary-refresh')" @clock="$emit('clock', $event)" />
        </div>
      </template>
    </BaseDropdown>
  </div>
</template>

<style scoped>
.card-header.passage-table-header {
  box-sizing: border-box;
  width: 100%;
  min-width: 0;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 12px;
  min-height: 53px;
  height: auto;
  padding: 8px 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.passage-table-header__title { flex: 1 1 0; min-width: 0; }
.passage-table-header__full-title :deep(.card-title) { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.passage-table-header__short-title { display: none; margin: 0; color: var(--accent-text); font-size: 16px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.passage-table-header__counter, .passage-table-header__refresh, .passage-table-header__menu { flex: 0 0 auto; }
.passage-table-header__counter:empty { display: none; }
.passage-table-header__counter :deep(.items-count) { white-space: nowrap; margin: 0; }
.passage-table-header .passage-table-header__menu--empty { display: none; }
.passage-table-commands { display: flex; flex-wrap: nowrap; align-items: center; gap: 12px; }
.passage-table-commands--popup { flex-direction: column; align-items: stretch; gap: 12px; padding: 12px; }
.passage-table-commands--popup :deep(.history-btn) { display: inline-flex; justify-content: center; padding: 8px 12px; }
.passage-table-commands--popup :deep(.switch-toggle) { justify-content: space-between; }
.passage-table-commands--popup :deep(.lk-button) { white-space: normal; }
.passage-table-header__more { display: flex; justify-content: center; align-items: center; min-width: 36px; width: 36px; height: 36px; padding: 0; font-size: 24px; }
.card-header.passage-table-header--compact { gap: 8px; padding: 8px 12px; }
.passage-table-header--compact .passage-table-header__full-title { display: none; }
.passage-table-header--compact .passage-table-header__short-title { display: block; }
.passage-table-header--compact .passage-table-header__counter :deep(.items-count) { font-size: 12px; }
.passage-table-header--compact :deep(.items-count__label) { display: none; }
.passage-table-header--compact :deep(.items-count__label-short) { display: inline; }
.passage-table-header :deep(.items-count__label-mini) { display: none; }
.passage-table-header--tight :deep(.items-count__label-short) { display: none; }
.passage-table-header--tight :deep(.items-count__label-mini) { display: inline; }
.passage-table-header--tight .passage-table-header__short-title { font-size: 14px; }
.passage-table-header--compact .passage-table-header__refresh { width: 36px; min-width: 36px; height: 36px; margin: 0; padding: 0; }
.passage-table-header--compact .passage-table-header__refresh :deep(.refresh-btn__text) { display: none; }
</style>
