<template>
  <span
    class="rl-badge"
    :class="[toneClass, `rl-badge--${variant}`]"
  >{{ text }}</span>
</template>

<script setup>
import { computed } from 'vue';
import { getMethodClass, getStatusClass } from '@/utils/requestLogsFormat';

/**
 * Метод или код ответа цветным бейджем. Один компонент на таблицу и карточку
 * запроса: цвета у них общие, различается только плотность.
 */
const props = defineProps({
  kind: { type: String, required: true, validator: (v) => ['method', 'status'].includes(v) },
  value: { type: [String, Number], default: null },
  // cell - строка таблицы (своя минимальная ширина), detail - карточка запроса.
  variant: { type: String, default: 'cell' },
});

const toneClass = computed(() => (
  props.kind === 'method' ? getMethodClass(props.value) : getStatusClass(props.value)
));

const text = computed(() => (
  props.kind === 'method' ? props.value : (props.value || 'N/A')
));
</script>

<style scoped>
.rl-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: 11px;
  font-weight: 600;
  text-align: center;
}

.rl-badge--detail {
  padding: 2px 8px;
  font-size: 0.9em;
}

.method-get { background: var(--success); color: var(--fill-text); min-width: 50px; }
.method-post { background: var(--info); color: var(--fill-text); min-width: 50px; }
.method-put { background: var(--warning); color: var(--fill-text); min-width: 50px; }
.method-delete { background: var(--danger); color: var(--fill-text); min-width: 50px; }
/* Сиреневый и серый в тёмной теме светлее подписи на них, поэтому текст здесь
   берёт цвет подложки, а не общий --fill-text. */
.method-patch { background: var(--updated-accent); color: var(--surface); min-width: 50px; }
.method-other { background: var(--text-muted); color: var(--surface); min-width: 50px; }

.status-success { background: var(--success); color: var(--fill-text); min-width: 40px; }
.status-redirect { background: var(--info); color: var(--fill-text); min-width: 40px; }
.status-client-error { background: var(--warning); color: var(--fill-text); min-width: 40px; }
.status-server-error { background: var(--danger); color: var(--fill-text); min-width: 40px; }
.status-unknown { background: var(--text-muted); color: var(--surface); min-width: 40px; }

/* В карточке запроса бейдж стоит в строке значения и ширину не занимает. */
.rl-badge--detail.method-get,
.rl-badge--detail.method-post,
.rl-badge--detail.method-put,
.rl-badge--detail.method-delete,
.rl-badge--detail.method-patch,
.rl-badge--detail.method-other,
.rl-badge--detail.status-success,
.rl-badge--detail.status-redirect,
.rl-badge--detail.status-client-error,
.rl-badge--detail.status-server-error,
.rl-badge--detail.status-unknown {
  min-width: 0;
}
</style>
