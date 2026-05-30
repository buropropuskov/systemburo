<template>
  <label
    class="enlarged-toggle"
    :class="{ 'enlarged-toggle--active': modelValue }"
    :title="title"
  >
    <input
      type="checkbox"
      class="enlarged-toggle__input"
      :checked="modelValue"
      @change="onChange"
    >
    <span class="enlarged-toggle__switch">
      <span class="enlarged-toggle__thumb" />
    </span>
    <span class="enlarged-toggle__label">{{ label }}</span>
  </label>
</template>

<script>
/**
 * Переключатель "Увеличенный режим" для таблиц.
 * Используется в CarsTable и PeopleTable как v-model.
 * Сохранение в localStorage делает родитель по своему ключу.
 */
export default {
  name: 'EnlargedToggle',
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
.enlarged-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
  font-size: 12px;
  color: #333;
}

.enlarged-toggle__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  border: 0;
}

.enlarged-toggle__switch {
  position: relative;
  width: 32px;
  height: 18px;
  background: #e6e6e6;
  border-radius: 50px;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
}

.enlarged-toggle__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  background: #fff;
  border-radius: 50%;
  transition: transform 0.2s ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}

.enlarged-toggle--active .enlarged-toggle__switch {
  background: #4F5BDF;
}

.enlarged-toggle--active .enlarged-toggle__thumb {
  transform: translateX(14px);
}

.enlarged-toggle__label {
  white-space: nowrap;
}

.enlarged-toggle:hover .enlarged-toggle__switch {
  filter: brightness(0.95);
}

.enlarged-toggle__input:focus-visible + .enlarged-toggle__switch {
  outline: 2px solid #4F5BDF;
  outline-offset: 2px;
}
</style>
