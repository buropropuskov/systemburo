<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        data-testid="blacklist-history-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="blacklist-history-modal"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>{{ title }}</h3>
            <div class="header-actions">
              <button
                class="export-btn"
                :disabled="filteredHistory.length === 0 || isExporting"
                @click="exportToExcel"
              >
                <AppIcon
                  v-if="!isExporting"
                  name="export"
                  class="export-icon"
                />
                <span v-if="!isExporting">Экспорт</span>
                <div
                  v-else
                  class="export-loader"
                />
              </button>
              <button
                class="close-btn"
                aria-label="Закрыть"
                @click="close"
              >
                ×
              </button>
            </div>
          </div>

          <div class="history-filters">
            <div class="filter-row">
              <div class="search-filter">
                <span class="filter-label">Поиск:</span>
                <input
                  v-model="searchQuery"
                  type="text"
                  class="search-input"
                  placeholder="Поиск по пользователю, действию..."
                >
              </div>

              <div class="user-filter">
                <span class="filter-label">Пользователь:</span>
                <div
                  ref="userSelect"
                  class="custom-select"
                  @click="toggleUserDropdown"
                >
                  <div class="select-trigger">
                    <span class="selected-value">{{ selectedUserName }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ 'arrow-open': userDropdownOpen }"
                    />
                  </div>
                  <transition name="fade">
                    <div
                      v-if="userDropdownOpen"
                      class="select-dropdown"
                    >
                      <div
                        class="select-option"
                        :class="{ selected: selectedUserId === null }"
                        @click.stop="selectUser(null)"
                      >
                        Все пользователи
                      </div>
                      <div
                        v-for="user in uniqueUsers"
                        :key="user.id"
                        class="select-option"
                        :class="{ selected: selectedUserId === user.id }"
                        @click.stop="selectUser(user.id)"
                      >
                        {{ user.name }}
                      </div>
                    </div>
                  </transition>
                </div>
              </div>

              <div class="date-filter">
                <span class="filter-label">Период:</span>
                <input
                  v-model="dateFrom"
                  type="date"
                  class="date-input"
                >
                <span class="date-separator">-</span>
                <input
                  v-model="dateTo"
                  type="date"
                  class="date-input"
                >
              </div>

              <div class="sort-filter">
                <span class="filter-label">Сортировка:</span>
                <button
                  class="sort-btn"
                  @click="toggleSortOrder"
                >
                  <AppIcon
                    name="sort"
                    class="sort-icon"
                    :class="{ 'sort-asc': sortOrder === 'asc' }"
                  />
                  <span>{{ sortOrder === 'desc' ? 'Сначала новые' : 'Сначала старые' }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="modal-content">
            <div
              v-if="loading"
              class="history-loading"
            >
              <LoaderSpinner label="Загрузка истории..." />
            </div>

            <div
              v-else-if="filteredHistory.length === 0"
              class="history-empty"
            >
              История пуста
            </div>

            <div
              v-else
              class="history-timeline"
            >
              <template
                v-for="group in historyGroupedByDate"
                :key="group.date"
              >
                <div class="history-date-separator">
                  {{ group.date }}
                </div>
                <div
                  v-for="(item, i) in group.items"
                  :key="item.id"
                  class="history-item"
                >
                  <div
                    class="timeline-dot"
                    :class="getActionClass(item.action_type)"
                  />
                  <div
                    v-if="i < group.items.length - 1"
                    class="timeline-line"
                  />

                  <div class="history-content">
                    <div
                      v-if="getEntityLabel(item)"
                      class="history-entity"
                    >
                      {{ getEntityLabel(item) }}
                    </div>

                    <div class="history-header">
                      <span class="user-name">{{ item.user_name || 'Система' }}</span>
                      <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                    </div>

                    <div class="action-text">
                      {{ getActionText(item) }}
                    </div>

                    <div
                      v-if="getReasonDiff(item)"
                      class="action-diff"
                    >
                      <span class="diff-old">{{ getReasonDiff(item).from }}</span>
                      <svg
                        class="diff-arrow"
                        viewBox="0 0 24 24"
                        width="16"
                        height="16"
                        aria-hidden="true"
                      >
                        <path
                          d="M4 12h14M13 6l6 6-6 6"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                          stroke-linecap="round"
                          stroke-linejoin="round"
                        />
                      </svg>
                      <span class="diff-new">{{ getReasonDiff(item).to }}</span>
                    </div>

                    <div
                      v-else-if="getActionComment(item)"
                      class="action-comment"
                    >
                      {{ getActionComment(item) }}
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useDeletionsStore } from '@/stores/deletions';
import { formatDateTime } from '@/utils/datetime';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import ExcelJS from 'exceljs';

const ACTION_DOT_CLASS = {
  created: 'dot-create',
  archived: 'dot-deactivate',
  restored: 'dot-activate',
  updated: 'dot-update',
  purged: 'dot-delete',
};

/**
 * Базовая модалка истории записи чёрного списка (#443). Структура и оформление -
 * по образцу SystemTableHistoryModal (Teleport, фильтры в строку, timeline,
 * ExcelJS-экспорт). Сущность-специфика (заголовок, тексты действий, лейблы полей
 * details, загрузчик) приходит через props - так машины и люди делят один layout.
 */
export default {
  name: 'BlacklistHistoryModalBase',
  components: { LoaderSpinner, AppIcon },
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, required: true },
    /** Имя записи для имени файла экспорта (санируется). */
    entityLabel: { type: String, default: 'zapis' },
    /** Загрузчик истории: () => Promise<Array>. */
    loadFn: { type: Function, required: true },
    /** action_type -> человекочитаемый текст действия. */
    actionTexts: { type: Object, required: true },
    /** ключ поля details -> лейбл; показываются только присутствующие тут ключи. */
    fieldLabels: { type: Object, default: () => ({}) },
    /**
     * Опц. (item) => string - лейбл сущности (номер+марка / ФИО) для общего журнала ЧС.
     * Берётся из item.details, поэтому показывается и для физически удалённых записей.
     */
    entityLabelFn: { type: Function, default: null },
    /** ключи details, которые НЕ показывать в комментарии (рендерятся отдельно - лейбл, diff). */
    commentExcludeKeys: { type: Array, default: () => [] },
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  setup(_, { emit }) {
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
    return { onOverlayMousedown, onOverlayMouseup };
  },
  data() {
    return {
      loading: false,
      history: [],
      sortOrder: 'desc',
      searchQuery: '',
      selectedUserId: null,
      dateFrom: '',
      dateTo: '',
      userDropdownOpen: false,
      isExporting: false,
    };
  },
  computed: {
    uniqueUsers() {
      const users = new Map();
      this.history.forEach((item) => {
        if (item.user_id && !users.has(item.user_id)) {
          users.set(item.user_id, { id: item.user_id, name: item.user_name || 'Система' });
        }
      });
      return Array.from(users.values()).sort((a, b) => a.name.localeCompare(b.name));
    },

    selectedUserName() {
      if (this.selectedUserId === null) return 'Все пользователи';
      const user = this.uniqueUsers.find((u) => u.id === this.selectedUserId);
      return user ? user.name : 'Все пользователи';
    },

    filteredHistory() {
      let filtered = [...this.history];

      if (this.searchQuery && this.searchQuery.trim() !== '') {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter((item) => {
          const userName = (item.user_name || '').toLowerCase();
          const actionText = this.getActionText(item).toLowerCase();
          const comment = this.getActionComment(item).toLowerCase();
          const entity = this.getEntityLabel(item).toLowerCase();
          const diff = this.getReasonDiff(item);
          const diffText = diff ? `${diff.from} ${diff.to}`.toLowerCase() : '';
          return userName.includes(query) || actionText.includes(query)
            || comment.includes(query) || entity.includes(query) || diffText.includes(query);
        });
      }

      if (this.selectedUserId) {
        filtered = filtered.filter((item) => item.user_id === this.selectedUserId);
      }

      if (this.dateFrom) {
        const fromDate = new Date(this.dateFrom);
        fromDate.setHours(0, 0, 0, 0);
        filtered = filtered.filter((item) => new Date(item.created_at) >= fromDate);
      }

      if (this.dateTo) {
        const toDate = new Date(this.dateTo);
        toDate.setHours(23, 59, 59, 999);
        filtered = filtered.filter((item) => new Date(item.created_at) <= toDate);
      }

      return filtered.sort((a, b) => {
        const timeA = new Date(a.created_at).getTime();
        const timeB = new Date(b.created_at).getTime();
        return this.sortOrder === 'desc' ? timeB - timeA : timeA - timeB;
      });
    },

    historyGroupedByDate() {
      const groups = [];
      const seen = new Map();
      this.filteredHistory.forEach((item) => {
        const date = new Date(item.created_at).toLocaleDateString('ru-RU', {
          day: 'numeric', month: 'long', year: 'numeric',
        });
        if (!seen.has(date)) {
          const group = { date, items: [] };
          groups.push(group);
          seen.set(date, group);
        }
        seen.get(date).items.push(item);
      });
      return groups;
    },

    formattedCurrentDateTime() {
      const now = new Date();
      return now.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      }).replace(',', '');
    },

    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter((p) => p && p !== 'null' && p !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    },

    safeEntityLabel() {
      const name = this.entityLabel || 'zapis';
      return name.replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_');
    },

    exportData() {
      const hasEntity = !!this.entityLabelFn;
      return this.filteredHistory.map((item) => {
        const row = {
          'Дата и время': this.formatDateTime(item.created_at),
          Пользователь: item.user_name || 'Система',
        };
        if (hasEntity) row['Объект'] = this.getEntityLabel(item) || '-';
        row['Действие'] = this.getActionText(item);
        row['Детали'] = this.getDetailText(item);
        row['Тип действия'] = item.action_type;
        row['ID записи'] = item.id;
        return row;
      });
    },
  },
  watch: {
    // Модалка всегда смонтирована (для leave-анимации): историю грузим при
    // открытии, а не на mount. immediate - чтобы загрузка сработала и когда
    // модалку сразу монтируют с show=true (guard не грузит при закрытой).
    show: {
      immediate: true,
      handler(visible) {
        if (visible) this.loadHistory();
      },
    },
  },
  mounted() {
    document.addEventListener('click', this.handleClickOutside);
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    formatDateTime,
    onKeydown(e) {
      if (!this.show) return;
      if (e.key === 'Escape') this.close();
    },
    close() {
      this.$emit('close');
    },
    handleClickOutside(event) {
      // Через Teleport this.$el - anchor-комментарий без querySelector, поэтому
      // селект ищем по template-ref (он резолвится в реальный DOM-узел в body).
      const select = this.$refs.userSelect;
      if (select && !select.contains(event.target)) {
        this.userDropdownOpen = false;
      }
    },

    async loadHistory() {
      this.loading = true;
      try {
        const data = await this.loadFn();
        this.history = Array.isArray(data) ? data : [];
      } catch (error) {
        console.error('Error loading blacklist history:', error);
        this.history = [];
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю', type: 'error' });
      } finally {
        this.loading = false;
      }
    },

    getActionClass(actionType) {
      return ACTION_DOT_CLASS[actionType] || 'dot-default';
    },

    getActionText(item) {
      return this.actionTexts[item.action_type] || item.action_type;
    },

    /** Лейбл сущности (номер+марка / ФИО) для строки общего журнала. */
    getEntityLabel(item) {
      if (!this.entityLabelFn) return '';
      try {
        return this.entityLabelFn(item) || '';
      } catch {
        return '';
      }
    },

    /**
     * Диф причины для action=updated: { from, to }. Рендерится отдельной строкой со
     * стрелкой (#1) вместо текстового "Было: / Стало:". null - если это не правка причины.
     */
    getReasonDiff(item) {
      if (item.action_type !== 'updated') return null;
      const d = item.details;
      if (!d || typeof d !== 'object') return null;
      if (!('reason_old' in d) && !('reason_new' in d)) return null;
      return {
        from: d.reason_old ? String(d.reason_old) : '-',
        to: d.reason_new ? String(d.reason_new) : '-',
      };
    },

    /**
     * Читаемые детали для timeline. Показываем только ключи из fieldLabels,
     * пропуская пустые значения, нулевые счётчики (иначе "Деактивировано: 0" - шум) и
     * ключи из commentExcludeKeys (лейбл сущности и причина-диф рендерятся отдельно).
     */
    getActionComment(item) {
      const d = item.details;
      if (!d || typeof d !== 'object') return '';
      const parts = [];
      // Порядок - по объявлению в fieldLabels (а не по ключам details: jsonb не хранит
      // порядок, иначе поля показывались бы в произвольном порядке).
      for (const key of Object.keys(this.fieldLabels)) {
        if (this.commentExcludeKeys.includes(key)) continue;
        if (!(key in d)) continue;
        const raw = d[key];
        if (raw === null || raw === undefined || raw === '') continue;
        if (typeof raw === 'number' && raw === 0) continue;
        parts.push(`${this.fieldLabels[key]}: ${this.formatFieldValue(raw)}`);
      }
      return parts.join(' / ');
    },

    formatFieldValue(value) {
      if (typeof value === 'boolean') return value ? 'да' : 'нет';
      const s = String(value);
      return s.length > 80 ? `${s.slice(0, 80)}...` : s;
    },

    /** Детали для экспорта: комментарий + причина-диф (в Excel стрелку рисовать нечем). */
    getDetailText(item) {
      const diff = this.getReasonDiff(item);
      const comment = this.getActionComment(item);
      if (diff) {
        const d = `Причина: ${diff.from} -> ${diff.to}`;
        return comment ? `${comment} / ${d}` : d;
      }
      return comment;
    },

    toggleSortOrder() {
      this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
    },

    toggleUserDropdown() {
      this.userDropdownOpen = !this.userDropdownOpen;
    },

    selectUser(userId) {
      this.selectedUserId = userId;
      this.userDropdownOpen = false;
    },

    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      this.isExporting = true;
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('История');

        const headers = Object.keys(this.exportData[0]);
        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          };
        });

        this.exportData.forEach((item, index) => {
          const row = worksheet.addRow(headers.map((h) => item[h]));
          row.height = 20;
          const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          row.eachCell((cell) => {
            cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
            cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle', wrapText: true };
            cell.border = {
              top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            };
          });
        });

        const lastDataRow = this.exportData.length;
        const cols = headers.length;
        for (let row = 1; row <= lastDataRow + 1; row++) {
          const rightCell = worksheet.getCell(row, cols);
          rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
          const leftCell = worksheet.getCell(row, 1);
          leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        for (let col = 1; col <= cols; col++) {
          const topCell = worksheet.getCell(1, col);
          topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
          const bottomCell = worksheet.getCell(lastDataRow + 1, col);
          bottomCell.border = { ...bottomCell.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
        }

        worksheet.addRow([]);
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserDisplayName]);
        const infoRow2 = worksheet.addRow(['Дата формирования:', this.formattedCurrentDateTime]);
        [infoRow1, infoRow2].forEach((row) => {
          row.eachCell((cell) => {
            cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
            cell.border = {
              top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            };
          });
        });

        const colWidths = {
          'Дата и время': 22,
          Пользователь: 30,
          Объект: 32,
          Действие: 26,
          Детали: 60,
          'Тип действия': 18,
          'ID записи': 12,
        };
        worksheet.columns = headers.map((h) => ({ width: colWidths[h] || 20 }));

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `Istoriya_chs_${this.safeEntityLabel}_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (error) {
        console.error('Error exporting blacklist history to Excel:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить историю', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 12000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.blacklist-history-modal {
  background: var(--surface);
  border-radius: 30px;
  width: 900px;
  max-width: 95%;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px var(--shadow-drop);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 25px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.export-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 6px 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
}

.export-btn:hover:not(:disabled) {
  background: var(--surface-2);
  border-color: var(--accent);
}

.export-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.export-icon {
  width: 14px;
  height: 14px;
}

.export-loader {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border);
  border-top: 2px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--text-muted);
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: var(--surface-2);
  color: var(--text);
}

.history-filters {
  padding: 15px 25px;
  border-bottom: 1px solid var(--border);
  background-color: var(--surface-2);
}

.filter-row {
  display: flex;
  gap: 15px;
  align-items: center;
  flex-wrap: wrap;
}

.search-filter,
.user-filter,
.date-filter,
.sort-filter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
}

.search-input {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  width: 200px;
  height: 32px;
  transition: all 0.2s ease;
}

.search-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.custom-select {
  position: relative;
  width: 200px;
  cursor: pointer;
}

.select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  transition: all 0.2s ease;
  height: 32px;
}

.select-trigger:hover {
  border-color: var(--accent);
  background: var(--surface-2);
}

.selected-value {
  font-size: 12px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.select-arrow {
  width: 8px;
  height: 8px;
  transition: transform 0.2s ease;
}

.select-arrow.arrow-open {
  transform: rotate(90deg);
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from, .fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 300px;
  overflow-y: auto;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  box-shadow: 0 4px 12px var(--shadow-drop);
  z-index: 1000;
}

.select-option {
  padding: 10px 14px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.select-option:hover {
  background-color: var(--accent-tint);
}

.select-option.selected {
  background-color: var(--accent-tint);
  font-weight: 500;
}

.date-input {
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 12px;
  width: 120px;
  height: 32px;
}

.date-separator {
  color: var(--text-muted);
  font-size: 12px;
}

.sort-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
  width: 170px;
}

.sort-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.sort-icon {
  color: var(--text-muted);
  width: 14px;
  height: 14px;
  transition: transform 0.2s ease;
}

.sort-icon.sort-asc {
  transform: rotate(180deg);
}

.modal-content {
  padding: 20px 25px;
  overflow-y: auto;
  max-height: calc(var(--app-vh, 1vh) * 80 - 180px);
  position: relative;
}

.history-loading,
.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: var(--text-muted);
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.history-timeline {
  position: relative;
  padding-left: 20px;
  min-height: 100px;
}

.history-date-separator {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-text);
  padding: 8px 0 4px;
  margin-bottom: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  letter-spacing: 0.02em;
}

.history-item {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  position: relative;
}

.history-item:last-child {
  margin-bottom: 0;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 4px;
  z-index: 1;
  position: relative;
}

.timeline-line {
  position: absolute;
  left: 4px;
  top: 18px;
  width: 2px;
  height: calc(100% + 2px);
  background: var(--border);
}

.dot-create { background: var(--accent-text); }
.dot-update { background: #f59e0b; }
.dot-activate { background: #10b981; }
.dot-deactivate { background: #6b7280; }
.dot-delete { background: #dc2626; }
.dot-default { background: #9ca3af; }

.history-content {
  flex: 1;
  min-width: 0;
}

.history-entity {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 4px;
  word-break: break-word;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
}

.user-name {
  font-weight: 500;
  color: var(--text);
  font-size: 13px;
}

.action-time {
  color: var(--text-muted);
  font-size: 11px;
}

.action-text {
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 4px;
  padding-left: 6px;
  border-left: 2px solid var(--border);
  word-break: break-word;
}

.action-diff {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 6px;
  font-size: 12px;
}

.diff-old,
.diff-new {
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  word-break: break-word;
}

.diff-old {
  background: var(--accent-tint);
  color: var(--text-muted);
  text-decoration: line-through;
}

.diff-new {
  background: var(--success-bg);
  color: var(--success-text);
  font-weight: 500;
}

.diff-arrow {
  flex-shrink: 0;
  color: var(--text-muted);
}

@media (max-width: 768px) {
  /* Bottom-sheet: hand-rolled модалка (не BaseModal) - паттерн скопирован 1:1 с
     BaseModal/App.vue/ApplicationHistory (align-items:flex-end + 90dvh + скруглённый
     только верх), см. эталон в responsive-tables.css и уроки проекта про Teleport-модалки. */
  .modal-overlay {
    padding: 0;
    align-items: flex-end;
  }

  .blacklist-history-modal {
    width: 100vw;
    max-width: 100vw;
    max-height: 90dvh;
    border-radius: 16px 16px 0 0;
    margin: 0;
  }

  .close-btn {
    min-width: 44px;
    min-height: 44px;
  }

  /* Фильтры на мобильном стекаются в колонку (4 поля вместо 1 строки) - фиксированный
     calc(80vh - 180px) desktop-формулы modal-content больше не подходит; отдаём
     content flex-остаток внутри max-height:90dvh колонки .blacklist-history-modal. */
  .modal-content {
    flex: 1;
    min-height: 0;
    max-height: none;
  }

  .filter-row {
    flex-direction: column;
    align-items: flex-start;
  }
  .search-filter,
  .user-filter,
  .date-filter,
  .sort-filter {
    width: 100%;
  }
  .custom-select,
  .search-input,
  .date-input,
  .sort-btn {
    width: 100%;
  }
  .date-input {
    width: calc(50% - 20px);
  }
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
