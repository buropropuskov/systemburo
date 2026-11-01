<script setup>
defineProps({ groups: { type: Array, required: true }, disabled: Boolean, expanded: Boolean, current: { type: String, default: '' } });
const emit = defineEmits(['navigate']);
const value = (table, key) => table.table?.[key] || table[key];
</script>
<template>
  <div
    class="dropdown-below"
    :class="{ open: expanded }"
    :inert="!expanded || undefined"
  >
    <div
      class="dropdown-below__inner"
      data-testid="nav-tables-list"
    >
      <template
        v-for="group in groups"
        :key="group.key"
      >
        <div
          v-if="groups.length > 1"
          class="dropdown-group-title"
        >
          {{ group.label }}
        </div>
        <button
          v-for="table in group.tables"
          :key="value(table, 'id')"
          type="button"
          :disabled="disabled"
          class="dropdown-item"
          :class="{ active: value(table, 'name') && current === value(table, 'name') }"
          @click="emit('navigate', value(table, 'name'))"
        >
          {{ value(table, 'display_name') || 'Без названия' }}
        </button>
      </template>
      <div
        v-if="!groups.length"
        class="dropdown-item disabled"
      >
        Нет доступных таблиц
      </div>
    </div>
  </div>
</template>
<style scoped>
.dropdown-below { display: grid; grid-template-rows: 0fr; transition: grid-template-rows 0.25s cubic-bezier(0.4, 0, 0.2, 1); }
.dropdown-below.open { grid-template-rows: 1fr; }
.dropdown-below__inner { overflow: hidden; min-height: 0; margin: 0 7px 4px; padding-left: 22px; }
.dropdown-group-title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 9.5px;
  letter-spacing: 0.04em;
  color: var(--nav-text-muted);
  text-transform: uppercase;
  padding: 6px 12px 4px;
  white-space: nowrap;
  overflow: hidden;
}
.dropdown-group-title + .dropdown-item { margin-top: 2px; }
.dropdown-item + .dropdown-group-title { margin-top: 14px; }
.dropdown-item {
  padding: 7px 12px;
  border-radius: 10px;
  font-family: 'Montserrat', sans-serif;
  font-size: 12.5px;
  color: var(--nav-text-muted);
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: background-color 0.2s ease, color 0.2s ease;
}
.dropdown-item.active { color: var(--nav-primary); background-color: var(--nav-primary-soft); font-weight: 600; }
.dropdown-item:hover { background-color: var(--nav-hover); color: var(--nav-primary); }
.dropdown-item.disabled { color: var(--nav-text-faint); cursor: not-allowed; }
.dropdown-item.disabled:hover { background-color: transparent; color: var(--nav-text-faint); }
button:disabled { pointer-events: none; opacity: 0.4; cursor: not-allowed; }
@media (max-width: 1366px), (hover: none) and (pointer: coarse) {
  .dropdown-item { display: flex; align-items: center; min-height: 44px; }
}
</style>
