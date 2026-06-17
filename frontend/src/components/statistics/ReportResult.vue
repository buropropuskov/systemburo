<template>
  <div class="rr">
    <!-- Загрузка -->
    <div
      v-if="loading"
      class="rr__state"
    >
      <LoaderSpinner />
      <span>Строим отчёт…</span>
    </div>

    <!-- Ошибка -->
    <div
      v-else-if="error"
      class="rr__state rr__state--error"
    >
      {{ error }}
    </div>

    <!-- Пусто (отчёт ещё не построен) -->
    <div
      v-else-if="!result"
      class="rr__state rr__empty"
    >
      <svg
        width="44"
        height="44"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M3 3v18h18" />
        <path d="M7 14l3-4 3 3 4-6" />
      </svg>
      <p>Здесь появится ваш отчёт</p>
      <span>Задайте параметры выше и нажмите «Построить отчёт».</span>
    </div>

    <!-- Агрегатный отчёт -->
    <template v-else-if="result.mode === 'aggregate'">
      <div class="rr__table-wrap">
        <table class="rr__table">
          <thead>
            <tr>
              <th>Значение разреза</th>
              <th class="rr__num">
                Количество{{ result.unit ? `, ${result.unit}` : '' }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in result.rows"
              :key="i"
            >
              <td>{{ row.label }}</td>
              <td class="rr__num">
                {{ formatNumber(row.value) }}
              </td>
            </tr>
            <tr v-if="!result.rows.length">
              <td
                colspan="2"
                class="rr__norows"
              >
                Нет данных за выбранный период
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="rr__footer">
        Итого: <b>{{ formatNumber(result.total) }}</b>{{ result.unit ? ` ${result.unit}` : '' }}
        <span class="rr__footer-sep">·</span> строк: {{ result.rows.length }}
      </div>
    </template>

    <!-- Выгрузка строк (list) -->
    <template v-else>
      <div class="rr__table-wrap">
        <table class="rr__table">
          <thead>
            <tr>
              <th
                v-for="col in (result.columns || [])"
                :key="col.key"
              >
                {{ col.label }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in result.rows"
              :key="i"
            >
              <td
                v-for="col in (result.columns || [])"
                :key="col.key"
              >
                {{ formatCell(row[col.key]) }}
              </td>
            </tr>
            <tr v-if="!result.rows.length">
              <td
                :colspan="(result.columns || []).length || 1"
                class="rr__norows"
              >
                Нет данных за выбранный период
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="rr__footer">
        Всего: <b>{{ result.total }}</b>
        <span class="rr__footer-sep">·</span> показано строк: {{ result.rows.length }}
      </div>
    </template>
  </div>
</template>

<script setup>
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';

defineProps({
  result: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
});

function formatNumber(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n.toLocaleString('ru-RU') : value;
}

function formatCell(value) {
  if (value === null || value === undefined || value === '') return '—';
  return value;
}
</script>

<style scoped>
.rr {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rr__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 220px;
  text-align: center;
  color: var(--color-text-muted);
}

.rr__state--error {
  color: #c0392b;
}

.rr__empty svg {
  opacity: 0.35;
}

.rr__empty p {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text);
}

.rr__empty span {
  font-size: 13px;
  max-width: 320px;
}

.rr__table-wrap {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.rr__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}

.rr__table thead th {
  position: sticky;
  top: 0;
  background: var(--color-bg);
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 11px 14px;
  white-space: nowrap;
  border-bottom: 1px solid var(--color-border);
}

.rr__table tbody td {
  padding: 10px 14px;
  color: var(--color-text);
  border-bottom: 1px solid var(--color-border);
  vertical-align: top;
}

.rr__table tbody tr:last-child td {
  border-bottom: none;
}

.rr__table tbody tr:hover td {
  background: var(--color-bg);
}

.rr__num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.rr__norows {
  text-align: center;
  color: var(--color-text-muted);
  padding: 24px 14px;
}

.rr__footer {
  font-size: 14px;
  color: var(--color-text);
}

.rr__footer-sep {
  color: var(--color-text-muted);
  margin: 0 4px;
}
</style>
