<template>
  <div class="requests-view dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Мониторинг запросов</h3>
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
            <span class="stat-value" :class="{ 'stat-error': stats.error_rate > 5 }">
              {{ (stats.error_rate || 0).toFixed(1) }}%
            </span>
          </span>
          <span class="stat-item">
            <span class="stat-label">RPM:</span>
            <span class="stat-value">{{ (stats.requests_per_minute || 0).toFixed(1) }}</span>
          </span>
          <span class="stat-item realtime-stat" v-if="realtime.last_minute_count != null">
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
        <h4 class="chart-title">Запросы за последние 24ч</h4>
        <span class="chart-interval">интервал: 1 час</span>
      </div>
      <RealTimeChart
        :data="timelineData"
        :height="180"
        color="#4F5BDF"
        intervalLabel="ч"
      />
    </div>

    <div class="filters-bar">
      <SearchComponent
        :title="'Поиск по логам...'"
        v-model="searchQuery"
        @keyup.enter="refreshLogs"
      />
      <div class="filter-controls">
        <select v-model="filterMethod" @change="refreshLogs" class="filter-select">
          <option value="">Все методы</option>
          <option value="GET">GET</option>
          <option value="POST">POST</option>
          <option value="PUT">PUT</option>
          <option value="DELETE">DELETE</option>
          <option value="PATCH">PATCH</option>
        </select>
        <select v-model="filterStatus" @change="refreshLogs" class="filter-select">
          <option value="">Все статусы</option>
          <option value="200">200 OK</option>
          <option value="400">400 Bad Request</option>
          <option value="401">401 Unauthorized</option>
          <option value="403">403 Forbidden</option>
          <option value="404">404 Not Found</option>
          <option value="500">500 Server Error</option>
        </select>
        <select v-model="filterUser" @change="refreshLogs" class="filter-select">
          <option value="">Все пользователи</option>
          <option v-for="u in users" :key="u.id" :value="u.id">
            {{ u.username }}
          </option>
        </select>
        <input
          type="date"
          v-model="filterStartDate"
          @change="refreshLogs"
          class="date-input"
        />
        <input
          type="date"
          v-model="filterEndDate"
          @change="refreshLogs"
          class="date-input"
        />
      </div>
      <button @click="refreshLogs" class="refresh-button">
        <img src="@/assets/icons/refresh.png" class="refresh-icon" />
      </button>
      <button @click="clearFilters" class="clear-filters-btn">
        Сбросить
      </button>
      <button @click="exportLogs" class="export-btn" :disabled="isExporting">
        {{ isExporting ? 'Экспорт...' : 'Экспорт' }}
      </button>
    </div>

    <div class="content-container">
      <div class="logs-table-section">
        <div class="table-container">
          <div class="table-header">
            <div class="header-col time-col" @click="sortBy('created_at')">
              <p :class="{ 'active-sort': sortField === 'created_at' }">Время</p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'created_at',
                  'desc': sortField === 'created_at' && sortDirection === 'desc'
                }"
              />
            </div>
            <div class="header-col method-col" @click="sortBy('method')">
              <p :class="{ 'active-sort': sortField === 'method' }">Метод</p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'method',
                  'desc': sortField === 'method' && sortDirection === 'desc'
                }"
              />
            </div>
            <div class="header-col path-col" @click="sortBy('url')">
              <p :class="{ 'active-sort': sortField === 'url' }">URL</p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'url',
                  'desc': sortField === 'url' && sortDirection === 'desc'
                }"
              />
            </div>
            <div class="header-col status-col" @click="sortBy('response_status')">
              <p :class="{ 'active-sort': sortField === 'response_status' }">Статус</p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'response_status',
                  'desc': sortField === 'response_status' && sortDirection === 'desc'
                }"
              />
            </div>
            <div class="header-col user-col" @click="sortBy('username')">
              <p :class="{ 'active-sort': sortField === 'username' }">Пользователь</p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'username',
                  'desc': sortField === 'username' && sortDirection === 'desc'
                }"
              />
            </div>
            <div class="header-col duration-col" @click="sortBy('duration_ms')">
              <p :class="{ 'active-sort': sortField === 'duration_ms' }">Время</p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'duration_ms',
                  'desc': sortField === 'duration_ms' && sortDirection === 'desc'
                }"
              />
            </div>
          </div>

          <div class="table-body" ref="logsBody">
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
                <span class="cell-content" :title="formatFullDate(log.created_at)">
                  {{ formatTime(log.created_at) }}
                </span>
              </div>
              <div class="table-col method-col">
                <span class="method-badge" :class="getMethodClass(log.method)">
                  {{ log.method }}
                </span>
              </div>
              <div class="table-col path-col">
                <span class="truncate-text" :title="log.url">
                  {{ truncatePath(log.url) }}
                </span>
              </div>
              <div class="table-col status-col">
                <span class="status-badge" :class="getStatusClass(log.response_status)">
                  {{ log.response_status || 'N/A' }}
                </span>
              </div>
              <div class="table-col user-col">
                <span class="cell-content">
                  {{ log.username || 'Аноним' }}
                  <span v-if="log.user_id" class="user-id">(ID: {{ log.user_id }})</span>
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
                @click="prevPage"
                :disabled="pagination.page <= 1"
                class="pagination-btn"
              >
                &larr;
              </button>
              <span class="page-info">
                Страница {{ pagination.page }} из {{ totalPages }}
              </span>
              <button
                @click="nextPage"
                :disabled="pagination.page >= totalPages"
                class="pagination-btn"
              >
                &rarr;
              </button>
              <select v-model="perPage" @change="changePageSize" class="page-size-select">
                <option value="20">20</option>
                <option value="50">50</option>
                <option value="100">100</option>
              </select>
            </div>
            <span class="items-count">
              Показано {{ logs.length }} из {{ pagination.total || 0 }} записей
            </span>
          </div>
        </div>
      </div>

      <div class="log-details-section" :class="{'with-details': selectedLog}">
        <div v-if="selectedLog" class="log-details-content">
          <div class="details-header">
            <h3 class="details-title">Детали запроса</h3>
            <button @click="selectedLog = null" class="close-details-btn">
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
                <span class="detail-value method-value" :class="getMethodClass(selectedLog.method)">
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
                <span class="detail-value status-value" :class="getStatusClass(selectedLog.response_status)">
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
                  <span v-if="selectedLog.user_id" class="user-id">(ID: {{ selectedLog.user_id }})</span>
                </span>
              </div>

              <div class="detail-group" v-if="selectedLog.headers">
                <label class="detail-label">Заголовки:</label>
                <pre class="detail-value code-block">{{ formatJson(selectedLog.headers) }}</pre>
              </div>

              <div class="detail-group" v-if="selectedLog.request_body">
                <label class="detail-label">Тело запроса:</label>
                <pre class="detail-value code-block request-body">{{ formatJson(selectedLog.request_body) }}</pre>
              </div>

              <div class="detail-group" v-if="selectedLog.response_body">
                <label class="detail-label">Тело ответа:</label>
                <pre class="detail-value code-block response-body">{{ formatJson(selectedLog.response_body) }}</pre>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="no-selection-message">
          <p>Выберите запрос для просмотра деталей</p>
        </div>
      </div>
    </div>

    <div v-if="isLoading" class="loading-overlay">
      <div class="spinner"></div>
    </div>
  </div>
</template>

<script>
import { apiRequest, apiRequestRaw } from '@/api/client'
import SearchComponent from '@/components/SearchComponent.vue'
import RealTimeChart from '@/components/RealTimeChart.vue'

export default {
  name: 'RequestsView',
  components: {
    SearchComponent,
    RealTimeChart
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
      timelineInterval: null
    };
  },
  computed: {
    totalPages() {
      const perPage = this.pagination.per_page || 20;
      return Math.max(1, Math.ceil((this.pagination.total || 0) / perPage));
    }
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
        const params = new URLSearchParams({
          interval: '3600',
          limit: '24'
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

.filter-select {
  padding: 6px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  font-size: 0.85em;
  background: #fff;
  min-width: 120px;
}

.date-input {
  padding: 6px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  font-size: 0.85em;
  width: 140px;
}

.refresh-button {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  transition: background-color 0.2s;
}

.refresh-button:hover {
  background-color: #f0f0f0;
}

.refresh-icon {
  width: 20px;
  height: 20px;
}

.clear-filters-btn {
  padding: 6px 12px;
  background: #6b7280;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 0.85em;
  cursor: pointer;
  transition: background-color 0.2s;
}

.clear-filters-btn:hover {
  background: #4b5563;
}

.export-btn {
  padding: 6px 12px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 0.85em;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.export-btn:hover {
  background: #3d49c7;
}

.export-btn:disabled {
  opacity: 0.6;
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
  border-radius: 4px;
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
