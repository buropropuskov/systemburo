<template>
  <Teleport to="body">
    <transition
      name="modal-fade"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="visible"
        class="modal-overlay"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="employees-history-modal"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          @mousedown.stop
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />
          <div class="modal-header">
            <h3>История проходов сотрудников (таблица)</h3>
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
                  placeholder="Поиск по сотруднику, пользователю..."
                  @input="applyFilters"
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
          
              <div class="employee-filter">
                <span class="filter-label">Сотрудник:</span>
                <div
                  ref="employeeSelect"
                  class="custom-select"
                  @click="toggleEmployeeDropdown"
                >
                  <div class="select-trigger">
                    <span class="selected-value">{{ selectedEmployeeName }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ 'arrow-open': employeeDropdownOpen }"
                    />
                  </div>
                  <transition name="fade">
                    <div
                      v-if="employeeDropdownOpen"
                      class="select-dropdown"
                    >
                      <div 
                        class="select-option"
                        :class="{ 'selected': selectedEmployeeId === null }"
                        @click="selectEmployee(null)"
                      >
                        Все сотрудники
                      </div>
                      <div 
                        v-for="emp in uniqueEmployees" 
                        :key="emp.id"
                        class="select-option"
                        :class="{ 'selected': selectedEmployeeId === emp.id }"
                        @click="selectEmployee(emp.id)"
                      >
                        {{ emp.name }}
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
                  @change="applyFilters"
                >
                <span class="date-separator">—</span>
                <input 
                  v-model="dateTo" 
                  type="date" 
                  class="date-input"
                  @change="applyFilters"
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

          <div
            ref="scrollContainer"
            class="history-scroll"
          >
            <div
              v-if="loading"
              class="history-loading"
            >
              <div class="loader" />
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
                      <span class="employee-info">{{ getEmployeeName(item) }}</span>
                      <span
                        v-if="item.table_name"
                        class="table-name"
                      >{{ item.table_name }}</span>
                      <span class="user-name">{{ item.user_name || 'Система' }}</span>
                      <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                    </div>

                    <div class="action-text">
                      {{ getActionText(item) }}
                    </div>

                    <div class="action-comment">
                      {{ item.comment || getActionComment(item) }}
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
import { apiRequest } from '@/api/client';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { useDeletionsStore } from '@/stores/deletions';
import ExcelJS from 'exceljs';
import AppIcon from '@/components/icons/AppIcon.vue';
import { formatMoscow, formatMoscowDateTime } from '@/utils/serverTime';

export default {
  name: 'EmployeesTableHistoryModal',
  components: { AppIcon },
  props: {
    tableId: {
      type: Number,
      required: true
    },
    currentUserId: {
      type: Number,
      default: null
    },
    currentUserName: {
      type: String,
      default: ''
    }
  },
  emits: ['close'],
  setup(_, { emit }) {
    // Анимация закрытия: внутренний visible (enter по mounted, leave по requestClose),
    // emit('close') только ПОСЛЕ leave-перехода (@after-leave) - иначе родитель
    // размонтирует мгновенно и анимация не проиграется.
    const visible = ref(false);
    const requestClose = () => { visible.value = false; };
    const onAfterLeave = () => emit('close');
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(requestClose);
    // Bottom-sheet на мобилке: свайп вниз за ползунок закрывает (как в истории сотрудника).
    const scrollContainer = ref(null);
    const swipe = useSwipeDismiss(requestClose, {
      getScrollTop: () => scrollContainer.value?.scrollTop ?? 0,
      handleSelector: '.sheet-handle',
    });
    return {
      visible, requestClose, onAfterLeave, onOverlayMousedown, onOverlayMouseup,
      scrollContainer,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      onSheetTouchStart: swipe.onTouchStart,
      onSheetTouchMove: swipe.onTouchMove,
      onSheetTouchEnd: swipe.onTouchEnd,
    };
  },
  data() {
    return {
      loading: false,
      history: [],
      sortOrder: 'desc',
      searchQuery: '',
      selectedUserId: null,
      selectedEmployeeId: null,
      dateFrom: '',
      dateTo: '',
      userDropdownOpen: false,
      employeeDropdownOpen: false,
      isExporting: false
    };
  },
  computed: {
    uniqueUsers() {
      const users = new Map();
      this.history.forEach(item => {
        if (item.user_id && !users.has(item.user_id)) {
          users.set(item.user_id, {
            id: item.user_id,
            name: item.user_name || 'Система'
          });
        }
      });
      return Array.from(users.values()).sort((a, b) => a.name.localeCompare(b.name));
    },

    uniqueEmployees() {
      const emps = new Map();
      this.history.forEach(item => {
        if (item.employee_id && !emps.has(item.employee_id)) {
          const fullName = [item.employee_last_name, item.employee_first_name, item.employee_middle_name].filter(Boolean).join(' ');
          emps.set(item.employee_id, {
            id: item.employee_id,
            name: fullName || `ID: ${item.employee_id}`
          });
        }
      });
      return Array.from(emps.values()).sort((a, b) => a.name.localeCompare(b.name));
    },

    selectedUserName() {
      if (this.selectedUserId === null) return 'Все пользователи';
      const user = this.uniqueUsers.find(u => u.id === this.selectedUserId);
      return user ? user.name : 'Все пользователи';
    },

    selectedEmployeeName() {
      if (this.selectedEmployeeId === null) return 'Все сотрудники';
      const emp = this.uniqueEmployees.find(e => e.id === this.selectedEmployeeId);
      return emp ? emp.name : 'Все сотрудники';
    },

    filteredHistory() {
      let filtered = [...this.history];
      
      if (this.searchQuery && this.searchQuery.trim() !== '') {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter(item => {
          const employeeName = this.getEmployeeName(item).toLowerCase();
          const userName = (item.user_name || '').toLowerCase();
          const actionText = this.getActionText(item).toLowerCase();
          const comment = (item.comment || '').toLowerCase();
          
          return employeeName.includes(query) || 
                 userName.includes(query) || 
                 actionText.includes(query) || 
                 comment.includes(query);
        });
      }
      
      if (this.selectedUserId) {
        filtered = filtered.filter(item => item.user_id === this.selectedUserId);
      }
      
      if (this.selectedEmployeeId) {
        filtered = filtered.filter(item => item.employee_id === this.selectedEmployeeId);
      }
      
      if (this.dateFrom) {
        const fromDate = new Date(this.dateFrom);
        fromDate.setHours(0, 0, 0, 0);
        filtered = filtered.filter(item => new Date(item.created_at) >= fromDate);
      }
      
      if (this.dateTo) {
        const toDate = new Date(this.dateTo);
        toDate.setHours(23, 59, 59, 999);
        filtered = filtered.filter(item => new Date(item.created_at) <= toDate);
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
        const dateKey = formatMoscow(new Date(item.created_at), { day: 'numeric', month: 'long', year: 'numeric' });
        if (!dateMap.has(dateKey)) {
          dateMap.set(dateKey, []);
          groups.push({ date: dateKey, items: dateMap.get(dateKey) });
        }
        dateMap.get(dateKey).push(item);
      }
      return groups;
    },

    exportData() {
      return this.filteredHistory.map(item => ({
        'Дата и время': this.formatDateTime(item.created_at),
        'Сотрудник': this.getEmployeeName(item),
        'Пользователь': item.user_name || 'Система',
        'Действие': this.getActionText(item),
        'Комментарий': item.comment || this.getActionComment(item),
        'Место': item.table_name || ''
      }));
    },

    formattedCurrentDateTime() {
      return formatMoscowDateTime();
    },

    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter(part => part && part !== 'null' && part !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    }
  },
  mounted() {
    this.visible = true;
    this.loadHistory();
    document.addEventListener('click', this.handleClickOutside);
    document.addEventListener('keydown', this.onKeydown);
    setBodyScrollLock(this, true);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
    document.removeEventListener('keydown', this.onKeydown);
    releaseBodyScrollLock(this);
  },
  methods: {
    async loadHistory() {
      this.loading = true;
      try {
        const response = await apiRequest(`/employees/history/table/${this.tableId}`, { method: "GET" });

        if (response.ok) {
          const data = await response.json();
          this.history = data;
        } else {
          console.error('Ошибка загрузки истории таблицы:', response.status);
        }
      } catch (error) {
        console.error("Error loading table history:", error);
      } finally {
        this.loading = false;
      }
    },

    getEmployeeName(item) {
      return [item.employee_last_name, item.employee_first_name, item.employee_middle_name].filter(Boolean).join(' ') || `ID: ${item.employee_id}`;
    },

    getActionClass(actionType) {
      // Зелёная точка - сотрудник появляется/остаётся в таблице (проход, восстановление,
      // добавление/перенос, ввод в работу, снятие с ЧС). Остальное (выход, удаление,
      // вывод из работы по истечении срока, добавление в ЧС) - красная по умолчанию.
      if (['entry', 'restore', 'added_to_table', 'moved_between_tables', 'activate', 'create', 'unblacklisted', 'blacklist_override'].includes(actionType)) return 'dot-entry';
      return 'dot-exit';
    },

    getActionText(item) {
      // GetByTable (/employees/history/table/:id) не фильтрует action_type (урок #1085),
      // поэтому словарь обязан покрывать все действия сотрудника, иначе deactivate/create/
      // прочие текут в журнал сырым английским кодом.
      if (item.action_type === 'entry') {
        return 'Проход на территорию';
      } else if (item.action_type === 'exit') {
        return 'Выход с территории';
      } else if (item.action_type === 'delete') {
        return 'Удаление из таблицы';
      } else if (item.action_type === 'restore') {
        return 'Восстановление в таблице';
      } else if (item.action_type === 'purge') {
        return 'Безвозвратное удаление';
      } else if (item.action_type === 'added_to_table') {
        return 'Добавлен в таблицу проходной';
      } else if (item.action_type === 'moved_between_tables') {
        return 'Перенесён между таблицами';
      } else if (item.action_type === 'unbound_from_table') {
        return 'Снят с таблицы';
      } else if (item.action_type === 'create') {
        return 'Подана заявка на сотрудника';
      } else if (item.action_type === 'activate') {
        return 'Сотрудник введён в работу';
      } else if (item.action_type === 'deactivate') {
        return 'Сотрудник выведен из работы';
      } else if (item.action_type === 'blacklisted') {
        return 'Добавлен в чёрный список';
      } else if (item.action_type === 'unblacklisted') {
        return 'Снят с чёрного списка';
      } else if (item.action_type === 'blacklist_override') {
        return 'Пропущен несмотря на подозрение в обходе ЧС';
      } else if (item.action_type === 'blacklist_override_revoke') {
        return 'Отменено подтверждение пропуска (обход ЧС)';
      } else if (item.action_type === 'update') {
        return item.field_name ? `Изменено поле "${item.field_name}"` : 'Данные обновлены';
      }
      return item.action_type;
    },

    getActionComment(item) {
      const fullName = this.getEmployeeName(item);
      const userName = item.user_name || 'Система';
      if (item.action_type === 'entry') {
        return `Пользователь ${userName} отметил проход сотрудника ${fullName} на территорию`;
      } else if (item.action_type === 'exit') {
        return `Пользователь ${userName} отметил выход сотрудника ${fullName} с территории`;
      }
      return item.comment || '';
    },

    formatDateTime(dateTimeString) {
      if (!dateTimeString) return '';
      return formatMoscowDateTime(new Date(dateTimeString));
    },

    toggleSortOrder() {
      this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
    },

    toggleUserDropdown() {
      this.userDropdownOpen = !this.userDropdownOpen;
      this.employeeDropdownOpen = false;
    },

    toggleEmployeeDropdown() {
      this.employeeDropdownOpen = !this.employeeDropdownOpen;
      this.userDropdownOpen = false;
    },

    selectUser(userId) {
      this.selectedUserId = userId;
      this.userDropdownOpen = false;
    },

    selectEmployee(employeeId) {
      this.selectedEmployeeId = employeeId;
      this.employeeDropdownOpen = false;
    },

    applyFilters() {},

    handleClickOutside(event) {
      // refs, не this.$el.querySelectorAll: при <Teleport> $el - это якорный комментарий
      // без querySelector, и дропдауны фильтров не закрывались бы по клику снаружи.
      const selects = [this.$refs.userSelect, this.$refs.employeeSelect];
      const clickedInside = selects.some(
        (select) => select && select.contains(event.target),
      );
      if (!clickedInside) {
        this.userDropdownOpen = false;
        this.employeeDropdownOpen = false;
      }
    },

    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      
      this.isExporting = true;
      
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Istoriya_prokhodov');
        
        const headers = [
          'Дата и время',
          'Сотрудник',
          'Пользователь',
          'Действие',
          'Комментарий',
          'Место'
        ];
        
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
        
        this.exportData.forEach((item, index) => {
          const row = worksheet.addRow([
            item['Дата и время'],
            item['Сотрудник'],
            item['Пользователь'],
            item['Действие'],
            item['Комментарий'],
            item['Место']
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
        
        const lastDataRow = this.exportData.length;
        
        for (let row = 1; row <= lastDataRow + 1; row++) {
          const rightCell = worksheet.getCell(row, 6);
          rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
          const leftCell = worksheet.getCell(row, 1);
          leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        for (let col = 1; col <= 6; col++) {
          const topCell = worksheet.getCell(1, col);
          topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        for (let col = 1; col <= 6; col++) {
          const bottomCell = worksheet.getCell(lastDataRow + 1, col);
          bottomCell.border = { ...bottomCell.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        worksheet.addRow([]);
        
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserDisplayName]);
        const infoRow2 = worksheet.addRow(['Дата формирования:', this.formattedCurrentDateTime]);
        
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
        
        worksheet.columns = [
          { width: 25 },
          { width: 40 },
          { width: 40 },
          { width: 30 },
          { width: 60 },
          { width: 30 }
        ];
        
        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        
        a.download = `Istoriya_prokhodov_tabl_${this.tableId}_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
        
      } catch (error) {
        console.error('Error exporting to Excel:', error);
        useDeletionsStore().notify({ bold: 'Ошибка при экспорте в Excel', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },

    onKeydown(e) {
      if (e.key === 'Escape') this.requestClose();
    },
  }
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
  z-index: 13000;
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

.modal-fade-enter-active .employees-history-modal {
  animation: modal-scale-in 0.25s ease;
}

.modal-fade-leave-active .employees-history-modal {
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

.sheet-handle {
  display: none;
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 2px;
  flex-shrink: 0;
}

@keyframes employeesTableHistorySlideUp {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.employees-history-modal {
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
  padding: 15px 20px;
  border-bottom: 1px solid var(--border);
  background-color: var(--surface-2);
  flex-shrink: 0;
}

.filter-row {
  display: flex;
  gap: 15px;
  align-items: center;
  flex-wrap: wrap;
}

.search-filter,
.user-filter,
.employee-filter,
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

.select-dropdown::-webkit-scrollbar {
  width: 0px;
}

.select-option {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  border-bottom: 1px solid var(--border);
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background-color: var(--surface-2);
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
  width: 150px;
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

/* Имя .modal-content ловило глобальное min-width:100vw из App.vue и распирало
   внутренний блок внутри самого окна - у скролл-контейнера своё имя. */
.history-scroll {
  padding: 20px;
  overflow-y: auto;
  flex: 1;
}

.history-loading,
.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: var(--text-muted);
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid var(--surface-2);
  border-top: 3px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.history-timeline {
  position: relative;
  padding-left: 20px;
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
  background: var(--border);
}

.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }

.history-content {
  flex: 1;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
  flex-wrap: wrap;
  gap: 5px;
}

.employee-info {
  font-weight: 600;
  color: var(--accent-text);
  font-size: 13px;
}

.table-name {
  font-weight: 500;
  color: var(--accent-text);
  font-size: 12px;
  font-style: italic;
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
}

@media (max-width: 768px) {
  /* Bottom-sheet: окно выезжает снизу, свайп вниз за ползунок закрывает. Своё имя
     класса не ловит глобальный паттерн .modal-content из App.vue, поэтому лист
     описан здесь целиком - раньше карточка 900px с радиусом 30px просто липла к
     нижней кромке экрана. */
  .modal-overlay {
    padding: 0;
    align-items: flex-end;
    top: 0;
    height: 100dvh;
    bottom: auto;
  }

  .employees-history-modal {
    width: 100%;
    max-width: 100%;
    max-height: 90dvh;
    border-radius: 16px 16px 0 0;
    transition: transform 0.3s ease;
  }

  .employees-history-modal.is-dragging {
    transition: none;
  }

  .sheet-handle {
    display: block;
  }

  .close-btn,
  .export-btn {
    min-height: 40px;
  }

  .modal-fade-enter-active .employees-history-modal {
    animation: employeesTableHistorySlideUp 0.3s ease-out;
  }

  .modal-fade-leave-active .employees-history-modal {
    animation: none;
    transition: transform 0.3s cubic-bezier(0.32, 0.72, 0, 1);
  }

  .modal-fade-leave-to .employees-history-modal {
    transform: translateY(100%);
  }

  .modal-fade-leave-active {
    transition: opacity 0.3s ease;
  }

  /* flex-элемент с min-width:auto в колонке = ширина по содержимому: без width
     фильтры распирали окно шире экрана. */
  .filter-row {
    flex-direction: column;
    align-items: flex-start;
    width: 100%;
    min-width: 0;
  }
  
  .search-filter,
  .user-filter,
  .employee-filter,
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