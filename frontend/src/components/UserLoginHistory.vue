<template>
  <div class="login-history">
    <div class="lh-filters">
      <div class="lh-search">
        <img
          src="@/assets/icons/search.png"
          class="lh-search__icon"
          alt=""
        >
        <input
          v-model="search"
          class="lk-input lh-search__input"
          type="text"
          placeholder="Поиск по IP или устройству…"
        >
      </div>
      <BaseDropdown
        :model-value="filters.category"
        class="lh-category"
        :options="categoryOptions"
        value-key="value"
        label-key="label"
        placeholder="Все события"
        @update:model-value="onCategoryChange"
      />
      <div class="lh-dates">
        <input
          v-model="filters.from"
          class="lk-input lh-date"
          type="date"
          aria-label="С даты"
          @change="applyFilters"
        >
        <span class="lh-dates__sep">—</span>
        <input
          v-model="filters.to"
          class="lk-input lh-date"
          type="date"
          aria-label="По дату"
          @change="applyFilters"
        >
      </div>
      <button
        v-if="hasActiveFilters"
        type="button"
        class="lk-button lk-button--ghost lh-reset"
        @click="resetFilters"
      >
        Сбросить
      </button>
      <button
        type="button"
        class="lk-button lk-button--secondary lh-export"
        :disabled="isExporting || total === 0"
        @click="exportToExcel"
      >
        {{ isExporting ? 'Готовим…' : 'Экспорт' }}
      </button>
    </div>

    <div class="lh-table-wrap">
      <table class="lh-table">
        <thead>
          <tr>
            <th class="lh-col-when">
              Дата и время
            </th>
            <th class="lh-col-event">
              Событие
            </th>
            <th class="lh-col-ip">
              IP-адрес
            </th>
            <th class="lh-col-device">
              Устройство
            </th>
            <th class="lh-col-detail">
              Детали
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in visibleItems"
            :key="item.id"
          >
            <td class="lh-when">
              {{ formatDateTime(item.created_at) }}
            </td>
            <td>
              <span
                class="lh-badge"
                :class="eventBadge(item.event_type).cls"
              >
                {{ eventBadge(item.event_type).label }}
              </span>
            </td>
            <td class="lh-ip">
              {{ item.ip_address || '—' }}
            </td>
            <td class="lh-device">
              <template v-if="parseDevice(item.user_agent).browser || parseDevice(item.user_agent).os">
                <span class="lh-device__browser">{{ parseDevice(item.user_agent).browser || 'Браузер' }}</span>
                <small
                  v-if="parseDevice(item.user_agent).os"
                  class="lh-device__os"
                >{{ parseDevice(item.user_agent).os }}</small>
              </template>
              <span
                v-else
                class="lh-muted"
              >—</span>
            </td>
            <td class="lh-detail">
              {{ item.detail || '—' }}
            </td>
          </tr>
          <tr v-if="visibleItems.length === 0">
            <td
              colspan="5"
              class="lh-empty"
            >
              {{ loading ? 'Загрузка…' : (search ? 'На этой странице ничего не найдено' : 'Записей о входах нет') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="lh-footer">
      <div class="lh-legend">
        <span><i class="lh-dot lh-dot--ok" />Вход</span>
        <span><i class="lh-dot lh-dot--neutral" />Выход</span>
        <span><i class="lh-dot lh-dot--danger" />Неудачный вход</span>
        <span><i class="lh-dot lh-dot--warn" />Блокировка</span>
      </div>
      <div class="lh-pager">
        <span class="lh-total">Всего: {{ total }}</span>
        <button
          type="button"
          class="lk-button lk-button--ghost lh-page-btn"
          :disabled="page <= 1 || loading"
          @click="goToPage(page - 1)"
        >
          Назад
        </button>
        <span class="lh-page-num">{{ page }} / {{ totalPages }}</span>
        <button
          type="button"
          class="lk-button lk-button--ghost lh-page-btn"
          :disabled="page >= totalPages || loading"
          @click="goToPage(page + 1)"
        >
          Вперёд
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import ExcelJS from 'exceljs'
import BaseDropdown from './ui/BaseDropdown.vue'
import { getUserAuthEvents } from '@/api/users'
import { formatDateTime } from '@/utils/datetime'
import { parseUserAgent, formatDevice } from '@/utils/userAgent'
import { useDeletionsStore } from '@/stores/deletions'

// Маппинг типа события в человекочитаемый бейдж. Сырые коды (login_success и т.п.)
// в интерфейс не попадают - только понятные подписи с цветом по смыслу.
const EVENT_BADGES = {
  login_success: { cls: 'lh-badge--ok', label: 'Вход выполнен' },
  logout: { cls: 'lh-badge--neutral', label: 'Выход' },
  login_failed: { cls: 'lh-badge--danger', label: 'Неудачный вход' },
  login_locked: { cls: 'lh-badge--warn', label: 'Вход заблокирован' },
  account_locked: { cls: 'lh-badge--warn', label: 'Аккаунт заблокирован' },
  refresh: { cls: 'lh-badge--muted', label: 'Сессия обновлена' },
  token_reuse_detected: { cls: 'lh-badge--danger', label: 'Подозрительная сессия' },
}

// Экспорт тянет до этого числа последних событий по текущим фильтрам (лимит бэка).
const EXPORT_LIMIT = 100

export default {
  name: 'UserLoginHistory',
  components: { BaseDropdown },
  props: {
    username: {
      type: String,
      required: true,
    },
    currentUserName: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      items: [],
      total: 0,
      page: 1,
      limit: 25,
      loading: false,
      isExporting: false,
      search: '',
      filters: { category: '', from: '', to: '' },
      categoryOptions: [
        { value: '', label: 'Все события' },
        { value: 'login', label: 'Входы' },
        { value: 'logout', label: 'Выходы' },
        { value: 'failed', label: 'Неудачные входы' },
        { value: 'locked', label: 'Блокировки' },
        { value: 'session', label: 'Сессии' },
      ],
    }
  },
  computed: {
    totalPages() {
      return Math.max(1, Math.ceil(this.total / this.limit))
    },
    hasActiveFilters() {
      return this.filters.category !== '' || this.filters.from !== '' || this.filters.to !== '' || this.search !== ''
    },
    // Поиск - клиентская дорасфильтровка загруженной страницы (по IP/устройству/деталям).
    visibleItems() {
      const q = this.search.trim().toLowerCase()
      if (!q) return this.items
      return this.items.filter((it) => {
        const device = formatDevice(it.user_agent).toLowerCase()
        return (it.ip_address || '').toLowerCase().includes(q)
          || device.includes(q)
          || (it.detail || '').toLowerCase().includes(q)
      })
    },
  },
  watch: {
    // Смена пользователя (переоткрытие карточки на другом юзере) - сброс и перезагрузка.
    username() {
      this.page = 1
      this.search = ''
      this.filters = { category: '', from: '', to: '' }
      this.fetch()
    },
  },
  mounted() {
    this.fetch()
  },
  methods: {
    formatDateTime,
    parseDevice(ua) {
      return parseUserAgent(ua)
    },
    eventBadge(eventType) {
      return EVENT_BADGES[eventType] || { cls: 'lh-badge--neutral', label: eventType || '—' }
    },
    async fetch() {
      this.loading = true
      try {
        const data = await getUserAuthEvents(this.username, {
          page: this.page,
          limit: this.limit,
          category: this.filters.category,
          from: this.filters.from,
          to: this.filters.to,
        }) || {}
        this.items = Array.isArray(data.items) ? data.items : []
        this.total = typeof data.total === 'number' ? data.total : 0
      } catch (error) {
        console.error('Ошибка загрузки истории входов:', error)
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю входов', type: 'error' })
        this.items = []
        this.total = 0
      } finally {
        this.loading = false
      }
    },
    applyFilters() {
      this.page = 1
      this.fetch()
    },
    onCategoryChange(value) {
      this.filters.category = value
      this.applyFilters()
    },
    resetFilters() {
      this.filters = { category: '', from: '', to: '' }
      this.search = ''
      this.page = 1
      this.fetch()
    },
    goToPage(next) {
      if (next < 1 || next > this.totalPages) return
      this.page = next
      this.fetch()
    },
    async exportToExcel() {
      if (this.total === 0) return
      this.isExporting = true
      try {
        const data = await getUserAuthEvents(this.username, {
          page: 1,
          limit: EXPORT_LIMIT,
          category: this.filters.category,
          from: this.filters.from,
          to: this.filters.to,
        }) || {}
        const rows = Array.isArray(data.items) ? data.items : []
        if (rows.length === 0) return
        await this.buildWorkbook(rows)
        if (typeof data.total === 'number' && data.total > rows.length) {
          useDeletionsStore().notify({ prefix: 'Выгружены последние ', bold: `${rows.length} событий`, suffix: ` из ${data.total}` })
        }
      } catch (error) {
        console.error('Ошибка экспорта истории входов:', error)
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить историю входов', type: 'error' })
      } finally {
        this.isExporting = false
      }
    },
    async buildWorkbook(rows) {
      const workbook = new ExcelJS.Workbook()
      const worksheet = workbook.addWorksheet(`Vhody_${this.safeUsername()}`)

      const headers = ['Дата и время', 'Событие', 'Результат', 'IP-адрес', 'Устройство', 'Детали']
      const headerRow = worksheet.addRow(headers)
      headerRow.height = 25
      headerRow.eachCell((cell) => {
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } }
        cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } }
        cell.alignment = { vertical: 'middle', horizontal: 'center' }
        cell.border = thinBorder()
      })

      rows.forEach((item, index) => {
        const row = worksheet.addRow([
          formatDateTime(item.created_at),
          this.eventBadge(item.event_type).label,
          item.success ? 'Успешно' : 'Отказ',
          item.ip_address || '—',
          formatDevice(item.user_agent),
          item.detail || '—',
        ])
        row.height = 20
        const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF'
        row.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } }
          cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } }
          cell.alignment = { vertical: 'middle', wrapText: true }
          cell.border = thinBorder()
        })
      })

      worksheet.addRow([])
      const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserName || 'Пользователь'])
      const infoRow2 = worksheet.addRow(['Дата формирования:', formatDateTime(new Date().toISOString())])
      ;[infoRow1, infoRow2].forEach((row) => {
        row.eachCell((cell) => {
          cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } }
          cell.alignment = { vertical: 'middle' }
          cell.border = thinBorder()
        })
      })

      worksheet.columns = [
        { width: 20 },
        { width: 22 },
        { width: 12 },
        { width: 18 },
        { width: 26 },
        { width: 40 },
      ]

      const buffer = await workbook.xlsx.writeBuffer()
      const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.download = `Istoriya_vhodov_${this.safeUsername()}.xlsx`
      a.href = url
      a.click()
      window.URL.revokeObjectURL(url)
    },
    safeUsername() {
      return (this.username || 'user').replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_')
    },
  },
}

function thinBorder() {
  const side = { style: 'thin', color: { argb: 'FFE6E6E6' } }
  return { top: side, bottom: side, left: side, right: side }
}
</script>

<style scoped>
.login-history {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.lh-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.lh-search {
  position: relative;
  flex: 1 1 200px;
  min-width: 160px;
}

.lh-search__icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 14px;
  height: 14px;
  opacity: 0.45;
  pointer-events: none;
}

.lh-search__input {
  width: 100%;
  padding-left: 34px;
}

.lh-category {
  min-width: 170px;
}

.lh-dates {
  display: flex;
  align-items: center;
  gap: 6px;
}

.lh-date {
  width: 150px;
}

.lh-dates__sep {
  color: #a2a2a2;
}

.lh-export {
  margin-left: auto;
}

.lh-table-wrap {
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  overflow: auto;
  max-height: 320px;
}

.lh-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.lh-table thead th {
  position: sticky;
  top: 0;
  z-index: 1;
  text-align: left;
  padding: 11px 14px;
  background: #f8fafc;
  border-bottom: 1px solid #e6e6e6;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #a2a2a2;
  white-space: nowrap;
}

.lh-table tbody td {
  padding: 10px 14px;
  border-bottom: 1px solid #f0f0f0;
  vertical-align: middle;
}

.lh-table tbody tr:last-child td {
  border-bottom: none;
}

.lh-table tbody tr:hover {
  background: #fafbff;
}

.lh-when {
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  color: #333;
}

.lh-ip {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 12px;
  color: #6b6f8a;
}

.lh-device__browser {
  color: #333;
}

.lh-device__os {
  display: block;
  color: #a2a2a2;
  font-size: 11px;
}

.lh-detail {
  color: #6b6f8a;
  font-size: 12.5px;
}

.lh-muted {
  color: #a2a2a2;
}

.lh-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.lh-badge::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.lh-badge--ok { background: #dcfce7; color: #15803d; }
.lh-badge--neutral { background: #eef1f6; color: #475569; }
.lh-badge--danger { background: #fee2e2; color: #b91c1c; }
.lh-badge--warn { background: #fef3c7; color: #92400e; }
.lh-badge--muted { background: #eef2ff; color: #6366f1; }

.lh-empty {
  text-align: center;
  padding: 36px 12px;
  color: #a2a2a2;
}

.lh-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.lh-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
  font-size: 11.5px;
  color: #6b6f8a;
}

.lh-legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.lh-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.lh-dot--ok { background: #15803d; }
.lh-dot--neutral { background: #475569; }
.lh-dot--danger { background: #b91c1c; }
.lh-dot--warn { background: #92400e; }

.lh-pager {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: #6b6f8a;
}

.lh-page-num {
  font-variant-numeric: tabular-nums;
}

@media (max-width: 640px) {
  .lh-export {
    margin-left: 0;
  }

  .lh-footer {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
