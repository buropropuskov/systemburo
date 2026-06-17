<template>
  <div class="dashboard">

    <!-- ===== ГРУППА: ДАННЫЕ ===== -->
    <div class="dashboard__group">
      <div class="dashboard__group-head">
        <h2 class="dashboard__group-title">Данные</h2>
        <span class="dashboard__group-chip">бизнес-показатели за период</span>
        <span class="dashboard__group-rule" />
      </div>

      <div
        v-if="summaryLoading"
        class="dashboard__tiles"
      >
        <div
          v-for="n in 10"
          :key="n"
          class="dashboard__tile dashboard__tile--skeleton"
        />
      </div>

      <div
        v-else
        class="dashboard__tiles"
      >
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Всего заявок</div>
          <div class="dashboard__tile-val">{{ fmt(summary.total_applications) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Вложения (всего)</div>
          <div class="dashboard__tile-val">{{ fmt(attachmentsTotal) }}</div>
          <div
            v-if="attachmentBreakdown.length"
            class="dashboard__tile-breakdown"
          >
            <span
              v-for="item in attachmentBreakdown"
              :key="item.label"
              class="dashboard__breakdown-item"
            >{{ item.label }}: {{ item.count }}</span>
          </div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Обработано</div>
          <div class="dashboard__tile-val">{{ fmt(summary.processed) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">В работе</div>
          <div class="dashboard__tile-val">{{ fmt(summary.in_work) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Машин заехало</div>
          <div class="dashboard__tile-val">{{ fmt(summary.cars_entered) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Среднее машин / день</div>
          <div class="dashboard__tile-val">{{ fmtAvg(summary.avg_cars_per_day) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Людей прошло</div>
          <div class="dashboard__tile-val">{{ fmt(summary.people_entered) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Сумма товаров</div>
          <div class="dashboard__tile-val">{{ fmt(summary.items_sum) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Машин на территории</div>
          <div class="dashboard__tile-val">{{ fmt(summary.cars_on_territory) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Людей на территории</div>
          <div class="dashboard__tile-val">{{ fmt(summary.people_on_territory) }}</div>
        </div>
      </div>
    </div>

    <!-- ===== ГРУППА: СИСТЕМА ===== -->
    <div class="dashboard__group">
      <div class="dashboard__group-head">
        <h2 class="dashboard__group-title">Система</h2>
        <span class="dashboard__group-chip">технические показатели</span>
        <span class="dashboard__group-rule" />
      </div>

      <div
        v-if="summaryLoading"
        class="dashboard__tiles"
      >
        <div
          v-for="n in 9"
          :key="n"
          class="dashboard__tile dashboard__tile--skeleton"
        />
      </div>

      <div
        v-else
        class="dashboard__tiles"
      >
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Пользователей онлайн</div>
          <div class="dashboard__tile-val">{{ fmt(summary.users_online) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Активных пользователей</div>
          <div class="dashboard__tile-val">{{ fmt(summary.active_users) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Заблокировано</div>
          <div class="dashboard__tile-val">{{ fmt(summary.banned_users) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Открытых обращений</div>
          <div class="dashboard__tile-val">{{ fmt(summary.open_feedback) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Активных мест разгрузки</div>
          <div class="dashboard__tile-val">{{ fmt(summary.active_unload_places) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">ЧС: машины</div>
          <div class="dashboard__tile-val">{{ fmt(summary.blacklist_cars) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">ЧС: люди</div>
          <div class="dashboard__tile-val">{{ fmt(summary.blacklist_people) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Уникальных машин</div>
          <div class="dashboard__tile-val">{{ fmt(summary.unique_cars) }}</div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Уникальных людей</div>
          <div class="dashboard__tile-val">{{ fmt(summary.unique_people) }}</div>
        </div>
      </div>
    </div>

    <!-- ===== ГРАФИК ===== -->
    <div class="dashboard__chart-card">
      <div class="dashboard__chart-head">
        <div>
          <h3 class="dashboard__chart-title">{{ chartTitle }}</h3>
          <div class="dashboard__chart-sub">{{ chartSubtitle }}</div>
        </div>
        <div class="dashboard__chart-controls">
          <div class="dashboard__seg">
            <button
              v-for="m in metricOptions"
              :key="m.value"
              class="dashboard__seg-btn"
              :class="{ 'dashboard__seg-btn--active': activeMetric === m.value }"
              @click="activeMetric = m.value"
            >
              {{ m.label }}
            </button>
          </div>
          <div class="dashboard__seg">
            <button
              v-for="g in granularityOptions"
              :key="g.value"
              class="dashboard__seg-btn"
              :class="{ 'dashboard__seg-btn--active': activeGranularity === g.value }"
              @click="activeGranularity = g.value"
            >
              {{ g.label }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="timelineLoading"
        class="dashboard__chart-skeleton"
      />
      <RealTimeChart
        v-else
        :data="chartData"
        :height="300"
        :color="'var(--color-primary)'"
        interval-label="ед."
      />
    </div>

    <!-- ===== ЖИВЫЕ ЛЕНТЫ ===== -->
    <div class="dashboard__feeds">

      <!-- Лента людей -->
      <div class="dashboard__feed">
        <div class="dashboard__feed-head">
          <h3 class="dashboard__feed-title">
            <span class="dashboard__live-dot" />
            Проход людей
          </h3>
          <span class="dashboard__feed-meta">обновление каждые 10 сек · UTC+3</span>
        </div>

        <div class="dashboard__feed-list">
          <template v-if="feedLoading">
            <div
              v-for="n in 5"
              :key="n"
              class="dashboard__feed-row dashboard__feed-row--skeleton"
            />
          </template>
          <template v-else-if="peopleFeed.length === 0">
            <div class="dashboard__feed-empty">Нет записей</div>
          </template>
          <template v-else>
            <div
              v-for="(row, idx) in peopleFeed"
              :key="idx"
              class="dashboard__feed-row"
            >
              <div class="dashboard__feed-main">
                <div class="dashboard__feed-name">{{ row.subject }}</div>
                <div class="dashboard__feed-sub">
                  {{ row.organization }}<template v-if="row.place && row.place !== '—'"> · {{ row.place }}</template>
                </div>
              </div>
              <div class="dashboard__feed-right">
                <div class="dashboard__feed-time">{{ formatTime(row.time) }}</div>
                <span
                  class="dashboard__dir-badge"
                  :class="row.action_type === 'entry' ? 'dashboard__dir-badge--in' : 'dashboard__dir-badge--out'"
                >
                  {{ row.action_type === 'entry' ? 'Вход' : 'Выход' }}
                </span>
              </div>
            </div>
          </template>
        </div>
      </div>

      <!-- Лента машин -->
      <div class="dashboard__feed">
        <div class="dashboard__feed-head">
          <h3 class="dashboard__feed-title">
            <span class="dashboard__live-dot" />
            Проезд машин
          </h3>
          <span class="dashboard__feed-meta">обновление каждые 10 сек · UTC+3</span>
        </div>

        <div class="dashboard__feed-list">
          <template v-if="feedLoading">
            <div
              v-for="n in 5"
              :key="n"
              class="dashboard__feed-row dashboard__feed-row--skeleton"
            />
          </template>
          <template v-else-if="carsFeed.length === 0">
            <div class="dashboard__feed-empty">Нет записей</div>
          </template>
          <template v-else>
            <div
              v-for="(row, idx) in carsFeed"
              :key="idx"
              class="dashboard__feed-row"
            >
              <div class="dashboard__plate">{{ row.subject }}</div>
              <div class="dashboard__feed-main">
                <div class="dashboard__feed-name">
                  {{ row.mark || '' }}
                </div>
                <div class="dashboard__feed-sub">
                  {{ row.organization }}<template v-if="row.place && row.place !== '—'"> · {{ row.place }}</template>
                </div>
              </div>
              <div class="dashboard__feed-right">
                <div class="dashboard__feed-time">{{ formatTime(row.time) }}</div>
                <span
                  class="dashboard__dir-badge"
                  :class="row.action_type === 'entry' ? 'dashboard__dir-badge--in' : 'dashboard__dir-badge--out'"
                >
                  {{ row.action_type === 'entry' ? 'Въезд' : 'Выезд' }}
                </span>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue';
import RealTimeChart from '@/components/RealTimeChart.vue';
import { getSummary, getTimeline, getRecentPassages } from '@/api/statistics.js';

const props = defineProps({
  from: {
    type: String,
    default: '',
  },
  to: {
    type: String,
    default: '',
  },
});

// ---- состояние данных ----
const summary = ref({});
const summaryLoading = ref(false);

const timeline = ref([]);
const timelineLoading = ref(false);

const peopleFeed = ref([]);
const carsFeed = ref([]);
const feedLoading = ref(false);

// ---- переключатели графика ----
const metricOptions = [
  { label: 'Заявки', value: 'applications' },
  { label: 'Проходы людей', value: 'people_entries' },
  { label: 'Проезды машин', value: 'car_entries' },
];
const granularityOptions = [
  { label: 'День', value: 'day' },
  { label: 'Неделя', value: 'week' },
  { label: 'Месяц', value: 'month' },
];
const activeMetric = ref('applications');
const activeGranularity = ref('day');

const chartTitles = {
  applications: 'Динамика заявок',
  people_entries: 'Динамика проходов людей',
  car_entries: 'Динамика проездов машин',
};

const chartTitle = computed(() => chartTitles[activeMetric.value] ?? '');
const chartSubtitle = computed(() => {
  const labels = { day: 'по дням', week: 'по неделям', month: 'по месяцам' };
  const period = [props.from, props.to].filter(Boolean).join(' — ');
  return [period, labels[activeGranularity.value]].filter(Boolean).join(' · ');
});

// RealTimeChart ожидает [{timestamp, count}], бэк отдаёт [{date, count}]
const chartData = computed(() =>
  (timeline.value || []).map((d) => ({ timestamp: d.date, count: d.count }))
);

// ---- вычисляемые из summary ----
const attachmentsTotal = computed(() => {
  const map = summary.value.by_attachment_type;
  if (!map || typeof map !== 'object') return 0;
  return Object.values(map).reduce((s, v) => s + (v || 0), 0);
});

const attachmentBreakdown = computed(() => {
  const map = summary.value.by_attachment_type;
  if (!map || typeof map !== 'object') return [];
  return Object.entries(map).map(([label, count]) => ({ label, count }));
});

// ---- форматирование ----
function fmt(val) {
  if (val == null) return '—';
  return Number(val).toLocaleString('ru-RU');
}

function fmtAvg(val) {
  if (val == null) return '—';
  return Math.round(Number(val)).toLocaleString('ru-RU');
}

function formatTime(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  // Смещение UTC+3
  const utc3 = new Date(d.getTime() + 3 * 60 * 60 * 1000);
  return utc3.toISOString().substring(11, 19);
}

// ---- загрузка данных ----
async function loadSummary() {
  summaryLoading.value = true;
  try {
    const data = await getSummary(props.from, props.to);
    summary.value = data || {};
  } catch {
    summary.value = {};
  } finally {
    summaryLoading.value = false;
  }
}

async function loadTimeline() {
  timelineLoading.value = true;
  try {
    const data = await getTimeline({
      from: props.from,
      to: props.to,
      metric: activeMetric.value,
      granularity: activeGranularity.value,
    });
    timeline.value = Array.isArray(data) ? data : [];
  } catch {
    timeline.value = [];
  } finally {
    timelineLoading.value = false;
  }
}

async function loadFeed() {
  feedLoading.value = true;
  try {
    const data = await getRecentPassages(15);
    peopleFeed.value = Array.isArray(data?.people) ? data.people : [];
    carsFeed.value = Array.isArray(data?.cars) ? data.cars : [];
  } catch {
    peopleFeed.value = [];
    carsFeed.value = [];
  } finally {
    feedLoading.value = false;
  }
}

// ---- публичный метод для обновления из родителя ----
async function refresh() {
  await Promise.all([loadSummary(), loadTimeline(), loadFeed()]);
}

defineExpose({ refresh });

// ---- реакция на смену периода ----
watch([() => props.from, () => props.to], () => {
  loadSummary();
  loadTimeline();
});

// ---- реакция на смену настроек графика ----
watch([activeMetric, activeGranularity], () => {
  loadTimeline();
});

// ---- polling живых лент (10 сек) ----
let feedInterval = null;

onMounted(() => {
  loadSummary();
  loadTimeline();
  loadFeed();
  feedInterval = setInterval(loadFeed, 10000);
});

onUnmounted(() => {
  if (feedInterval) clearInterval(feedInterval);
});
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 30px;
}

/* ===== ГРУППЫ ===== */
.dashboard__group {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.dashboard__group-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dashboard__group-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  white-space: nowrap;
}

.dashboard__group-chip {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.dashboard__group-rule {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

/* ===== ПЛИТКИ ===== */
.dashboard__tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(158px, 1fr));
  gap: 12px;
}

.dashboard__tile {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  cursor: default;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
  animation: tile-in 0.35s ease both;
}

.dashboard__tile:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

@keyframes tile-in {
  from { opacity: 0; transform: translateY(10px); }
  to   { opacity: 1; transform: translateY(0); }
}

.dashboard__tile--skeleton {
  background: var(--color-skeleton);
  min-height: 72px;
  border: none;
  cursor: default;
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0.55; }
}

.dashboard__tile-label {
  font-size: 11px;
  color: var(--color-text-muted);
  font-weight: 500;
  line-height: 1.3;
}

.dashboard__tile-val {
  font-size: 26px;
  font-weight: 700;
  color: var(--color-text);
  margin-top: 6px;
  line-height: 1;
}

.dashboard__tile-breakdown {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}

.dashboard__breakdown-item {
  font-size: 10px;
  color: var(--color-text-muted);
  background: var(--color-bg);
  border-radius: var(--radius-pill);
  padding: 2px 7px;
  border: 1px solid var(--color-border);
}

/* ===== ГРАФИК ===== */
.dashboard__chart-card {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 20px 22px;
  background: #fff;
}

.dashboard__chart-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 18px;
}

.dashboard__chart-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

.dashboard__chart-sub {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 2px;
}

.dashboard__chart-controls {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.dashboard__seg {
  display: inline-flex;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  padding: 3px;
  gap: 2px;
}

.dashboard__seg-btn {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
  padding: 5px 12px;
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.dashboard__seg-btn:hover {
  color: var(--color-primary);
}

.dashboard__seg-btn--active {
  background: var(--color-primary);
  color: #fff;
}

.dashboard__chart-skeleton {
  height: 300px;
  background: var(--color-skeleton);
  border-radius: var(--radius-sm);
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

/* ===== ЖИВЫЕ ЛЕНТЫ ===== */
.dashboard__feeds {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 1100px) {
  .dashboard__feeds {
    grid-template-columns: 1fr;
  }
}

.dashboard__feed {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  background: #fff;
  display: flex;
  flex-direction: column;
}

.dashboard__feed-head {
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-shrink: 0;
}

.dashboard__feed-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.dashboard__live-dot {
  display: inline-block;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--color-success);
  flex-shrink: 0;
  animation: live-pulse 1.8s ease-in-out infinite;
}

@keyframes live-pulse {
  0%   { box-shadow: 0 0 0 0 rgba(40, 167, 69, 0.55); }
  70%  { box-shadow: 0 0 0 8px rgba(40, 167, 69, 0); }
  100% { box-shadow: 0 0 0 0 rgba(40, 167, 69, 0); }
}

.dashboard__feed-meta {
  font-size: 11px;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.dashboard__feed-list {
  max-height: 360px;
  overflow-y: auto;
}

.dashboard__feed-list::-webkit-scrollbar {
  width: 6px;
}

.dashboard__feed-list::-webkit-scrollbar-thumb {
  background: #e1e3f0;
  border-radius: var(--radius-pill);
}

.dashboard__feed-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 18px;
  border-bottom: 1px solid #f2f3fa;
  animation: row-in 0.3s ease;
}

@keyframes row-in {
  from { opacity: 0; transform: translateY(-6px); }
  to   { opacity: 1; transform: translateY(0); }
}

.dashboard__feed-row--skeleton {
  min-height: 56px;
  background: var(--color-skeleton);
  border-bottom: 1px solid #fff;
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

.dashboard__feed-main {
  min-width: 0;
  flex: 1;
}

.dashboard__feed-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dashboard__feed-sub {
  font-size: 11px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
}

.dashboard__feed-right {
  text-align: right;
  flex-shrink: 0;
}

.dashboard__feed-time {
  font-size: 11px;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}

.dashboard__dir-badge {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: var(--radius-pill);
  margin-top: 4px;
}

.dashboard__dir-badge--in {
  background: rgba(40, 167, 69, 0.12);
  color: var(--color-success);
}

.dashboard__dir-badge--out {
  background: var(--color-bg);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* Номерной знак машины */
.dashboard__plate {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 64px;
  padding: 3px 8px;
  border: 2px solid var(--color-text);
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text);
  letter-spacing: 0.04em;
  background: #fff;
  text-transform: uppercase;
}

.dashboard__feed-empty {
  padding: 24px 18px;
  font-size: 13px;
  color: var(--color-text-muted);
  text-align: center;
}
</style>
