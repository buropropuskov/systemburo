<template>
  <AdminPageShell>
    <div class="requests-view dashboard-card">
      <div class="management-header">
        <h3 class="management-title">
          Мониторинг запросов
        </h3>
        <RefreshButton
          :loading="isBusy"
          @refresh="refreshAll"
        />
      </div>

      <KpiRow
        class="rv-stats"
        :items="kpis"
      />

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
        ref="journalTab"
        :active="activeTab === 'journal'"
        :hidden="tabHidden"
        @update:loading="value => (journalLoading = value)"
        @refresh-stats="fetchStats"
      />

      <AnalyticsTab
        v-show="activeTab === 'analytics'"
        ref="analyticsTab"
        :active="activeTab === 'analytics'"
        @update:loading="value => (analyticsLoading = value)"
      />
    </div>
  </AdminPageShell>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import KpiRow from '@/components/monitoring/KpiRow.vue';
import JournalTab from '@/components/monitoring/JournalTab.vue';
import AnalyticsTab from '@/components/monitoring/AnalyticsTab.vue';
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { describeLoadError, headerKpis } from '@/utils/requestLogsFormat';

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
// Загрузку держат сами вкладки, оболочка лишь сводит её на кнопку обновления:
// раньше общий признак гасил весь раздел пеленой, включая шапку, показатели и
// график, причём и на самообновлении ленты раз в десять секунд (#1305/#1306).
const journalLoading = ref(false);
const analyticsLoading = ref(false);
const sectionLoading = ref(false);
const tabHidden = ref(false);
const stats = ref({
  total: 0, today: 0, avg_duration: 0, median_duration: 0,
  p95_duration: 0, error_rate: 0, requests_per_minute: 0,
});
const realtime = ref({ last_second_count: 0, last_minute_count: 0 });
const journalTab = ref(null);
const analyticsTab = ref(null);
const kpis = computed(() => headerKpis(stats.value, realtime.value));
const isBusy = computed(() => sectionLoading.value || journalLoading.value || analyticsLoading.value);
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

/**
 * Кнопка обновления в шапке: показатели раздела и содержимое открытой вкладки.
 * Скрытую вкладку не трогаем - её данные обновятся, когда её откроют.
 */
async function refreshAll() {
  const tab = activeTab.value === 'journal' ? journalTab.value : analyticsTab.value;
  sectionLoading.value = true;
  try {
    await Promise.all([fetchStats(), fetchRealtime(), tab?.refresh?.()]);
  } finally {
    sectionLoading.value = false;
  }
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
/* Карточка растёт по содержимому, а прокручивается оболочка admin-page
   (у неё overflow: auto и высота под вьюпорт). Прежде здесь стояли height:100%
   от оболочки и overflow:hidden: раздел упирался в высоту экрана и всё, что не
   влезло, обрезал. На 1440x900 таблица начиналась на отметке 693 при своих 500
   пикселях - до низа окна оставалось 207, то есть три строки, а пагинация под
   ней не была видна вовсе. Селектор с двумя классами намеренно: правило
   оболочки .admin-page :deep(.dashboard-card) задаёт height:100% и по весу
   равно одноклассовому. */
.requests-view.dashboard-card {
  background: var(--surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  height: auto;
  min-height: 100%;
  overflow: visible;
}

/* Шапка раздела по эталону TableConstructor: фиксированные 50px и разделитель.
   Показатели переехали из неё в ряд карточек ниже - шесть подписей со
   значениями в строку разгоняли шапку до двух рядов и своей высоты у неё не
   было вовсе. */
.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.rv-stats {
  padding: 16px 20px;
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

@media (max-width: 768px) {
  .management-header {
    padding: 0 16px;
  }

  .rv-stats {
    padding: 12px 16px;
  }
}
</style>
