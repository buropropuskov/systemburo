<template>
  <Teleport to="body">
    <transition
      name="trash-modal-fade"
      appear
      @after-leave="$emit('close')"
    >
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="close"
      >
        <div class="trash-history-modal">
          <div class="modal-header">
            <h3>История корзины — {{ tableDisplayName }}</h3>
            <div class="header-actions">
              <button
                class="export-btn"
                data-testid="trash-history-export"
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
                data-testid="trash-history-close"
                @click="close"
              >
                ×
              </button>
            </div>
          </div>

          <div
            v-if="history.length"
            class="history-filters"
          >
            <input
              v-model="searchQuery"
              type="text"
              class="hf-input"
              placeholder="Поиск по действию или пользователю..."
            >
            <select
              v-model="selectedUser"
              class="hf-select"
            >
              <option :value="null">
                Все пользователи
              </option>
              <option
                v-for="u in uniqueUsers"
                :key="u"
                :value="u"
              >
                {{ u }}
              </option>
            </select>
            <button
              class="hf-sort"
              @click="sortOrder = sortOrder === 'desc' ? 'asc' : 'desc'"
            >
              <img
                src="@/assets/icons/sort.png"
                class="hf-sort-icon"
                :class="{ 'hf-sort-icon--asc': sortOrder === 'asc' }"
              >
              <span>{{ sortOrder === 'desc' ? 'Сначала новые' : 'Сначала старые' }}</span>
            </button>
          </div>

          <div class="modal-content">
            <div
              v-if="loading"
              class="history-empty"
            >
              <div class="loader" />
            </div>
            <div
              v-else-if="filteredHistory.length === 0"
              class="history-empty"
            >
              {{ history.length ? 'Ничего не найдено' : 'История пуста' }}
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
                    <span class="action-title">{{ getActionText(item) }}</span>
                    <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                  </div>
                  <div class="action-user">
                    {{ item.user_name || 'Система' }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import ExcelJS from 'exceljs';
import { getTrashHistory } from '@/api/trash';

export default {
  name: 'TrashHistoryModal',
  props: {
    tableId: { type: Number, required: true },
    tableDisplayName: { type: String, default: '' },
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  data() {
    return {
      show: false,
      history: [],
      loading: false,
      isExporting: false,
      searchQuery: '',
      selectedUser: null,
      sortOrder: 'desc',
    };
  },
  computed: {
    uniqueUsers() {
      return [...new Set(this.history.map(h => h.user_name || 'Система'))];
    },
    filteredHistory() {
      let arr = [...this.history];
      if (this.selectedUser) {
        arr = arr.filter(h => (h.user_name || 'Система') === this.selectedUser);
      }
      if (this.searchQuery) {
        const q = this.searchQuery.toLowerCase();
        arr = arr.filter(h =>
          this.getActionText(h).toLowerCase().includes(q)
          || (h.user_name || 'Система').toLowerCase().includes(q),
        );
      }
      arr.sort((a, b) => {
        const da = new Date(a.created_at).getTime();
        const db = new Date(b.created_at).getTime();
        return this.sortOrder === 'asc' ? da - db : db - da;
      });
      return arr;
    },
    formattedCurrentDateTime() {
      return new Date().toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
      }).replace(',', '');
    },
    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter(p => p && p !== 'null' && p !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    },
  },
  mounted() {
    this.show = true;
    this.loadHistory();
  },
  methods: {
    close() {
      this.show = false;
    },
    async loadHistory() {
      this.loading = true;
      try {
        const data = await getTrashHistory(this.tableId);
        this.history = Array.isArray(data) ? data : [];
      } catch {
        this.history = [];
      } finally {
        this.loading = false;
      }
    },
    getActionText(item) {
      if (item.action_type === 'cleared') {
        return `Корзина очищена (${item.affected_count} запис.)`;
      }
      if (item.action_type === 'bulk_restored') {
        return `Восстановлено ${item.affected_count} элемент(ов)`;
      }
      return item.action_type;
    },
    getActionClass(actionType) {
      return actionType === 'bulk_restored' ? 'dot-restore' : 'dot-clear';
    },
    formatDateTime(s) {
      if (!s) return '';
      return new Date(s).toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
      }).replace(',', '');
    },
    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      this.isExporting = true;
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Istoriya_korziny');
        const headers = ['Дата и время', 'Действие', 'Затронуто', 'Пользователь'];

        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
        });

        this.filteredHistory.forEach((item, index) => {
          const row = worksheet.addRow([
            this.formatDateTime(item.created_at),
            this.getActionText(item),
            item.affected_count,
            item.user_name || 'Система',
          ]);
          row.height = 20;
          const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          row.eachCell((cell) => {
            cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
            cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
          });
        });

        worksheet.addRow([]);
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserDisplayName]);
        const infoRow2 = worksheet.addRow(['Дата формирования:', this.formattedCurrentDateTime]);
        [infoRow1, infoRow2].forEach((row) => {
          row.eachCell((cell) => {
            cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
          });
        });

        worksheet.columns = [{ width: 22 }, { width: 40 }, { width: 14 }, { width: 32 }];

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `istoriya_korziny_${this.tableDisplayName}_${this.formattedCurrentDateTime.replace(/[.:,\s]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (e) {
        console.error('Ошибка экспорта истории корзины', e);
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
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 13000;
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.trash-history-modal {
  background: white;
  border-radius: 30px;
  width: 620px;
  max-width: 95%;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  font-family: 'Montserrat', sans-serif;
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
  font-family: 'Montserrat', sans-serif;
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
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 12px 25px;
  border-bottom: 1px solid #e6e6e6;
  background: #fafafa;
}

.hf-input {
  flex: 1 1 220px;
  height: 32px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  padding: 0 12px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  outline: none;
}

.hf-input:focus {
  border-color: #4F5BDF;
}

.hf-select {
  height: 32px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  padding: 0 10px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  background: #fff;
  cursor: pointer;
  outline: none;
}

.hf-sort {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 14px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: #333;
  cursor: pointer;
  transition: all 0.2s ease;
}

.hf-sort:hover {
  border-color: #4F5BDF;
}

.hf-sort-icon {
  width: 12px;
  height: 12px;
  transition: transform 0.2s ease;
}

.hf-sort-icon--asc {
  transform: rotate(180deg);
}

.modal-content {
  padding: 20px 25px;
  overflow-y: auto;
  flex: 1;
}

.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: #a2a2a2;
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid #f3f3f3;
  border-top: 3px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.history-timeline {
  position: relative;
  padding-left: 8px;
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
}

.timeline-line {
  position: absolute;
  left: 4px;
  top: 18px;
  width: 2px;
  height: calc(100% + 2px);
  background: #e6e6e6;
}

.dot-restore { background: #059669; }
.dot-clear { background: #dc2626; }

.history-content {
  flex: 1;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 4px;
}

.action-title {
  font-size: 14px;
  font-weight: 500;
  color: #000;
}

.action-time {
  font-size: 12px;
  color: #a2a2a2;
  white-space: nowrap;
}

.action-user {
  font-size: 13px;
  color: #555;
}

.trash-modal-fade-enter-active,
.trash-modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.trash-modal-fade-enter-from,
.trash-modal-fade-leave-to {
  opacity: 0;
}
</style>
