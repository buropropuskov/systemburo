<template>
  <label
    class="toggle-switch"
    :class="{ checked: modelValue, disabled }"
  >
    <input
      type="checkbox"
      :checked="modelValue"
      :disabled="disabled"
      @change="$emit('update:modelValue', $event.target.checked)"
    >
    <span class="toggle-switch__track">
      <span class="toggle-switch__thumb" />
    </span>
    <span
      v-if="$slots.default"
      class="toggle-switch__label"
    >
      <slot />
    </span>
  </label>
</template>

<script>
export default {
  name: 'ToggleSwitch',
  props: {
    modelValue: { type: Boolean, default: false },
    disabled: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
};
</script>

<style scoped>
.toggle-switch {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.toggle-switch.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.toggle-switch input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-switch__track {
  position: relative;
  width: 36px;
  height: 20px;
  border-radius: 10px;
  background: #ccc;
  transition: background 0.2s ease;
  flex-shrink: 0;
}

.toggle-switch.checked .toggle-switch__track {
  background: var(--color-primary);
}

.toggle-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  transition: transform 0.2s ease;
}

.toggle-switch.checked .toggle-switch__thumb {
  transform: translateX(16px);
}

.toggle-switch__label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}
</style>
