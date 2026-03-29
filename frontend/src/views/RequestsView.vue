<!-- src/views/RequestsView.vue -->
<template>
  <div class="requests-view">
    <div class="requests-header">
      <h2 class="requests-title">Журнал запросов</h2>
      <div class="header-actions">
        <button class="export-btn" @click="exportLogs" title="Экспорт в TXT">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 3v12m0 0-3-3m3 3 3-3M5 21h14" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Статистика -->
    <div class="stats-bar">
      <div class="stat-card">
        <div class="stat-value">{{ stats.total }}</div>
        <div class="stat-label">Всего запросов</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ stats.today }}</div>
        <div class="stat-label">Сегодня</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ formatNumber(stats.avgDuration) }} мс</div>
        <div class="stat-label">Среднее время</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ formatNumber(stats.errorRate) }}%</div>
        <div class="stat-label">Ошибки (4xx/5xx)</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ realtimeStats.lastSecondCount }}</div>
        <div class="stat-label">Запросов/сек (последняя сек)</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ realtimeStats.lastMinuteCount }}</div>
        <div class="stat-label">Запросов/мин (последняя мин)</div>
      </div>
    </div>

    <!-- Фильтры графика -->
    <div class="timeline-filters">
    <button
      v-for="opt in timelineOptions"
      :key="opt.label"
      class="timeline-filter-btn"
      :class="{ active: currentTimelineOption === opt.label }"
      @click="setTimelineOption(opt)"
    >
      {{ opt.label }}
    </button>
  </div>

  <div class="charts-container">
    <div class="chart-card">
      <h3>Динамика запросов</h3>
      <RealTimeChart
        ref="chart"
        :data="timeline"
        :color="'#4F5BDF'"
        :interval-label="currentTimelineOption"
        height="320"
      />
    </div>
  </div>

    <div class="filters-bar">
      <div class="filters-row">
        <div class="filter-group">
          <label>Пользователь</label>
          <select v-model="filters.user_id" class="filter-select">
            <option :value="null">Все</option>
            <option v-for="user in usersList" :key="user.id" :value="user.id">
              {{ user.username }}
            </option>
          </select>
        </div>
        <div class="filter-group">
          <label>Метод</label>
          <select v-model="filters.method" class="filter-select">
            <option :value="null">Все</option>
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
          </select>
        </div>
        <div class="filter-group">
          <label>Статус</label>
          <select v-model="filters.status" class="filter-select">
            <option :value="null">Все</option>
            <option value="200">200 OK</option>
            <option value="201">201 Created</option>
            <option value="400">400 Bad Request</option>
            <option value="401">401 Unauthorized</option>
            <option value="403">403 Forbidden</option>
            <option value="404">404 Not Found</option>
            <option value="500">500 Server Error</option>
          </select>
        </div>
        <div class="filter-group">
          <label>Дата от</label>
          <input type="datetime-local" v-model="filters.from_date" class="filter-input">
        </div>
        <div class="filter-group">
          <label>Дата до</label>
          <input type="datetime-local" v-model="filters.to_date" class="filter-input">
        </div>
        <div class="filter-group search-group">
          <label>Поиск</label>
          <input type="text" v-model="filters.search" placeholder="URL или пользователь..." class="filter-input">
        </div>
      </div>
    </div>

    <!-- Таблица -->
    <div class="table-container">
      <table class="requests-table">
        <thead>
           …
            <th>ID</th>
            <th>Пользователь</th>
            <th>Дата и время (МСК)</th>
            <th>Метод</th>
            <th>URL</th>
            <th>Статус</th>
            <th>Длительность (мс)</th>
            <th>Детали</th>
           </thead>
        <tbody>
          <tr v-for="log in logs" :key="log.id">
              <td>{{ log.id }}</td>
              <td>{{ log.username || 'Неавторизован' }}</td>
              <td>{{ formatDateTime(log.created_at) }}</td>
              <td><span class="method-badge" :class="methodClass(log.method)">{{ log.method }}</span></td>
              <td class="url-cell" :title="log.url">{{ truncate(log.url, 50) }}</td>
              <td><span class="status-badge" :class="statusClass(log.response_status)">{{ log.response_status }}</span></td>
              <td>{{ log.duration_ms }}</td>
              <td><button class="detail-btn" @click="showDetails(log)">Подробнее</button></td>
            </tr>
          <tr v-if="logs.length === 0"><td colspan="8" class="empty-state">Нет записей</td></tr>
        </tbody>
      </table>
    </div>

    <div class="pagination" v-if="totalPages > 1">
      <button class="page-btn" :disabled="currentPage === 1" @click="goToPage(currentPage - 1)">←</button>
      <span class="page-info">{{ currentPage }} из {{ totalPages }}</span>
      <button class="page-btn" :disabled="currentPage === totalPages" @click="goToPage(currentPage + 1)">→</button>
    </div>

    <!-- Модальное окно деталей -->
    <transition name="modal-fade">
      <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">Детали запроса #{{ selectedLog?.id }}</h3>
            <button class="modal-close" @click="closeModal">
              <svg width="10" height="10" viewBox="0 0 14 14" fill="none"><path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="detail-row"><strong>Пользователь:</strong> {{ selectedLog?.username || 'Неавторизован' }}</div>
            <div class="detail-row"><strong>Дата:</strong> {{ formatDateTime(selectedLog?.created_at) }}</div>
            <div class="detail-row"><strong>Метод:</strong> {{ selectedLog?.method }}</div>
            <div class="detail-row"><strong>URL:</strong> {{ selectedLog?.url }}</div>
            <div class="detail-row"><strong>Статус:</strong> {{ selectedLog?.response_status }}</div>
            <div class="detail-row"><strong>Длительность:</strong> {{ selectedLog?.duration_ms }} мс</div>
            <div class="detail-row"><strong>Заголовки:</strong><pre class="json-preview">{{ formatHeaders(selectedLog?.headers) }}</pre></div>
          </div>
          <div class="modal-footer"><button class="btn close-modal-btn" @click="closeModal">Закрыть</button></div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script>
import RealTimeChart from '@/components/RealTimeChart.vue'

export default {
  name: 'RequestsView',
  components: { RealTimeChart },
  data() {
    return {
      logs: [],
      usersList: [],
      stats: {
        total: 0,
        today: 0,
        avgDuration: 0,
        errorRate: 0
      },
      realtimeStats: {
        lastSecondCount: 0,
        lastMinuteCount: 0
      },
      timeline: [],
      filters: {
        user_id: null,
        method: null,
        status: null,
        from_date: null,
        to_date: null,
        search: ''
      },
      currentPage: 1,
      totalPages: 1,
      total: 0,
      perPage: 20,
      showModal: false,
      selectedLog: null,
      ws: null,
      wsReconnectTimer: null,

      // Опции временной шкалы
      timelineOptions: [
        { label: 'Минута', intervalSec: 60, limit: 60, dateFormat: 'HH:mm' },
        { label: 'Час', intervalSec: 3600, limit: 24, dateFormat: 'HH:00' },
        { label: 'День', intervalSec: 86400, limit: 30, dateFormat: 'DD MMM' },
        { label: 'Неделя', intervalSec: 604800, limit: 52, dateFormat: 'DD MMM' },
        { label: 'Месяц', intervalSec: 2592000, limit: 12, dateFormat: 'MMM YYYY' },
        { label: 'Год', intervalSec: 31536000, limit: 10, dateFormat: 'YYYY' }
      ],
      currentTimelineOption: 'Час',
      timelineInterval: 3600,
      timelineLimit: 24
    }
  },
  watch: {
    filters: {
      deep: true,
      handler() {
        this.currentPage = 1
        this.fetchAllData()
      }
    }
  },
  methods: {
    async fetchAllData() {
      await Promise.all([
        this.fetchLogs(),
        this.fetchStats(),
        this.fetchRealtimeStats(),
        this.fetchTimeline(),
        this.fetchUsers()
      ])
    },
    async fetchLogs() {
      try {
        const token = localStorage.getItem('token')
        const params = new URLSearchParams()
        if (this.filters.user_id) params.append('user_id', this.filters.user_id)
        if (this.filters.method) params.append('method', this.filters.method)
        if (this.filters.status) params.append('status', this.filters.status)
        if (this.filters.from_date) params.append('from_date', this.filters.from_date)
        if (this.filters.to_date) params.append('to_date', this.filters.to_date)
        if (this.filters.search) params.append('search', this.filters.search)
        params.append('page', this.currentPage)
        params.append('per_page', this.perPage)

        const response = await fetch(`http://localhost:8080/request-logs?${params}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (response.ok) {
          const data = await response.json()
          this.logs = data.logs
          this.total = data.total
          this.totalPages = data.total_pages
        }
      } catch (error) {
        console.error('Error fetching logs:', error)
      }
    },
    async fetchStats() {
      try {
        const token = localStorage.getItem('token')
        const params = new URLSearchParams()
        if (this.filters.from_date) params.append('from_date', this.filters.from_date)
        if (this.filters.to_date) params.append('to_date', this.filters.to_date)
        const response = await fetch(`http://localhost:8080/request-logs/stats?${params}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (response.ok) {
          const data = await response.json()
          this.stats = {
            total: data.total,
            today: data.today,
            avgDuration: data.avg_duration,
            errorRate: data.error_rate
          }
        }
      } catch (error) {
        console.error('Error fetching stats:', error)
      }
    },
    async fetchRealtimeStats() {
      try {
        const token = localStorage.getItem('token')
        const response = await fetch('http://localhost:8080/request-logs/realtime', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (response.ok) {
          const data = await response.json()
          this.realtimeStats = {
            lastSecondCount: data.last_second_count,
            lastMinuteCount: data.last_minute_count
          }
        }
      } catch (error) {
        console.error('Error fetching realtime stats:', error)
      }
    },
    async fetchTimeline() {
      try {
        const token = localStorage.getItem('token')
        const params = new URLSearchParams()
        params.append('interval', this.timelineInterval)
        params.append('limit', this.timelineLimit)
        if (this.filters.from_date) params.append('from_date', this.filters.from_date)
        if (this.filters.to_date) params.append('to_date', this.filters.to_date)
        const response = await fetch(`http://localhost:8080/request-logs/timeline?${params}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (response.ok) {
          const data = await response.json()
          this.timeline = data
        }
      } catch (error) {
        console.error('Error fetching timeline:', error)
      }
    },
    async fetchUsers() {
      try {
        const token = localStorage.getItem('token')
        const response = await fetch('http://localhost:8080/request-logs/users', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (response.ok) {
          this.usersList = await response.json()
        }
      } catch (error) {
        console.error('Error fetching users:', error)
      }
    },
    async exportLogs() {
      try {
        const token = localStorage.getItem('token')
        const params = new URLSearchParams()
        if (this.filters.user_id) params.append('user_id', this.filters.user_id)
        if (this.filters.method) params.append('method', this.filters.method)
        if (this.filters.status) params.append('status', this.filters.status)
        if (this.filters.from_date) params.append('from_date', this.filters.from_date)
        if (this.filters.to_date) params.append('to_date', this.filters.to_date)
        if (this.filters.search) params.append('search', this.filters.search)
        params.append('format', 'txt')

        const response = await fetch(`http://localhost:8080/request-logs/export?${params}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (response.ok) {
          const blob = await response.blob()
          const url = window.URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = `logs_${new Date().toISOString().slice(0,19)}.txt`
          document.body.appendChild(a)
          a.click()
          document.body.removeChild(a)
          window.URL.revokeObjectURL(url)
        }
      } catch (error) {
        console.error('Error exporting logs:', error)
      }
    },
    formatNumber(value) {
      if (value === undefined || value === null) return '0'
      return value.toFixed(2)
    },
    goToPage(page) {
      this.currentPage = page
      this.fetchLogs()
    },
    setTimelineOption(opt) {
      this.currentTimelineOption = opt.label
      this.timelineInterval = opt.intervalSec
      this.timelineLimit = opt.limit
      this.fetchTimeline()
    },
    formatDateTime(dateStr) {
      if (!dateStr) return ''
      const date = new Date(dateStr)
      return date.toLocaleString('ru-RU', { timeZone: 'Europe/Moscow' })
    },
    truncate(str, length) {
      if (!str) return ''
      return str.length > length ? str.substring(0, length) + '...' : str
    },
    methodClass(method) {
      return {
        'GET': 'method-get',
        'POST': 'method-post',
        'PUT': 'method-put',
        'DELETE': 'method-delete'
      }[method] || ''
    },
    statusClass(status) {
      if (status >= 200 && status < 300) return 'status-success'
      if (status >= 400 && status < 500) return 'status-client-error'
      if (status >= 500) return 'status-server-error'
      return ''
    },
    formatHeaders(headers) {
      if (!headers) return 'Нет данных'
      try {
        return JSON.stringify(headers, null, 2)
      } catch {
        return headers
      }
    },
    showDetails(log) {
      this.selectedLog = log
      this.showModal = true
    },
    closeModal() {
      this.showModal = false
      this.selectedLog = null
    },
    setupWebSocket() {
      const token = localStorage.getItem('token')
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.hostname}:8080/ws/logs?token=${encodeURIComponent(token)}`
      console.log('Connecting to WebSocket:', wsUrl)
      this.ws = new WebSocket(wsUrl)
      this.ws.onopen = () => console.log('WebSocket connected')
      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          console.log('WebSocket message received:', data)
          if (data.type === 'new_log') {
            const newLog = data.log
            let matches = true
            if (this.filters.user_id && newLog.user_id !== this.filters.user_id) matches = false
            if (matches && this.filters.method && newLog.method !== this.filters.method) matches = false
            if (matches && this.filters.status && newLog.response_status !== this.filters.status) matches = false
            if (matches && this.filters.search && !(newLog.url.includes(this.filters.search) || (newLog.username && newLog.username.includes(this.filters.search)))) matches = false
            if (matches && this.filters.from_date) {
              const from = new Date(this.filters.from_date)
              const logDate = new Date(newLog.created_at)
              if (logDate < from) matches = false
            }
            if (matches && this.filters.to_date) {
              const to = new Date(this.filters.to_date)
              const logDate = new Date(newLog.created_at)
              if (logDate > to) matches = false
            }
            if (matches && this.currentPage === 1) {
              this.logs.unshift(newLog)
              if (this.logs.length > this.perPage) this.logs.pop()
            }
          } else if (data.type === 'stats_update') {
            this.stats = {
              total: data.stats.total,
              today: data.stats.today,
              avgDuration: data.stats.avg_duration,
              errorRate: data.stats.error_rate
            }
            this.realtimeStats = {
              lastSecondCount: data.realtime.last_second_count,
              lastMinuteCount: data.realtime.last_minute_count
            }
          } else if (data.type === 'timeline_update') {
            this.timeline = data.timeline
          }
        } catch (e) {
          console.error('Error parsing WebSocket message:', e)
        }
      }
      this.ws.onerror = (err) => console.error('WebSocket error:', err)
      this.ws.onclose = () => {
        console.log('WebSocket closed, reconnecting...')
        if (this.wsReconnectTimer) clearTimeout(this.wsReconnectTimer)
        this.wsReconnectTimer = setTimeout(() => this.setupWebSocket(), 5000)
      }
    }
  },
  mounted() {
    this.fetchAllData()
    this.setupWebSocket()
  },
  beforeUnmount() {
    if (this.ws) this.ws.close()
    if (this.wsReconnectTimer) clearTimeout(this.wsReconnectTimer)
  }
}
</script>

<style scoped>
/* добавлено для фильтров графика */
.timeline-filters {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.timeline-filter-btn {
  background: #f0f0f0;
  border: none;
  border-radius: 20px;
  padding: 6px 16px;
  font-family: 'Montserrat', sans-serif;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  color: #666;
}
.timeline-filter-btn:hover {
  background: #e0e0e0;
}
.timeline-filter-btn.active {
  background: #4F5BDF;
  color: white;
}

/* стили остаются как в предыдущей версии, добавим только для графика */
.charts-container {
  margin-bottom: 20px;
}
.chart-card {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 24px;
  padding: 20px;
}
.chart-card h3 {
  margin: 0 0 15px 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.requests-view {
  padding: 20px;
  font-family: 'Montserrat', sans-serif;
}

.requests-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.requests-title {
  font-size: 24px;
  font-weight: 700;
  color: #1a1a1a;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.refresh-btn, .export-btn {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 30px;
  padding: 6px 12px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #333;
}

.refresh-btn:hover, .export-btn:hover {
  background: #f5f5f5;
  border-color: #4F5BDF;
}

.stats-bar {
  display: flex;
  gap: 20px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.stat-card {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  padding: 12px 20px;
  min-width: 140px;
  text-align: center;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: #4F5BDF;
  margin-bottom: 4px;
}

.stat-label {
  font-size: 12px;
  color: #a2a2a2;
}

.charts-container {
  margin-bottom: 20px;
}

.chart-card {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 24px;
  padding: 20px;
}

.chart-card h3 {
  margin: 0 0 15px 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.chart-wrapper {
  position: relative;
  height: 300px;
  width: 100%;
}

.filters-bar {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  padding: 20px;
  margin-bottom: 20px;
}

.filters-row {
  display: flex;
  flex-wrap: wrap;
  gap: 15px;
  align-items: flex-end;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.filter-group label {
  font-size: 12px;
  color: #a2a2a2;
}

.filter-select, .filter-input {
  padding: 6px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  font-size: 13px;
  outline: none;
}

.filter-select:focus, .filter-input:focus {
  border-color: #4F5BDF;
}

.search-group {
  flex: 1;
  min-width: 200px;
}

.table-container {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 30px;
  overflow-x: auto;
}

.requests-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.requests-table th,
.requests-table td {
  padding: 8px 12px;
  text-align: left;
  border-bottom: 1px solid #f0f0f0;
}

.requests-table th {
  background: #fafafa;
  font-weight: 600;
  color: #333;
  padding: 10px 12px;
}

.requests-table tr:hover {
  background: #fafafa;
}

.method-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
}

.method-get {
  background: #e3f2fd;
  color: #1976d2;
}

.method-post {
  background: #e8f5e9;
  color: #2e7d32;
}

.method-put {
  background: #fff3e0;
  color: #f57c00;
}

.method-delete {
  background: #ffebee;
  color: #c62828;
}

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
}

.status-success {
  background: #e8f5e9;
  color: #2e7d32;
}

.status-client-error {
  background: #fff3e0;
  color: #f57c00;
}

.status-server-error {
  background: #ffebee;
  color: #c62828;
}

.url-cell {
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-btn {
  background: none;
  border: none;
  color: #4F5BDF;
  cursor: pointer;
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 15px;
  transition: background 0.2s;
}

.detail-btn:hover {
  background: #f0f4ff;
}

.empty-state {
  text-align: center;
  padding: 40px;
  color: #a2a2a2;
}

.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 15px;
  margin-top: 20px;
}

.page-btn {
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  padding: 6px 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.page-btn:hover:not(:disabled) {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.page-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.page-info {
  font-size: 14px;
  color: #666;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(1px);
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.9) translateY(-20px);
}

.modal-content {
  background: #fff;
  border-radius: 30px;
  width: 700px;
  max-width: 90vw;
  max-height: 80vh;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 30px;
  border-bottom: 1px solid #f0f0f0;
}

.modal-title {
  font-size: 18px;
  font-weight: 600;
  color: #1a1a1a;
  margin: 0;
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s;
}

.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-body {
  padding: 20px 30px;
  overflow-y: auto;
  flex: 1;
}

.detail-row {
  margin-bottom: 16px;
}

.detail-row strong {
  display: block;
  font-size: 13px;
  color: #a2a2a2;
  margin-bottom: 4px;
}

.json-preview {
  background: #f8f9fa;
  padding: 10px;
  border-radius: 12px;
  font-family: monospace;
  font-size: 12px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.modal-footer {
  padding: 16px 30px 24px;
  border-top: 1px solid #f0f0f0;
  display: flex;
  justify-content: flex-end;
}

.close-modal-btn {
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 30px;
  padding: 8px 24px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.close-modal-btn:hover {
  background: #3a45c0;
}
</style>