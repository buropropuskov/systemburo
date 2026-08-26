<template>
  <span
    class="badge"
    :class="['badge--' + variant, 'badge--' + size]"
  >
    <span
      v-if="dot"
      class="badge__dot"
    />
    <slot>{{ label }}</slot>
  </span>
</template>

<script>
const VARIANTS = ['neutral', 'success', 'warning', 'danger', 'info', 'primary'];
const SIZES = ['sm', 'md', 'lg'];

export default {
  name: 'UiBadge',
  props: {
    label: { type: String, default: '' },
    variant: {
      type: String,
      default: 'neutral',
      validator: v => VARIANTS.includes(v)
    },
    size: {
      type: String,
      default: 'md',
      validator: v => SIZES.includes(v)
    },
    dot: { type: Boolean, default: false }
  }
};
</script>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 999px;
  font-weight: 500;
  white-space: nowrap;
  line-height: 1;
  border: 1px solid transparent;
}

.badge--sm { font-size: 11px; padding: 3px 8px; }
.badge--md { font-size: 12px; padding: 4px 10px; }
.badge--lg { font-size: 13px; padding: 6px 12px; }

.badge__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.badge--neutral { background: var(--surface-2); color: var(--text-muted); border-color: var(--border); }
.badge--neutral .badge__dot { background: var(--text-muted); }

.badge--primary { background: var(--accent-tint); color: var(--accent-text); border-color: color-mix(in srgb, var(--accent) 25%, var(--surface)); }
.badge--primary .badge__dot { background: var(--accent); }

.badge--success { background: var(--success-bg); color: var(--success-text); border-color: color-mix(in srgb, var(--success) 30%, var(--surface)); }
.badge--success .badge__dot { background: var(--success); }

.badge--warning { background: var(--warning-bg); color: var(--warning-text); border-color: color-mix(in srgb, var(--warning) 30%, var(--surface)); }
.badge--warning .badge__dot { background: var(--warning); }

.badge--danger { background: var(--danger-bg); color: var(--danger-text); border-color: color-mix(in srgb, var(--danger) 30%, var(--surface)); }
.badge--danger .badge__dot { background: var(--danger); }

.badge--info { background: var(--info-bg); color: var(--info-text); border-color: color-mix(in srgb, var(--info) 30%, var(--surface)); }
.badge--info .badge__dot { background: var(--info); }
</style>
