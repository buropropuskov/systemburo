<template>
  <div class="password-input">
    <input
      :value="modelValue"
      :type="visible ? 'text' : 'password'"
      :placeholder="placeholder"
      :required="required"
      :disabled="disabled"
      :autocomplete="autocomplete"
      autocorrect="off"
      autocapitalize="off"
      spellcheck="false"
      class="password-input__field"
      :class="inputClass"
      @input="onInput"
    >
    <button
      type="button"
      class="password-input__toggle"
      :aria-label="visible ? 'Скрыть пароль' : 'Показать пароль'"
      :title="visible ? 'Скрыть пароль' : 'Показать пароль'"
      :disabled="disabled"
      tabindex="-1"
      @click="visible = !visible"
    >
      <svg
        v-if="!visible"
        viewBox="0 0 24 24"
        width="18"
        height="18"
        aria-hidden="true"
      >
        <path
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"
        />
        <circle
          cx="12"
          cy="12"
          r="3"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
        />
      </svg>
      <svg
        v-else
        viewBox="0 0 24 24"
        width="18"
        height="18"
        aria-hidden="true"
      >
        <path
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M3 3l18 18M10.6 6.1A11 11 0 0 1 12 6c6.5 0 10 6 10 6a17.7 17.7 0 0 1-3.5 4.2M6.5 6.5C3.6 8.4 2 12 2 12s3.5 7 10 7c1.7 0 3.2-.4 4.5-1.1M9.9 9.9a3 3 0 0 0 4.2 4.2"
        />
      </svg>
    </button>
  </div>
</template>

<script setup>
import { ref } from 'vue';

defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  required: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  autocomplete: { type: String, default: 'new-password' },
  inputClass: { type: [String, Array, Object], default: '' }
});

const emit = defineEmits(['update:modelValue']);
const visible = ref(false);

function onInput(event) {
  emit('update:modelValue', event.target.value);
}
</script>

<style scoped>
.password-input {
  position: relative;
  width: 100%;
}

.password-input__field {
  width: 100%;
  padding-right: 36px;
}

.password-input__toggle {
  position: absolute;
  top: 50%;
  right: 8px;
  transform: translateY(-50%);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  background: transparent;
  color: #6e7280;
  cursor: pointer;
  border-radius: 6px;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.password-input__toggle:hover:not(:disabled) {
  background: #eef0ff;
  color: var(--color-primary, #4F5BDF);
}

.password-input__toggle:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.25);
}

.password-input__toggle:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
