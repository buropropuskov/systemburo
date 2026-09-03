<template>
  <div class="rb__picker">
    <template
      v-for="(sec, secIndex) in visibleMetricSections"
      :key="sec.key"
    >
      <div class="rb__group-title">
        {{ sec.title }}
      </div>
      <div class="rb__metrics">
        <label
          v-for="m in sec.metrics"
          :key="m.key"
          class="rb__metric"
          :class="{ 'rb__metric--on': isOn(m.key) }"
        >
          <input
            type="checkbox"
            class="rb__metric-input"
            :checked="isOn(m.key)"
            @change="toggle(m.key)"
          >
          <span class="rb__metric-box">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="3"
              stroke-linecap="round"
              stroke-linejoin="round"
            ><path d="M20 6 9 17l-5-5" /></svg>
          </span>
          <span class="rb__metric-text">
            <span class="rb__metric-name">{{ m.label }}</span>
            <span
              v-if="m.unit"
              class="rb__metric-unit"
            >{{ m.unit }}</span>
          </span>
          <HintTooltip
            v-if="metricHint(m.key)"
            class="rb__metric-hint"
            :text="metricHint(m.key)"
          />
        </label>
      </div>

      <button
        v-if="secIndex === 0 && hasPrimarySection && extraMetricsCount"
        type="button"
        class="rb__more"
        :aria-expanded="String(showAllMetrics)"
        @click="showAllMetrics = !showAllMetrics"
      >
        <span
          class="rb__more-caret"
          :class="{ 'rb__more-caret--open': showAllMetrics }"
          aria-hidden="true"
        />
        <span class="rb__more-text">Показатели обработки</span>
        <span class="rb__more-count">{{ extraMetricsCount }}</span>
        <span class="rb__more-note">времена, медианы, перцентили, доли</span>
      </button>
    </template>
  </div>
</template>

<script setup>
/**
 * Шаг «Что считаем» конструктора отчётов: карточки показателей, свёрнутая часть с
 * показателями обработки и подсказки к неочевидным.
 *
 * Отдельным компонентом, потому что ReportBuilder упёрся в порог размера, а шаг
 * самодостаточен: на входе каталог, на выходе выбранные ключи.
 */
import { ref, computed, watch } from 'vue';
import HintTooltip from '@/components/ui/HintTooltip.vue';

const props = defineProps({
  catalog: { type: Object, required: true },
});

const selected = defineModel({ type: Array, default: () => [] });

// Ходовые показатели: с них начинают почти любой отчёт. Остальные 22 - времена,
// медианы, перцентили и доли - нужны точечно и прячутся под раскрывашку.
const PRIMARY_METRIC_KEYS = ['applications_count', 'items_sum', 'car_entries_count', 'people_entries_count'];

// Подсказки к неочевидным показателям. Формулировки сверены с выражениями в
// report_quality_metrics.go и report_duration_metrics.go, а не написаны по памяти.
const METRIC_HINTS = {
  refusal_rate: 'Обе ветки отказа вместе: отказал принимающий или не согласовали согласующие. Считается от заявок, поданных за период.',
  rejected_rate: 'Только заявки со статусом «Отказано» - решение принимающего.',
  not_approved_rate: 'Только заявки, которые не согласовали согласующие. С отказом принимающего может совпасть, поэтому две доли не складываются в общую.',
};

function metricHint(key) {
  if (METRIC_HINTS[key]) return METRIC_HINTS[key];
  if (key.startsWith('p50_')) return 'Медиана: половина заявок укладывается в это время, половина идёт дольше.';
  if (key.startsWith('p90_')) return 'Девять заявок из десяти укладываются в это время. Показывает длинный хвост, который среднее сглаживает.';
  return '';
}

const metricGroups = computed(() => {
  const order = [];
  const byGroup = {};
  for (const m of props.catalog.metrics || []) {
    const g = m.group || 'Прочее';
    if (!byGroup[g]) {
      byGroup[g] = [];
      order.push(g);
    }
    byGroup[g].push(m);
  }
  return order.map((g) => ({ group: g, metrics: byGroup[g] }));
});

// Секции шага «Что считаем»: первая - «Основное», дальше исходные группы каталога
// без вошедших в неё метрик. Пустые группы отбрасываем, иначе останется заголовок
// без карточек.
const metricSections = computed(() => {
  const primary = [];
  for (const key of PRIMARY_METRIC_KEYS) {
    const m = (props.catalog.metrics || []).find((x) => x.key === key);
    if (m) primary.push(m);
  }
  const primaryKeys = new Set(primary.map((m) => m.key));
  const rest = metricGroups.value
    .map((g) => ({ key: g.group, title: g.group, metrics: g.metrics.filter((m) => !primaryKeys.has(m.key)) }))
    .filter((g) => g.metrics.length);
  if (!primary.length) return rest;
  return [{ key: 'primary', title: 'Основное', metrics: primary }, ...rest];
});

// Раскрывашка нужна, только когда «Основное» реально что-то скрывает.
const hasPrimarySection = computed(() => metricSections.value[0]?.key === 'primary');
const extraMetricsCount = computed(
  () => (hasPrimarySection.value ? metricSections.value.slice(1) : []).reduce((n, g) => n + g.metrics.length, 0),
);
const showAllMetrics = ref(false);
const visibleMetricSections = computed(
  () => (hasPrimarySection.value && !showAllMetrics.value ? metricSections.value.slice(0, 1) : metricSections.value),
);

// Пресет или шаблон могут включить показатель из свёрнутой части - тогда блок надо
// открыть, иначе галочка стоит там, где её не видно.
watch(() => selected.value.slice(), (keys) => {
  if (showAllMetrics.value || !hasPrimarySection.value) return;
  if (keys.some((k) => !PRIMARY_METRIC_KEYS.includes(k))) showAllMetrics.value = true;
}, { immediate: true });

function isOn(key) {
  return selected.value.includes(key);
}

function toggle(key) {
  const next = [...selected.value];
  const idx = next.indexOf(key);
  if (idx >= 0) next.splice(idx, 1);
  else next.push(key);
  selected.value = next;
}
</script>

<style scoped>
.rb__picker {
  display: contents;
}

.rb__more-caret {
  width: 0;
  height: 0;
  border-left: 5px solid currentColor;
  border-top: 4px solid transparent;
  border-bottom: 4px solid transparent;
  transition: transform 0.18s ease;
}
.rb__more-caret--open {
  transform: rotate(90deg);
}

/* Раскрывашка остальных показателей: тон служебной строки, не вторая главная кнопка. */
.rb__more {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  margin-top: 4px;
  padding: 10px 14px;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  background: none;
  color: var(--color-text-muted);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, color 0.18s ease;
}
.rb__more:hover {
  border-color: var(--color-primary);
  color: var(--color-text);
}
.rb__more-text {
  font-weight: 600;
}
.rb__more-count {
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  background: var(--color-border);
  font-size: 12px;
}
.rb__more-note {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rb__metric-hint {
  flex: none;
  margin-left: auto;
}

.rb__group-title {
  margin: 14px 0 9px 36px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.rb__group-title:first-of-type {
  margin-top: 2px;
}

.rb__metrics {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-left: 36px;
}

.rb__metric {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--surface);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease;
}

.rb__metric:hover {
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
  background: var(--accent-tint);
}

.rb__metric--on {
  border-color: var(--accent);
  background: var(--color-primary-tint);
}

.rb__metric-input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.rb__metric-box {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  margin-top: 1px;
  border: 2px solid var(--color-border);
  border-radius: 6px;
  background: var(--surface);
  display: grid;
  place-items: center;
  color: var(--accent-contrast);
  transition: background 0.18s ease, border-color 0.18s ease;
}

.rb__metric-box svg {
  width: 13px;
  height: 13px;
  opacity: 0;
  transition: opacity 0.18s ease;
}

.rb__metric--on .rb__metric-box {
  background: var(--color-primary);
  border-color: var(--accent);
}

.rb__metric--on .rb__metric-box svg {
  opacity: 1;
}

.rb__metric-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rb__metric-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.rb__metric-unit {
  font-size: 11px;
  color: var(--color-text-muted);
}

/* Мобилка: колонка шага без левого отступа, карточки плотнее, на 480 - в один столбец. */
@media (max-width: 768px) {
  .rb__metrics,
  .rb__group-title {
    margin-left: 0;
  }

  .rb__metric {
    padding: 13px 14px;
  }

  .rb__more {
    margin-left: 0;
  }
}

@media (max-width: 480px) {
  .rb__metrics {
    grid-template-columns: 1fr;
  }
}
</style>
