<template>
  <div class="kpi-row">
    <div
      v-for="kpi in items"
      :key="kpi.label"
      class="kpi"
      data-testid="monitoring-kpi"
    >
      <div
        class="kpi-val"
        :class="{ bad: kpi.bad, live: kpi.live }"
      >
        {{ kpi.value }}
        <span
          v-if="kpi.sub"
          class="kpi-sub"
        >{{ kpi.sub }}</span>
      </div>
      <div
        class="kpi-lab"
        :title="kpi.hint"
      >
        {{ kpi.label }}
      </div>
    </div>
  </div>
</template>

<script setup>
/**
 * Ряд показателей раздела мониторинга. Один вид у показателей шапки и у сводки
 * вкладки «Аналитика»: числа там разной природы, но читает их один человек, и
 * два разных оформления одних и тех же величин он сверяет глазами дважды.
 */
defineProps({
  /** @type {{label: string, value: string, sub?: string, hint?: string, bad?: boolean, live?: boolean}[]} */
  items: { type: Array, required: true },
});
</script>

<style scoped>
/* auto-fit вместо жёсткого числа колонок: ряд шапки из шести показателей и
   сводка аналитики из четырёх ложатся одной сеткой, а пустые треки схлопываются
   и карточки занимают всю ширину. */
.kpi-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.kpi {
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  min-width: 0;
}

.kpi-val {
  font-size: 1.5em;
  font-weight: 700;
  color: var(--text);
  letter-spacing: -0.5px;
}

.kpi-val.bad {
  color: var(--danger-text);
}

.kpi-val.live {
  color: var(--success-text);
}

.kpi-sub {
  font-size: 0.6em;
  font-weight: 500;
  letter-spacing: 0;
  color: var(--text-muted);
  margin-left: 4px;
}

.kpi-lab {
  font-size: 0.72em;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: var(--text-muted);
  margin-top: 4px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .kpi-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
