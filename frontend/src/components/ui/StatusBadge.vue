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
        green: ['активен', 'активна', 'согласовано', 'выполнена', 'завершено', 'active', 'completed', 'открыто', 'открыто сейчас', 'в архиве'],
        gray: ['неактивен', 'неактивна', 'inactive', 'неактивно', 'вложение удалено'],
        yellow: ['непрочитано', 'в обработке', 'на рассмотрении', 'согласование', 'pending', 'закрыто', 'закрыто сейчас', 'на обслуживании', 'нет шаблона', 'в очереди'],
        red: ['не согласовано', 'отклонено', 'отказано', 'rejected', 'чёрный список', 'черный список', 'ошибка выгрузки', 'заблокировано', 'ошибка', 'нет места'],
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

/* Минимум 120px выравнивает пилюли в столбце таблицы - в карточке столбца нет, а
   120px из ~320px ширины забирают у соседнего поля половину строки и выдавливают его
   текст под бейдж. На узком экране пилюля живёт по содержимому. */
@media (max-width: 767.98px) {
  .status-badge {
    min-width: 0;
  }
}

.status-badge__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* Пары фон/текст берутся из темы: они замерены на контраст в каждой из шести
   палитр, рамка выводится подмесом того же цвета к поверхности. */
.status-badge--green { background: var(--success-bg); color: var(--success-text); border: 1px solid color-mix(in srgb, var(--success) 30%, var(--surface)); }
.status-badge--green .status-badge__dot { background: var(--success); }

.status-badge--gray { background: var(--surface-2); color: var(--text-muted); border: 1px solid var(--border); }
.status-badge--gray .status-badge__dot { background: var(--text-muted); }

.status-badge--yellow { background: var(--warning-bg); color: var(--warning-text); border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface)); }
.status-badge--yellow .status-badge__dot { background: var(--warning); }

.status-badge--red { background: var(--danger-bg); color: var(--danger-text); border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface)); }
.status-badge--red .status-badge__dot { background: var(--danger); }

.status-badge--blue { background: var(--info-bg); color: var(--info-text); border: 1px solid color-mix(in srgb, var(--info) 30%, var(--surface)); }
.status-badge--blue .status-badge__dot { background: var(--info); }
</style>
