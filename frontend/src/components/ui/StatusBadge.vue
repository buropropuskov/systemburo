<template>
  <span
    class="status-badge"
    :class="[variantClass, colorClass]"
  >
    <span
      v-if="variant === 'dot'"
      class="status-badge__dot"
    />
    {{ status }}
  </span>
</template>

<script>
export default {
  name: 'StatusBadge',
  props: {
    status: { type: String, required: true },
    variant: { type: String, default: 'badge' }
  },
  computed: {
    variantClass() {
      return `status-badge--${this.variant}`
    },
    colorClass() {
      const s = this.status.toLowerCase()
      const map = {
        green: ['активен', 'активна', 'согласовано', 'выполнена', 'завершено', 'active', 'completed', 'открыто', 'открыто сейчас'],
        gray: ['неактивен', 'неактивна', 'inactive', 'неактивно'],
        yellow: ['непрочитано', 'в обработке', 'на рассмотрении', 'согласование', 'pending', 'закрыто', 'закрыто сейчас', 'на обслуживании'],
        red: ['не согласовано', 'отклонено', 'отказано', 'rejected'],
        blue: ['в работе', 'in_work']
      }
      for (const [color, keywords] of Object.entries(map)) {
        if (keywords.includes(s)) return `status-badge--${color}`
      }
      return 'status-badge--gray'
    }
  }
}
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  line-height: 1.4;
}

.status-badge__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* Green */
.status-badge--green { background: #dcfce7; color: #166534; border: 1px solid #bbf7d0; }
.status-badge--green .status-badge__dot { background: #22c55e; }

/* Gray */
.status-badge--gray { background: #e5e7eb; color: #374151; border: 1px solid #d1d5db; }
.status-badge--gray .status-badge__dot { background: #9ca3af; }

/* Yellow */
.status-badge--yellow { background: #fef3c7; color: #92400e; border: 1px solid #fde68a; }
.status-badge--yellow .status-badge__dot { background: #f59e0b; }

/* Red */
.status-badge--red { background: #fee2e2; color: #991b1b; border: 1px solid #fecaca; }
.status-badge--red .status-badge__dot { background: #ef4444; }

/* Blue */
.status-badge--blue { background: #dbeafe; color: #1e40af; border: 1px solid #bfdbfe; }
.status-badge--blue .status-badge__dot { background: #3b82f6; }
</style>
