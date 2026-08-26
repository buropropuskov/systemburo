<template>
  <div class="login-history">
    <div class="lh-filters">
      <div class="lh-search">
        <AppIcon
          name="search"
          class="lh-search__icon"
        />
        <input
          v-model="search"
          class="lk-input lh-search__input"
          type="text"
          placeholder="Поиск"
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
      <DateFilter
        ref="dateFilter"
        class="lh-datefilter"
        :mode="'range'"
        :selected-date="selectedDate"
        :date-range-start="dateRangeStart"
        :date-range-end="dateRangeEnd"
        @update:selected-date="updateSelectedDate"
        @update:date-range-start="updateDateRangeStart"
        @update:date-range-end="updateDateRangeEnd"
        @apply="applyDateFilters"
        @clear="clearDateRange"
      />
      <button
        type="button"
        class="lk-button lk-button--ghost lh-reset"
        :disabled="!hasActiveFilters"
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
          <template
            v-for="row in displayRows"
            :key="row.key"
          >
            <!-- Обычное событие -->
            <tr v-if="row.kind === 'row'">
              <td class="lh-when">
                {{ formatDateTime(row.event.created_at) }}
              </td>
              <td>
                <span
                  class="lh-badge"
                  :class="eventBadge(row.event.event_type).cls"
                >
                  {{ eventBadge(row.event.event_type).label }}
                </span>
              </td>
              <td class="lh-ip">
                {{ row.event.ip_address || '—' }}
              </td>
              <td class="lh-device">
                <span
                  v-if="deviceParts(row.event.user_agent).browser || deviceParts(row.event.user_agent).os"
                  class="lh-device__browser"
                >{{ deviceParts(row.event.user_agent).browser || 'Браузер' }}<small
                  v-if="deviceParts(row.event.user_agent).os"
                  class="lh-device__os"
                >{{ deviceParts(row.event.user_agent).os }}</small></span>
                <span
                  v-else
                  class="lh-muted"
                >—</span>
              </td>
              <td class="lh-detail">
                {{ formatDetail(row.event.detail) }}
              </td>
            </tr>
            <!-- Группа подряд идущих обновлений сессии -->
            <template v-else>
              <tr
                class="lh-group-row"
                :class="{ 'lh-group-row--open': isExpanded(row.key) }"
                @click="toggleGroup(row.key)"
              >
                <td class="lh-when">
                  {{ formatDateTime(row.events[0].created_at) }}
                  <small class="lh-when__more">и ещё {{ row.count - 1 }}</small>
                </td>
                <td>
                  <span class="lh-event-group">
                    <span class="lh-badge lh-badge--muted">Сессия обновлена</span>
                    <span class="lh-count">×{{ row.count }}</span>
                  </span>
                </td>
                <td class="lh-ip">
                  {{ groupCommon(row, 'ip') || '—' }}
                </td>
                <td class="lh-device">
                  <span
                    v-if="groupCommon(row, 'device')"
                    class="lh-device__browser"
                  >{{ groupCommon(row, 'device') }}</span>
                  <span
                    v-else
                    class="lh-muted"
                  >разные</span>
                </td>
                <td class="lh-detail lh-group-toggle">
                  <span
                    class="lh-chevron"
                    :class="{ 'lh-chevron--open': isExpanded(row.key) }"
                  >▸</span>
                  {{ isExpanded(row.key) ? 'Скрыть' : 'Показать' }}
                </td>
              </tr>
              <tr
                v-for="ev in (isExpanded(row.key) ? row.events : [])"
                :key="ev.id"
                class="lh-subrow"
              >
                <td class="lh-when">
                  {{ formatDateTime(ev.created_at) }}
                </td>
                <td>
                  <span class="lh-badge lh-badge--muted">Сессия обновлена</span>
                </td>
                <td class="lh-ip">
                  {{ ev.ip_address || '—' }}
                </td>
                <td class="lh-device">
                  <span
                    v-if="deviceParts(ev.user_agent).browser || deviceParts(ev.user_agent).os"
                    class="lh-device__browser"
                  >{{ deviceParts(ev.user_agent).browser || 'Браузер' }}<small
                    v-if="deviceParts(ev.user_agent).os"
                    class="lh-device__os"
                  >{{ deviceParts(ev.user_agent).os }}</small></span>
                  <span
                    v-else
                    class="lh-muted"
                  >—</span>
                </td>
                <td class="lh-detail">
                  {{ formatDetail(ev.detail) }}
                </td>
              </tr>
            </template>
          </template>
          <tr v-if="displayRows.length === 0">
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
      <Pager
        class="lh-pager"
        :page="page"
        :total-pages="totalPages"
        :total="total"
        :loading="loading"
        @update:page="goToPage"
      >
        <template #lead>
          <span
            v-if="isLiveHead"
            class="lh-live"
            title="Список обновляется автоматически"
          ><i class="lh-live__dot" />в реальном времени</span>
        </template>
      </Pager>
    </div>
  </div>
</template>

<script>
import ExcelJS from 'exceljs'
import BaseDropdown from './ui/BaseDropdown.vue'
import Pager from './ui/Pager.vue'
import DateFilter from './DateFilter.vue'
import { getUserAuthEvents } from '@/api/users'
import { formatDateTime } from '@/utils/datetime'
import { parseUserAgent, formatDevice } from '@/utils/userAgent'
import { detailLabel } from '@/utils/authEventDetail'
import { useDeletionsStore } from '@/stores/deletions'
import AppIcon from '@/components/icons/AppIcon.vue';

// Маппинг типа события в человекочитаемый бейдж. Сырые коды (login_success и т.п.)
// в интерфейс не попадают - только понятные подписи с цветом по смыслу.
const EVENT_BADGES = {
  login_success: { cls: 'lh-badge--ok', label: 'Вход выполнен' },
  logout: { cls: 'lh-badge--neutral', label: 'Выход' },
  login_failed: { cls: 'lh-badge--danger', label: 'Неудачный вход' },
  login_locked: { cls: 'lh-badge--warn', label: 'Вход заблокирован' },
  account_locked: { cls: 'lh-badge--warn', label: 'Аккаунт заблокирован' },
  lockout_reset: { cls: 'lh-badge--ok', label: 'Блокировка снята' },
  refresh: { cls: 'lh-badge--muted', label: 'Сессия обновлена' },
  token_reuse_detected: { cls: 'lh-badge--danger', label: 'Подозрительная сессия' },
}

// Экспорт тянет до этого числа последних событий по текущим фильтрам (лимит бэка).
const EXPORT_LIMIT = 100
// Период опроса живой ленты (первая страница без фильтров) для real-time обновления.
const POLL_INTERVAL_MS = 15000

// toYMD - дата в 'YYYY-MM-DD' по ЛОКАЛЬНЫМ частям (не toISOString: UTC-сдвиг увёл бы
// выбранный день на предыдущий у пользователей восточнее UTC). Бэк трактует день в МСК.
function toYMD(d) {
  if (!(d instanceof Date)) return ''
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export default {
  name: 'UserLoginHistory',
  components: { AppIcon, BaseDropdown, DateFilter, Pager },
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
      filters: { category: '' },
      selectedDate: null,
      dateRangeStart: null,
      dateRangeEnd: null,
      expanded: {},
      fetchSeq: 0,
      pollTimer: null,
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
    apiFrom() {
      return toYMD(this.selectedDate || this.dateRangeStart)
    },
    apiTo() {
      return toYMD(this.selectedDate || this.dateRangeEnd)
    },
    hasActiveFilters() {
      return this.filters.category !== ''
        || !!this.selectedDate || !!this.dateRangeStart || !!this.dateRangeEnd
        || this.search !== ''
    },
    // Живая лента: первая страница без фильтров - только её опрашиваем в реальном времени,
    // чтобы не дёргать бэк и не сбивать админа, листающего/фильтрующего историю.
    isLiveHead() {
      return this.page === 1
        && this.filters.category === ''
        && !this.selectedDate && !this.dateRangeStart && !this.dateRangeEnd
        && this.search === ''
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
    // Схлопывает подряд идущие refresh (≥2) в одну раскрываемую группу; любое иное
    // событие обрывает группу. Одиночный refresh остаётся обычной строкой.
    displayRows() {
      const out = []
      let run = []
      const flush = () => {
        if (run.length >= 2) {
          out.push({ kind: 'group', key: `g:${run[0].id}:${run[run.length - 1].id}`, events: run.slice(), count: run.length })
        } else if (run.length === 1) {
          out.push({ kind: 'row', key: `r:${run[0].id}`, event: run[0] })
        }
        run = []
      }
      for (const ev of this.visibleItems) {
        if (ev.event_type === 'refresh') {
          run.push(ev)
        } else {
          flush()
          out.push({ kind: 'row', key: `r:${ev.id}`, event: ev })
        }
      }
      flush()
      return out
    },
  },
  watch: {
    // Смена пользователя (переоткрытие карточки на другом юзере) - сброс и перезагрузка.
    username() {
      this.resetState()
      this.fetch()
    },
  },
  mounted() {
    this.fetch()
    this.pollTimer = setInterval(this.pollTick, POLL_INTERVAL_MS)
  },
  beforeUnmount() {
    if (this.pollTimer) {
      clearInterval(this.pollTimer)
      this.pollTimer = null
    }
  },
  methods: {
    formatDateTime,
    deviceParts(ua) {
      return parseUserAgent(ua)
    },
    eventBadge(eventType) {
      return EVENT_BADGES[eventType] || { cls: 'lh-badge--neutral', label: eventType || '—' }
    },
    formatDetail(detail) {
      return detailLabel(detail) || '—'
    },
    isExpanded(key) {
      return !!this.expanded[key]
    },
    toggleGroup(key) {
      this.expanded = { ...this.expanded, [key]: !this.expanded[key] }
    },
    // Общее значение поля по всей группе (IP/устройство), иначе null.
    groupCommon(group, field) {
      const val = (e) => (field === 'ip' ? (e.ip_address || '') : formatDevice(e.user_agent))
      const first = val(group.events[0])
      return group.events.every((e) => val(e) === first) ? first : null
    },
    resetState() {
      this.page = 1
      this.search = ''
      this.filters.category = ''
      this.selectedDate = null
      this.dateRangeStart = null
      this.dateRangeEnd = null
      this.expanded = {}
    },
    // Тихий опрос живой ленты. Не трогает loading/тосты и не чистит данные при сбое,
    // чтобы обновление в фоне не мигало спиннером и не гасило таблицу (см. #840).
    pollTick() {
      if (!this.isLiveHead || this.loading || this.isExporting) return
      this.fetch(true)
    },
    async fetch(silent = false) {
      // seq-токен: параллельные вызовы (опрос + ручной фильтр/страница) не должны
      // затирать друг друга устаревшим ответом (last-resolve-wins), см. #632/#840.
      const seq = ++this.fetchSeq
      if (!silent) this.loading = true
      try {
        const data = await getUserAuthEvents(this.username, {
          page: this.page,
          limit: this.limit,
          category: this.filters.category,
          from: this.apiFrom,
          to: this.apiTo,
        }) || {}
        if (seq !== this.fetchSeq) return
        this.items = Array.isArray(data.items) ? data.items : []
        this.total = typeof data.total === 'number' ? data.total : 0
      } catch (error) {
        if (seq !== this.fetchSeq) return
        // Тихий опрос при сбое сохраняет последние данные (без мигания пустотой).
        if (!silent) {
          console.error('Ошибка загрузки истории входов:', error)
          useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю входов', type: 'error' })
          this.items = []
          this.total = 0
        }
      } finally {
        if (!silent && seq === this.fetchSeq) this.loading = false
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
    updateSelectedDate(date) {
      this.selectedDate = date
    },
    updateDateRangeStart(date) {
      this.dateRangeStart = date
    },
    updateDateRangeEnd(date) {
      this.dateRangeEnd = date
    },
    applyDateFilters() {
      this.applyFilters()
    },
    clearDateRange() {
      this.selectedDate = null
      this.dateRangeStart = null
      this.dateRangeEnd = null
      this.applyFilters()
    },
    resetFilters() {
      this.resetState()
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
          from: this.apiFrom,
          to: this.apiTo,
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
          this.formatDetail(item.detail),
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
  /* Заполняем всю высоту вкладки: таблица тянется, футер (легенда+пагинация) прижат вниз. */
  flex: 1 1 auto;
  min-height: 0;
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
  stroke-width: 2.2;
  color: var(--text);
}

.lh-search__input {
  width: 100%;
  padding-left: 34px;
}

.lh-category {
  min-width: 170px;
}

.lh-datefilter {
  min-width: 200px;
}

.lh-export {
  margin-left: auto;
  /* Фиксированная ширина: "Экспорт" и "Готовим…" не должны менять размер кнопки. */
  min-width: 108px;
  justify-content: center;
}

.lh-table-wrap {
  border: 1px solid var(--border);
  border-radius: 15px;
  overflow: auto;
  /* Резервируем место под вертикальный скроллбар всегда: иначе его появление при
     раскрытии группы сессий сужает контент и колонки дёргаются влево. */
  scrollbar-gutter: stable;
  /* Тянется на доступную высоту вкладки, но с ПОТОЛКОМ: без него flex:1 1 auto при
     большом числе строк раздувает всю модалку за 92vh и футер уезжает за экран
     (flexbox-ловушка: overflow бесполезен, пока высота не ограничена). Потолок
     подобран так, чтобы фильтры+таблица+футер уложились в min-height вкладки (524px),
     дальше строки скроллятся ВНУТРИ таблицы, футер остаётся внизу окна. */
  flex: 1 1 auto;
  min-height: 140px;
  max-height: 395px;
}

.lh-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  /* Фиксированная раскладка: ширины колонок не пересчитываются от контента, поэтому
     раскрытие группы (подстроки с отступом) НЕ двигает соседние колонки. Ширины
     подобраны под самый широкий контент каждой колонки (see #1076). */
  table-layout: fixed;
}

.lh-col-when { width: 23%; }
.lh-col-event { width: 25%; }
.lh-col-ip { width: 16%; }
.lh-col-device { width: 17%; }
.lh-col-detail { width: 19%; }

.lh-table thead th {
  position: sticky;
  top: 0;
  z-index: 1;
  text-align: left;
  padding: 11px 14px;
  /* Непрозрачная поверхность, а не --accent-tint: тот полупрозрачный (0.22 в тёмной
     теме), и уезжающие под шапку строки читались сквозь неё. */
  background: var(--surface-2);
  border-bottom: 1px solid var(--border);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  white-space: nowrap;
}

.lh-table tbody td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}

.lh-table tbody tr:last-child td {
  border-bottom: none;
}

.lh-table tbody tr:hover {
  background: var(--accent-tint);
}

.lh-when {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-variant-numeric: tabular-nums;
  color: var(--text);
}

.lh-when__more {
  display: block;
  color: var(--text-muted);
  font-size: 11px;
}

.lh-ip {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.lh-device__browser {
  color: var(--text);
}

.lh-device__os {
  display: block;
  color: var(--text-muted);
  font-size: 11px;
}

.lh-detail {
  color: var(--text-muted);
  font-size: 12.5px;
}

.lh-muted {
  color: var(--text-muted);
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

/* Группа обновлений сессии */
.lh-group-row {
  cursor: pointer;
}

.lh-group-row:hover {
  background: var(--accent-tint);
}

/* Бейдж и счётчик ×N держим одним неразрывным блоком - иначе при 2+ значном N
   (×13) счётчик переносится под бейдж. */
.lh-event-group {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
}

.lh-count {
  margin-left: 6px;
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-text);
}

.lh-group-toggle {
  color: var(--accent-text);
  font-weight: 600;
  font-size: 12px;
  white-space: nowrap;
}

.lh-chevron {
  display: inline-block;
  margin-right: 4px;
  transition: transform 0.15s ease;
}

.lh-chevron--open {
  transform: rotate(90deg);
}

.lh-subrow td {
  background: var(--accent-tint);
  font-size: 12.5px;
}

.lh-subrow .lh-when {
  color: var(--text-muted);
}

.lh-empty {
  text-align: center;
  padding: 36px 12px;
  color: var(--text-muted);
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
  color: var(--text-muted);
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
  color: var(--text-muted);
}

.lh-live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--success-text);
}

.lh-live__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success);
  animation: lh-pulse 1.8s ease-in-out infinite;
}

@keyframes lh-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}

@media (prefers-reduced-motion: reduce) {
  .lh-live__dot {
    animation: none;
  }
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
