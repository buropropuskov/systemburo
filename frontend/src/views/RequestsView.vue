<template>
  <AdminPageShell>
    <div class="requests-view dashboard-card">
      <div class="management-header">
        <h3 class="management-title">
          Мониторинг запросов
        </h3>
        <div class="header-controls">
          <div class="stats-summary">
            <span class="stat-item">
              <span class="stat-label">Всего:</span>
              <span class="stat-value">{{ stats.total || 0 }}</span>
            </span>
            <span class="stat-item">
              <span class="stat-label">Сегодня:</span>
              <span class="stat-value">{{ stats.today || 0 }}</span>
            </span>
            <span class="stat-item">
              <span
                class="stat-label"
                title="Медиана и 95-й перцентиль времени ответа за последний час. Долгоживущие подписки на события не учитываются: у них в журнале записано время жизни соединения, а не время ответа."
              >Отклик:</span>
              <span class="stat-value">
                {{ formatMs(stats.median_duration, false) }} / {{ formatMs(stats.p95_duration) }}
              </span>
            </span>
            <span class="stat-item">
              <span class="stat-label">Ошибки:</span>
              <span
                class="stat-value"
                :class="{ 'stat-error': stats.error_rate > 5 }"
              >
                {{ (stats.error_rate || 0).toFixed(1) }}%
              </span>
            </span>
            <span class="stat-item">
              <span class="stat-label">RPM:</span>
              <span class="stat-value">{{ (stats.requests_per_minute || 0).toFixed(1) }}</span>
            </span>
            <span
              v-if="realtime.last_minute_count != null"
              class="stat-item realtime-stat"
            >
              <span class="stat-label">Сейчас:</span>
              <span class="stat-value live-value">
                {{ realtime.last_second_count || 0 }}/с
                <span class="stat-minute">{{ realtime.last_minute_count || 0 }}/мин</span>
              </span>
            </span>
          </div>
        </div>
      </div>

      <div class="rv-tabs">
        <button
          v-for="tab in TABS"
          :key="tab.key"
          class="rv-tab"
          :class="{ active: activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <JournalTab
        v-show="activeTab === 'journal'"
        :active="activeTab === 'journal'"
        :hidden="tabHidden"
        @update:loading="value => (isLoading = value)"
        @refresh-stats="fetchStats"
      />

      <AnalyticsTab
        v-show="activeTab === 'analytics'"
        :active="activeTab === 'analytics'"
        @update:loading="value => (isLoading = value)"
      />

      <div
        v-if="isLoading"
        class="loading-overlay"
      >
        <LoaderSpinner
          size="large"
          :label="''"
        />
      </div>
    </div>
  </AdminPageShell>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import JournalTab from '@/components/monitoring/JournalTab.vue';
import AnalyticsTab from '@/components/monitoring/AnalyticsTab.vue';
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { describeLoadError, formatMs } from '@/utils/requestLogsFormat';

/**
 * Раздел мониторинга обращений: показатели в шапке и две вкладки - живой журнал
 * и история по свёрнутым суткам. Сами вкладки самостоятельны, оболочка держит
 * только общие для них показатели и счётчики ленты.
 */
const TABS = [
  { key: 'journal', label: 'Журнал · live' },
  { key: 'analytics', label: 'Аналитика · история' },
];

const deletions = useDeletionsStore();

const activeTab = ref('journal');
const isLoading = ref(false);
const tabHidden = ref(false);
const stats = ref({
  total: 0, today: 0, avg_duration: 0, median_duration: 0,
  p95_duration: 0, error_rate: 0, requests_per_minute: 0,
});
const realtime = ref({ last_second_count: 0, last_minute_count: 0 });
// Причины отказов, о которых уже сообщили: показатели и счётчики опрашиваются
// по таймеру, и тост на каждый отказ выстраивал бы очередь одинаковых сообщений.
const reported = new Set();
let realtimeTimer = null;
let statsTimer = null;

/**
 * Читает раздел шапки. Сбой не гасит вкладки: показатели, счётчики и списки
 * живут независимо друг от друга.
 */
async function loadSection(path, apply, label) {
  try {
    const response = await apiRequest(path);
    if (!response.ok) return reportError(response, label);
    const data = await response.json();
    if (data) apply(data);
  } catch (error) {
    reportError(error, label);
  }
}

function reportError(source, label) {
  const key = `${label}:${(source && source.status) || 'net'}`;
  if (reported.has(key)) return;
  reported.add(key);
  deletions.notify({ bold: describeLoadError(source, `загрузить ${label}`), type: 'error' });
}

function fetchStats() {
  return loadSection('/request-logs/stats', data => { stats.value = data; }, 'показатели');
}

function fetchRealtime() {
  return loadSection('/request-logs/realtime', data => { realtime.value = data; }, 'счётчики ленты');
}

function onVisibilityChange() {
  tabHidden.value = document.hidden;
  // Вернувшись из фона, счётчики догоняют пропущенное сразу, а не через
  // очередной интервал: иначе первое, что видит человек, - устаревшие числа.
  if (!document.hidden) fetchRealtime();
}

onMounted(async () => {
  await Promise.all([fetchStats(), fetchRealtime()]);
  // Опросы в фоновой вкладке не идут вовсе: раздел сам же и вычищали от шума
  // самозапросов, а показатели за время отсутствия догоняются при возврате.
  realtimeTimer = setInterval(() => {
    if (!tabHidden.value) fetchRealtime();
  }, 5000);
  statsTimer = setInterval(() => {
    if (!tabHidden.value) fetchStats();
  }, 30000);
  document.addEventListener('visibilitychange', onVisibilityChange);
});

onBeforeUnmount(() => {
  [realtimeTimer, statsTimer].forEach(clearInterval);
  realtimeTimer = null;
  statsTimer = null;
  document.removeEventListener('visibilitychange', onVisibilityChange);
});
</script>

<style scoped>
.requests-view {
  background: var(--surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  overflow: hidden;
}

.management-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
  border-bottom: 1px solid var(--border);
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.stats-summary {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.stat-label {
  font-size: 0.85em;
  color: var(--text-muted);
}

.stat-value {
  font-size: 0.9em;
  font-weight: 600;
  color: var(--text);
}

.stat-value.stat-error {
  color: var(--danger-text);
}

.live-value {
  color: var(--success-text);
}

.stat-minute {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
  margin-left: 4px;
}

.realtime-stat {
  padding-left: 12px;
  border-left: 1px solid var(--border);
}

/* Вкладки журнал / аналитика */
.rv-tabs {
  display: flex;
  gap: 2px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
}

.rv-tab {
  font-family: inherit;
  font-weight: 600;
  font-size: 13px;
  cursor: pointer;
  border: none;
  background: none;
  color: var(--text-muted);
  padding: 12px 16px;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.rv-tab.active {
  color: var(--accent-text);
  border-bottom-color: var(--accent-text);
}

.loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

@media (max-width: 768px) {
  .management-header {
    padding: 16px;
  }

  .header-controls {
    flex-direction: column;
    align-items: stretch;
  }

  .stats-summary {
    justify-content: space-between;
    width: 100%;
  }
}
</style>
