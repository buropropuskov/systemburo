<template>
  <span v-if="label" class="passage-indicator" :class="{ 'passage-indicator--attention': passageState?.needs_attention }">
    <button type="button" class="passage-indicator__button" :aria-label="label" :aria-expanded="expanded" @click.stop="expanded = !expanded" @keydown.esc.stop="expanded = false">
      <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true"><circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" stroke-width="1.5" /><path d="M8 4.5v4M8 11v.5" stroke="currentColor" stroke-width="1.5" /></svg>
    </button>
    <span v-if="expanded" class="passage-indicator__description" role="status">{{ label }}</span>
  </span>
</template>
<script>
export default {
  name: 'PassageStateIndicator',
  props: { passageState: { type: Object, default: null }, expired: { type: Boolean, default: false } },
  data() { return { expanded: false }; },
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
  watch: { label() { this.expanded = false; } },
};
</script>
<style scoped>
.passage-indicator { position: relative; display: inline-flex; align-items: center; vertical-align: middle; max-height: 1em; color: var(--warning); }
.passage-indicator--attention { color: var(--danger-text); }
.passage-indicator__button { display: inline-flex; padding: 0; margin: 0; border: 0; background: transparent; color: inherit; cursor: pointer; line-height: 1; }
.passage-indicator__button:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; border-radius: 3px; }
.passage-indicator__description { position: absolute; top: 100%; left: 0; z-index: 30; width: max-content; max-width: min(280px, 80vw); padding: 8px 10px; border: 1px solid var(--border); border-radius: var(--radius-md); background: var(--surface); color: var(--text); white-space: normal; font-size: 12px; line-height: 1.5; box-shadow: var(--shadow-md); }
</style>
