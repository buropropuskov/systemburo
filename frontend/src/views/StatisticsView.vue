<template>
  <AdminPageShell>
    <div class="statistics dashboard-card">

      <!-- ===== ШАПКА ===== -->
      <div class="management-header statistics__header">
        <div class="statistics__header-left">
          <h1 class="management-title">Аналитика</h1>
        </div>
        <div class="statistics__header-right">
          <div class="period-presets">
            <button
              v-for="p in periodPresets"
              :key="p.key"
              type="button"
              class="period-preset"
              :class="{ 'period-preset--active': activePreset === p.key }"
              @click="applyPeriodPreset(p.key)"
            >
              {{ p.label }}
            </button>
          </div>
          <button
            class="lk-button lk-button--ghost"
            @click="showInstruction = true"
          >
            <svg
              width="15"
              height="15"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="10" />
              <path d="M12 16v-4M12 8h.01" />
            </svg>
            Инструкция
          </button>
          <DateFilter
            mode="range"
            :date-range-start="rangeStart"
            :date-range-end="rangeEnd"
            @update:date-range-start="rangeStart = $event"
            @update:date-range-end="rangeEnd = $event"
            @apply="onPeriodApply"
          />
          <RefreshButton
            :loading="false"
            @refresh="onRefresh"
          />
        </div>
      </div>

      <!-- ===== ВКЛАДКИ ===== -->
      <div class="statistics__tabs">
        <button
          class="statistics__tab"
          :class="{ 'statistics__tab--active': activeTab === 'dashboard' }"
          @click="activeTab = 'dashboard'"
        >
          Дашборд
        </button>
        <button
          class="statistics__tab"
          :class="{ 'statistics__tab--active': activeTab === 'reports' }"
          @click="activeTab = 'reports'"
        >
          Отчёты
        </button>
        <button
          class="statistics__tab"
          :class="{ 'statistics__tab--active': activeTab === 'insights' }"
          @click="activeTab = 'insights'"
        >
          Инсайты
        </button>
      </div>

      <!-- ===== ТЕЛО ===== -->
      <div class="statistics__body">

        <!-- Дашборд -->
        <div
          v-if="activeTab === 'dashboard'"
          class="statistics__panel"
        >
          <StatisticsDashboard
            ref="dashboardRef"
            :from="fromStr"
            :to="toStr"
          />
        </div>

        <!-- Отчёты -->
        <div
          v-else-if="activeTab === 'reports'"
          class="statistics__panel"
        >
          <ReportsTab
            :from="fromStr"
            :to="toStr"
          />
        </div>

        <!-- Инсайты -->
        <div
          v-else
          class="statistics__panel"
        >
          <InsightsTab
            ref="insightsRef"
            :from="fromStr"
            :to="toStr"
          />
        </div>
      </div>
    </div>

    <!-- Модалка инструкции -->
    <AnalyticsInstructionModal
      :show="showInstruction"
      @close="showInstruction = false"
    />
  </AdminPageShell>
</template>

<script setup>
import { ref, computed } from 'vue';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import StatisticsDashboard from '@/components/statistics/StatisticsDashboard.vue';
import ReportsTab from '@/components/statistics/ReportsTab.vue';
import InsightsTab from '@/components/statistics/InsightsTab.vue';
import AnalyticsInstructionModal from '@/components/statistics/AnalyticsInstructionModal.vue';

// ---- вкладки ----
const activeTab = ref('dashboard');

// ---- модалка инструкции ----
const showInstruction = ref(false);

// ---- период: дефолт — текущая неделя (пн — сегодня) ----
function weekStart() {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  const day = d.getDay() || 7;
  d.setDate(d.getDate() - day + 1);
  return d;
}

const rangeStart = ref(weekStart());
const rangeEnd = ref((() => { const d = new Date(); d.setHours(23, 59, 59, 999); return d; })());

// Быстрые кнопки периода в шапке — частые диапазоны без открытия календаря.
const periodPresets = [
  { key: 'today', label: 'Сегодня' },
  { key: 'week', label: 'Неделя' },
  { key: 'month', label: 'Месяц' },
  { key: 'year', label: 'Год' },
];
const activePreset = ref('week');

function applyPeriodPreset(key) {
  const now = new Date();
  const end = new Date(now);
  end.setHours(23, 59, 59, 999);
  let start;
  if (key === 'today') {
    start = new Date(now);
  } else if (key === 'week') {
    start = weekStart();
  } else if (key === 'month') {
    start = new Date(now.getFullYear(), now.getMonth(), 1);
  } else {
    start = new Date(now.getFullYear(), 0, 1);
  }
  start.setHours(0, 0, 0, 0);
  rangeStart.value = start;
  rangeEnd.value = end;
  activePreset.value = key;
}

function padTwo(n) {
  return String(n).padStart(2, '0');
}

function toDateStr(d) {
  if (!d) return '';
  return `${d.getFullYear()}-${padTwo(d.getMonth() + 1)}-${padTwo(d.getDate())}`;
}

const fromStr = computed(() => toDateStr(rangeStart.value));
const toStr = computed(() => toDateStr(rangeEnd.value));

// ---- ссылки на вкладки для вызова refresh ----
const dashboardRef = ref(null);
const insightsRef = ref(null);

function onPeriodApply() {
  // Ручной выбор из календаря — период больше не соответствует кнопке-пресету.
  activePreset.value = null;
}

function onRefresh() {
  // Обновляем активную вкладку: неактивные размонтированы через v-if.
  if (activeTab.value === 'dashboard') {
    dashboardRef.value?.refresh();
  } else if (activeTab.value === 'insights') {
    insightsRef.value?.refresh();
  }
}
</script>

<style scoped>
.statistics {
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: 35px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  height: 100%;
}

.statistics__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 16px 24px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.statistics__header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.statistics__header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* Быстрые кнопки периода */
.period-presets {
  display: inline-flex;
  gap: 4px;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  padding: 3px;
}

.period-preset {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 5px 13px;
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
  white-space: nowrap;
}

.period-preset:hover {
  color: var(--color-primary);
}

.period-preset--active {
  background: var(--color-primary);
  color: #fff;
}

/* Кнопка «Инструкция» в стиле проекта */
.lk-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: var(--radius-pill);
  padding: 6px 14px;
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid transparent;
  transition: background 0.18s ease, color 0.18s ease, border-color 0.18s ease;
  white-space: nowrap;
  line-height: 1;
}

.lk-button--ghost {
  background: transparent;
  color: var(--color-text-muted);
  border-color: var(--color-border);
}

.lk-button--ghost:hover {
  background: var(--color-bg);
  color: var(--color-text);
}

/* ===== ВКЛАДКИ ===== */
.statistics__tabs {
  display: flex;
  gap: 0;
  padding: 0 24px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.statistics__tab {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 14px 18px;
  cursor: pointer;
  position: relative;
  transition: color 0.18s ease;
}

.statistics__tab:hover {
  color: var(--color-text);
}

.statistics__tab--active {
  color: var(--color-primary);
}

.statistics__tab--active::after {
  content: '';
  position: absolute;
  left: 14px;
  right: 14px;
  bottom: -1px;
  height: 3px;
  border-radius: 3px 3px 0 0;
  background: var(--color-primary);
}

/* ===== ТЕЛО ===== */
.statistics__body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.statistics__body::-webkit-scrollbar {
  width: 8px;
}

.statistics__body::-webkit-scrollbar-thumb {
  background: #e1e3f0;
  border-radius: var(--radius-pill);
  border: 2px solid #fff;
}

.statistics__panel {
  padding: 26px 28px 40px;
}

/* management-header / management-title подхватываются из AdminPageShell :deep */
.management-title {
  font-size: 1.2em;
  font-weight: 600;
  color: #000;
  margin: 0;
}
</style>
