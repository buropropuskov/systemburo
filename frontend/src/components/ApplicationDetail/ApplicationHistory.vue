<template>
  <div class="application-history">
    <button
      class="history-toggle"
      @click="openModal"
    >
      История заявки
    </button>

    <!-- Модальное окно истории -->
    <Teleport to="body">
      <div
        v-if="showModal"
        class="history-modal-overlay"
        @click.self="closeModal"
      >
        <div class="history-modal">
          <div class="modal-header">
            <h3>История заявки {{ applicationNumber }}</h3>
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
                @click="closeModal"
              >
                ×
              </button>
            </div>
          </div>

          <!-- Фильтры -->
          <div class="history-filters">
            <div class="filter-row">
              <div class="user-filter">
                <span class="filter-label">Пользователь:</span>
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
                        :class="{ 'selected': selectedUserId === null }"
                        @click="selectUser(null)"
                      >
                        Все пользователи
                      </div>
                      <div 
                        v-for="user in uniqueUsers" 
                        :key="user.id"
                        class="select-option"
                        :class="{ 'selected': selectedUserId === user.id }"
                        @click="selectUser(user.id)"
                      >
                        {{ user.name }}
                      </div>
                    </div>
                  </transition>
                </div>
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

          <div
            ref="scrollContainer"
            class="modal-content"
          >
            <div
              v-if="loading"
              class="history-loading"
            >
              <LoaderSpinner label="Загрузка истории…" />
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
                      <!-- Для системных действий показываем "Система" -->
                      <span
                        v-if="item.action_type === 'confirmation_change' || item.action_type === 'status_change' || !item.user_id"
                        class="user-name system-name"
                      >
                        Система
                      </span>
                      <span
                        v-else
                        class="user-name"
                      >{{ item.user_name }}</span>
                      <span class="action-time">{{ formatTime(item.created_at) }}</span>
                    </div>

                    <div class="action-text">
                      {{ getActionText(item) }}
                    </div>

                    <!-- Для пересылки показываем дополнительную информацию -->
                    <div
                      v-if="item.action_type === 'assigned_responsible' && item.metadata?.forwarded_by"
                      class="forward-info"
                    >
                      Переслано пользователем {{ item.metadata.forwarded_by }}
                    </div>

                    <!-- Для просмотра показываем дополнительную информацию -->
                    <div
                      v-if="item.action_type === 'assigned_viewer' && item.metadata?.forwarded_by"
                      class="forward-info"
                    >
                      Переслано пользователем {{ item.metadata.forwarded_by }}
                    </div>

                    <!-- Бейдж обязательного согласования -->
                    <div
                      v-if="item.metadata?.required_approval"
                      class="required-badge"
                    >
                      Обязательно
                    </div>

                    <!-- Статус изменения -->
                    <div
                      v-if="item.old_value && item.new_value && item.old_value !== item.new_value"
                      class="status-change"
                    >
                      <span class="old-status">{{ item.old_value }}</span>
                      <span class="arrow">→</span>
                      <span class="new-status">{{ item.new_value }}</span>
                    </div>

                    <!-- Комментарий (если есть) -->
                    <div
                      v-if="item.comment && item.action_type !== 'revoke_approval'"
                      class="action-comment"
                    >
                      {{ item.comment }}
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import ExcelJS from 'exceljs';

export default {
    name: 'ApplicationHistory',
    components: { LoaderSpinner },
    props: {
        applicationId: {
            type: Number,
            required: true
        },
        applicationNumber: {
            type: String,
            default: ''
        },
        currentUserId: {
            type: Number,
            default: null
        },
        currentUserName: {
            type: String,
            default: ''
        },
        applicationOrganization: {
            type: String,
            default: ''
        }
    },
    data() {
        return {
            showModal: false,
            loading: false,
            history: [],
            sortOrder: 'desc', // 'desc' - новые сверху, 'asc' - старые сверху
            selectedUserId: null,
            userDropdownOpen: false,
            isExporting: false
        }
    },
    computed: {
        // Уникальные пользователи из истории
        uniqueUsers() {
            const users = new Map();
            
            this.history.forEach(item => {
                if (item.user_id && !users.has(item.user_id)) {
                    users.set(item.user_id, {
                        id: item.user_id,
                        name: item.user_name
                    });
                }
            });
            
            return Array.from(users.values()).sort((a, b) => a.name.localeCompare(b.name));
        },

        selectedUserName() {
            if (this.selectedUserId === null) return 'Все пользователи';
            const user = this.uniqueUsers.find(u => u.id === this.selectedUserId);
            return user ? user.name : 'Все пользователи';
        },

        // Отфильтрованная история
        filteredHistory() {
            // Сначала фильтруем по пользователю
            let filtered = this.history;
            
            if (this.selectedUserId) {
                filtered = filtered.filter(item => item.user_id === this.selectedUserId);
            }
            
            // Убираем мусорные записи
            filtered = filtered.filter(item => {
                if (item.old_value === 'approved' && item.new_value === 'pending') return false;
                if (item.old_value === 'rejected' && item.new_value === 'pending') return false;
                
                // Убираем записи где old_value и new_value одинаковые
                if (item.old_value && item.new_value && item.old_value === item.new_value) return false;
                
                // Убираем change_status (старый тип)
                if (item.action_type === 'change_status') return false;
                
                return true;
            });
            
            // Сортируем
            return filtered.sort((a, b) => {
                const timeA = new Date(a.created_at).getTime();
                const timeB = new Date(b.created_at).getTime();
                
                if (this.sortOrder === 'desc') {
                    // Новые сверху, старые снизу
                    return timeB - timeA;
                } else {
                    // Старые сверху, новые снизу
                    return timeA - timeB;
                }
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

        // Данные для экспорта в формате таблицы
        exportData() {
            return this.filteredHistory.map(item => ({
                'Дата и время': this.formatTime(item.created_at),
                'Пользователь': !item.user_id ? 'Система' : item.user_name,
                'Действие': this.getActionText(item),
                'Старое значение': item.old_value || '',
                'Новое значение': item.new_value || '',
                'Комментарий': item.comment || '',
                'Обязательно': item.metadata?.required_approval ? 'Да' : 'Нет',
                'Кто переслал': item.metadata?.forwarded_by || ''
            }));
        },

        // Форматированная дата для подписи
        formattedCurrentDateTime() {
            const now = new Date();
            return now.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            }).replace(',', '');
        },

        // Имя текущего пользователя для подписи (фильтруем null/undefined значения)
        currentUserDisplayName() {
            if (!this.currentUserName) return 'Пользователь';
            
            // Разбиваем строку и фильтруем пустые значения
            const parts = this.currentUserName.split(' ').filter(part => part && part !== 'null' && part !== 'undefined');
            return parts.length > 0 ? parts.join(' ') : 'Пользователь';
        },

        // Организация заявки для имени файла (фильтруем null/undefined)
        fileOrganizationName() {
            if (!this.applicationOrganization) return 'Без_организации';
            
            // Убираем null/undefined из строки
            const cleanName = this.applicationOrganization
                .replace(/null/g, '')
                .replace(/undefined/g, '')
                .trim();
                
            return cleanName || 'Без_организации';
        }
    },
    mounted() {
        document.addEventListener('click', this.handleClickOutside);
    },
    beforeUnmount() {
        document.removeEventListener('click', this.handleClickOutside);
    },
    methods: {
        openModal() {
            this.showModal = true;
            this.loadHistory();
        },

        closeModal() {
            this.showModal = false;
            this.userDropdownOpen = false;
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

        async loadHistory() {
            this.loading = true;
            try {
                const response = await apiRequest(`/applications/${this.applicationId}/history`, {});

                if (response.ok) {
                    this.history = await response.json();
                    console.log("History loaded:", this.history);
                }
            } catch (error) {
                console.error("Error loading history:", error);
            } finally {
                this.loading = false;
            }
        },

        async exportToExcel() {
            if (this.filteredHistory.length === 0) return;
            
            this.isExporting = true;
            
            // Имитация процесса загрузки (2 секунды)
            await new Promise(resolve => setTimeout(resolve, 500));
            
            try {
                // Создаем рабочую книгу
                const workbook = new ExcelJS.Workbook();
                const worksheet = workbook.addWorksheet('История');
                
                // Заголовки
                const headers = [
                    'Дата и время',
                    'Пользователь',
                    'Действие',
                    'Старое значение',
                    'Новое значение',
                    'Комментарий',
                    'Обязательно',
                    'Кто переслал'
                ];
                
                // Добавляем заголовки
                const headerRow = worksheet.addRow(headers);
                
                // Стиль для заголовков (без жирных границ)
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
                    cell.alignment = { 
                        vertical: 'middle', 
                        horizontal: 'center',
                        wrapText: true
                    };
                    cell.border = {
                        top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                    };
                });
                
                // Добавляем данные
                this.exportData.forEach((item, index) => {
                    const row = worksheet.addRow([
                        item['Дата и время'],
                        item['Пользователь'],
                        item['Действие'],
                        item['Старое значение'],
                        item['Новое значение'],
                        item['Комментарий'],
                        item['Обязательно'],
                        item['Кто переслал']
                    ]);
                    
                    row.height = 20;
                    
                    // Чередование цветов строк
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
                        cell.alignment = { 
                            vertical: 'middle'
                        };
                        
                        // Внутренние границы тонкие
                        cell.border = {
                            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                        };
                    });
                });
                
                // Добавляем жирные внешние границы для таблицы с данными
                const lastDataRow = this.exportData.length; // последняя строка данных
                
                // Левая и правая границы для всех строк данных
                for (let row = 1; row <= lastDataRow + 1; row++) { // +1 для строки заголовков
                    // Правая граница для последнего столбца
                    const rightCell = worksheet.getCell(row, 8);
                    rightCell.border = {
                        ...rightCell.border,
                        right: { style: 'medium', color: { argb: 'FF000000' } }
                    };
                    
                    // Левая граница для первого столбца
                    const leftCell = worksheet.getCell(row, 1);
                    leftCell.border = {
                        ...leftCell.border,
                        left: { style: 'medium', color: { argb: 'FF000000' } }
                    };
                }
                
                // Верхняя граница для первой строки (заголовки)
                for (let col = 1; col <= 8; col++) {
                    const topCell = worksheet.getCell(1, col);
                    topCell.border = {
                        ...topCell.border,
                        top: { style: 'medium', color: { argb: 'FF000000' } }
                    };
                }
                
                // Нижняя граница для последней строки данных
                for (let col = 1; col <= 8; col++) {
                    const bottomCell = worksheet.getCell(lastDataRow + 1, col);
                    bottomCell.border = {
                        ...bottomCell.border,
                        bottom: { style: 'medium', color: { argb: 'FF000000' } }
                    };
                }
                
                // Добавляем пустую строку для отступа
                worksheet.addRow([]);
                
                // Добавляем строки с информацией о формировании (в разных ячейках)
                const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserDisplayName]);
                const infoRow2 = worksheet.addRow(['Дата формирования:', this.formattedCurrentDateTime]);
                
                // Стиль для информационных строк
                [infoRow1, infoRow2].forEach(row => {
                    row.eachCell((cell, colNumber) => {
                        cell.font = {
                            name: 'Verdana',
                            size: 10,
                            color: { argb: 'FF333333' }
                        };
                        cell.alignment = { 
                            vertical: 'middle',
                            horizontal: colNumber === 1 ? 'left' : 'left'
                        };
                        
                        // Добавляем тонкие границы для информационных строк
                        cell.border = {
                            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                        };
                    });
                });
                
                // Настраиваем ширину колонок
                worksheet.columns = [
                    { width: 25 }, // Дата и время
                    { width: 40 }, // Пользователь
                    { width: 50 }, // Действие
                    { width: 30 }, // Старое значение
                    { width: 30 }, // Новое значение
                    { width: 40 }, // Комментарий
                    { width: 20 }, // Обязательно
                    { width: 35 }  // Переслано
                ];
                
                // Генерируем и сохраняем файл
                const buffer = await workbook.xlsx.writeBuffer();
                const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                
                // Формируем имя файла: События_[Номер заявки]_[Организация заявки]
                const safeOrgName = this.fileOrganizationName.replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_');
                const safeAppNumber = this.applicationNumber.replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_');
                
                a.download = `События_${safeAppNumber}_${safeOrgName}.xlsx`;
                a.href = url;
                a.click();
                window.URL.revokeObjectURL(url);
                
            } catch (error) {
                console.error('Error exporting to Excel:', error);
                alert('Ошибка при экспорте в Excel');
            } finally {
                this.isExporting = false;
            }
        },

        getActionClass(actionType) {
            const classes = {
                'create': 'dot-create',
                'read': 'dot-read',
                'approve': 'dot-approve',
                'reject': 'dot-reject',
                'revoke_approval': 'dot-revoke',
                'take_to_work': 'dot-success',
                'revoke_from_work': 'dot-warning',
                'restore_to_work': 'dot-info',
                'assigned_responsible': 'dot-assign',
                'assigned_viewer': 'dot-view',
                'confirmation_change': 'dot-system',
                'status_change': 'dot-system',
                'blacklist_override': 'dot-success',
                'blacklist_override_revoke': 'dot-warning'
            };
            return classes[actionType] || 'dot-default';
        },

        getActionText(item) {
            // Для reject проверяем, было ли это отказом от принятия или отказом в согласовании
            if (item.action_type === 'reject') {
                // Если old_value был "В обработке" или "В работе", значит это отказ от принятия
                if (item.old_value === 'В обработке' || item.old_value === 'В работе') {
                    return 'Не принял(а) в работу';
                }
                return 'Не согласовал(а) заявку';
            }
            
            // Новый тип для просматривающих
            if (item.action_type === 'assigned_viewer') {
                
                return `Получил(-а) доступ к просмотру заявки`;
            }
            
            const texts = {
                'create': 'Создал(-а) заявку',
                'read': 'Прочитал(-а) заявку',
                'approve': 'Согласовал(-а) заявку',
                'revoke_approval': 'Отозвал(-а) согласование',
                'take_to_work': 'Принял(-а) в работу',
                'revoke_from_work': 'Отозвал(-а) из работы',
                'restore_to_work': 'Вернул(-а) в работу',
                'assigned_responsible': 'Назначен(-а) ответственным получателем',
                'assigned_viewer': 'Получил(-а) доступ к просмотру заявки',
                'confirmation_change': 'Статус согласования изменился',
                'status_change': 'Статус заявки изменился',
                'blacklist_override': 'Подтвердил(-а) пропуск (возможный обход ЧС)',
                'blacklist_override_revoke': 'Отменил(-а) подтверждение пропуска'
            };
            
            let text = texts[item.action_type] || item.action_type;
            
            
            
            return text;
        },

        formatTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            }).replace(',', '');
        },

        handleClickOutside(event) {
            const select = this.$el.querySelector('.custom-select');
            if (select && !select.contains(event.target) && this.userDropdownOpen) {
                this.userDropdownOpen = false;
            }
        }
    }
}
</script>

<style scoped>
.history-toggle {
    background: none;
    border: 1px solid #e6e6e6;
    border-radius: 50px;
    padding: 6px 12px;
    font-size: 13px;
    color: #666;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 6px;
    transition: all 0.2s ease;
}

.history-toggle:hover {
    background: #f5f5f5;
    border-color: #4F5BDF;
}

.history-modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 20000;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
    animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

.history-modal {
    background: white;
    border-radius: 30px;
    width: 580px;
    max-width: 95%;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
    animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
    from {
        transform: translateY(20px);
        opacity: 0;
    }
    to {
        transform: translateY(0);
        opacity: 1;
    }
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 25px;
    border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #333;
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 10px;
}

.close-btn {
    background: none;
    border: none;
    font-size: 20px;
    color: #a2a2a2;
    cursor: pointer;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: all 0.2s ease;
}

.close-btn:hover {
    background: #f5f5f5;
    border-color: #4F5BDF;
}

/* Кнопка экспорта в шапке */
.export-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 4px 12px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    font-size: 12px;
    color: #000;
    cursor: pointer;
    transition: all 0.2s ease;
    width: 100px;
    height: 25px;
}

.export-btn:hover:not(:disabled) {
    background: #f5f5f5;
    border-color: #4F5BDF;
}

.export-btn:disabled {
    opacity: 1;
    cursor: not-allowed;
}

.export-icon {
    width: 12px;
    height: 12px;
}

.export-loader {
    width: 16px;
    height: 16px;
    border: 2px solid #e6e6e6;
    border-top: 2px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

/* Фильтры */
.history-filters {
    padding: 10px 25px;
    border-bottom: 1px solid #e6e6e6;
    background-color: #fafafa;
}

.filter-row {
    display: flex;
    gap: 10px;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
}

.user-filter, .sort-filter {
    display: flex;
    align-items: center;
    gap: 8px;
}

.filter-label {
    font-size: 12px;
    color: #a2a2a2;
    white-space: nowrap;
}

/* Кастомный селект */
.custom-select {
    position: relative;
    width: 180px;
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

/* Анимация для дропдауна */
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
    height: auto;
    max-height: 300px;
    overflow-y: auto;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    z-index: 1000;
}

.select-dropdown::-webkit-scrollbar {
    width: 0px;
}

.select-dropdown::-webkit-scrollbar-track {
    background: transparent;
}

.select-dropdown::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 2px;
}

.select-option {
    padding: 8px 12px;
    font-size: 12px;
    color: #333;
    cursor: pointer;
    transition: all 0.2s ease;
    border-bottom: 1px solid #f0f0f0;
}

.select-option:last-child {
    border-bottom: none;
}

.select-option:hover {
    background-color: #f5f5f5;
}

.select-option.selected {
    background-color: #f0f3ff;
    font-weight: 500;
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
    width: 150px;
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
    height: calc(80vh - 150px);
    max-height: calc(80vh - 150px);
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-content::-webkit-scrollbar {
    display: none;
}

.history-loading,
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
    border: 2px solid #f3f3f3;
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
}

.history-date-separator {
    font-size: 11px;
    font-weight: 600;
    color: #a2a2a2;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 8px 0 4px;
    margin-bottom: 4px;
    border-bottom: 1px solid #f0f0f0;
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

.dot-create { background: #4F5BDF; }
.dot-read { background: #3b82f6; }
.dot-approve { background: #059669; }
.dot-reject { background: #dc2626; }
.dot-revoke { background: #f59e0b; }
.dot-success { background: #10b981; }
.dot-warning { background: #f59e0b; }
.dot-info { background: #3b82f6; }
.dot-assign { background: #8b5cf6; }
.dot-view { background: #9b59b6; }
.dot-system { background: #8b5cf6; }
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
    font-size: 13px;
}

.system-name {
    color: #8b5cf6;
    font-style: italic;
}

.action-time {
    color: #a2a2a2;
    font-size: 11px;
}

.action-text {
    color: #666;
    font-size: 12px;
    margin-bottom: 4px;
}

.forward-info {
    font-size: 11px;
    color: #8b5cf6;
    font-style: italic;
    margin-bottom: 4px;
}

.required-badge {
    display: inline-block;
    background: #4F5BDF;
    color: white;
    font-size: 10px;
    padding: 2px 8px;
    border-radius: 12px;
    margin-bottom: 4px;
}

.status-change {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    background: #f9f9f9;
    padding: 3px 8px;
    border-radius: 16px;
    display: inline-flex;
    margin-top: 2px;
}

.old-status {
    color: #dc2626;
    text-decoration: line-through;
    font-size: 11px;
}

.arrow {
    color: #a2a2a2;
    font-size: 10px;
}

.new-status {
    color: #059669;
    font-weight: 500;
    font-size: 11px;
}

.action-comment {
    font-size: 11px;
    color: #666;
    font-style: italic;
    margin-top: 4px;
    padding-left: 6px;
    border-left: 2px solid #e6e6e6;
}

@media (max-width: 768px) {
    .filter-row {
        flex-direction: column;
        align-items: flex-start;
        gap: 10px;
    }
    
    .user-filter, .sort-filter {
        width: 100%;
    }
    
    .custom-select {
        width: 100%;
    }
    
    .sort-btn {
        width: 100%;
    }
}
</style>