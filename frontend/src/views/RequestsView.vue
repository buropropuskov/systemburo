<template>
  <div class="requests-view dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Мониторинг запросов</h3>
      <div class="header-controls">
        <div class="stats-summary">
          <span class="stat-item">
            <span class="stat-label">Всего:</span>
            <span class="stat-value">{{ stats.total_requests || 0 }}</span>
          </span>
          <span class="stat-item">
            <span class="stat-label">Среднее время:</span>
            <span class="stat-value">{{ Math.round(stats.avg_duration_ms || 0) }}мс</span>
          </span>
          <span class="stat-item">
            <span class="stat-label">Успешных:</span>
            <span class="stat-value">{{ Math.round(stats.success_rate || 0) }}%</span>
          </span>
        </div>
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
        <button 
          @click="showLive = !showLive" 
          :class="['live-button', { 'live-active': showLive }]"
        >
          {{ showLive ? 'Стоп' : 'Live' }}
        </button>
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица логов -->
      <div class="logs-table-section">
        <div class="table-container">
          <div class="table-header">
            <div class="header-col time-col" @click="sortBy('timestamp')">
              <p :class="{ 'active-sort': sortField === 'timestamp' }">Время</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'timestamp',
                  'desc': sortField === 'timestamp' && sortDirection === 'desc'
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
            <div class="header-col path-col" @click="sortBy('path')">
              <p :class="{ 'active-sort': sortField === 'path' }">Путь</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'path',
                  'desc': sortField === 'path' && sortDirection === 'desc'
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
              v-for="log in filteredLogs" 
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
                <span class="cell-content" :title="formatFullDate(log.timestamp)">
                  {{ formatTime(log.timestamp) }}
                </span>
              </div>
              <div class="table-col method-col">
                <span class="method-badge" :class="getMethodClass(log.method)">
                  {{ log.method }}
                </span>
              </div>
              <div class="table-col path-col">
                <span class="truncate-text" :title="log.path + (log.query_params ? '?' + log.query_params : '')">
                  {{ truncatePath(log.path) }}
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
                ←
              </button>
              <span class="page-info">
                Страница {{ pagination.page }} из {{ pagination.pages || 1 }}
              </span>
              <button 
                @click="nextPage" 
                :disabled="pagination.page >= pagination.pages"
                class="pagination-btn"
              >
                →
              </button>
              <select v-model="pageSize" @change="changePageSize" class="page-size-select">
                <option value="20">20</option>
                <option value="50">50</option>
                <option value="100">100</option>
                <option value="200">200</option>
              </select>
            </div>
            <span class="items-count">
              Показано {{ filteredLogs.length }} из {{ pagination.total || 0 }} записей
            </span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали лога -->
      <div class="log-details-section" :class="{'with-details': selectedLog}">
        <div v-if="selectedLog" class="log-details-content">
          <div class="details-header">
            <h3 class="details-title">Детали запроса</h3>
            <button @click="selectedLog = null" class="close-details-btn">
              ×
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
                <span class="detail-value">{{ formatFullDate(selectedLog.timestamp) }}</span>
              </div>
              
              <div class="detail-group">
                <label class="detail-label">Метод:</label>
                <span class="detail-value method-value" :class="getMethodClass(selectedLog.method)">
                  {{ selectedLog.method }}
                </span>
              </div>
              
              <div class="detail-group">
                <label class="detail-label">Путь:</label>
                <span class="detail-value path-value">
                  {{ selectedLog.path }}
                </span>
              </div>
              
              <div class="detail-group" v-if="selectedLog.query_params">
                <label class="detail-label">Query параметры:</label>
                <pre class="detail-value code-block">{{ selectedLog.query_params }}</pre>
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
              
              <div class="detail-group" v-if="selectedLog.ip_address">
                <label class="detail-label">IP адрес:</label>
                <span class="detail-value">{{ selectedLog.ip_address }}</span>
              </div>
              
              <div class="detail-group" v-if="selectedLog.user_agent">
                <label class="detail-label">User-Agent:</label>
                <span class="detail-value">{{ selectedLog.user_agent }}</span>
              </div>
              
              <div class="detail-group" v-if="selectedLog.request_body">
                <label class="detail-label">Тело запроса:</label>
                <pre class="detail-value code-block request-body">
                  {{ formatJson(selectedLog.request_body) }}
                </pre>
              </div>
              
              <div class="detail-group" v-if="selectedLog.response_body">
                <label class="detail-label">Тело ответа:</label>
                <pre class="detail-value code-block response-body">
                  {{ formatJson(selectedLog.response_body) }}
                </pre>
              </div>
              
              <div class="detail-group" v-if="selectedLog.error_message">
                <label class="detail-label">Ошибка:</label>
                <pre class="detail-value error-message">{{ selectedLog.error_message }}</pre>
              </div>
              
              <div class="detail-group" v-if="selectedLog.request_headers">
                <label class="detail-label">Заголовки запроса:</label>
                <pre class="detail-value code-block">{{ selectedLog.request_headers }}</pre>
              </div>
              
              <div class="detail-group" v-if="selectedLog.response_headers">
                <label class="detail-label">Заголовки ответа:</label>
                <pre class="detail-value code-block">{{ selectedLog.response_headers }}</pre>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="no-selection-message">
          <p>Выберите запрос для просмотра деталей</p>
        </div>
      </div>
    </div>

    <!-- Статистика -->
    <div class="stats-section">
      <div class="stats-card">
        <h4>Самые частые пути</h4>
        <div class="top-paths">
          <div v-for="path in stats.top_paths" :key="path.path" class="path-item">
            <span class="path-name">{{ path.path }}</span>
            <span class="path-count">{{ path.count }} запросов</span>
            <span class="path-duration">{{ Math.round(path.avg_duration_ms) }}мс</span>
          </div>
        </div>
      </div>
      
      <div class="stats-card">
        <h4>Самые активные пользователи</h4>
        <div class="top-users">
          <div v-for="user in stats.top_users" :key="user.username" class="user-item">
            <span class="user-name">{{ user.username }}</span>
            <span class="user-count">{{ user.count }} запросов</span>
            <span class="user-last">{{ formatTime(user.last_request) }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="isLoading" class="loading-overlay">
      <div class="spinner"></div>
    </div>
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';

export default {
  name: 'RequestsView',
  components: {
    SearchComponent
  },
  data() {
    return {
      logs: [],
      selectedLog: null,
      searchQuery: '',
      filterMethod: '',
      filterStatus: '',
      filterStartDate: '',
      filterEndDate: '',
      sortField: 'timestamp',
      sortDirection: 'desc',
      pagination: {
        page: 1,
        limit: 50,
        total: 0,
        pages: 1
      },
      pageSize: '50',
      isLoading: false,
      showLive: false,
      liveInterval: null,
      stats: {
        total_requests: 0,
        avg_duration_ms: 0,
        success_rate: 0,
        top_paths: [],
        top_users: []
      }
    };
  },
  computed: {
    filteredLogs() {
      return this.logs;
    }
  },
  methods: {
    async fetchLogs() {
      this.isLoading = true;
      try {
        const token = localStorage.getItem("token");
        const params = new URLSearchParams({
          page: this.pagination.page,
          limit: this.pagination.limit,
          ...(this.searchQuery && { search: this.searchQuery }),
          ...(this.filterMethod && { method: this.filterMethod }),
          ...(this.filterStatus && { status: parseInt(this.filterStatus) }),
          ...(this.filterStartDate && { start_date: this.filterStartDate }),
          ...(this.filterEndDate && { end_date: this.filterEndDate })
        });

        const response = await fetch(`http://localhost:8080/request-logs?${params}`, {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          this.logs = data.logs;
          this.pagination = data.pagination;
          
          // Прокручиваем вверх при обновлении в режиме Live
          if (this.showLive) {
            this.$nextTick(() => {
              const logsBody = this.$refs.logsBody;
              if (logsBody) {
                logsBody.scrollTop = 0;
              }
            });
          }
        }
      } catch (error) {
        console.error("Error fetching logs:", error);
        this.showNotification("Ошибка при загрузке логов", "error");
      } finally {
        this.isLoading = false;
      }
    },

    async fetchStats() {
      try {
        const token = localStorage.getItem("token");
        const params = new URLSearchParams({
          ...(this.filterStartDate && { start_date: this.filterStartDate }),
          ...(this.filterEndDate && { end_date: this.filterEndDate })
        });

        const response = await fetch(`http://localhost:8080/request-logs/stats?${params}`, {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          this.stats = await response.json();
        }
      } catch (error) {
        console.error("Error fetching stats:", error);
      }
    },

    refreshLogs() {
      this.pagination.page = 1;
      this.fetchLogs();
      this.fetchStats();
    },

    startLiveUpdates() {
      this.liveInterval = setInterval(() => {
        this.fetchLogs();
        this.fetchStats();
      }, 3000); // Обновление каждые 3 секунды
    },

    stopLiveUpdates() {
      if (this.liveInterval) {
        clearInterval(this.liveInterval);
        this.liveInterval = null;
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
      if (this.pagination.page < this.pagination.pages) {
        this.pagination.page++;
        this.fetchLogs();
      }
    },

    changePageSize() {
      this.pagination.limit = parseInt(this.pageSize);
      this.pagination.page = 1;
      this.fetchLogs();
    },

    clearFilters() {
      this.searchQuery = '';
      this.filterMethod = '';
      this.filterStatus = '';
      this.filterStartDate = '';
      this.filterEndDate = '';
      this.pagination.page = 1;
      this.fetchLogs();
      this.fetchStats();
    },

    formatTime(timestamp) {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    },

    formatFullDate(timestamp) {
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
  watch: {
    showLive(newVal) {
      if (newVal) {
        this.startLiveUpdates();
      } else {
        this.stopLiveUpdates();
      }
    }
  },
  async mounted() {
    await this.fetchLogs();
    await this.fetchStats();
  },
  beforeUnmount() {
    this.stopLiveUpdates();
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
  margin-right: auto;
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

.filter-controls {
  display: flex;
  gap: 8px;
  align-items: center;
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

.live-button {
  padding: 6px 12px;
  background: #dc2626;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 0.85em;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.live-button.live-active {
  background: #10b981;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { opacity: 1; }
  50% { opacity: 0.7; }
  100% { opacity: 1; }
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

.error-message {
  background: #fff5f5;
  color: #dc2626;
  padding: 8px;
  border-radius: 6px;
  border-left: 3px solid #dc2626;
  font-size: 0.85em;
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

.stats-section {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  padding: 20px;
  border-top: 1px solid #e6e6e6;
}

.stats-card {
  background: #f8fafc;
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  padding: 16px;
}

.stats-card h4 {
  margin: 0 0 12px 0;
  font-size: 0.95em;
  color: #333;
  font-weight: 600;
}

.top-paths, .top-users {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.path-item, .user-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px;
  background: #fff;
  border-radius: 6px;
  border: 1px solid #f0f0f0;
}

.path-name, .user-name {
  font-size: 0.85em;
  font-weight: 500;
  color: #333;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.path-count, .user-count {
  font-size: 0.8em;
  color: #6b7280;
  margin-left: 8px;
}

.path-duration, .user-last {
  font-size: 0.75em;
  color: #a2a2a2;
  margin-left: 8px;
  min-width: 60px;
  text-align: right;
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
  
  .stats-section {
    grid-template-columns: 1fr;
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
}
</style>