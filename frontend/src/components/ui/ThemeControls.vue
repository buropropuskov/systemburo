<template>
  <div class="theme-controls">
    <div
      v-for="item in items"
      :key="item.testid"
      class="nav-item nav-item--theme"
      :data-testid="item.icon === 'moon' ? 'nav-theme-toggle' : 'nav-style-toggle'"
      role="switch"
      tabindex="0"
      :aria-checked="item.checked ? 'true' : 'false'"
      :aria-label="item.title"
      :title="item.title"
      @click="item.toggle"
      @keydown.enter.prevent="item.toggle"
      @keydown.space.prevent="item.toggle"
    >
      <div class="nav-item-content">
        <NavIcon
          :name="item.icon"
          :size="18"
          class="nav-icon"
        />
        <span class="nav-text">{{ item.label }}</span>
      </div>
      <SwitchToggle
        :model-value="item.checked"
        label=""
        :title="item.title"
        class="nav-theme-switch"
        aria-hidden="true"
        inert
      />
    </div>
  </div>
</template>
<script setup>
import { computed } from 'vue';
import NavIcon from '@/components/icons/NavIcon.vue';
import SwitchToggle from '@/components/ui/SwitchToggle.vue';
import { useThemeStore } from '@/stores/theme';
const theme = useThemeStore();
const dark = computed(() => theme.current === 'dark' || theme.current === 'glass-dark');
const alternative = computed(() => theme.current.startsWith('glass-'));
const select = (alt, isDark) => theme.setTheme((alt ? 'glass-' : '') + (isDark ? 'dark' : 'light'));
const items = computed(() => [
  { testid: 'nav-style-toggle', icon: 'theme', label: 'Альтернативное',
    title: 'Оформление: ' + (alternative.value ? 'Альтернативное' : 'Обычное'),
    checked: alternative.value, toggle: () => select(!alternative.value, dark.value) },
  { testid: 'nav-theme-toggle', icon: 'moon', label: 'Тёмная тема',
    title: 'Переключить светлую и тёмную тему', checked: dark.value,
    toggle: () => select(alternative.value, !dark.value) },
]);
</script>
<style scoped>
.nav-item { display:flex; align-items:center; justify-content:space-between; gap:12px;
  min-height:40px; padding:0 12px 0 9px; margin:4px 7px; border-radius:12px;
  cursor:pointer; overflow:hidden; color:var(--nav-text); flex-shrink:0; }
.nav-item:hover { background-color:var(--nav-hover); }
.nav-item:focus-visible { outline:2px solid var(--accent-text); outline-offset:-2px; }
.nav-item-content { display:flex; align-items:center; gap:12px; min-width:0; }
.nav-icon { flex-shrink:0; }
.nav-text { white-space:nowrap; font-size:12px; opacity:0; }
.nav-theme-switch { flex-shrink:0; pointer-events:none; opacity:0; }
:global(.nav-menu.expanded) .nav-text, :global(.nav-menu.expanded) .nav-theme-switch { opacity:1; }
@media (max-width:1366px), (hover:none) and (pointer:coarse) {
  .nav-text, .nav-theme-switch { opacity:1; }
  .nav-item { min-height:44px; }
}
</style>
