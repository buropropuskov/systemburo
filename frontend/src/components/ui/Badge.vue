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

.badge--neutral { background: #f1f3f5; color: #475569; border-color: #e2e8f0; }
.badge--neutral .badge__dot { background: #94a3b8; }

.badge--primary { background: #eef0ff; color: #3a45c0; border-color: #d9deff; }
.badge--primary .badge__dot { background: var(--color-primary, #4F5BDF); }

.badge--success { background: #dcfce7; color: #166534; border-color: #bbf7d0; }
.badge--success .badge__dot { background: #22c55e; }

.badge--warning { background: #fef3c7; color: #92400e; border-color: #fde68a; }
.badge--warning .badge__dot { background: #f59e0b; }

.badge--danger { background: #fee2e2; color: #991b1b; border-color: #fecaca; }
.badge--danger .badge__dot { background: #ef4444; }

.badge--info { background: #dbeafe; color: #1e40af; border-color: #bfdbfe; }
.badge--info .badge__dot { background: #3b82f6; }
</style>
