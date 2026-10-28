<template>
  <BaseDropdown
    ref="dropdown"
    :key="`${disabled}-${inline}`"
    class="theme-controls"
    :class="{ 'theme-controls--inline': inline }"
    :teleport="!inline"
    :disabled="disabled"
    :menu-min-width="260"
    :menu-z-index="10010"
    menu-placement="right"
    :options="[]"
  >
    <template #trigger="{ toggle, isOpen }">
      <button
        ref="trigger"
        type="button"
        class="nav-item theme-trigger"
        data-testid="nav-theme-toggle"
        title="Оформление"
        aria-haspopup="true"
        :aria-expanded="isOpen"
        :disabled="disabled"
        @click="open(toggle, $event)"
      >
        <NavIcon
          name="theme"
          :size="18"
          class="nav-icon"
        />
        <span class="nav-text">Оформление</span>
        <AppIcon
          name="arrow"
          :size="14"
          class="theme-arrow"
          :class="{ 'theme-arrow--open': isOpen }"
        />
      </button>
    </template>
    <template #content>
      <fieldset
        ref="panel"
        class="theme-options"
        :disabled="disabled"
        aria-label="Настройка оформления"
        @mouseleave="emit('panel-leave')"
      >
        <SwitchToggle
          class="theme-option"
          data-testid="theme-alternative-toggle"
          label="Альтернативное"
          title="Альтернативное оформление"
          :model-value="alternative"
          @update:model-value="select($event, dark)"
        />
        <SwitchToggle
          class="theme-option"
          data-testid="theme-dark-toggle"
          label="Тёмная тема"
          title="Тёмная тема"
          :model-value="dark"
          @update:model-value="select(alternative, $event)"
        />
      </fieldset>
    </template>
  </BaseDropdown>
</template>
<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import NavIcon from '@/components/icons/NavIcon.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import SwitchToggle from '@/components/ui/SwitchToggle.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { useThemeStore } from '@/stores/theme';

const props = defineProps({ disabled: { type: Boolean, default: false }, inline: { type: Boolean, default: false } });
const emit = defineEmits(['open-change', 'panel-leave']);
const theme = useThemeStore();
const dropdown = ref(null);
const trigger = ref(null);
const panel = ref(null);
let keyboardOpen = false;
const open = (toggle, event) => {
  if (props.disabled) return;
  keyboardOpen = event.detail === 0;
  toggle();
};
watch(() => dropdown.value?.isOpen, async (isOpen, wasOpen) => {
  emit('open-change', Boolean(isOpen));
  if (isOpen && keyboardOpen) {
    await nextTick();
    panel.value?.querySelector('input')?.focus();
  } else if (!isOpen && wasOpen && !props.disabled && panel.value?.contains(document.activeElement)) {
    trigger.value?.focus();
  }
});
const close = () => { if (dropdown.value) dropdown.value.isOpen = false; };
defineExpose({ close, isHovered: () => Boolean(panel.value?.matches(':hover')) });
const alternative = computed(() => theme.current.startsWith('glass-'));
const dark = computed(() => theme.current.endsWith('dark'));
const select = (alt, isDark) => {
  if (props.disabled) return;
  return theme.setTheme((alt ? 'glass-' : '') + (isDark ? 'dark' : 'light'));
};
</script>
<style scoped>
.theme-trigger {
  width: calc(100% - 14px);
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 12px 0 9px;
  margin: 4px 7px;
  min-height: 40px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  text-align: left;
  color: var(--nav-text);
  font-family: inherit;
  cursor: pointer;
  overflow: hidden;
  transition: background-color 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.theme-trigger:hover:not(:disabled) { background-color: var(--nav-hover); }
.theme-trigger:hover:not(:disabled) .nav-icon { color: var(--nav-primary); }
.theme-trigger:focus-visible { outline: 2px solid var(--accent-text); outline-offset: -2px; }
.theme-trigger:disabled { opacity: 0.4; cursor: not-allowed; }
.nav-icon { flex-shrink: 0; color: var(--nav-text-muted); transition: color 0.2s ease; }
.theme-arrow {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--nav-text-muted);
  opacity: 0;
  transform: rotate(0deg);
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.theme-arrow--open { transform: rotate(180deg); }
.theme-controls--inline .theme-arrow { transform: rotate(90deg); }
.theme-controls--inline .theme-arrow--open { transform: rotate(270deg); }
.nav-menu.expanded .theme-arrow { opacity: 0.7; transition-delay: 0.15s; }
.nav-text {
  font-size: 13px;
  font-weight: 500;
  line-height: 16px;
  color: var(--nav-text);
  white-space: nowrap;
  opacity: 0;
  transform: translateX(-5px);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.nav-menu.expanded .nav-text { opacity: 1; transform: translateX(0); transition-delay: 0.1s; }
.theme-options { margin: 0; border: 0; min-width: 0; padding: 4px 14px; display: flex; flex-direction: column; }
.theme-option { width: 100%; justify-content: space-between; flex-direction: row-reverse; min-height: 40px; font-size: 13px; }
.theme-option + .theme-option { border-top: 1px solid var(--border); }
@media (max-width: 1366px), (hover: none) and (pointer: coarse) {
  .nav-text { opacity: 1; transform: none; }
  .theme-arrow { opacity: 1; }
  .theme-trigger { min-height: 44px; }
}
.theme-controls--inline :deep(.base-dropdown__menu) {
  position: static;
  width: auto;
  min-width: 0;
  margin: 0 7px 4px;
  box-shadow: none;
  border: 0;
  border-radius: 0;
  background: transparent;
  overflow: visible;
}
.theme-controls--inline .theme-options {
  margin: 2px 8px 8px 23px;
  padding: 2px 0;
  border-left: 2px solid var(--nav-active-text, var(--accent));
  border-radius: 0 12px 12px 0;
  background: var(--nav-hover);
}
.theme-controls--inline .theme-option {
  box-sizing: border-box;
  min-height: 44px;
  padding: 0 12px 0 14px;
  color: var(--nav-text);
  font-size: 13px;
  font-weight: 500;
  border: 0;
  border-radius: 12px;
}
.theme-controls--inline .theme-option:hover { background: var(--nav-hover); }
@media (prefers-reduced-motion: reduce) {
  .theme-trigger, .nav-icon, .nav-text, .theme-arrow { transition: none; }
}
</style>
