<template>
  <div class="modal-overlay" @mousedown="onOverlayMousedown" @mouseup="onOverlayMouseup">
    <div class="car-history-modal" @mousedown.stop>
      <div class="modal-header">
        <h3>История автомобиля {{ carNumber }}</h3>
        <div class="header-actions">
          <button class="export-btn" @click="exportToExcel" :disabled="filteredHistory.length === 0 || isExporting">
            <img v-if="!isExporting" src="@/assets/icons/export.png" class="export-icon" />
            <span v-if="!isExporting">Экспорт</span>
            <div v-else class="export-loader"></div>
          </button>
          <button class="close-btn" @click="close">×</button>
        </div>
      </div>

      <div class="history-filters">
        <div class="filter-row">
          <div class="search-filter">
            <span class="filter-label">Поиск:</span>
            <input 
              type="text" 
              v-model="searchQuery" 
              class="search-input" 
              placeholder="Поиск по пользователю, действию..."
              @input="applyFilters"
            />
          </div>
          <div class="user-filter">
            <span class="filter-label">Пользователь:</span>
            <div class="custom-select" @click="toggleUserDropdown">
              <div class="select-trigger">
                <span class="selected-value">{{ selectedUserName }}</span>
                <img 
                  src="@/assets/icons/arrow.png" 
                  class="select-arrow" 
                  :class="{ 'arrow-open': userDropdownOpen }"
                />
              </div>
              <transition name="fade">
                <div v-if="userDropdownOpen" class="select-dropdown">
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
          
          <div class="date-filter">
            <span class="filter-label">Период:</span>
            <input 
              type="date" 
              v-model="dateFrom" 
              class="date-input"
              @change="applyFilters"
            />
            <span class="date-separator">—</span>
            <input 
              type="date" 
              v-model="dateTo" 
              class="date-input"
              @change="applyFilters"
            />
          </div>
          
          <div class="sort-filter">
            <span class="filter-label">Сортировка:</span>
            <button class="sort-btn" @click="toggleSortOrder">
              <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sort-asc': sortOrder === 'asc' }" />
              <span>{{ sortOrder === 'desc' ? 'Сначала новые' : 'Сначала старые' }}</span>
            </button>
          </div>
        </div>
      </div>

      <div class="modal-content" ref="scrollContainer">
        <div v-if="loading" class="history-loading">
          <LoaderSpinner label="Загрузка истории…" />
        </div>
        
        <div v-else-if="filteredHistory.length === 0" class="history-empty">
          История пуста
        </div>
        
        <div v-else class="history-timeline">
          <div 
            v-for="(item, index) in filteredHistory" 
            :key="item.id" 
            class="history-item"
          >
            <div class="timeline-dot" :class="getActionClass(item.action_type)"></div>
            <div class="timeline-line" v-if="index < filteredHistory.length - 1"></div>
            
            <div class="history-content">
              <div class="history-header">
                <span class="user-name">{{ item.user_name || 'Система' }}</span>
                <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
              </div>
              
              <div class="action-text">{{ getActionText(item) }}</div>
              
              <div class="action-comment" v-if="item.action_type === 'entry' || item.action_type === 'exit'">
                {{ getActionComment(item) }}
              </div>
              
              <div v-if="item.comment && item.action_type !== 'entry' && item.action_type !== 'exit'" class="action-comment">
                {{ item.comment }}
              </div>
              
              <div v-if="item.old_value && item.new_value && item.old_value !== item.new_value" class="value-change">
                <span class="old-value">{{ item.old_value }}</span>
                <span class="arrow">→</span>
                <span class="new-value">{{ item.new_value }}</span>
              </div>
              
              <div v-if="item.field_name" class="field-name">
                Поле: {{ item.field_name }}
              </div>
              
              <div v-if="item.table_name" class="place-name">
                {{ item.table_name }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useOverlayClose } from '@/composables/useOverlayClose';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import ExcelJS from 'exceljs';

export default {
  name: 'CarHistoryModal',
  components: { LoaderSpinner },
  setup(_, { emit }) {
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
    return { onOverlayMousedown, onOverlayMouseup };
  },
  props: {
    carId: {
      type: Number,
      required: true
    },
    carNumber: {
      type: String,
      default: ''
    },
    carBrand: {
      type: String,
      default: ''
    },
    organizationId: {
      type: Number,
      default: null
    },
    organizationName: {
      type: String,
      default: ''
    },
    companyId: {
      type: Number,
      default: null
    },
    companyName: {
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
    }
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

    selectedUserName() {
      if (this.selectedUserId === null) return 'Все пользователи';
      const user = this.uniqueUsers.find(u => u.id === this.selectedUserId);
      return user ? user.name : 'Все пользователи';
    },

    filteredHistory() {
      let filtered = [...this.history];
      
      if (this.searchQuery && this.searchQuery.trim() !== '') {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter(item => {
          const userName = (item.user_name || '').toLowerCase();
          const actionText = this.getActionText(item).toLowerCase();
          const comment = this.getActionComment(item).toLowerCase();
          const fieldName = (item.field_name || '').toLowerCase();
          const carInfo = `${item.car_number || ''} ${item.car_brand || ''} ${item.organization || ''} ${item.company || ''}`.toLowerCase();
          
          return userName.includes(query) || 
                 actionText.includes(query) || 
                 comment.includes(query) ||
                 fieldName.includes(query) ||
                 carInfo.includes(query);
        });
      }
      
      if (this.selectedUserId) {
        filtered = filtered.filter(item => item.user_id === this.selectedUserId);
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
      
      filtered = filtered.filter(item => {
        if (item.old_value === 'approved' && item.new_value === 'pending') return false;
        if (item.old_value === 'rejected' && item.new_value === 'pending') return false;
        if (item.old_value && item.new_value && item.old_value === item.new_value) return false;
        return true;
      });
      
      return filtered.sort((a, b) => {
        const timeA = new Date(a.created_at).getTime();
        const timeB = new Date(b.created_at).getTime();
        return this.sortOrder === 'desc' ? timeB - timeA : timeA - timeB;
      });
    },

    exportData() {
      return this.filteredHistory.map(item => ({
        'Дата и время': this.formatDateTime(item.created_at),
        'Пользователь': item.user_name || 'Система',
        'Действие': this.getActionText(item),
        'Комментарий': this.getActionComment(item),
        'Тип действия': item.action_type,
        'Старое значение': item.old_value || '',
        'Новое значение': item.new_value || '',
        'Поле': item.field_name || '',
        'ID записи': item.car_id,
        'Номер': item.car_number || '',
        'Марка': item.car_brand || '',
        'Организация': item.organization || '',
        'Компания': item.company || '',
        'Место': item.table_name || ''
      }));
    },

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

    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter(part => part && part !== 'null' && part !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    },

    safeCarNumber() {
      return this.carNumber.replace(/[\\/:"*?<>|]/g, '_').replace(/\s+/g, '_') || 'Avtomobil';
    }
  },
  methods: {
    async loadHistory() {
      this.loading = true;
      try {
        // apiRequest ожидает строку-путь, не URL object — собираем query руками.
        const params = new URLSearchParams();
        params.append('car_number', this.carNumber);
        params.append('car_brand', this.carBrand || '');
        if (this.organizationId) params.append('organization_id', this.organizationId);
        if (this.companyId) params.append('company_id', this.companyId);

        const response = await apiRequest(`/cars/history/unified?${params.toString()}`, {});

        if (response.ok) {
          this.history = await response.json();
          console.log('Загружена объединённая история автомобиля:', this.history);
        } else {
          console.error('Ошибка загрузки истории:', response.status);
        }
      } catch (error) {
        console.error("Error loading car history:", error);
      } finally {
        this.loading = false;
      }
    },

    getActionClass(actionType) {
      const classes = {
        'create': 'dot-create',
        'entry': 'dot-entry',
        'exit': 'dot-exit',
        'update': 'dot-update',
        'delete': 'dot-delete',
        'activate': 'dot-activate',
        'deactivate': 'dot-deactivate'
      };
      return classes[actionType] || 'dot-default';
    },

    getActionText(item) {
      if (item.action_type === 'entry') {
        return 'Отметил о прибытии';
      } else if (item.action_type === 'exit') {
        return 'Машина уехала';
      }
      
      const texts = {
        'create': 'Подана заявка на автомобиль',
        'update': 'Данные обновлены',
        'delete': 'Автомобиль удалён',
        'activate': 'Автомобиль введён в работу',
        'deactivate': 'Автомобиль выведен из работы',
        'restore': 'Автомобиль восстановлен'
      };
      
      let text = texts[item.action_type] || item.action_type;
      
      if (item.action_type === 'update' && item.field_name) {
        text = `Изменено поле "${item.field_name}"`;
      }
      
      return text;
    },

    getActionComment(item) {
      const userName = item.user_name || 'Система';
      
      const carNumber = item.car_number || this.carNumber;
      const carBrand = item.car_brand || this.carBrand;
      
      switch (item.action_type) {
        case 'entry':
          return `Пользователь ${userName} отметил о прибытии автомобиля ${carNumber} ${carBrand} на территорию`;
        
        case 'exit':
          return `Пользователь ${userName} отметил об убытии автомобиля ${carNumber} ${carBrand} с территории`;
        
        case 'create':
          return `Подана заявка на автомобиль ${carNumber} ${carBrand}`;
        
        case 'activate':
          return `Автомобиль ${carNumber} ${carBrand} введён в работу пользователем ${userName}`;
        
        case 'deactivate':
          return `Автомобиль ${carNumber} ${carBrand} выведен из работы пользователем ${userName}`;
        
        case 'delete':
          return `Автомобиль ${carNumber} ${carBrand} удалён пользователем ${userName}`;
        
        case 'restore':
          return `Автомобиль ${carNumber} ${carBrand} восстановлен пользователем ${userName}`;
        
        case 'update':
          if (item.field_name) {
            const oldVal = this.formatValue(item.old_value);
            const newVal = this.formatValue(item.new_value);
            return `Пользователь ${userName} изменил поле "${item.field_name}" с "${oldVal}" на "${newVal}"`;
          }
          return `Пользователь ${userName} обновил данные автомобиля ${carNumber} ${carBrand}`;
        
        default:
          return item.comment || '';
      }
    },

    formatValue(value) {
      if (!value) return 'пусто';
      
      if (typeof value === 'string' && (value.startsWith('{') || value.startsWith('['))) {
        try {
          const parsed = JSON.parse(value);
          if (parsed && typeof parsed === 'object') {
            if (parsed.car_number) return `Номер: ${parsed.car_number}`;
            if (parsed.organization) return `Организация: ${parsed.organization}`;
            if (parsed.unload_places) return `Мест разгрузки: ${parsed.unload_places.length}`;
            return JSON.stringify(parsed);
          }
        } catch {
          // Невалидный JSON, оставляем как есть
        }
      }
      
      return value;
    },

    formatDateTime(dateTimeString) {
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

    applyFilters() {},

    handleClickOutside(event) {
      const select = this.$el.querySelector('.custom-select');
      if (select && !select.contains(event.target)) {
        this.userDropdownOpen = false;
      }
    },

    async exportToExcel() {
      if (this.filteredHistory.length === 0) return;
      
      this.isExporting = true;
      
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet(`Istoriya_${this.safeCarNumber}`);
        
        const headers = [
          'Дата и время',
          'Пользователь',
          'Действие',
          'Комментарий',
          'Тип действия',
          'Старое значение',
          'Новое значение',
          'Поле',
          'ID записи',
          'Номер',
          'Марка',
          'Организация',
          'Компания',
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
            item['Пользователь'],
            item['Действие'],
            item['Комментарий'],
            item['Тип действия'],
            item['Старое значение'],
            item['Новое значение'],
            item['Поле'],
            item['ID записи'],
            item['Номер'],
            item['Марка'],
            item['Организация'],
            item['Компания'],
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
          const rightCell = worksheet.getCell(row, 14);
          rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
          const leftCell = worksheet.getCell(row, 1);
          leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        for (let col = 1; col <= 14; col++) {
          const topCell = worksheet.getCell(1, col);
          topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        for (let col = 1; col <= 14; col++) {
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
          { width: 30 },
          { width: 60 },
          { width: 20 },
          { width: 25 },
          { width: 25 },
          { width: 20 },
          { width: 15 },
          { width: 20 },
          { width: 20 },
          { width: 30 },
          { width: 30 },
          { width: 30 }
        ];
        
        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        
        a.download = `Istoriya_avtomobilya_${this.safeCarNumber}_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`;
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

    close() {
      this.$emit('close');
    }
  },
  mounted() {
    this.loadHistory();
    console.log('CarHistoryModal получил пропсы:', {
      carNumber: this.carNumber,
      carBrand: this.carBrand,
      organizationId: this.organizationId,
      companyId: this.companyId
    });
    document.addEventListener('click', this.handleClickOutside);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
  }
};
</script>

<style scoped>
.place-name {
  font-size: 11px;
  color: #4F5BDF;
  margin-top: 2px;
  font-weight: 500;
}

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
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.car-history-modal {
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

.select-dropdown::-webkit-scrollbar {
  width: 0px;
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
.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }
.dot-update { background: #f59e0b; }
.dot-delete { background: #6b7280; }
.dot-activate { background: #10b981; }
.dot-deactivate { background: #9ca3af; }
.dot-default { background: #9ca3af; }

.history-content {
  flex: 1;
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
}

.value-change {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  background: #f9f9f9;
  padding: 3px 8px;
  border-radius: 16px;
  display: inline-flex;
  margin-top: 4px;
}

.old-value {
  color: #dc2626;
  text-decoration: line-through;
  font-size: 11px;
}

.arrow {
  color: #a2a2a2;
  font-size: 10px;
}

.new-value {
  color: #059669;
  font-weight: 500;
  font-size: 11px;
}

.field-name {
  font-size: 11px;
  color: #8b5cf6;
  margin-top: 2px;
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