<template>
  <div
    class="form-field"
    :class="{ 'form-field--error': error }"
  >
    <label
      v-if="label"
      class="form-field__label"
    >
      {{ label }}
      <span
        v-if="required"
        class="form-field__required"
      >*</span>
    </label>
    <div class="form-field__control">
      <slot />
    </div>
    <transition name="error-fade">
      <div
        v-if="error"
        class="form-field__error"
        role="alert"
      >
        {{ error }}
      </div>
    </transition>
  </div>
</template>

<script>
export default {
  name: 'FormField',
  props: {
    label: { type: String, default: '' },
    required: { type: Boolean, default: false },
    error: { type: String, default: '' },
  },
}
</script>

<style scoped>
.form-field {
  position: relative;
  margin-bottom: 16px;
}

.form-field__label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
  margin-bottom: 4px;
}

.form-field__required {
  color: var(--danger-text);
  margin-left: 2px;
}

.form-field--error :deep(input),
.form-field--error :deep(select),
.form-field--error :deep(textarea) {
  border-color: var(--color-danger) !important;
}

.form-field__error {
  font-size: 11px;
  color: var(--danger-text);
  margin-top: 2px;
}

.error-fade-enter-active,
.error-fade-leave-active {
  transition: opacity 0.2s ease;
}
.error-fade-enter-from,
.error-fade-leave-to {
  opacity: 0;
}
</style>
