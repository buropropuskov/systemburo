<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="close"
      >
        <div class="modal-wrapper">
          <!-- Основное модальное окно с деталями сотрудника -->
          <div 
            class="modal-content compact-modal main-modal"
            :class="{ 'shifted': isMainShifted }"
          >
            <div class="modal-header">
              <h3 class="modal-title">
                {{ modalTitle }}
              </h3>
              <div class="header-actions">
                <button
                  v-if="showHistoryButton"
                  class="history-btn"
                  @click="openFullHistory"
                >
                  <span>Полная история</span>
                </button>
                <button
                  v-if="source !== 'application' && employee?.applicationId"
                  class="application-btn"
                  @click="openApplication"
                >
                  <span>Открыть заявку</span>
                </button>
              </div>
              <button
                class="modal-close"
                @click="close"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
                  fill="none"
                >
                  <path
                    d="M13 1L1 13M1 1L13 13"
                    stroke="#666"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
            </div>
                    
            <div class="modal-body">
              <div
                v-if="employee"
                class="employee-details"
              >
                <!-- Основная информация -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Основная информация
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="details-grid two-columns">
                      <div class="detail-item">
                        <span class="detail-label">Фамилия:</span>
                        <span class="detail-value">{{ employee.last_name || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Имя:</span>
                        <span class="detail-value">{{ employee.first_name || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Отчество:</span>
                        <span class="detail-value">{{ employee.middle_name || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Должность:</span>
                        <span class="detail-value">{{ employee.position || 'Не указана' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Гражданство:</span>
                        <span class="detail-value">{{ employee.citizenshipName || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Организация:</span>
                        <span class="detail-value">{{ employee.organization || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Компания:</span>
                        <span class="detail-value">{{ employee.company || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Действует до:</span>
                        <span class="detail-value">{{ formatDate(employee.entry_date_to) || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Время прохода:</span>
                        <span class="detail-value">{{ employee.pass_time || '-' }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Документы (чувствительные данные) -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Документы
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="details-grid two-columns">
                      <div class="detail-item">
                        <span class="detail-label">Серия и номер паспорта:</span>
                        <div class="sensitive-data">
                          <span 
                            class="data-text"
                            :class="{ 'hidden-data': !showFullPassport }"
                          >
                            {{ employee.passport_series_number || 'Не указан' }}
                          </span>
                          <button 
                            v-if="employee.passport_series_number"
                            class="show-more-btn"
                            @click="togglePassport"
                          >
                            {{ showFullPassport ? 'Скрыть' : 'Показать' }}
                          </button>
                        </div>
                      </div>
                      <div
                        v-if="employee.patent_number"
                        class="detail-item"
                      >
                        <span class="detail-label">Номер патента:</span>
                        <div class="sensitive-data">
                          <span 
                            class="data-text"
                            :class="{ 'hidden-data': !showFullPatent }"
                          >
                            {{ employee.patent_number }}
                          </span>
                          <button 
                            class="show-more-btn"
                            @click="togglePatent"
                          >
                            {{ showFullPatent ? 'Скрыть' : 'Показать' }}
                          </button>
                        </div>
                      </div>
                      <div
                        v-if="employee.other_permission"
                        class="detail-item full-width"
                      >
                        <span class="detail-label">Иное разрешение:</span>
                        <span class="detail-value">{{ employee.other_permission }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Места прохода (таблицы) -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Места прохода
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="places-list">
                      <div 
                        v-for="tableId in employee.target_tables" 
                        :key="tableId"
                        class="place-item"
                        :class="{ 'active': showPlaceModal && selectedTable && selectedTable.id === tableId }"
                        @click="showTableDetails(tableId)"
                      >
                        {{ getTableName(tableId) }}
                      </div>
                      <div
                        v-if="!employee.target_tables || employee.target_tables.length === 0"
                        class="no-places"
                      >
                        Места прохода не указаны
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Статус территории (скрыт только для employeeslist) -->
                <div
                  v-if="showStatusSection"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      Статус
                    </h4>
                  </div>
                  <div class="section-body">
                    <div
                      class="status-badge"
                      :class="getStatusClass"
                    >
                      {{ getStatusText }}
                    </div>
                  </div>
                </div>

                <!-- История проходов (скрыта только для employeeslist) -->
                <div
                  v-if="showHistorySection"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      История проходов
                    </h4>
                    <button
                      class="export-btn"
                      :disabled="entryExitHistory.length === 0 || isExporting"
                      @click="exportHistory"
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
                  </div>
                  <div class="section-body">
                    <div
                      v-if="loadingHistory"
                      class="loading-container"
                    >
                      <div class="loader" />
                      <span>Загрузка истории...</span>
                    </div>
                                    
                    <div
                      v-else-if="entryExitHistory.length === 0"
                      class="no-history"
                    >
                      История проходов отсутствует
                    </div>
                                    
                    <div
                      v-else
                      class="history-timeline"
                    >
                      <div 
                        v-for="(item, index) in entryExitHistory" 
                        :key="item.id" 
                        class="history-item"
                      >
                        <div
                          class="timeline-dot"
                          :class="getActionClass(item)"
                        />
                        <div
                          v-if="index < entryExitHistory.length - 1"
                          class="timeline-line"
                        />
                                            
                        <div class="history-content">
                          <div class="history-header">
                            <span class="user-name">{{ item.user_name || 'Система' }}</span>
                            <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                          </div>
                                                
                          <div class="action-text">
                            {{ getActionText(item) }}
                          </div>
                                                
                          <div class="action-comment">
                            {{ item.comment || getActionComment(item) }}
                          </div>
                          <div
                            v-if="item.table_name"
                            class="place-name"
                          >
                            {{ item.table_name }}
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Дополнительное модальное окно с деталями места прохода -->
          <transition 
            name="place-slide"
            @after-leave="onPlaceLeave"
          >
            <div
              v-if="showPlaceModal"
              class="place-modal-container"
            >
              <TableInfoModal
                :table="selectedTable"
                :all-tables="allTables"
                @close="closeTableDetails"
              />
            </div>
          </transition>
        </div>
      </div>
    </transition>
  </Teleport>

  <!-- Модальное окно полной истории сотрудника -->
  <EmployeeHistoryModal
    v-if="showFullHistoryModal"
    :last-name="employee?.last_name"
    :first-name="employee?.first_name"
    :middle-name="employee?.middle_name"
    :current-user-id="currentUserId"
    :current-user-name="currentUserName"
    @close="showFullHistoryModal = false"
  />
</template>

<script>
import { apiRequest } from '@/api/client';
import TableInfoModal from './TableInfoModal.vue';
import EmployeeHistoryModal from './EmployeeHistoryModal.vue';
import ExcelJS from 'exceljs';

export default {
    name: 'EmployeeDetailsModal',
    components: {
        TableInfoModal,
        EmployeeHistoryModal
    },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        employee: {
            type: Object,
            default: null
        },
        allTables: {
            type: Array,
            default: () => []
        },
        currentUserId: {
            type: Number,
            default: null
        },
        currentUserName: {
            type: String,
            default: ''
        },
        source: {
            type: String,
            default: 'general'
        }
    },
    emits: ['close', 'open-application'],
    data() {
        return {
            selectedTable: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
            showFullPassport: false,
            showFullPatent: false,
            history: [],
            loadingHistory: false,
            isExporting: false,
            showFullHistoryModal: false,
            territoryStatus: 0
        };
    },
    computed: {
        modalTitle() {
            if (this.source === 'application' || this.source === 'employeeslist') {
                return 'Информация о сотруднике';
            } else if (this.source === 'peopletable') {
                return 'Информация';
            }
            return 'Детальная информация о сотруднике';
        },
        showHistoryButton() {
            return this.source !== 'employeeslist';
        },
        entryExitHistory() {
            return this.history.filter(item => item.action_type === 'entry' || item.action_type === 'exit');
        },
        getStatusClass() {
            const status = this.territoryStatus;
            if (status === 1) return 'status-on-territory';
            if (status === 2) return 'status-exited';
            return 'status-not-entered';
        },
        getStatusText() {
            const status = this.territoryStatus;
            if (status === 1) return 'На территории';
            if (status === 2) return 'Покинул территорию';
            return 'Не входил';
        },
        showStatusSection() {
            return this.source !== 'employeeslist';
        },
        showHistorySection() {
            return this.source !== 'employeeslist';
        }
    },
    watch: {
        show: {
        immediate: true,
        handler(val) {
            if (val) {
                this.loadHistory();
                this.loadEmployeeStatus(); // для EmployeeDetailsModal
                // для VehicleDetailsModal: this.loadCarStatus(); this.loadCarHistory();
            } else {
                this.close(); // вместо this.closeTableDetails()
            }
        }
    },
        employee: {
            deep: true,
            handler(newVal) {
                if (newVal && this.show) {
                    this.loadHistory();
                    this.loadEmployeeStatus();
                }
            }
        }
    },
    methods: {
        close() {
    this.$emit('close');
    this.closeTableDetails();
    if (this.shiftTimer) {
        clearTimeout(this.shiftTimer);
        this.shiftTimer = null;
    }
    this.isMainShifted = false; // принудительно сбрасываем
    this.selectedTable = null;
},

        togglePassport() {
            this.showFullPassport = !this.showFullPassport;
        },

        togglePatent() {
            this.showFullPatent = !this.showFullPatent;
        },

        getTableName(tableId) {
            let found = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (found) {
                let tbl = found.table || found;
                return tbl.display_name || tbl.name || `ID: ${tableId}`;
            }
            return `Неизвестное место (ID: ${tableId})`;
        },

        showTableDetails(tableId) {
            const tableData = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (!tableData) {
                alert('Информация о месте прохода недоступна');
                return;
            }
            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || [],
                photos: tableData.photos || [],
                current_status: tableData.current_status || 'closed'
            };
            if (this.shiftTimer) clearTimeout(this.shiftTimer);
            this.isMainShifted = true;
            this.shiftTimer = setTimeout(() => {
                this.showPlaceModal = true;
                this.shiftTimer = null;
            }, 300);
        },

        closeTableDetails() {
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            this.isMainShifted = false;
            this.selectedTable = null;
        },

        formatDate(dateString) {
            if (!dateString) return '';
            try {
                const [y, m, d] = dateString.split('-');
                const date = new Date(y, m-1, d);
                return date.toLocaleDateString('ru-RU');
            } catch { return ''; }
        },

        formatDateTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit', month: '2-digit', year: 'numeric',
                hour: '2-digit', minute: '2-digit'
            }).replace(',', '');
        },

        getActionClass(item) {
            if (!item.user_id) return 'dot-system';
            const classes = { entry: 'dot-entry', exit: 'dot-exit' };
            return classes[item.action_type] || 'dot-default';
        },

        getActionText(item) {
            if (item.action_type === 'entry') return 'Проход на территорию';
            if (item.action_type === 'exit') return 'Выход с территории';
            return '';
        },

        getActionComment(item) {
            if (item.comment) return item.comment;
            const userName = item.user_name || 'Система';
            const fullName = `${this.employee?.last_name || ''} ${this.employee?.first_name || ''}`.trim();
            if (item.action_type === 'entry') return `Пользователь ${userName} отметил проход сотрудника ${fullName}`;
            if (item.action_type === 'exit') return `Пользователь ${userName} отметил выход сотрудника ${fullName}`;
            return '';
        },

        async loadHistory() {
            if (!this.employee?.id) return;
            this.loadingHistory = true;
            try {
                const res = await apiRequest(`/employees/${this.employee.id}/history`, { method: 'GET' });
                if (res.ok) {
                    this.history = await res.json();
                }
            } catch (e) {
                console.error('Network error:', e);
            } finally {
                this.loadingHistory = false;
            }
        },

        async loadEmployeeStatus() {
            if (!this.employee?.id) return;
            try {
                const response = await apiRequest('/employees/history/current-status', { method: 'GET' });
                if (response.ok) {
                    const statuses = await response.json();
                    const status = statuses.find(s => s.employee_id === this.employee.id);
                    if (status) {
                        this.territoryStatus = status.territory_status;
                    } else {
                        this.territoryStatus = 0;
                    }
                }
            } catch (error) {
                console.error('Ошибка при загрузке статуса сотрудника:', error);
            }
        },

        async exportHistory() {
            if (this.entryExitHistory.length === 0) return;
            this.isExporting = true;
            try {
                const workbook = new ExcelJS.Workbook();
                const worksheet = workbook.addWorksheet('Istoriya_prokhodov');
                
                const headers = ['Дата и время', 'Пользователь', 'Действие', 'Комментарий', 'Место'];
                const headerRow = worksheet.addRow(headers);
                headerRow.height = 25;
                headerRow.eachCell((cell) => {
                    cell.fill = {
                        type: 'pattern',
                        pattern: 'solid',
                        fgColor: { argb: 'FF4F5BDF' }
                    };
                    cell.font = {
                        name: 'Verdana',
                        size: 11,
                        bold: true,
                        color: { argb: 'FFFFFFFF' }
                    };
                    cell.alignment = { vertical: 'middle', horizontal: 'center' };
                    cell.border = {
                        top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                    };
                });

                this.entryExitHistory.forEach((item, index) => {
                    const row = worksheet.addRow([
                        this.formatDateTime(item.created_at),
                        item.user_name || 'Система',
                        this.getActionText(item),
                        item.comment || this.getActionComment(item),
                        item.table_name || ''
                    ]);
                    
                    row.height = 20;
                    const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
                    
                    row.eachCell((cell) => {
                        cell.fill = {
                            type: 'pattern',
                            pattern: 'solid',
                            fgColor: { argb: fillColor }
                        };
                        cell.font = {
                            name: 'Verdana',
                            size: 9,
                            color: { argb: 'FF333333' }
                        };
                        cell.alignment = { vertical: 'middle' };
                        cell.border = {
                            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                        };
                    });
                });

                const lastDataRow = this.entryExitHistory.length;
                for (let row = 1; row <= lastDataRow + 1; row++) {
                    const rightCell = worksheet.getCell(row, 5);
                    rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
                    const leftCell = worksheet.getCell(row, 1);
                    leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
                }
                for (let col = 1; col <= 5; col++) {
                    const topCell = worksheet.getCell(1, col);
                    topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
                    const bottomCell = worksheet.getCell(lastDataRow + 1, col);
                    bottomCell.border = { ...bottomCell.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
                }

                worksheet.addRow([]);
                const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserName || 'Пользователь']);
                const infoRow2 = worksheet.addRow(['Дата формирования:', new Date().toLocaleString('ru-RU')]);
                [infoRow1, infoRow2].forEach(row => {
                    row.eachCell((cell) => {
                        cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
                        cell.alignment = { vertical: 'middle' };
                        cell.border = {
                            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                        };
                    });
                });

                worksheet.columns = [ { width: 25 }, { width: 40 }, { width: 30 }, { width: 60 }, { width: 30 } ];

                const buffer = await workbook.xlsx.writeBuffer();
                const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                const fullName = `${this.employee.last_name}_${this.employee.first_name}`.replace(/[^a-zA-Z0-9]/g, '_');
                a.download = `Istoriya_${fullName}.xlsx`;
                a.href = url;
                a.click();
                window.URL.revokeObjectURL(url);
            } catch (e) { console.error(e); alert('Ошибка экспорта'); } finally { this.isExporting = false; }
        },

        openFullHistory() {
            this.showFullHistoryModal = true;
        },

        openApplication() {
            if (this.employee?.applicationId) {
                this.$emit('open-application', this.employee.applicationId);
            }
        }
    }
};
</script>



<style scoped>
/* Все стили остаются без изменений, как в предыдущей версии */
.place-name {
    font-size: 10px;
    color: #4F5BDF;
    margin-top: 2px;
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
    z-index: 10001;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
    animation: overlayAppear 0.4s ease-out;
}

@keyframes overlayAppear {
    from {
        background: rgba(0, 0, 0, 0);
    
    }
    to {
        background: rgba(0, 0, 0, 0.5);
       
    }
}

.modal-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;  /* добавляем */
}

.modal-content {
    background: #fff;
    border-radius: 50px;
    padding: 0;
    padding-bottom: 15px;
    width: 520px;
    height: 450px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
    position: absolute;
    pointer-events: auto;  /* добавляем */
}

.modal-content .modal-body {
    overflow-y: auto;
    height: calc(450px - 70px);
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-content .modal-body::-webkit-scrollbar {
    display: none;
}

.modal-content.main-modal {
    left: calc(50% - 260px);
    transition: transform 0.5s cubic-bezier(0.25, 0.1, 0.15, 1);
    transform: translateX(0);
}

.modal-content.main-modal.shifted {
    transform: translateX(-280px);
}

.place-modal-container {
    position: absolute;
    left: 50%;
    width: 520px;
    height: 450px;
    pointer-events: auto;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 30px 16px;
    border-bottom: 1px solid #f0f0f0;
    flex-shrink: 0;
    height: 70px;
    box-sizing: border-box;
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-right: 10px;
}

.history-btn, .application-btn {
    padding: 6px 12px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    font-size: 12px;
    color: #333;
    cursor: pointer;
    transition: all 0.2s ease;
    white-space: nowrap;
}

.history-btn:hover, .application-btn:hover {
    background: #f5f5f5;
    border-color: #4F5BDF;
}

.modal-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #1a1a1a;
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
    transition: all 0.2s ease;
}

.modal-close:hover {
    background-color: #f5f5f5;
}

.modal-body {
    padding: 20px 30px;
    overflow-y: auto;
    flex: 1;
}

.employee-details {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.details-section {
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    background: #fafafa;
    overflow: hidden;
}

.section-header {
    padding: 12px 20px;
    border-bottom: 1px solid #e6e6e6;
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.section-title {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: #333;
}

.section-body {
    padding: 16px 20px;
}

.details-grid.two-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px 16px;
}

.detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.detail-item.full-width {
    grid-column: 1 / -1;
}

.detail-label {
    font-size: 11px;
    color: #a2a2a2;
    font-weight: 400;
    letter-spacing: 0.3px;
}

.detail-value {
    font-size: 14px;
    color: #333;
    font-weight: 500;
    word-break: break-word;
}

.sensitive-data {
    display: flex;
    align-items: center;
    gap: 15px;
}

.data-text {
    font-size: 13px;
    color: #333;
    font-weight: 500;
    letter-spacing: 0.5px;
    transition: all 0.3s ease;
    word-break: break-all;
}

.data-text.hidden-data {
    filter: blur(4px);
    user-select: none;
}

.show-more-btn {
    background: #f8f9fa;
    border: 1px solid #e0e0e0;
    color: #4F5BDF;
    font-size: 11px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 15px;
    transition: all 0.2s;
    font-weight: 500;
    white-space: nowrap;
    min-width: 75px;
    text-align: center;
}

.show-more-btn:hover {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.places-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
}

.place-item {
    border: 1px solid #e6e6e6;
    border-radius: 50px;
    padding: 6px 12px;
    font-size: 12px;
    color: #333;
    transition: all 0.2s ease;
    display: inline-block;
    cursor: pointer;
}

.place-item:hover {
    background: #f0f0f0;
    border-color: #4F5BDF;
}

.place-item.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.no-places {
    text-align: center;
    color: #a2a2a2;
    font-size: 13px;
    font-style: italic;
    padding: 10px;
}

.status-badge {
    display: inline-block;
    padding: 6px 16px;
    border-radius: 50px;
    font-size: 13px;
    font-weight: 500;
}

.status-on-territory {
    background: rgba(79, 91, 223, 0.1);
    color: #4F5BDF;
    border: 1px solid rgba(79, 91, 223, 0.3);
}

.status-exited {
    background: rgba(220, 38, 38, 0.1);
    color: #dc2626;
    border: 1px solid rgba(220, 38, 38, 0.3);
}

.status-not-entered {
    background: #f5f5f5;
    color: #9ca3af;
    border: 1px solid #e6e6e6;
}

.export-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 12px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    font-size: 12px;
    color: #333;
    cursor: pointer;
    transition: all 0.2s ease;
    height: 28px;
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
    width: 12px;
    height: 12px;
}

.export-loader {
    width: 14px;
    height: 14px;
    border: 2px solid #e6e6e6;
    border-top: 2px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.history-timeline {
    position: relative;
    max-height: 200px;
    overflow-y: auto;
}

.history-item {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;
    position: relative;
}

.history-item:last-child {
    margin-bottom: 0;
}

.timeline-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    margin-top: 4px;
    z-index: 1;
}

.timeline-line {
    position: absolute;
    left: 3px;
    top: 16px;
    width: 2px;
    height: calc(100% + 2px);
    background: #e6e6e6;
}

.dot-system { background: #8b5cf6; }
.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }
.dot-default { background: #9ca3af; }

.history-content {
    flex: 1;
}

.history-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 2px;
}

.user-name {
    font-weight: 500;
    color: #333;
    font-size: 12px;
}

.action-time {
    color: #a2a2a2;
    font-size: 10px;
}

.action-text {
    color: #666;
    font-size: 11px;
    margin-bottom: 2px;
}

.action-comment {
    font-size: 10px;
    color: #666;
    font-style: italic;
    margin-top: 2px;
    padding-left: 6px;
    border-left: 2px solid #e6e6e6;
}

.loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 20px;
    gap: 10px;
}

.loader {
    width: 30px;
    height: 30px;
    border: 3px solid #f3f3f3;
    border-top: 3px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

.no-history {
    text-align: center;
    color: #a2a2a2;
    padding: 20px;
    font-size: 13px;
    font-style: italic;
}

.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
    transition: all 0.4s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
    transition: all 0.4s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
    background: rgba(0, 0, 0, 0);
 
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
    opacity: 0;
    transform: scale(0.9) translateY(-20px);
}

.place-slide-enter-active,
.place-slide-leave-active {
    transition: transform 0.6s cubic-bezier(0.2, 0.9, 0.1, 1), opacity 0.5s ease;
}
.place-slide-enter-from {
    transform: translateY(100vh);
    opacity: 0;
}
.place-slide-enter-to {
    transform: translateY(0);
    opacity: 1;
}
.place-slide-leave-from {
    transform: translateY(0);
    opacity: 1;
}
.place-slide-leave-to {
    transform: translateY(600px);
    opacity: 0;
}

@media (max-width: 768px) {
    .modal-content {
        width: 90%;
        left: 5% !important;
        transform: none !important;
        height: auto;
        max-height: 80vh;
    }
    
    .modal-content .modal-body {
        height: auto;
        max-height: calc(80vh - 70px);
    }
    
    .modal-content.main-modal.shifted {
        transform: none !important;
    }
    
    .place-modal-container {
        width: 90%;
        left: 5%;
        height: auto;
        max-height: 80vh;
    }
    
    .modal-header {
        padding: 16px 20px;
        flex-wrap: wrap;
        height: auto;
    }
    
    .header-actions {
        order: 3;
        width: 100%;
        justify-content: flex-start;
        margin-top: 10px;
    }
    
    .modal-body {
        padding: 16px 20px;
    }
    
    .details-section {
        border-radius: 16px;
    }
    
    .section-header {
        padding: 10px 16px;
    }
    
    .section-body {
        padding: 14px 16px;
    }
    
    .section-title {
        font-size: 13px;
    }
    
    .details-grid.two-columns {
        grid-template-columns: 1fr;
    }
    
    .places-list {
        gap: 6px;
    }
    
    .place-item {
        padding: 4px 10px;
        font-size: 11px;
    }
    
    .sensitive-data {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }
    
    .show-more-btn {
        align-self: flex-start;
    }
}
</style>