<template>
  <span v-if="label" class="passage-indicator" :class="{ 'passage-indicator--attention': passageState?.needs_attention }">
    <HintTooltip :key="label" :text="label" :width="280" clickable class="passage-indicator__button">
      <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true"><circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" stroke-width="1.5" /><path d="M8 4.5v4M8 11v.5" stroke="currentColor" stroke-width="1.5" /></svg>
    </HintTooltip>
  </span>
</template>
<script>
import HintTooltip from './ui/HintTooltip.vue';

export default {
  name: 'PassageStateIndicator',
  components: { HintTooltip },
  props: { passageState: { type: Object, default: null }, expired: { type: Boolean, default: false } },
  computed: {
    label() {
      const state = this.passageState;
      if (!state) return '';
      if (state.open && !state.entry_time_known) return 'Выход не отмечен; время входа неизвестно. Открытая отметка не подтверждает присутствие на территории.';
      if (state.open && this.expired) return 'Срок истёк, выход не отмечен. Можно отметить выход; новый вход запрещён.';
      if (state.open && state.needs_attention) return 'Выход не отмечен более 48 часов. Открытая отметка не подтверждает присутствие на территории.';
      if (state.in_exit_grace && this.expired) return 'Срок истёк. Выход зарегистрирован; запись остаётся в таблице на 5 минут. Новый вход запрещён.';
      return '';
    },
  },
};
</script>
<style scoped>
.passage-indicator { position: relative; display: inline-flex; align-items: center; vertical-align: middle; max-height: 1em; color: var(--warning); }
.passage-indicator--attention { color: var(--danger-text); }
.passage-indicator :deep(.passage-indicator__button) { padding: 0; margin: 0; border: 0; background: transparent; color: inherit; cursor: pointer; line-height: 1; }
</style>
