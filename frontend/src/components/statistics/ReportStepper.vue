<template>
  <div class="stepper">
    <template
      v-for="(step, i) in steps"
      :key="step.label"
    >
      <div
        v-if="i > 0"
        class="step-line"
        :class="{ 'step-line--filled': steps[i - 1].state === 'done' }"
      />
      <div
        class="step"
        :class="`step--${step.state}`"
      >
        <span class="step-dot">
          <svg
            v-if="step.state === 'done'"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          ><path d="M20 6 9 17l-5-5" /></svg>
          <template v-else>{{ i + 1 }}</template>
        </span>
        <span class="step-label">{{ step.label }}</span>
      </div>
    </template>
  </div>
</template>

<script setup>
/**
 * Шаги мастера отчётов: визуальный индикатор прогресса. Состояние шага считает
 * родитель (ReportsTab) по заполненности формы — компонент только рисует.
 * @typedef {{ label: string, state: 'done' | 'current' | 'upcoming' }} Step
 */
defineProps({
  steps: { type: Array, required: true },
});
</script>

<style scoped>
.stepper {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 18px 24px;
  background: #fbfbfe;
  border-bottom: 1px solid var(--color-border);
  overflow-x: auto;
}

.step {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.step-dot {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: #fff;
  border: 2px solid var(--color-border);
  display: grid;
  place-items: center;
  font-weight: 700;
  font-size: 13px;
  color: var(--color-text-muted);
  transition: background 0.2s ease, border-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
}

.step-dot svg {
  width: 14px;
  height: 14px;
}

.step--done .step-dot {
  background: var(--color-success);
  border-color: var(--color-success);
  color: #fff;
}

.step--current .step-dot {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
  box-shadow: 0 0 0 4px rgba(79, 91, 223, 0.15);
}

.step-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.step--current .step-label,
.step--done .step-label {
  color: var(--color-text);
}

.step-line {
  flex: 1;
  height: 2px;
  background: var(--color-border);
  min-width: 18px;
  border-radius: 2px;
  transition: background 0.2s ease;
}

.step-line--filled {
  background: var(--color-success);
}
</style>
