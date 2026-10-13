<template>
  <BaseDropdown
    ref="dropdown"
    :key="disabled ? 'disabled' : 'enabled'"
    class="theme-controls"
    teleport
    :disabled="disabled"
    :menu-min-width="260"
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
import SwitchToggle from '@/components/ui/SwitchToggle.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { useThemeStore } from '@/stores/theme';

const props = defineProps({ disabled: { type: Boolean, default: false } });
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
}
.theme-trigger:hover { background-color: var(--nav-hover); }
.theme-trigger:focus-visible { outline: 2px solid var(--accent-text); outline-offset: -2px; }
.theme-trigger:disabled { opacity: 0.4; cursor: not-allowed; }
.nav-icon { flex-shrink: 0; }
.nav-text { font-size: 13px; font-weight: 500; line-height: 16px; white-space: nowrap; opacity: 0; }
.nav-menu.expanded .nav-text { opacity: 1; }
.theme-options { margin: 0; border: 0; min-width: 0; padding: 4px 14px; display: flex; flex-direction: column; }
.theme-option { width: 100%; justify-content: space-between; flex-direction: row-reverse; min-height: 40px; font-size: 13px; }
.theme-option + .theme-option { border-top: 1px solid var(--border); }
@media (max-width: 1366px), (hover: none) and (pointer: coarse) {
  .nav-text { opacity: 1; }
  .theme-trigger { min-height: 44px; }
}
</style>
