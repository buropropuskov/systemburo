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
        red: ['не согласовано', 'отклонено', 'отказано', 'rejected', 'чёрный список', 'черный список'],
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
  justify-content: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  line-height: 1.4;
  min-width: 120px;
}

.status-badge__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* Green */
.status-badge--green { background: #e8f5e8; color: #2e7d32; border: 1px solid #c8e6c9; }
.status-badge--green .status-badge__dot { background: #2e7d32; }

/* Gray */
.status-badge--gray { background: #f5f5f5; color: #616161; border: 1px solid #e0e0e0; }
.status-badge--gray .status-badge__dot { background: #9ca3af; }

/* Yellow. Текст #92400e: прежний #ef6c00 на своей подложке давал 2.81 - ниже нормы AA (#1415). */
.status-badge--yellow { background: #fff3e0; color: #92400e; border: 1px solid #ffe0b2; }
.status-badge--yellow .status-badge__dot { background: #f59e0b; }

/* Red */
.status-badge--red { background: #ffebee; color: #c62828; border: 1px solid #ffcdd2; }
.status-badge--red .status-badge__dot { background: #ef4444; }

/* Blue */
.status-badge--blue { background: #e3f2fd; color: #1565c0; border: 1px solid #bbdefb; }
.status-badge--blue .status-badge__dot { background: #3b82f6; }
</style>
