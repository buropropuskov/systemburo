<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      :class="{ 'modal-leaving': leaving }"
      data-testid="user-history-modal"
      @mousedown="onOverlayMousedown"
      @mouseup="onOverlayMouseup"
    >
      <div
        class="user-history-modal"
        @mousedown.stop
      >
        <div class="modal-header">
          <h3>История учётной записи «{{ user.username }}»</h3>
          <div class="header-actions">
            <button
              class="export-btn"
              :disabled="filteredHistory.length === 0 || isExporting"
              @click="exportToExcel"
            >
              <img
                v-if="!isExporting"
                src="@/assets/icons/export.png"
                class="export-icon"
              >
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
              <span class="filter-label">Кто:</span>
              <div
                class="custom-select"
                @click="toggleUserDropdown"
              >
                <div class="select-trigger">
                  <span class="selected-value">{{ selectedUserName }}</span>
                  <img
                    src="@/assets/icons/arrow.png"
                    class="select-arrow"
                    :class="{ 'arrow-open': userDropdownOpen }"
                  >
                </div>
                <transition name="fade">
                  <div
                    v-if="userDropdownOpen"
                    class="select-dropdown"
                  >
                    <div
                      class="select-option"
                      :class="{ 'selected': selectedActorId === null }"
                      @click.stop="selectUser(null)"
                    >
                      Все пользователи
                    </div>
                    <div
                      v-for="actor in uniqueUsers"
                      :key="actor.id"
                      class="select-option"
                      :class="{ 'selected': selectedActorId === actor.id }"
                      @click.stop="selectUser(actor.id)"
                    >
                      {{ actor.name }}
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
                <img
                  src="@/assets/icons/sort.png"
                  class="sort-icon"
                  :class="{ 'sort-asc': sortOrder === 'asc' }"
                >
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
            <div
              v-for="(item, index) in filteredHistory"
              :key="item.id"
              class="history-item"
            >
              <div
                class="timeline-dot"
                :class="getActionClass(item.action_type)"
              />
              <div
                v-if="index < filteredHistory.length - 1"
                class="timeline-line"
              />

              <div class="history-content">
                <div class="history-header">
                  <span class="user-name">{{ item.actor_name || 'Система' }}</span>
                  <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                </div>

                <div class="action-text">
                  {{ getActionText(item) }}
                </div>

                <div
                  v-if="getActionComment(item)"
                  class="action-comment"
                >
                  {{ getActionComment(item) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useDeletionsStore } from '@/stores/deletions';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import ExcelJS from 'exceljs';

const ACTION_TEXTS = {
  created: 'Учётная запись создана',
  updated: 'Изменены данные',
  type_changed: 'Изменён тип пользователя',
  org_changed: 'Изменена организация',
  company_changed: 'Изменена компания',
  password_reset: 'Сброшен пароль',
  archived: 'Учётная запись архивирована',
  restored: 'Учётная запись восстановлена из архива',
};

const ACTION_DOT_CLASS = {
  created: 'dot-create',
  updated: 'dot-update',
  type_changed: 'dot-update',
  org_changed: 'dot-update',
  company_changed: 'dot-update',
  password_reset: 'dot-update',
  archived: 'dot-deactivate',
  restored: 'dot-activate',
};

// Читаемые лейблы для полей в details (updated/created).
const FIELD_LABELS = {
  username: 'Логин',
  last_name: 'Фамилия',
  first_name: 'Имя',
  middle_name: 'Отчество',
  position: 'Должность',
  email: 'Email',
  phone: 'Телефон',
};

export default {
  name: 'UserHistoryModal',
  components: { LoaderSpinner },
  props: {
    user: { type: Object, required: true },
    organizations: { type: Array, default: () => [] },
    companies: { type: Array, default: () => [] },
    userTypes: { type: Array, default: () => [] },
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  setup() {
    // close() (с leave-анимацией) присваивается в created - чтобы overlay-клик уходил через неё.
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      loading: false,
      history: [],
      sortOrder: 'desc',
      searchQuery: '',
      selectedActorId: null,
      dateFrom: '',
      dateTo: '',
      userDropdownOpen: false,
      isExporting: false,
      leaving: false,
    };
  },
  computed: {
    uniqueUsers() {
      const users = new Map();
      this.history.forEach((item) => {
        if (item.actor_user_id && !users.has(item.actor_user_id)) {
          users.set(item.actor_user_id, {
            id: item.actor_user_id,
            name: item.actor_name || 'Система',
          });
        }
      });
      return Array.from(users.values()).sort((a, b) => a.name.localeCompare(b.name));
    },

    selectedUserName() {
      if (this.selectedActorId === null) return 'Все пользователи';
      const actor = this.uniqueUsers.find((u) => u.id === this.selectedActorId);
      return actor ? actor.name : 'Все пользователи';
    },

    filteredHistory() {
      let filtered = [...this.history];

      if (this.searchQuery && this.searchQuery.trim() !== '') {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter((item) => {
          const actorName = (item.actor_name || '').toLowerCase();
          const actionText = this.getActionText(item).toLowerCase();
          const comment = this.getActionComment(item).toLowerCase();
          return actorName.includes(query)
            || actionText.includes(query)
            || comment.includes(query);
        });
      }

      if (this.selectedActorId) {
        filtered = filtered.filter((item) => item.actor_user_id === this.selectedActorId);
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

    safeUserName() {
      const name = this.user.username || `id_${this.user.id}`;
      return name.replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_');
    },

    exportData() {
      return this.filteredHistory.map((item) => ({
        'Дата и время': this.formatDateTime(item.created_at),
        'Пользователь': item.actor_name || 'Система',
        'Действие': this.getActionText(item),
        'Детали': this.getActionComment(item),
        'Тип действия': item.action_type,
        'ID записи': item.id,
      }));
    },
  },
  created() {
    this.overlay.close = () => this.close();
  },
  mounted() {
    this.loadHistory();
    document.addEventListener('click', this.handleClickOutside);
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape') this.close();
    },
    close() {
      // leave-анимация: ставим флаг, эмитим close после её завершения.
      if (this.leaving) return;
      this.leaving = true;
      setTimeout(() => this.$emit('close'), 250);
    },
    handleClickOutside(event) {
      // Контент в Teleport -> ищем дропдаун в body (this.$el под Teleport = якорь-коммент).
      const select = document.querySelector('.user-history-modal .custom-select');
      if (select && !select.contains(event.target)) {
        this.userDropdownOpen = false;
      }
    },

    async loadHistory() {
      this.loading = true;
      try {
        const response = await apiRequest(`/users/${encodeURIComponent(this.user.username)}/history`);
        if (response.ok) {
          const data = await response.json();
          this.history = Array.isArray(data) ? data : [];
        } else {
          useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю пользователя', type: 'error' });
        }
      } catch (error) {
        console.error('Error loading user history:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю пользователя', type: 'error' });
      } finally {
        this.loading = false;
      }
    },

    getActionClass(actionType) {
      return ACTION_DOT_CLASS[actionType] || 'dot-default';
    },

    getActionText(item) {
      return ACTION_TEXTS[item.action_type] || item.action_type;
    },

    orgName(id) {
      if (id === null || id === undefined) return 'Не выбрано';
      const org = this.organizations.find((o) => o.id === id);
      return org ? org.name : `#${id}`;
    },

    companyName(id) {
      if (id === null || id === undefined) return 'Не выбрано';
      const company = this.companies.find((c) => c.id === id);
      return company ? company.name : `#${id}`;
    },

    typeName(id) {
      if (id === null || id === undefined) return '-';
      const type = this.userTypes.find((t) => t.id === id);
      return type ? type.name : `#${id}`;
    },

    /**
     * Читаемые детали для timeline. ID организации/компании/типа резолвим в имена
     * через переданные справочники; пустые/null поля не показываем.
     */
    getActionComment(item) {
      const d = item.details;
      if (!d || typeof d !== 'object') return '';

      switch (item.action_type) {
        case 'created': {
          const parts = [];
          if (d.username) parts.push(`Логин: ${d.username}`);
          if (d.type_id !== undefined && d.type_id !== null) parts.push(`Тип: ${this.typeName(d.type_id)}`);
          return parts.join(' / ');
        }
        case 'updated': {
          const parts = [];
          for (const [key, raw] of Object.entries(d)) {
            const label = FIELD_LABELS[key] || key;
            if (this.isDiff(raw)) {
              parts.push(`${label}: ${this.fieldOrDash(raw.old)} → ${this.fieldOrDash(raw.new)}`);
            } else if (raw !== null && raw !== undefined && raw !== '') {
              // старый формат записей (снапшот значения без old/new)
              parts.push(`${label}: ${this.formatFieldValue(raw)}`);
            }
          }
          return parts.join(' / ');
        }
        case 'type_changed':
          if (this.isDiff(d)) return `${this.typeName(d.old)} → ${this.typeName(d.new)}`;
          return `Новый тип: ${this.typeName(d.type_id)}`;
        case 'org_changed':
          if (this.isDiff(d)) return `${this.orgName(d.old)} → ${this.orgName(d.new)}`;
          return `Новая организация: ${this.orgName(d.organization_id)}`;
        case 'company_changed':
          if (this.isDiff(d)) return `${this.companyName(d.old)} → ${this.companyName(d.new)}`;
          return `Новая компания: ${this.companyName(d.company_id)}`;
        case 'password_reset':
        case 'archived':
        case 'restored':
        default:
          return '';
      }
    },

    isDiff(v) {
      return v !== null && typeof v === 'object' && ('old' in v || 'new' in v);
    },

    fieldOrDash(value) {
      if (value === null || value === undefined || value === '') return '—';
      return this.formatFieldValue(value);
    },

    formatFieldValue(value) {
      if (typeof value === 'boolean') return value ? 'да' : 'нет';
      const s = String(value);
      return s.length > 80 ? `${s.slice(0, 80)}...` : s;
    },

    formatDateTime(s) {
      if (!s) return '';
      const d = new Date(s);
      return d.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      }).replace(',', '');
    },

    toggleSortOrder() {
      this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
    },

    toggleUserDropdown() {
      this.userDropdownOpen = !this.userDropdownOpen;
    },

    selectUser(actorId) {
      this.selectedActorId = actorId;
      this.userDropdownOpen = false;
    },

    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      this.isExporting = true;
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet(`Istoriya_${this.safeUserName}`);

        const headers = ['Дата и время', 'Пользователь', 'Действие', 'Детали', 'Тип действия', 'ID записи'];
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
          const row = worksheet.addRow([
            item['Дата и время'],
            item['Пользователь'],
            item['Действие'],
            item['Детали'],
            item['Тип действия'],
            item['ID записи'],
          ]);
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

        worksheet.columns = [
          { width: 22 },
          { width: 30 },
          { width: 30 },
          { width: 60 },
          { width: 22 },
          { width: 12 },
        ];

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `Istoriya_polzovatelya_${this.safeUserName}_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (error) {
        console.error('Error exporting history to Excel:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить историю в Excel', type: 'error' });
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
  background: rgba(0, 0, 0, 0.5);
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

.user-history-modal {
  background: white;
  border-radius: 30px;
  width: 900px;
  max-width: 95%;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

/* Анимация закрытия: класс modal-leaving вешается на overlay перед эмитом close. */
.modal-overlay.modal-leaving {
  animation: fadeOut 0.25s ease-in forwards;
}

.modal-overlay.modal-leaving .user-history-modal {
  animation: slideDown 0.25s ease-in forwards;
}

@keyframes fadeOut {
  from { opacity: 1; }
  to { opacity: 0; }
}

@keyframes slideDown {
  from { transform: translateY(0); opacity: 1; }
  to { transform: translateY(20px); opacity: 0; }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 25px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #333;
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
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  font-size: 13px;
  color: #000;
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
}

.export-btn:hover:not(:disabled) {
  background: #f5f5f5;
  border-color: #4F5BDF;
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
  border: 2px solid #e6e6e6;
  border-top: 2px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: #a2a2a2;
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
  background: #f5f5f5;
  color: #333;
}

.history-filters {
  padding: 15px 25px;
  border-bottom: 1px solid #e6e6e6;
  background-color: #fafafa;
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
  color: #a2a2a2;
  white-space: nowrap;
}

.search-input {
  padding: 6px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  font-size: 12px;
  width: 200px;
  height: 32px;
  transition: all 0.2s ease;
}

.search-input:focus {
  outline: none;
  border-color: #4F5BDF;
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
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  transition: all 0.2s ease;
  height: 32px;
}

.select-trigger:hover {
  border-color: #4F5BDF;
  background: #f5f5f5;
}

.selected-value {
  font-size: 12px;
  color: #000;
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
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  z-index: 1000;
}

.select-option {
  padding: 10px 14px;
  font-size: 12px;
  color: #000;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.select-option:hover {
  background-color: #f0f3ff;
}

.select-option.selected {
  background-color: #f0f3ff;
  font-weight: 500;
}

.date-input {
  padding: 6px 8px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  font-size: 12px;
  width: 120px;
  height: 32px;
}

.date-separator {
  color: #a2a2a2;
  font-size: 12px;
}

.sort-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  font-size: 12px;
  color: #000;
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
  width: 170px;
}

.sort-btn:hover {
  background: #f5f5f5;
  border-color: #4F5BDF;
}

.sort-icon {
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
  max-height: calc(80vh - 180px);
  position: relative;
}

.history-loading,
.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: #a2a2a2;
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
  background: #e6e6e6;
}

.dot-create { background: #4F5BDF; }
.dot-update { background: #f59e0b; }
.dot-activate { background: #10b981; }
.dot-deactivate { background: #6b7280; }
.dot-default { background: #9ca3af; }

.history-content {
  flex: 1;
  min-width: 0;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
}

.user-name {
  font-weight: 500;
  color: #333;
  font-size: 13px;
}

.action-time {
  color: #a2a2a2;
  font-size: 11px;
}

.action-text {
  color: #666;
  font-size: 12px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 11px;
  color: #666;
  font-style: italic;
  margin-top: 4px;
  padding-left: 6px;
  border-left: 2px solid #e6e6e6;
  word-break: break-word;
}

@media (max-width: 768px) {
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
</style>
