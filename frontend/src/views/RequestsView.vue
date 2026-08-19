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
          class="rv-tab"
          :class="{ active: activeTab === 'journal' }"
          @click="activeTab = 'journal'"
        >
          Журнал · live
        </button>
        <button
          class="rv-tab"
          :class="{ active: activeTab === 'analytics' }"
          @click="switchToAnalytics"
        >
          Аналитика · история
        </button>
      </div>

      <div v-show="activeTab === 'journal'">
        <div class="chart-section">
          <div class="chart-header">
            <h4 class="chart-title">
              Запросы {{ selectedPeriod.title }}
            </h4>
            <div class="chart-header-actions">
              <span class="chart-interval">интервал: {{ selectedPeriod.intervalHuman }}</span>
              <div class="chart-period-dropdown">
                <select
                  v-model="chartPeriod"
                  class="chart-period-select"
                  @change="onChartPeriodChange"
                >
                  <option
                    v-for="p in chartPeriods"
                    :key="p.key"
                    :value="p.key"
                  >
                    {{ p.label }}
                  </option>
                </select>
              </div>
            </div>
          </div>
          <RealTimeChart
            :data="timelineData"
            :height="180"
            color="#4F5BDF"
            :interval-label="selectedPeriod.xAxisLabel"
          />
        </div>

        <div class="filters-bar">
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск по логам...'"
            @keyup.enter="refreshLogs"
          />
          <div class="filter-controls">
            <select
              v-model="filterMethod"
              class="filter-select"
              @change="refreshLogs"
            >
              <option value="">
                Все методы
              </option>
              <option value="GET">
                GET
              </option>
              <option value="POST">
                POST
              </option>
              <option value="PUT">
                PUT
              </option>
              <option value="DELETE">
                DELETE
              </option>
              <option value="PATCH">
                PATCH
              </option>
            </select>
            <select
              v-model="filterStatus"
              class="filter-select"
              @change="refreshLogs"
            >
              <option value="">
                Все статусы
              </option>
              <option value="200">
                200 OK
              </option>
              <option value="400">
                400 Bad Request
              </option>
              <option value="401">
                401 Unauthorized
              </option>
              <option value="403">
                403 Forbidden
              </option>
              <option value="404">
                404 Not Found
              </option>
              <option value="500">
                500 Server Error
              </option>
            </select>
            <select
              v-model="filterUser"
              class="filter-select"
              @change="refreshLogs"
            >
              <option value="">
                Все пользователи
              </option>
              <option
                v-for="u in users"
                :key="u.id"
                :value="u.id"
              >
                {{ formatLogin(u.username) }}
              </option>
            </select>
            <input
              v-model="filterStartDate"
              type="date"
              class="date-input"
              @change="refreshLogs"
            >
            <input
              v-model="filterEndDate"
              type="date"
              class="date-input"
              @change="refreshLogs"
            >
          </div>
          <RefreshButton
            :loading="isLoading"
            @refresh="refreshLogs"
          />
          <button
            class="clear-filters-btn"
            @click="clearFilters"
          >
            Сбросить
          </button>
          <button
            class="export-btn"
            :disabled="isExporting"
            @click="exportLogs"
          >
            {{ isExporting ? 'Экспорт...' : 'Экспорт' }}
          </button>
        </div>

        <div class="content-container">
          <div class="logs-table-section">
            <div class="table-container">
              <div class="table-header">
                <div
                  v-for="col in sortableColumns"
                  :key="col.field"
                  class="header-col"
                  :class="col.cls"
                  @click="sortBy(col.field)"
                >
                  <p :class="{ 'active-sort': sortField === col.field }">
                    {{ col.label }}
                  </p>
                  <AppIcon
                    name="sort"
                    class="sort-icon"
                    :class="{
                      'sorted': sortField === col.field,
                      'desc': sortField === col.field && sortDirection === 'desc'
                    }"
                  />
                </div>
              </div>

              <div
                ref="logsBody"
                class="table-body"
              >
                <div
                  v-for="log in logs"
                  :key="log.id"
                  class="table-row"
                  :class="{
                    'selected': selectedLog && selectedLog.id === log.id,
                    'error-row': log.response_status && log.response_status >= 400,
                    'success-row': log.response_status && log.response_status < 400
                  }"
                  @click="selectLog(log)"
                >
                  <div class="table-col time-col">
                    <span
                      class="cell-content"
                      :title="formatFullDate(log.created_at)"
                    >
                      {{ formatTime(log.created_at) }}
                    </span>
                  </div>
                  <div class="table-col method-col">
                    <span
                      class="method-badge"
                      :class="getMethodClass(log.method)"
                    >
                      {{ log.method }}
                    </span>
                  </div>
                  <div class="table-col path-col">
                    <span
                      class="truncate-text"
                      :title="log.url"
                    >
                      {{ truncatePath(log.url) }}
                    </span>
                  </div>
                  <div class="table-col status-col">
                    <span
                      class="status-badge"
                      :class="getStatusClass(log.response_status)"
                    >
                      {{ log.response_status || 'N/A' }}
                    </span>
                  </div>
                  <div class="table-col user-col">
                    <span class="cell-content">
                      {{ log.username || 'Аноним' }}
                      <span
                        v-if="log.user_id"
                        class="user-id"
                      >(ID: {{ log.user_id }})</span>
                    </span>
                  </div>
                  <div class="table-col duration-col">
                    <span class="cell-content">
                      {{ formatDuration(log) }}
                    </span>
                  </div>
                </div>
              </div>

              <div class="table-footer">
                <div class="pagination-controls">
                  <button
                    :disabled="pagination.page <= 1"
                    class="pagination-btn"
                    @click="prevPage"
                  >
                    &larr;
                  </button>
                  <span class="page-info">
                    Страница {{ pagination.page }} из {{ totalPages }}
                  </span>
                  <button
                    :disabled="pagination.page >= totalPages"
                    class="pagination-btn"
                    @click="nextPage"
                  >
                    &rarr;
                  </button>
                  <select
                    v-model="perPage"
                    class="page-size-select"
                    @change="changePageSize"
                  >
                    <option value="20">
                      20
                    </option>
                    <option value="50">
                      50
                    </option>
                    <option value="100">
                      100
                    </option>
                  </select>
                </div>
                <span class="items-count">
                  Показано {{ logs.length }} из {{ pagination.total || 0 }} записей
                </span>
              </div>
            </div>
          </div>

          <div
            class="log-details-section"
            :class="{'with-details': selectedLog}"
          >
            <div
              v-if="selectedLog"
              class="log-details-content"
            >
              <div class="details-header">
                <h3 class="details-title">
                  Детали запроса
                </h3>
                <button
                  class="close-details-btn"
                  @click="selectedLog = null"
                >
                  &times;
                </button>
              </div>

              <div class="details-body">
                <div class="details-grid">
                  <div class="detail-group">
                    <label class="detail-label">ID запроса:</label>
                    <span class="detail-value">{{ selectedLog.id }}</span>
                  </div>

                  <div class="detail-group">
                    <label class="detail-label">Время:</label>
                    <span class="detail-value">{{ formatFullDate(selectedLog.created_at) }}</span>
                  </div>

                  <div class="detail-group">
                    <label class="detail-label">Метод:</label>
                    <span
                      class="detail-value method-value"
                      :class="getMethodClass(selectedLog.method)"
                    >
                      {{ selectedLog.method }}
                    </span>
                  </div>

                  <div class="detail-group">
                    <label class="detail-label">URL:</label>
                    <span class="detail-value path-value">
                      {{ selectedLog.url }}
                    </span>
                  </div>

                  <div class="detail-group">
                    <label class="detail-label">Статус ответа:</label>
                    <span
                      class="detail-value status-value"
                      :class="getStatusClass(selectedLog.response_status)"
                    >
                      {{ selectedLog.response_status || 'N/A' }}
                    </span>
                  </div>

                  <div class="detail-group">
                    <label class="detail-label">Время выполнения:</label>
                    <span class="detail-value">{{ formatDuration(selectedLog) }}</span>
                  </div>

                  <div class="detail-group">
                    <label class="detail-label">Пользователь:</label>
                    <span class="detail-value">
                      {{ selectedLog.username || 'Аноним' }}
                      <span
                        v-if="selectedLog.user_id"
                        class="user-id"
                      >(ID: {{ selectedLog.user_id }})</span>
                    </span>
                  </div>

                  <div
                    v-if="selectedLog.headers"
                    class="detail-group"
                  >
                    <label class="detail-label">Заголовки:</label>
                    <pre class="detail-value code-block">{{ formatJson(selectedLog.headers) }}</pre>
                  </div>

                  <div
                    v-if="selectedLog.request_body"
                    class="detail-group"
                  >
                    <label class="detail-label">Тело запроса:</label>
                    <pre class="detail-value code-block request-body">{{ formatJson(selectedLog.request_body) }}</pre>
                  </div>

                  <div
                    v-if="selectedLog.response_body"
                    class="detail-group"
                  >
                    <label class="detail-label">Тело ответа:</label>
                    <pre class="detail-value code-block response-body">{{ formatJson(selectedLog.response_body) }}</pre>
                  </div>
                </div>
              </div>
            </div>
            <div
              v-else
              class="no-selection-message"
            >
              <p>Выберите запрос для просмотра деталей</p>
            </div>
          </div>
        </div>
      </div>

      <div
        v-show="activeTab === 'analytics'"
        class="analytics-tab"
      >
        <div class="analytics-toolbar">
          <label class="period-field">С даты
            <input
              v-model="historyFrom"
              type="date"
              class="date-input"
            >
          </label>
          <label class="period-field">По дату
            <input
              v-model="historyTo"
              type="date"
              class="date-input"
            >
          </label>
          <button
            class="apply-btn"
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

        <div class="kpi-row">
          <div class="kpi">
            <div class="kpi-val">
              {{ formatNum(history.totals.requests) }}
            </div>
            <div class="kpi-lab">
              Запросов за период
            </div>
          </div>
          <div class="kpi">
            <div
              class="kpi-val"
              :class="{ bad: history.totals.error_rate > 1 }"
            >
              {{ history.totals.error_rate.toFixed(2) }}%
            </div>
            <div class="kpi-lab">
              Доля ошибок
            </div>
          </div>
          <div class="kpi">
            <div class="kpi-val">
              {{ formatMs(history.totals.avg_duration_ms) }}
            </div>
            <div
              class="kpi-lab"
              title="Средняя взвешена по числу запросов. Долгоживущие подписки на события в неё не входят: у них в журнале записано время жизни соединения."
            >
              Средн. длительность
            </div>
          </div>
          <div class="kpi">
            <div class="kpi-val">
              {{ formatNum(history.totals.errors) }}
            </div>
            <div class="kpi-lab">
              Ошибок всего
            </div>
          </div>
        </div>

        <div class="analytics-panel">
          <h4 class="panel-title">
            Запросов по дням
          </h4>
          <div
            v-if="history.daily.length"
            class="bars"
          >
            <div
              v-for="d in history.daily"
              :key="d.day"
              class="bar-col"
              :title="`${d.day}: ${d.requests} запросов, ${d.errors} ошибок`"
            >
              <div
                class="bar"
                :style="{ height: barHeight(d.requests) + '%' }"
              />
            </div>
          </div>
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

<script>
import { apiRequest, apiRequestRaw } from '@/api/client'
import { formatLogin } from '@/utils/formatName';
import { useDeletionsStore } from '@/stores/deletions'
import SearchComponent from '@/components/SearchComponent.vue'
import RealTimeChart from '@/components/RealTimeChart.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import RefreshButton from '@/components/RefreshButton.vue'
import AdminPageShell from '@/views/admin/AdminPageShell.vue'
import AppIcon from '@/components/icons/AppIcon.vue';
import { SORTABLE_COLUMNS, journalStateFromQuery, mergeJournalQuery } from '@/utils/requestLogsQuery';
import {
  formatMs, formatDuration, formatDay, formatNum, formatTime,
  formatFullDate, truncatePath, formatJson, getMethodClass, getStatusClass
} from '@/utils/requestLogsFormat';


export default {
  name: 'RequestsView',
  components: {
    SearchComponent,
    RealTimeChart,
    LoaderSpinner,
    RefreshButton,
    AdminPageShell,
    AppIcon,
  },
  data() {
    return {
      activeTab: 'journal',
      history: {
        totals: { requests: 0, errors: 0, error_rate: 0, avg_duration_ms: 0 },
        coverage: null,
        daily: [],
        top_endpoints: [],
        top_users: []
      },
      historyFrom: '',
      historyTo: '',
      historyLoaded: false,
      logs: [],
      users: [],
      selectedLog: null,
      searchQuery: '',
      filterMethod: '',
      filterStatus: '',
      filterUser: '',
      filterStartDate: '',
      filterEndDate: '',
      sortField: 'created_at',
      sortDirection: 'desc',
      sortableColumns: SORTABLE_COLUMNS,
      // Номер последнего запроса списка. Список дёргают фильтр, сортировка,
      // страница и обновление подряд, а отвечают они не в том порядке, в каком
      // ушли: без номера медленный ответ прошлого фильтра затирает свежий.
      logsSeq: 0,
      pagination: {
        page: 1,
        per_page: 20,
        total: 0
      },
      perPage: '20',
      isLoading: false,
      isExporting: false,
      stats: {
        total: 0,
        today: 0,
        avg_duration: 0,
        median_duration: 0,
        p95_duration: 0,
        error_rate: 0,
        requests_per_minute: 0
      },
      realtime: {
        last_second_count: 0,
        last_minute_count: 0
      },
      timelineData: [],
      realtimeInterval: null,
      timelineInterval: null,
      chartPeriod: 'last-24h',
      chartPeriods: [
        { key: 'last-1m',    label: 'Минута',    title: 'за последнюю минуту',    interval: 1,      limit: 60, intervalHuman: '1 секунда',  xAxisLabel: 'с' },
        { key: 'last-10m',   label: '10 минут',  title: 'за последние 10 минут',  interval: 10,     limit: 60, intervalHuman: '10 секунд',  xAxisLabel: '10с' },
        { key: 'last-30m',   label: '30 минут',  title: 'за последние 30 минут',  interval: 30,     limit: 60, intervalHuman: '30 секунд',  xAxisLabel: '30с' },
        { key: 'last-1h',    label: '1 час',     title: 'за последний час',       interval: 60,     limit: 60, intervalHuman: '1 минута',   xAxisLabel: 'мин' },
        { key: 'last-24h',   label: '24 часа',   title: 'за последние 24ч',       interval: 3600,   limit: 24, intervalHuman: '1 час',      xAxisLabel: 'ч' },
        { key: 'last-week',  label: 'Неделя',    title: 'за последнюю неделю',    interval: 21600,  limit: 28, intervalHuman: '6 часов',    xAxisLabel: '6ч' },
        { key: 'last-month', label: 'Месяц',     title: 'за последний месяц',     interval: 86400,  limit: 30, intervalHuman: '1 сутки',    xAxisLabel: 'сут' },
        { key: 'last-year',  label: 'Год',       title: 'за последний год',       interval: 604800, limit: 52, intervalHuman: '1 неделя',   xAxisLabel: 'нед' }
      ]
    };
  },
  computed: {
    totalPages() {
      const perPage = this.pagination.per_page || 20;
      return Math.max(1, Math.ceil((this.pagination.total || 0) / perPage));
    },
    selectedPeriod() {
      return this.chartPeriods.find(p => p.key === this.chartPeriod) || this.chartPeriods[4];
    },
    /**
     * Что именно показано на вкладке: запрошенный период, сутки с записями и
     * источник чисел. Раньше пустой месяц выглядел как «запросов не было», хотя
     * данные просто ещё не свёрнуты.
     * @returns {string}
     */
    coverageNote() {
      const c = this.history.coverage;
      if (!c || !c.requested_from) return '';

      const period = `${this.formatDay(c.requested_from)} - ${this.formatDay(c.requested_to)}`;
      if (!c.days) {
        return c.aggregated_through
          ? `За период ${period} записей нет.`
          : `За период ${period} записей нет: журнал ещё не сворачивался в суточные итоги.`;
      }

      const sources = {
        aggregates: 'по свёрнутым итогам суток',
        detailed: 'по подробным записям журнала',
        mixed: 'по свёрнутым итогам и подробным записям'
      };
      const covered = c.from === c.to
        ? this.formatDay(c.from)
        : `${this.formatDay(c.from)} - ${this.formatDay(c.to)}`;
      const source = sources[c.source] ? `, ${sources[c.source]}` : '';
      return `Запрошен период ${period}. Записи есть за ${covered}, суток с данными: ${c.days}${source}.`;
    },
    /**
     * Оговорка про перцентиль. Пустая, когда весь период посчитан по самим
     * записям и p95 честный.
     * @returns {string}
     */
    p95Note() {
      const c = this.history.coverage;
      if (!c || !c.days || c.exact_p95) return '';
      return 'За свёрнутые сутки показано наибольшее суточное значение: отдельных длительностей у них уже нет.';
    }
  },
  created() {
    this.applyQueryToState();
  },
  async mounted() {
    await Promise.all([
      this.fetchLogs(),
      this.fetchStats(),
      this.fetchTimeline(),
      this.fetchUsers(),
      this.fetchRealtime()
    ]);
    this.startPolling();
  },
  beforeUnmount() {
    this.stopPolling();
  },
  methods: {
    formatLogin,
    formatMs,
    formatDuration,
    formatDay,
    formatNum,
    formatTime,
    formatFullDate,
    truncatePath,
    formatJson,
    getMethodClass,
    getStatusClass,

    switchToAnalytics() {
      this.activeTab = 'analytics'
      if (!this.historyLoaded) {
        this.fetchHistory()
      }
    },
    async fetchHistory() {
      this.isLoading = true
      try {
        const params = new URLSearchParams()
        if (this.historyFrom) params.set('from_date', this.historyFrom)
        if (this.historyTo) params.set('to_date', this.historyTo)
        const qs = params.toString()
        const response = await apiRequest(`/request-logs/history${qs ? '?' + qs : ''}`)
        if (response.ok) {
          const data = await response.json()
          if (data) {
            this.history = {
              totals: data.totals || { requests: 0, errors: 0, error_rate: 0, avg_duration_ms: 0 },
              coverage: data.coverage || null,
              daily: data.daily || [],
              top_endpoints: data.top_endpoints || [],
              top_users: data.top_users || []
            }
            this.historyLoaded = true
          }
        }
      } catch (error) {
        console.error('Error fetching history:', error)
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'аналитику', type: 'error' })
      } finally {
        this.isLoading = false
      }
    },
    barHeight(value) {
      const max = Math.max(...this.history.daily.map(d => d.requests), 1)
      return Math.max(4, Math.round((value / max) * 100))
    },
    buildFilterParams() {
      const params = {};
      if (this.searchQuery) params.search = this.searchQuery;
      if (this.filterMethod) params.method = this.filterMethod;
      if (this.filterStatus) params.status = this.filterStatus;
      if (this.filterUser) params.user_id = this.filterUser;
      if (this.filterStartDate) params.from_date = this.filterStartDate;
      if (this.filterEndDate) params.to_date = this.filterEndDate;
      return params;
    },

    async fetchLogs() {
      const seq = ++this.logsSeq;
      this.syncQueryFromState();
      this.isLoading = true;
      try {
        const params = new URLSearchParams({
          page: this.pagination.page,
          per_page: this.pagination.per_page,
          sort: this.sortField,
          order: this.sortDirection,
          ...this.buildFilterParams()
        });

        const response = await apiRequestRaw(`/request-logs?${params}`);
        if (seq !== this.logsSeq) return;

        if (response.ok) {
          const body = await response.json();
          if (seq !== this.logsSeq) return;
          if (body && body.success) {
            this.logs = body.data || [];
            if (body.meta) {
              this.pagination.total = body.meta.total || 0;
              this.pagination.page = body.meta.page || 1;
              this.pagination.per_page = body.meta.per_page || 20;
            }
          }
        }
      } catch (error) {
        if (seq !== this.logsSeq) return;
        console.error('Error fetching logs:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'логи', type: 'error' });
      } finally {
        if (seq === this.logsSeq) {
          this.isLoading = false;
        }
      }
    },




    /**
     * Читает отбор из адресной строки: присланная ссылка открывает тот же
     * экран, а обновление страницы его не теряет.
     */
    applyQueryToState() {
      const state = journalStateFromQuery(this.$route?.query || {});
      this.searchQuery = state.search;
      this.filterMethod = state.method;
      this.filterStatus = state.status;
      this.filterUser = state.user;
      this.filterStartDate = state.from;
      this.filterEndDate = state.to;
      this.sortField = state.sort;
      this.sortDirection = state.order;
      this.pagination.page = state.page;
      this.pagination.per_page = state.perPage;
      this.perPage = String(state.perPage);
    },

    /**
     * Текущий отбор в том виде, в каком его читают утилиты адреса.
     * @returns {object}
     */
    journalState() {
      return {
        search: this.searchQuery,
        method: this.filterMethod,
        status: this.filterStatus,
        user: this.filterUser,
        from: this.filterStartDate,
        to: this.filterEndDate,
        sort: this.sortField,
        order: this.sortDirection,
        page: this.pagination.page,
        perPage: this.pagination.per_page
      };
    },

    syncQueryFromState() {
      if (!this.$router) return;
      const next = mergeJournalQuery(this.$route?.query || {}, this.journalState());
      if (next) this.$router.replace({ query: next }).catch(() => {});
    },

    async fetchStats() {
      try {
        const response = await apiRequest('/request-logs/stats');
        if (response.ok) {
          const data = await response.json();
          if (data) {
            this.stats = data;
          }
        }
      } catch (error) {
        console.error('Error fetching stats:', error);
      }
    },

    async fetchRealtime() {
      try {
        const response = await apiRequest('/request-logs/realtime');
        if (response.ok) {
          const data = await response.json();
          if (data) {
            this.realtime = data;
          }
        }
      } catch (error) {
        console.error('Error fetching realtime:', error);
      }
    },

    async fetchTimeline() {
      try {
        const p = this.selectedPeriod;
        const params = new URLSearchParams({
          interval: String(p.interval),
          limit: String(p.limit)
        });
        const response = await apiRequest(`/request-logs/timeline?${params}`);
        if (response.ok) {
          const data = await response.json();
          if (Array.isArray(data)) {
            this.timelineData = data;
          }
        }
      } catch (error) {
        console.error('Error fetching timeline:', error);
      }
    },

    onChartPeriodChange() {
      this.fetchTimeline();
    },

    async fetchUsers() {
      try {
        const response = await apiRequest('/request-logs/users');
        if (response.ok) {
          const data = await response.json();
          if (Array.isArray(data)) {
            this.users = data;
          }
        }
      } catch (error) {
        console.error('Error fetching users:', error);
      }
    },

    async exportLogs() {
      this.isExporting = true;
      try {
        // Порядок тот же, что на экране: выгрузка «самых медленных» должна
        // начинаться с самых медленных, а не с последних по времени.
        const params = new URLSearchParams({
          sort: this.sortField,
          order: this.sortDirection,
          ...this.buildFilterParams()
        });
        const response = await apiRequest(`/request-logs/export?${params}`);
        if (response.ok) {
          const text = await response.text();
          const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `request-logs-${new Date().toISOString().slice(0, 10)}.txt`;
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          URL.revokeObjectURL(url);
          useDeletionsStore().notify({ bold: 'Экспорт завершён', type: 'success' });
        } else {
          useDeletionsStore().notify({ prefix: 'Не удалось выполнить ', bold: 'экспорт', type: 'error' });
        }
      } catch (error) {
        console.error('Error exporting logs:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить ', bold: 'экспорт', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },

    refreshLogs() {
      this.pagination.page = 1;
      this.fetchLogs();
      this.fetchStats();
    },

    startPolling() {
      this.realtimeInterval = setInterval(() => {
        this.fetchRealtime();
      }, 5000);

      this.timelineInterval = setInterval(() => {
        this.fetchTimeline();
        this.fetchStats();
      }, 30000);
    },

    stopPolling() {
      if (this.realtimeInterval) {
        clearInterval(this.realtimeInterval);
        this.realtimeInterval = null;
      }
      if (this.timelineInterval) {
        clearInterval(this.timelineInterval);
        this.timelineInterval = null;
      }
    },

    /**
     * Порядок строк задаёт сервер: в списке одна страница, и перестановка её на
     * месте показывала бы «самые медленные» только среди двадцати видимых.
     * Раньше клик переставлял стрелку и не трогал строки вообще.
     * @param {string} field
     */
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
      this.pagination.page = 1;
      this.fetchLogs();
    },

    selectLog(log) {
      this.selectedLog = log;
    },

    prevPage() {
      if (this.pagination.page > 1) {
        this.pagination.page--;
        this.fetchLogs();
      }
    },

    nextPage() {
      if (this.pagination.page < this.totalPages) {
        this.pagination.page++;
        this.fetchLogs();
      }
    },

    changePageSize() {
      this.pagination.per_page = parseInt(this.perPage);
      this.pagination.page = 1;
      this.fetchLogs();
    },

    clearFilters() {
      this.searchQuery = '';
      this.filterMethod = '';
      this.filterStatus = '';
      this.filterUser = '';
      this.filterStartDate = '';
      this.filterEndDate = '';
      this.pagination.page = 1;
      this.fetchLogs();
      this.fetchStats();
    },









  }
};
</script>

<style scoped>
.requests-view {
  background: var(--surface);
  border-radius: 16px;
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

.chart-section {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.chart-title {
  margin: 0;
  font-size: 0.95em;
  font-weight: 600;
  color: var(--text);
}

.chart-interval {
  font-size: 0.8em;
  color: var(--text-muted);
}

.chart-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chart-period-select {
  padding: 4px 28px 4px 12px;
  border: 1px solid var(--border);
  border-radius: 50px;
  font-size: 12px;
  background: var(--surface);
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml;charset=UTF-8,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%234F5BDF' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3e%3cpolyline points='6 9 12 15 18 9'%3e%3c/polyline%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 8px center;
  background-size: 12px;
  color: var(--text);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.chart-period-select:hover {
  border-color: var(--accent);
}

.chart-period-select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.15);
}

.filters-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  flex-wrap: wrap;
}

.filter-controls {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.filter-select,
.date-input {
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-size: 13px;
  font-family: inherit;
  background: var(--surface);
  color: var(--text);
  min-width: 130px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.date-input {
  width: 150px;
  min-width: auto;
}

.filter-select:hover,
.date-input:hover {
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.filter-select:focus,
.date-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.12);
}

.clear-filters-btn,
.export-btn {
  padding: 8px 14px;
  border: 1px solid transparent;
  border-radius: 999px;
  font-family: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.clear-filters-btn {
  background: var(--surface);
  color: var(--accent-text);
  border-color: var(--accent);
}

.clear-filters-btn:hover {
  background: var(--accent-tint);
}

.export-btn {
  background: var(--accent);
  color: var(--accent-contrast);
}

.export-btn:hover {
  background: var(--accent-hover);
}

.export-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.content-container {
  display: flex;
  height: 500px;
  width: 100%;
}

.logs-table-section {
  width: 65%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
}

.table-container {
  background: var(--surface);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: var(--text);
}

.header-col:hover .sort-icon {
  color: var(--text);
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  color: var(--text);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 600 !important;
}

.time-col { width: 12%; min-width: 80px; }
.method-col { width: 8%; min-width: 70px; }
.path-col { width: 30%; min-width: 150px; }
.status-col { width: 8%; min-width: 70px; }
.user-col { width: 20%; min-width: 120px; }
.duration-col { width: 10%; min-width: 80px; }

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 500px;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 40px;
  font-size: 12px;
}

.table-row:hover {
  background-color: var(--surface-2);
}

.table-row.selected {
  background-color: var(--accent-tint);
}

.table-row.error-row {
  background-color: var(--danger-bg);
}

.table-row.success-row {
  background-color: var(--success-bg);
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.method-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-align: center;
  min-width: 50px;
}

.method-get { background: #10b981; color: white; }
.method-post { background: #3b82f6; color: white; }
.method-put { background: #f59e0b; color: white; }
.method-delete { background: #ef4444; color: white; }
.method-patch { background: #8b5cf6; color: white; }
.method-other { background: #6b7280; color: white; }

.status-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-align: center;
  min-width: 40px;
}

.status-success { background: #10b981; color: white; }
.status-redirect { background: #3b82f6; color: white; }
.status-client-error { background: #f59e0b; color: white; }
.status-server-error { background: #ef4444; color: white; }
.status-unknown { background: #6b7280; color: white; }

.user-id {
  font-size: 10px;
  color: var(--text-muted);
  margin-left: 4px;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.table-footer {
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--accent-tint);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pagination-btn {
  padding: 4px 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}

.pagination-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.page-info {
  font-size: 12px;
  color: var(--text-muted);
}

.page-size-select {
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-size: 12px;
}

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.log-details-section {
  width: 35%;
  overflow-y: auto;
  background: var(--surface-2);
  border-left: 1px solid var(--border);
}

.log-details-content {
  padding: 20px;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.1em;
  font-weight: 600;
}

.close-details-btn {
  background: none;
  border: none;
  font-size: 1.5em;
  cursor: pointer;
  color: var(--text-muted);
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
}

.close-details-btn:hover {
  background-color: var(--border);
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.details-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-size: 0.8em;
  color: var(--text-muted);
  font-weight: 500;
}

.detail-value {
  font-size: 0.9em;
  color: var(--text);
  word-break: break-all;
}

.method-value, .status-value {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}

.path-value {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background: var(--surface-2);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.85em;
}

.code-block {
  background: var(--surface-2);
  padding: 8px;
  border-radius: 6px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.85em;
  overflow-x: auto;
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.request-body {
  background: var(--info-bg);
  border-left: 3px solid var(--info);
}

.response-body {
  background: var(--success-bg);
  border-left: 3px solid var(--success);
}

.no-selection-message {
  color: var(--text-muted);
  font-weight: 400;
  font-size: 14px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
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

@media (max-width: 1200px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }

  .logs-table-section,
  .log-details-section {
    width: 100% !important;
  }

  .logs-table-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
    height: 400px;
  }
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

  .filters-bar {
    flex-direction: column;
    align-items: stretch;
    padding: 12px 16px;
  }

  .filter-controls {
    flex-wrap: wrap;
  }

  .filter-select,
  .date-input {
    flex: 1;
    min-width: auto;
  }

  .table-header,
  .table-row {
    padding: 0 16px;
  }

  .time-col { width: 15%; }
  .method-col { width: 10%; }
  .path-col { width: 35%; }
  .status-col { width: 10%; }
  .user-col { width: 20%; }
  .duration-col { width: 10%; }

  .table-footer {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }

  .pagination-controls {
    justify-content: center;
  }

  .log-details-content {
    padding: 16px;
  }

  .code-block {
    font-size: 0.75em;
  }

  .chart-section {
    padding: 12px 16px;
  }
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

/* Вкладка аналитики */
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
.period-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--text-muted);
}
.apply-btn {
  border: 1px solid var(--accent);
  background: var(--accent);
  color: var(--accent-contrast);
  border-radius: 999px;
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}
.apply-btn:hover {
  background: var(--accent-hover);
}

.kpi-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}
.kpi {
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 15px;
  padding: 12px 14px;
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
.kpi-lab {
  font-size: 0.72em;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: var(--text-muted);
  margin-top: 4px;
  font-weight: 600;
}

.analytics-panel {
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 14px;
  margin-bottom: 16px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
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

.bars {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 130px;
}
.bar-col {
  flex: 1;
  display: flex;
  align-items: flex-end;
  height: 100%;
}
.bar {
  width: 100%;
  background: linear-gradient(180deg, color-mix(in srgb, var(--accent) 75%, var(--surface)), var(--accent));
  border-radius: 4px 4px 0 0;
  min-height: 4px;
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
  .kpi-row {
    grid-template-columns: repeat(2, 1fr);
  }
  .analytics-tables {
    grid-template-columns: 1fr;
  }
}
</style>
