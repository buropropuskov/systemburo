<template>
  <div
    class="loader-spinner"
    :class="[`loader-spinner--${size}`, { 'loader-spinner--inline': inline }]"
    role="status"
    :aria-label="label"
  >
    <span
      class="loader-spinner__circle"
      aria-hidden="true"
    />
    <span
      v-if="label"
      class="loader-spinner__label"
    >{{ label }}</span>
  </div>
</template>

<script>
export default {
  name: 'LoaderSpinner',
  props: {
    size: {
      type: String,
      default: 'medium',
      validator: v => ['small', 'medium', 'large'].includes(v),
    },
    label: {
      type: String,
      default: 'Загрузка…',
    },
    inline: {
      type: Boolean,
      default: false,
    },
  },
}
</script>

<style scoped>
.loader-spinner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--text-muted);
  font-size: 13px;
}

.loader-spinner--inline {
  display: inline-flex;
}

.loader-spinner__circle {
  border-style: solid;
  border-color: var(--border);
  border-top-color: var(--accent-text);
  border-radius: 50%;
  animation: loader-spinner-spin 1s linear infinite;
  flex-shrink: 0;
}

.loader-spinner--small .loader-spinner__circle {
  width: 14px;
  height: 14px;
  border-width: 2px;
}

.loader-spinner--medium .loader-spinner__circle {
  width: 20px;
  height: 20px;
  border-width: 2px;
}

.loader-spinner--large .loader-spinner__circle {
  width: 32px;
  height: 32px;
  border-width: 3px;
}

.loader-spinner__label {
  font-weight: 500;
  white-space: nowrap;
}

.loader-spinner--small .loader-spinner__label {
  font-size: 12px;
}

.loader-spinner--large .loader-spinner__label {
  font-size: 14px;
}

@keyframes loader-spinner-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .loader-spinner__circle {
    animation-duration: 2.4s;
  }
}
</style>
