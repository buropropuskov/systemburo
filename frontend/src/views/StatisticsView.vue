<template>
  <AdminPageShell>
    <div class="statistics dashboard-card">

      <!-- ===== ШАПКА ===== -->
      <div class="management-header statistics__header">
        <div class="statistics__header-left">
          <h1 class="management-title">Аналитика</h1>
        </div>
        <div class="statistics__header-right">
          <div
            ref="presetsEl"
            class="period-presets"
          >
            <span
              class="period-presets__indicator"
              :class="{ 'period-presets__indicator--ready': indicatorReady }"
              :style="presetIndicatorStyle"
              aria-hidden="true"
            />
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
          :class="{ 'statistics__tab--active': activeTab === 'processing' }"
          @click="activeTab = 'processing'"
        >
          Обработка заявок
        </button>
        <button
          class="statistics__tab"
          :class="{ 'statistics__tab--active': activeTab === 'reports' }"
          @click="activeTab = 'reports'"
        >
          Отчёты
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

        <!-- Обработка заявок -->
        <div
          v-else-if="activeTab === 'processing'"
          class="statistics__panel"
        >
          <ProcessingAnalytics
            ref="processingRef"
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
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import StatisticsDashboard from '@/components/statistics/StatisticsDashboard.vue';
import ProcessingAnalytics from '@/components/statistics/ProcessingAnalytics.vue';
import ReportsTab from '@/components/statistics/ReportsTab.vue';
import AnalyticsInstructionModal from '@/components/statistics/AnalyticsInstructionModal.vue';
import { getViewportZoom } from '@/utils/viewportScale';

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

// Скользящий индикатор активного пресета. Лейблы разной ширины (Сегодня шире Год),
// поэтому геометрию подложки меряем по активной кнопке, а не делим на равные доли.
// Индикатор position:absolute -> переход width не вызывает reflow соседей. При ручном
// выборе диапазона из календаря пресет сбрасывается -> индикатор скрывается через opacity.
const presetsEl = ref(null);
const indicatorReady = ref(false);
const presetIndicatorStyle = ref({ opacity: 0 });
const activePresetIndex = computed(() =>
  periodPresets.findIndex((p) => p.key === activePreset.value),
);

function updatePresetIndicator() {
  const cont = presetsEl.value;
  if (!cont) return;
  const idx = activePresetIndex.value;
  if (idx < 0) {
    presetIndicatorStyle.value = { ...presetIndicatorStyle.value, opacity: 0 };
    return;
  }
  const btn = cont.querySelectorAll('.period-preset')[idx];
  if (!btn) return;
  // На >1440 корень зумлен (viewportScale): rect'ы приходят в device-px, а style
  // применяется в layout-px и снова умножается на zoom - без деления индикатор
  // уезжает и растягивается в zoom раз (на 2539x1440 был dx +88, ширина +76).
  const z = getViewportZoom();
  const cRect = cont.getBoundingClientRect();
  const bRect = btn.getBoundingClientRect();
  presetIndicatorStyle.value = {
    width: `${Math.round(bRect.width / z)}px`,
    transform: `translateX(${Math.round((bRect.left - cRect.left) / z - cont.clientLeft)}px)`,
    opacity: 1,
  };
}

let presetResizeObserver = null;
onMounted(() => {
  nextTick(() => {
    updatePresetIndicator();
    // Переход включаем только после первой раскладки — иначе индикатор слайдится от 0 на маунте.
    requestAnimationFrame(() => {
      indicatorReady.value = true;
    });
    if (typeof ResizeObserver !== 'undefined' && presetsEl.value) {
      // Пересчёт при загрузке шрифтов, ресайзе и переносе шапки (presence != ready, урок #657).
      presetResizeObserver = new ResizeObserver(() => updatePresetIndicator());
      presetResizeObserver.observe(presetsEl.value);
    }
  });
});
onBeforeUnmount(() => {
  if (presetResizeObserver) presetResizeObserver.disconnect();
});
watch(activePreset, () => nextTick(updatePresetIndicator));

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
const processingRef = ref(null);

function onPeriodApply() {
  // Ручной выбор из календаря — период больше не соответствует кнопке-пресету.
  activePreset.value = null;
}

function onRefresh() {
  // Обновляем активную вкладку: неактивные размонтированы через v-if.
  // Отчёты строятся по кнопке, фонового обновления не требуют.
  if (activeTab.value === 'dashboard') {
    dashboardRef.value?.refresh();
  } else if (activeTab.value === 'processing') {
    processingRef.value?.refresh();
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
  position: relative;
  display: inline-flex;
  gap: 4px;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  padding: 3px;
}

/* Подложка активного пресета: геометрию задаёт JS по активной кнопке
   (presetIndicatorStyle). Лежит под текстом (z-index 0), out-of-flow -> переход
   width/transform не двигает соседей. Переход включается классом --ready после
   первой раскладки, чтобы не было слайда от нуля на маунте. */
.period-presets__indicator {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 0;
  width: 0;
  border-radius: var(--radius-pill);
  background: var(--color-primary);
  opacity: 0;
  pointer-events: none;
  z-index: 0;
}

.period-presets__indicator--ready {
  transition: transform 0.22s ease, width 0.22s ease, opacity 0.18s ease;
}

.period-preset {
  position: relative;
  z-index: 1;
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 5px 13px;
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: color 0.18s ease;
  white-space: nowrap;
}

.period-preset:hover {
  color: var(--color-primary);
}

.period-preset--active,
/* На активной кнопке hover не должен перекрашивать текст в синий — он сливается с подложкой. */
.period-preset--active:hover {
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
