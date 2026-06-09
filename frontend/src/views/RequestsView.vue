<template>
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
            <span class="stat-label">Среднее время:</span>
            <span class="stat-value">{{ Math.round(stats.avg_duration || 0) }}мс</span>
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
            {{ u.username }}
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
      <button
        class="refresh-button"
        @click="refreshLogs"
      >
        <img
          src="@/assets/icons/refresh.png"
          class="refresh-icon"
        >
      </button>
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
              class="header-col time-col"
              @click="sortBy('created_at')"
            >
              <p :class="{ 'active-sort': sortField === 'created_at' }">
                Время
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'created_at',
                  'desc': sortField === 'created_at' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col method-col"
              @click="sortBy('method')"
            >
              <p :class="{ 'active-sort': sortField === 'method' }">
                Метод
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'method',
                  'desc': sortField === 'method' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col path-col"
              @click="sortBy('url')"
            >
              <p :class="{ 'active-sort': sortField === 'url' }">
                URL
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'url',
                  'desc': sortField === 'url' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col status-col"
              @click="sortBy('response_status')"
            >
              <p :class="{ 'active-sort': sortField === 'response_status' }">
                Статус
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'response_status',
                  'desc': sortField === 'response_status' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col user-col"
              @click="sortBy('username')"
            >
              <p :class="{ 'active-sort': sortField === 'username' }">
                Пользователь
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'username',
                  'desc': sortField === 'username' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col duration-col"
              @click="sortBy('duration_ms')"
            >
              <p :class="{ 'active-sort': sortField === 'duration_ms' }">
                Время
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'duration_ms',
                  'desc': sortField === 'duration_ms' && sortDirection === 'desc'
                }"
              >
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
                  {{ log.duration_ms || 0 }}мс
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
                <span class="detail-value">{{ selectedLog.duration_ms || 0 }}мс</span>
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
</template>

<script>
import { apiRequest, apiRequestRaw } from '@/api/client'
import SearchComponent from '@/components/SearchComponent.vue'
import RealTimeChart from '@/components/RealTimeChart.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'

export default {
  name: 'RequestsView',
  components: {
    SearchComponent,
    RealTimeChart,
    LoaderSpinner
  },
  data() {
    return {
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
    }
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
      this.isLoading = true;
      try {
        const params = new URLSearchParams({
          page: this.pagination.page,
          per_page: this.pagination.per_page,
          ...this.buildFilterParams()
        });

        const response = await apiRequestRaw(`/request-logs?${params}`);

        if (response.ok) {
          const body = await response.json();
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
        console.error('Error fetching logs:', error);
        this.showNotification('Ошибка при загрузке логов', 'error');
      } finally {
        this.isLoading = false;
      }
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
        const params = new URLSearchParams(this.buildFilterParams());
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
          this.showNotification('Экспорт завершён', 'success');
        } else {
          this.showNotification('Ошибка экспорта', 'error');
        }
      } catch (error) {
        console.error('Error exporting logs:', error);
        this.showNotification('Ошибка экспорта', 'error');
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

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
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

    formatTime(timestamp) {
      if (!timestamp) return '';
      const date = new Date(timestamp);
      return date.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    },

    formatFullDate(timestamp) {
      if (!timestamp) return '';
      const date = new Date(timestamp);
      return date.toLocaleString('ru-RU', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    },

    truncatePath(path) {
      if (!path) return '';
      if (path.length > 40) {
        return path.substring(0, 37) + '...';
      }
      return path;
    },

    getMethodClass(method) {
      const classes = {
        'GET': 'method-get',
        'POST': 'method-post',
        'PUT': 'method-put',
        'DELETE': 'method-delete',
        'PATCH': 'method-patch'
      };
      return classes[method] || 'method-other';
    },

    getStatusClass(status) {
      if (!status) return 'status-unknown';
      if (status < 300) return 'status-success';
      if (status < 400) return 'status-redirect';
      if (status < 500) return 'status-client-error';
      return 'status-server-error';
    },

    formatJson(text) {
      if (!text) return '';
      try {
        const obj = JSON.parse(text);
        return JSON.stringify(obj, null, 2);
      } catch {
        return text;
      }
    },

    showNotification(message, type = 'info') {
      const notification = document.createElement('div');
      notification.className = `notification ${type}`;
      notification.textContent = message;
      notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 12px 20px;
        border-radius: 8px;
        color: white;
        font-weight: 500;
        z-index: 1000;
      `;

      if (type === 'success') notification.style.backgroundColor = '#10b981';
      if (type === 'error') notification.style.backgroundColor = '#ef4444';
      if (type === 'warning') notification.style.backgroundColor = '#f59e0b';
      if (type === 'info') notification.style.backgroundColor = '#3b82f6';

      document.body.appendChild(notification);

      setTimeout(() => {
        notification.remove();
      }, 3000);
    }
  }
};
</script>

<style scoped>
.requests-view {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  min-height: 80vh;
}

.management-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
  border-bottom: 1px solid #e6e6e6;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: #000;
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
  color: #666;
}

.stat-value {
  font-size: 0.9em;
  font-weight: 600;
  color: #333;
}

.stat-value.stat-error {
  color: #ef4444;
}

.live-value {
  color: #10b981;
}

.stat-minute {
  font-size: 0.85em;
  color: #666;
  font-weight: 500;
  margin-left: 4px;
}

.realtime-stat {
  padding-left: 12px;
  border-left: 1px solid #e6e6e6;
}

.chart-section {
  padding: 16px 20px;
  border-bottom: 1px solid #e6e6e6;
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
  color: #333;
}

.chart-interval {
  font-size: 0.8em;
  color: #a2a2a2;
}

.chart-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chart-period-select {
  padding: 4px 28px 4px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  font-size: 12px;
  background: white;
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml;charset=UTF-8,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%234F5BDF' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3e%3cpolyline points='6 9 12 15 18 9'%3e%3c/polyline%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 8px center;
  background-size: 12px;
  color: #333;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.chart-period-select:hover {
  border-color: #4F5BDF;
}

.chart-period-select:focus {
  outline: none;
  border-color: #4F5BDF;
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.15);
}

.filters-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid #e6e6e6;
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
  border: 1px solid #e0e0e0;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-family: inherit;
  background: #fff;
  color: #1a1a1a;
  min-width: 130px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.date-input {
  width: 150px;
  min-width: auto;
}

.filter-select:hover,
.date-input:hover {
  border-color: #c5c9d6;
}

.filter-select:focus,
.date-input:focus {
  outline: none;
  border-color: #4F5BDF;
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.12);
}

.refresh-button {
  background: none;
  border: 1px solid transparent;
  cursor: pointer;
  padding: 6px;
  border-radius: 10px;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.refresh-button:hover {
  background-color: #eef0ff;
  border-color: #4F5BDF;
}

.refresh-icon {
  width: 18px;
  height: 18px;
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
  background: #fff;
  color: #4F5BDF;
  border-color: #4F5BDF;
}

.clear-filters-btn:hover {
  background: #eef0ff;
}

.export-btn {
  background: #4F5BDF;
  color: #fff;
}

.export-btn:hover {
  background: #3d49c7;
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
  border-right: 1px solid #e6e6e6;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 12px;
  color: #a2a2a2;
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
  color: #333;
}

.header-col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #333 !important;
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
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 40px;
  font-size: 12px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f0f2ff;
}

.table-row.error-row {
  background-color: #fff5f5;
}

.table-row.success-row {
  background-color: #f0fff4;
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
  color: #666;
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
  border-top: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #f8fafc;
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pagination-btn {
  padding: 4px 8px;
  background: #fff;
  border: 1px solid #e6e6e6;
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
  color: #666;
}

.page-size-select {
  padding: 4px 8px;
  border: 1px solid #e6e6e6;
  border-radius: var(--radius-md);
  font-size: 12px;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.log-details-section {
  width: 35%;
  overflow-y: auto;
  background: #fafafa;
  border-left: 1px solid #e6e6e6;
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
  color: #000;
  font-size: 1.1em;
  font-weight: 600;
}

.close-details-btn {
  background: none;
  border: none;
  font-size: 1.5em;
  cursor: pointer;
  color: #666;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
}

.close-details-btn:hover {
  background-color: #f0f0f0;
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
  color: #a2a2a2;
  font-weight: 500;
}

.detail-value {
  font-size: 0.9em;
  color: #333;
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
  background: #f5f5f5;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.85em;
}

.code-block {
  background: #f5f5f5;
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
  background: #f0f9ff;
  border-left: 3px solid #3b82f6;
}

.response-body {
  background: #f0fff4;
  border-left: 3px solid #10b981;
}

.no-selection-message {
  color: #a2a2a2;
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
  background: rgba(255, 255, 255, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid #f3f3f3;
  border-top: 3px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
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
    border-bottom: 1px solid #e6e6e6;
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
</style>
