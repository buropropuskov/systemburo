<template>
  <label
    class="switch-toggle"
    :class="{ 'switch-toggle--active': modelValue }"
    :title="title"
  >
    <input
      type="checkbox"
      class="switch-toggle__input"
      :checked="modelValue"
      @change="onChange"
    >
    <span class="switch-toggle__switch">
      <span class="switch-toggle__thumb" />
    </span>
    <span class="switch-toggle__label">{{ label }}</span>
  </label>
</template>

<script>
/**
 * Универсальный тумблер режима отображения таблиц.
 * Используется для "Увеличенного режима" (CarsTable, PeopleTable) и "Сетки"
 * (TablesComponent) как v-model. Сохранение состояния делает родитель.
 */
export default {
  name: 'SwitchToggle',
  props: {
    modelValue: { type: Boolean, default: false },
    label: { type: String, default: 'Увеличенный режим' },
    title: { type: String, default: 'Увеличить шрифт и скрыть второстепенные столбцы' }
  },
  emits: ['update:modelValue'],
  methods: {
    onChange(e) {
      this.$emit('update:modelValue', e.target.checked);
    }
  }
};
</script>

<style scoped>
.switch-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
  font-size: 12px;
  color: var(--text);
}

.switch-toggle__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  border: 0;
}

.switch-toggle__switch {
  position: relative;
  width: 32px;
  height: 18px;
  background: var(--border);
  border-radius: 50px;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
}

.switch-toggle__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  background: var(--surface);
  border-radius: 50%;
  transition: transform 0.2s ease;
}

.switch-toggle--active .switch-toggle__switch {
  background: var(--accent);
}

.switch-toggle--active .switch-toggle__thumb {
  transform: translateX(14px);
}

.switch-toggle__label {
  white-space: nowrap;
}

.switch-toggle:hover .switch-toggle__switch {
  filter: brightness(0.95);
}

.switch-toggle__input:focus-visible + .switch-toggle__switch {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
</style>
