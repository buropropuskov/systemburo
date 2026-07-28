<template>
  <div class="top">
    <div class="top__head">
      <h3 class="top__title">{{ title }}</h3>
      <span
        v-if="subtitle"
        class="top__sub"
      >{{ subtitle }}</span>
    </div>

    <div
      v-if="items.length === 0"
      class="top__empty"
    >
      Нет данных
    </div>
    <ol
      v-else
      class="top__list"
    >
      <li
        v-for="(it, i) in items"
        :key="i"
        class="top__row"
      >
        <span class="top__rank">{{ i + 1 }}</span>
        <div class="top__body">
          <div class="top__line">
            <span
              class="top__name"
              :title="it.label"
            >{{ it.label }}</span>
            <span class="top__val">{{ fmt(it.value) }}</span>
          </div>
          <div class="top__bar">
            <span
              class="top__bar-fill"
              :style="{ transform: `scaleX(${barWidth(it.value) / 100})` }"
            />
          </div>
        </div>
      </li>
    </ol>
  </div>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({
  /** Заголовок списка-лидерборда. */
  title: {
    type: String,
    required: true,
  },
  /** Подпись под заголовком (что измеряем). */
  subtitle: {
    type: String,
    default: '',
  },
  /** Элементы лидерборда [{label, value}], уже отсортированные по убыванию. */
  items: {
    type: Array,
    default: () => [],
  },
});

const max = computed(() =>
  props.items.reduce((m, it) => Math.max(m, Number(it.value) || 0), 0)
);

// Ширина бара в процентах от лидера; минимум 4% — чтобы ненулевое значение
// всегда было видно полоской, а не схлопывалось в ноль.
function barWidth(v) {
  const m = max.value;
  if (!m) return 0;
  return Math.max(4, Math.round((Number(v) || 0) / m * 100));
}

function fmt(val) {
  if (val == null) return '—';
  return Number(val).toLocaleString('ru-RU');
}
</script>

<style scoped>
.top {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
  background: var(--surface);
}

.top__head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 14px;
}

.top__title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

.top__sub {
  font-size: 11px;
  color: var(--color-text-muted);
}

.top__empty {
  font-size: 13px;
  color: var(--color-text-muted);
  padding: 16px 0;
  text-align: center;
}

.top__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.top__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.top__rank {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.top__body {
  flex: 1;
  min-width: 0;
}

.top__line {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 5px;
}

.top__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  /* Без min-width:0 flex-элемент не сжимается ниже min-content, ellipsis не
     срабатывает и длинное имя организации задаёт огромный min-content всей
     колонке -> на мобилке распирает дашборд за вьюпорт. */
  min-width: 0;
}

.top__val {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-text);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.top__bar {
  height: 7px;
  border-radius: var(--radius-pill);
  background: var(--color-bg);
  overflow: hidden;
}

.top__bar-fill {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: var(--radius-pill);
  background: var(--color-primary);
  /* Длину бара задаём через scaleX (origin слева), а не width — анимация идёт
     через композитор, как требует стандарт анимаций проекта. */
  transform-origin: left center;
  transition: transform 0.4s ease;
}
</style>
