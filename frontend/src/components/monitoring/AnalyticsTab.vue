<template>
  <div class="analytics-tab">
    <div class="analytics-toolbar">
      <DateFilter
        mode="range"
        :date-range-start="range.start"
        :date-range-end="range.end"
        @update:date-range-start="value => (from = dateToYmd(value))"
        @update:date-range-end="value => (to = dateToYmd(value))"
        @apply="fetchHistory"
        @clear="fetchHistory"
      />
      <button
        class="lk-button lk-button--primary"
        @click="fetchHistory"
      >
        Показать
      </button>
    </div>

    <p
      v-if="coverageNote"
      class="coverage-note"
    >
      {{ coverageNote }}
    </p>

    <KpiRow
      class="analytics-kpis"
      :items="kpis"
    />

    <div class="analytics-panel">
      <h4 class="panel-title">
        Запросов по дням
      </h4>
      <DailyRequestsChart
        v-if="chartPoints.length"
        :points="chartPoints"
      />
      <p
        v-else
        class="empty-hint"
      >
        Нет данных за период
      </p>
    </div>

    <div class="analytics-tables">
      <div class="analytics-panel">
        <h4 class="panel-title">
          Топ эндпоинтов
        </h4>
        <div class="hist-table-wrap">
          <table class="hist-table">
            <thead>
              <tr>
                <th>Endpoint</th><th>Запросов</th><th>Avg</th>
                <th :title="p95Note">
                  p95<span v-if="p95Note">*</span>
                </th>
                <th>Ошибки</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="e in history.top_endpoints"
                :key="e.endpoint"
              >
                <td class="mono">
                  {{ e.endpoint }}
                </td>
                <td>{{ formatNum(e.requests) }}</td>
                <td>{{ formatMs(e.avg_duration_ms) }}</td>
                <td>{{ formatMs(e.p95_duration_ms) }}</td>
                <td>{{ e.error_rate }}%</td>
              </tr>
              <tr v-if="!history.top_endpoints.length">
                <td
                  colspan="5"
                  class="empty-hint"
                >
                  Нет данных
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p
          v-if="p95Note"
          class="coverage-note"
        >
          * {{ p95Note }}
        </p>
      </div>
      <div class="analytics-panel">
        <h4 class="panel-title">
          Топ пользователей
        </h4>
        <div class="hist-table-wrap">
          <table class="hist-table">
            <thead>
              <tr>
                <th>Пользователь</th><th>Запросов</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="u in history.top_users"
                :key="u.user_id"
              >
                <td>{{ formatLogin(u.username) }}</td>
                <td>{{ formatNum(u.requests) }}</td>
              </tr>
              <tr v-if="!history.top_users.length">
                <td
                  colspan="2"
                  class="empty-hint"
                >
                  Нет данных
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import DailyRequestsChart from '@/components/monitoring/DailyRequestsChart.vue';
import DateFilter from '@/components/DateFilter.vue';
import KpiRow from '@/components/monitoring/KpiRow.vue';
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { formatLogin } from '@/utils/formatName';
import { dateToYmd, ymdToDate } from '@/utils/requestLogsQuery';
import {
  analyticsKpis, coverageNote as buildCoverageNote, dailyChartPoints, emptyHistory, formatMs,
  formatNum, historyFromResponse, p95Note as buildP95Note
} from '@/utils/requestLogsFormat';

/**
 * Вкладка «Аналитика»: сводка по свёрнутым суткам за выбранный период.
 * Данные читаются при первом показе вкладки, а не при открытии раздела -
 * запрос тяжелее журнального, и платить за него, не открыв вкладку, незачем.
 */
const props = defineProps({
  active: { type: Boolean, default: false },
});

const emit = defineEmits(['update:loading']);

const history = ref(emptyHistory());
const from = ref('');
const to = ref('');
const loaded = ref(false);

const range = computed(() => ({ start: ymdToDate(from.value), end: ymdToDate(to.value) }));
const kpis = computed(() => analyticsKpis(history.value.totals));
const coverageNote = computed(() => buildCoverageNote(history.value.coverage));
const p95Note = computed(() => buildP95Note(history.value.coverage));
const chartPoints = computed(() => dailyChartPoints(history.value.daily));

async function fetchHistory() {
  emit('update:loading', true);
  try {
    const params = new URLSearchParams();
    if (from.value) params.set('from_date', from.value);
    if (to.value) params.set('to_date', to.value);
    const qs = params.toString();
    const response = await apiRequest(`/request-logs/history${qs ? '?' + qs : ''}`);
    if (response.ok) {
      const data = await response.json();
      if (data) {
        history.value = historyFromResponse(data);
        loaded.value = true;
      }
    }
  } catch {
    useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'аналитику', type: 'error' });
  } finally {
    emit('update:loading', false);
  }
}

defineExpose({ refresh: fetchHistory });

watch(() => props.active, (active) => {
  if (active && !loaded.value) fetchHistory();
}, { immediate: true });
</script>

<style scoped>
.analytics-tab {
  padding: 16px 20px;
}

.analytics-toolbar {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.analytics-kpis {
  margin-bottom: 16px;
}

.analytics-panel {
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 14px;
  margin-bottom: 16px;
  box-shadow: var(--shadow-sm);
  /* grid-элемент: без этого min-width:auto распирает панель под min-content
     таблицы и обрезает правые колонки за кромкой экрана на мобилке */
  min-width: 0;
}

.panel-title {
  margin: 0 0 12px;
  font-size: 0.95em;
  font-weight: 600;
  color: var(--text);
}

.empty-hint {
  color: var(--text-muted);
  font-size: 13px;
  text-align: center;
  padding: 16px;
}

.coverage-note {
  margin: -8px 0 16px;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.4;
}

.analytics-tables {
  display: grid;
  grid-template-columns: 1.5fr 1fr;
  gap: 16px;
}

/* Обёртка с горизонтальным скроллом: узкая таблица (Топ пользователей) влезает
   целиком, широкая (Топ эндпоинтов, 5 колонок) честно скроллится вместо тихого
   клипа правых колонок. Образец - ReportResult.rr__table-wrap. */
.hist-table-wrap {
  overflow-x: auto;
}

.hist-table {
  width: 100%;
  border-collapse: collapse;
}

.hist-table th {
  text-align: left;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: var(--text-muted);
  font-weight: 600;
  padding: 9px 10px;
  background: var(--accent-tint);
  border-bottom: 1px solid var(--border);
}

.hist-table td {
  padding: 9px 10px;
  border-bottom: 1px solid var(--border);
  font-size: 12px;
  color: var(--text);
}

.hist-table tr:last-child td {
  border-bottom: none;
}

.hist-table .mono {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 11px;
}

@media (max-width: 768px) {
  .analytics-tables {
    grid-template-columns: 1fr;
  }
}
</style>
