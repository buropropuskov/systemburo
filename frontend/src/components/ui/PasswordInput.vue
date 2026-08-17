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
        <!-- Скрыть пароль: тот же глаз, что в открытом состоянии, перечёркнутый
             по диагонали - пара читается как одно действие с двумя положениями. -->
        <g
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
          <circle
            cx="12"
            cy="12"
            r="3"
          />
          <line
            x1="4.5"
            y1="19.5"
            x2="19.5"
            y2="4.5"
          />
        </g>
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
  autocomplete: { type: String, default: 'new-password' }
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

/* Стиль input повторяет .modal-input / .form-input - чтобы поле выглядело
   так же, как остальные поля формы. Внутренние стили родителя сюда не
   достают (scoped CSS), поэтому стиль повторяется здесь явно. */
.password-input__field {
  width: 100%;
  padding: 10px 38px 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-size: 14px;
  font-family: inherit;
  background: var(--surface);
  color: inherit;
  transition: border-color 0.2s ease;
  box-sizing: border-box;
}

.password-input__field:focus {
  border-color: var(--color-primary, var(--accent));
  outline: none;
}

.password-input__field:disabled {
  background: var(--accent-tint);
  color: var(--text-muted);
  cursor: not-allowed;
}

.password-input__field::placeholder {
  color: var(--text-muted);
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
  color: var(--text-muted);
  cursor: pointer;
  border-radius: 6px;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.password-input__toggle:hover:not(:disabled) {
  background: var(--accent-tint);
  color: var(--color-primary, var(--accent-text));
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
