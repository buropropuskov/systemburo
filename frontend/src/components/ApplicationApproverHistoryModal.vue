<template>
  <Teleport to="body">
    <transition
      name="modal-fade"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="visible"
        class="modal-overlay"
        data-testid="approver-history-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="approver-history-modal"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>История принимающих заявки</h3>
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
                @click="requestClose"
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
                  placeholder="Поиск по пользователю, принимающему..."
                >
              </div>

              <div class="user-filter">
                <span class="filter-label">Пользователь:</span>
                <!-- Кастомный select, не BaseDropdown: повторяет фильтр-строку эталона
                     SystemTableHistoryModal (32px-контролы поиск/период/сортировка в ряд). -->
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
                        :class="{ 'selected': selectedUserId === null }"
                        @click.stop="selectUser(null)"
                      >
                        Все пользователи
                      </div>
                      <div
                        v-for="user in uniqueUsers"
                        :key="user.id"
                        class="select-option"
                        :class="{ 'selected': selectedUserId === user.id }"
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
                    <div class="history-header">
                      <span class="user-name">{{ item.actor_name || 'Система' }}</span>
                      <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                    </div>

                    <div class="action-text">
                      {{ getActionText(item) }}
                    </div>

                    <div
                      v-if="item.approver_name"
                      class="action-comment"
                    >
                      Принимающий: {{ item.approver_name }}
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
import { ref } from 'vue';
import { getApproverHistory } from '@/api/approvers';
import { useDeletionsStore } from '@/stores/deletions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import ExcelJS from 'exceljs';

const ACTION_TEXTS = {
  created: 'Добавлен принимающий',
  deleted: 'Удалён принимающий',
  renamed: 'Изменена маска принимающего',
};

const ACTION_DOT_CLASS = {
  created: 'dot-create',
  deleted: 'dot-deactivate',
  renamed: 'dot-update',
};

export default {
  name: 'ApplicationApproverHistoryModal',
  components: { LoaderSpinner, AppIcon },
  props: {
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  setup(_, { emit }) {
    // Анимация закрытия: показ управляем внутренним visible (enter по mounted,
    // leave по requestClose); emit('close') шлём только ПОСЛЕ leave-перехода
    // (@after-leave), иначе родитель размонтирует мгновенно и анимация не проиграется.
    const visible = ref(false);
    const requestClose = () => { visible.value = false; };
    const onAfterLeave = () => emit('close');
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(requestClose);
    return { visible, requestClose, onAfterLeave, onOverlayMousedown, onOverlayMouseup };
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
      // Акторы (кто назначил/снял), не принимающие - фильтр по тому, кто отображён в шапке записи.
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
      if (this.selectedUserId === null) return 'Все пользователи';
      const user = this.uniqueUsers.find((u) => u.id === this.selectedUserId);
      return user ? user.name : 'Все пользователи';
    },

    filteredHistory() {
      let filtered = [...this.history];

      if (this.searchQuery && this.searchQuery.trim() !== '') {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter((item) => {
          const actorName = (item.actor_name || '').toLowerCase();
          const approverName = (item.approver_name || '').toLowerCase();
          const actionText = this.getActionText(item).toLowerCase();
          return actorName.includes(query)
            || approverName.includes(query)
            || actionText.includes(query);
        });
      }

      if (this.selectedUserId) {
        filtered = filtered.filter((item) => item.actor_user_id === this.selectedUserId);
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
      const dateMap = new Map();
      for (const item of this.filteredHistory) {
        const dateKey = new Date(item.created_at).toLocaleDateString('ru-RU', {
          day: 'numeric', month: 'long', year: 'numeric',
        });
        if (!dateMap.has(dateKey)) {
          dateMap.set(dateKey, []);
          groups.push({ date: dateKey, items: dateMap.get(dateKey) });
        }
        dateMap.get(dateKey).push(item);
      }
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

    exportData() {
      return this.filteredHistory.map((item) => ({
        'Дата и время': this.formatDateTime(item.created_at),
        'Пользователь': item.actor_name || 'Система',
        'Действие': this.getActionText(item),
        'Принимающий': item.approver_name || '',
        'Тип действия': item.action_type,
        'ID записи': item.id,
      }));
    },
  },
  mounted() {
    this.visible = true;
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
      if (e.key === 'Escape') this.requestClose();
    },
    handleClickOutside(event) {
      // ref, не this.$el.querySelector: при <Teleport> $el - это якорный комментарий
      // без querySelector, и дропдаун фильтра не закрывался бы по клику снаружи.
      const select = this.$refs.userSelect;
      if (select && !select.contains(event.target)) {
        this.userDropdownOpen = false;
      }
    },

    async loadHistory() {
      this.loading = true;
      try {
        const data = await getApproverHistory();
        this.history = Array.isArray(data) ? data : [];
      } catch (error) {
        console.error('Error loading approver history:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю принимающих', type: 'error' });
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

    selectUser(userId) {
      this.selectedUserId = userId;
      this.userDropdownOpen = false;
    },

    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      this.isExporting = true;
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Istoriya_prinimayushchih');

        const headers = ['Дата и время', 'Пользователь', 'Действие', 'Принимающий', 'Тип действия', 'ID записи'];
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
            item['Принимающий'],
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
          { width: 26 },
          { width: 30 },
          { width: 16 },
          { width: 12 },
        ];

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `Istoriya_prinimayushchih_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (error) {
        console.error('Error exporting approver history to Excel:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить историю', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },
  },
};
</script>

<style scoped>
.history-date-separator {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-text);
  padding: 8px 0 4px;
  margin-bottom: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  letter-spacing: 0.02em;
}

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
}

/* Анимация открытия/закрытия (паттерн BaseModal): overlay fade + контент scale */
.modal-fade-enter-active {
  transition: opacity 0.25s ease;
}

.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .approver-history-modal {
  animation: modal-scale-in 0.25s ease;
}

.modal-fade-leave-active .approver-history-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from { transform: scale(0.95); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@keyframes modal-scale-out {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.95); opacity: 0; }
}

.approver-history-modal {
  background: var(--surface);
  border-radius: 30px;
  width: 900px;
  max-width: 95%;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px var(--shadow-drop);
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
  border-radius: 15px;
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
  border-radius: 15px;
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
.dot-deactivate { background: #6b7280; }
.dot-update { background: #f59e0b; }
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
