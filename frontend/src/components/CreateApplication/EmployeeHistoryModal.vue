<template>
  <BaseModal
    :show="show"
    :title="`История: ${fullName}`"
    width="800px"
    @close="$emit('close')"
  >
    <div class="history-modal">
      <!-- Фильтры -->
      <div class="filter-panel">
        <div class="filter-row">
          <input
            v-model="searchText"
            type="text"
            class="filter-input"
            placeholder="Поиск по действию, пользователю..."
          />
          <select v-model="filterUser" class="filter-select">
            <option value="">Все пользователи</option>
            <option v-for="user in uniqueUsers" :key="user.id" :value="user.id">
              {{ user.name }}
            </option>
          </select>
        </div>
        <div class="filter-row">
          <select v-model="filterPlace" class="filter-select">
            <option value="">Все места</option>
            <option v-for="place in uniquePlaces" :key="place.id" :value="place.id">
              {{ place.name }}
            </option>
          </select>
          <div class="date-range">
            <input v-model="dateFrom" type="date" class="filter-date" />
            <span class="date-separator">&mdash;</span>
            <input v-model="dateTo" type="date" class="filter-date" />
          </div>
          <button class="sort-btn" @click="toggleSort">
            {{ sortAsc ? 'Сначала старые' : 'Сначала новые' }}
          </button>
        </div>
      </div>

      <!-- Загрузка -->
      <div v-if="loading" class="loading-state">Загрузка...</div>

      <!-- Ошибка -->
      <div v-else-if="error" class="error-state">{{ error }}</div>

      <!-- Пустое состояние -->
      <div v-else-if="filteredHistory.length === 0" class="empty-state">
        История не найдена
      </div>

      <!-- Timeline -->
      <div v-else class="timeline">
        <div
          v-for="item in filteredHistory"
          :key="item.id"
          class="timeline-item"
        >
          <div class="timeline-marker">
            <span class="timeline-dot" :class="getDotClass(item.action)"></span>
            <span class="timeline-line"></span>
          </div>
          <div class="timeline-content">
            <div class="timeline-header">
              <span class="timeline-action">{{ getActionLabel(item.action) }}</span>
              <span class="timeline-date">{{ formatDate(item.created_at) }}</span>
            </div>
            <div class="timeline-user">{{ item.user_name }}</div>
            <div v-if="item.action === 'UPDATE' && item.field_name" class="timeline-changes">
              <span class="field-name">{{ item.field_name }}:</span>
              <span class="old-value">{{ item.old_value || '—' }}</span>
              <span class="change-arrow">&rarr;</span>
              <span class="new-value">{{ item.new_value || '—' }}</span>
            </div>
            <div v-if="item.comment" class="timeline-comment">{{ item.comment }}</div>
            <div v-if="item.table_name" class="timeline-place">{{ item.table_name }}</div>
          </div>
        </div>
      </div>
    </div>

    <template #actions>
      <button class="btn btn--export" :disabled="filteredHistory.length === 0" @click="exportToExcel">
        Экспорт
      </button>
      <button class="btn btn--secondary" @click="$emit('close')">Закрыть</button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import { apiRequest } from '@/api/client'
import ExcelJS from 'exceljs'

const actionLabels = {
  entry: 'Вход на территорию',
  exit: 'Выход с территории',
  CREATE: 'Создание записи',
  UPDATE: 'Обновление данных',
  DELETE: 'Удаление',
  TERRITORY_CHANGE: 'Изменение статуса территории',
  DEACTIVATE: 'Деактивация',
  ACTIVATE: 'Активация',
  RESTORE: 'Восстановление',
}

export default {
  name: 'EmployeeHistoryModal',
  components: { BaseModal },

  props: {
    show: { type: Boolean, default: false },
    lastName: { type: String, default: '' },
    firstName: { type: String, default: '' },
    middleName: { type: String, default: '' },
  },

  emits: ['close'],

  data() {
    return {
      history: [],
      loading: false,
      error: '',
      searchText: '',
      filterUser: '',
      filterPlace: '',
      dateFrom: '',
      dateTo: '',
      sortAsc: false,
    }
  },

  computed: {
    fullName() {
      return [this.lastName, this.firstName, this.middleName].filter(Boolean).join(' ')
    },

    uniqueUsers() {
      const map = new Map()
      for (const item of this.history) {
        if (item.user_id && !map.has(item.user_id)) {
          map.set(item.user_id, { id: item.user_id, name: item.user_name })
        }
      }
      return Array.from(map.values())
    },

    uniquePlaces() {
      const map = new Map()
      for (const item of this.history) {
        if (item.table_id && !map.has(item.table_id)) {
          map.set(item.table_id, { id: item.table_id, name: item.table_name })
        }
      }
      return Array.from(map.values())
    },

    filteredHistory() {
      let result = [...this.history]

      if (this.searchText) {
        const q = this.searchText.toLowerCase()
        result = result.filter((item) => {
          const label = this.getActionLabel(item.action).toLowerCase()
          return (
            (item.user_name && item.user_name.toLowerCase().includes(q)) ||
            label.includes(q) ||
            (item.comment && item.comment.toLowerCase().includes(q)) ||
            (item.field_name && item.field_name.toLowerCase().includes(q)) ||
            (item.old_value && item.old_value.toLowerCase().includes(q)) ||
            (item.new_value && item.new_value.toLowerCase().includes(q))
          )
        })
      }

      if (this.filterUser) {
        result = result.filter((item) => item.user_id === this.filterUser)
      }

      if (this.filterPlace) {
        result = result.filter((item) => item.table_id === this.filterPlace)
      }

      if (this.dateFrom) {
        const from = new Date(this.dateFrom)
        from.setHours(0, 0, 0, 0)
        result = result.filter((item) => new Date(item.created_at) >= from)
      }

      if (this.dateTo) {
        const to = new Date(this.dateTo)
        to.setHours(23, 59, 59, 999)
        result = result.filter((item) => new Date(item.created_at) <= to)
      }

      result.sort((a, b) => {
        const diff = new Date(a.created_at) - new Date(b.created_at)
        return this.sortAsc ? diff : -diff
      })

      return result
    },
  },

  watch: {
    show(val) {
      if (val) {
        this.fetchHistory()
      } else {
        this.resetFilters()
      }
    },
  },

  methods: {
    async fetchHistory() {
      this.loading = true
      this.error = ''
      this.history = []

      try {
        const params = new URLSearchParams()
        if (this.lastName) params.set('last_name', this.lastName)
        if (this.firstName) params.set('first_name', this.firstName)
        if (this.middleName) params.set('middle_name', this.middleName)

        const response = await apiRequest(`/employees/history/unified?${params.toString()}`)
        if (!response.ok) {
          const err = await response.json()
          throw new Error(err.message || 'Ошибка загрузки истории')
        }
        const data = await response.json()
        this.history = Array.isArray(data) ? data : []
      } catch (err) {
        this.error = err.message || 'Не удалось загрузить историю'
      } finally {
        this.loading = false
      }
    },

    resetFilters() {
      this.searchText = ''
      this.filterUser = ''
      this.filterPlace = ''
      this.dateFrom = ''
      this.dateTo = ''
      this.sortAsc = false
    },

    toggleSort() {
      this.sortAsc = !this.sortAsc
    },

    getActionLabel(action) {
      return actionLabels[action] || action
    },

    getDotClass(action) {
      const map = {
        entry: 'dot--entry',
        exit: 'dot--exit',
        CREATE: 'dot--create',
        UPDATE: 'dot--update',
        DELETE: 'dot--delete',
        ACTIVATE: 'dot--activate',
        DEACTIVATE: 'dot--deactivate',
        RESTORE: 'dot--restore',
        TERRITORY_CHANGE: 'dot--update',
      }
      return map[action] || 'dot--default'
    },

    formatDate(dateStr) {
      if (!dateStr) return ''
      return new Date(dateStr).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    },

    async exportToExcel() {
      const workbook = new ExcelJS.Workbook()
      const sheet = workbook.addWorksheet('История')

      sheet.columns = [
        { header: 'Дата', key: 'date', width: 20 },
        { header: 'Пользователь', key: 'user', width: 25 },
        { header: 'Действие', key: 'action', width: 30 },
        { header: 'Поле', key: 'field', width: 20 },
        { header: 'Старое значение', key: 'oldValue', width: 25 },
        { header: 'Новое значение', key: 'newValue', width: 25 },
        { header: 'Комментарий', key: 'comment', width: 30 },
        { header: 'Место', key: 'place', width: 25 },
      ]

      const headerRow = sheet.getRow(1)
      headerRow.font = { bold: true }
      headerRow.fill = {
        type: 'pattern',
        pattern: 'solid',
        fgColor: { argb: 'FFE8EAF6' },
      }

      for (const item of this.filteredHistory) {
        sheet.addRow({
          date: this.formatDate(item.created_at),
          user: item.user_name || '',
          action: this.getActionLabel(item.action),
          field: item.field_name || '',
          oldValue: item.old_value || '',
          newValue: item.new_value || '',
          comment: item.comment || '',
          place: item.table_name || '',
        })
      }

      const buffer = await workbook.xlsx.writeBuffer()
      const blob = new Blob([buffer], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `История_${this.fullName}.xlsx`
      link.click()
      URL.revokeObjectURL(url)
    },
  },
}
</script>

<style scoped>
.history-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-height: 55vh;
}

/* Фильтры */
.filter-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 16px;
  background: #fafafa;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.filter-row {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.filter-input {
  flex: 1;
  min-width: 180px;
  padding: 8px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: var(--color-text);
  background: #fff;
  outline: none;
  transition: border-color 0.2s;
}

.filter-input:focus {
  border-color: var(--color-primary);
}

.filter-select {
  padding: 8px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: var(--color-text);
  background: #fff;
  outline: none;
  min-width: 160px;
  cursor: pointer;
  transition: border-color 0.2s;
}

.filter-select:focus {
  border-color: var(--color-primary);
}

.date-range {
  display: flex;
  align-items: center;
  gap: 6px;
}

.filter-date {
  padding: 8px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: var(--color-text);
  background: #fff;
  outline: none;
  transition: border-color 0.2s;
}

.filter-date:focus {
  border-color: var(--color-primary);
}

.date-separator {
  color: #999;
  font-size: 14px;
}

.sort-btn {
  padding: 8px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-size: 13px;
  background: #fff;
  color: var(--color-text);
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s;
}

.sort-btn:hover {
  background: var(--color-bg-hover);
  border-color: var(--color-primary);
}

/* Состояния */
.loading-state,
.error-state,
.empty-state {
  text-align: center;
  padding: 40px 20px;
  font-size: 14px;
  color: #999;
}

.error-state {
  color: #f44336;
}

/* Timeline */
.timeline {
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  padding-right: 4px;
}

.timeline-item {
  display: flex;
  gap: 14px;
  min-height: 60px;
}

.timeline-marker {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-shrink: 0;
  width: 16px;
}

.timeline-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 4px;
}

.timeline-line {
  width: 2px;
  flex: 1;
  background: var(--color-border);
  min-height: 10px;
}

.timeline-item:last-child .timeline-line {
  display: none;
}

/* Цвета точек */
.dot--entry { background: #4caf50; }
.dot--exit { background: #f44336; }
.dot--create { background: #2196f3; }
.dot--update { background: #ff9800; }
.dot--delete { background: #9e9e9e; }
.dot--activate,
.dot--restore { background: #9c27b0; }
.dot--deactivate { background: #795548; }
.dot--default { background: #bdbdbd; }

.timeline-content {
  flex: 1;
  padding-bottom: 16px;
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.timeline-action {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.timeline-date {
  font-size: 12px;
  color: #999;
  white-space: nowrap;
}

.timeline-user {
  font-size: 13px;
  color: #666;
  margin-top: 2px;
}

.timeline-changes {
  margin-top: 6px;
  padding: 8px 12px;
  background: #f8f9fa;
  border-radius: var(--radius-md);
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.field-name {
  font-weight: 500;
  color: #555;
}

.old-value {
  color: #f44336;
  text-decoration: line-through;
}

.change-arrow {
  color: #999;
}

.new-value {
  color: #4caf50;
  font-weight: 500;
}

.timeline-comment {
  margin-top: 4px;
  font-size: 13px;
  color: #888;
  font-style: italic;
}

.timeline-place {
  margin-top: 4px;
  font-size: 12px;
  color: #aaa;
}

/* Кнопки */
.btn {
  padding: 10px 24px;
  border: none;
  border-radius: var(--radius-md);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn--export {
  background: #e8f5e9;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.btn--export:hover:not(:disabled) {
  background: #c8e6c9;
}

.btn--export:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn--secondary {
  background: #f5f5f5;
  color: var(--color-text);
  border: 1px solid var(--color-border);
}

.btn--secondary:hover {
  background: var(--color-bg-hover);
}

@media (max-width: 640px) {
  .filter-row {
    flex-direction: column;
  }

  .filter-input,
  .filter-select {
    width: 100%;
    min-width: unset;
  }

  .date-range {
    width: 100%;
  }

  .filter-date {
    flex: 1;
  }

  .sort-btn {
    width: 100%;
  }

  .timeline-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .timeline-changes {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
