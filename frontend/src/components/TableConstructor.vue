<template>
  <div class="table-constructor-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Таблицы системы</h3>
      <div class="header-controls">
        <SearchComponent
          :title="'Поиск таблиц...'"
          v-model="searchQuery"
        />
        <button @click="showAddModal = true" class="add-header-button">
          Создать таблицу
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список таблиц -->
      <div class="table-section" :class="{'with-details': selectedTable}">
        <div class="table-container">
          <div class="table-header">
            <div class="header-col id-col" @click="sortBy('id')">
              <p :class="{ 'active-sort': sortField === 'id' }">ID</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col name-col" @click="sortBy('name')">
              <p :class="{ 'active-sort': sortField === 'name' }">Наименование</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'name',
                  'desc': sortField === 'name' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col type-col" @click="sortBy('type')">
              <p :class="{ 'active-sort': sortField === 'type' }">Тип</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'type',
                  'desc': sortField === 'type' && sortDirection === 'desc'
                }" 
              />
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="table in sortedTables" 
              :key="table.id" 
              class="table-row"
              :class="{'selected': selectedTable && selectedTable.id === table.id}"
              @click="selectTable(table)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ table.id }}</span>
              </div>
              <div class="table-col name-col">
                <span class="truncate-text" :title="table.display_name">
                  {{ table.display_name }}
                </span>
              </div>
              <div class="table-col type-col">
                <span class="type-badge" :class="table.table_type">
                  {{ getTableTypeLabel(table.table_type) }}
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего таблиц: {{ filteredTables.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали таблицы -->
      <div v-if="selectedTable" class="details-section">
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <div class="table-info-title">
              <h3 class="details-title">{{ selectedTable.display_name }}</h3>
              <span class="table-type-badge" :class="selectedTable.table_type">
                  {{ getTableTypeLabel(selectedTable.table_type) }}
              </span>
              </div>
              <div class="table-info-row">
                <span class="system-name">{{ selectedTable.name }}</span>
              </div>
            </div>
            <div class="details-header-actions">
              <button @click="openTable" class="action-btn view-btn">
                Открыть
              </button>
              <button @click="confirmDeleteTable(selectedTable)" class="delete-icon-btn">
                <img src="@/assets/icons/delete.png" class="delete-icon" />
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="compact-form">
              <div class="form-row">
                <div class="form-group compact">
                  <label class="detail-label">Наименование таблицы:</label>
                  <input 
                    v-model="selectedTable.display_name" 
                    @change="updateTableDisplayName"
                    class="form-input-sm"
                    placeholder="Название таблицы"
                    autocomplete="off"
                  >
                </div>
                <div class="form-group compact">
                  <label class="detail-label">Тип таблицы:</label>
                  <div class="custom-select">
                    <div class="select-header" @click="toggleTableTypeDropdown">
                      <span class="select-value">{{ getTableTypeLabel(selectedTable.table_type) }}</span>
                      <img src="@/assets/icons/arrow.png" class="select-arrow" :class="{ rotated: tableTypeDropdownOpen }" />
                    </div>
                    <transition name="dropdown-fade">
                      <div v-if="tableTypeDropdownOpen" class="select-dropdown">
                        <div 
                          class="select-option"
                          :class="{ active: selectedTable.table_type === 'cars' }"
                          @click="selectTableType('cars')"
                        >
                          Машины
                        </div>
                        <div 
                          class="select-option"
                          :class="{ active: selectedTable.table_type === 'people' }"
                          @click="selectTableType('people')"
                        >
                          Люди
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
              </div>

              <div class="settings-section">
                <label class="section-label">Настройки отображения:</label>
                
                <div class="checkbox-group">
                  <label class="checkbox-label">
                    <input 
                      type="checkbox" 
                      v-model="selectedTable.show_fact_table"
                      @change="updateTableSettings"
                      class="checkbox-input"
                    />
                    <span class="checkbox-text">Отображать таблицу "по факту"</span>
                  </label>
                </div>

                <div v-if="selectedTable.show_fact_table" class="hint-section">
                  <div class="section-header-with-actions">
                    <label class="detail-label">Подсказка для таблицы "по факту":</label>
                    <div class="editor-actions" v-if="hintHasChanges">
                      <button @click="cancelHintEdit" class="compact-btn cancel-btn">Отмена</button>
                      <button @click="saveHint" class="compact-btn save-btn">Сохранить</button>
                    </div>
                  </div>
                  <TextConstructor
                    v-model="selectedTable.fact_table_hint"
                    :placeholder="getDefaultHint(selectedTable.table_type)"
                    rows="4"
                    ref="hintConstructor"
                  />
                </div>
              </div>

              <div class="instruction-section">
                <div class="section-header-with-actions">
                  <label class="detail-label">Инструкция к таблице:</label>
                  <div class="editor-actions" v-if="instructionHasChanges">
                    <button @click="cancelInstructionEdit" class="compact-btn cancel-btn">Отмена</button>
                    <button @click="saveInstruction" class="compact-btn save-btn">Сохранить</button>
                  </div>
                </div>
                <TextConstructor
                  v-model="selectedTable.instruction"
                  placeholder="Введите инструкцию для таблицы..."
                  rows="8"
                  ref="instructionConstructor"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div v-else class="no-selection-message">
        <p>Выберите таблицу для просмотра и редактирования</p>
      </div>
    </div>

    <div v-if="filteredTables.length === 0" class="no-results">
      <div class="no-results-icon">📊</div>
      <p>Таблицы не найдены</p>
    </div>

    <!-- Модальное окно создания таблицы -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content horizontal-modal">
        <div class="modal-header">
          <h3>Создать новую таблицу</h3>
          <button @click="showAddModal = false" class="modal-close">×</button>
        </div>
        
        <div class="modal-body-horizontal">
          <!-- Левая часть - основная информация -->
          <div class="modal-main-info">
            <div class="main-fields">
              <div class="form-group-compact">
                <label class="form-label-compact">Наименование таблицы *</label>
                <input
                  v-model="newTable.display_name"
                  placeholder="ПОСТ №27"
                  class="input-compact"
                >
              </div>
              
              <div class="form-group-compact">
                <label class="form-label-compact">Системное имя *</label>
                <input
                  v-model="newTable.name"
                  @input="validateSystemName"
                  placeholder="post_27"
                  class="input-compact"
                >
                <span class="form-hint">Латинские буквы, цифры и подчеркивания</span>
                <span v-if="nameError" class="form-error">{{ nameError }}</span>
              </div>

              <div class="form-group-compact">
                <label class="form-label-compact">Тип таблицы *</label>
                <div class="custom-select">
                  <div class="select-header" @click="toggleNewTableTypeDropdown">
                    <span class="select-value">{{ getTableTypeLabel(newTable.table_type) }}</span>
                    <img src="@/assets/icons/arrow.png" class="select-arrow" :class="{ rotated: newTableTypeDropdownOpen }" />
                  </div>
                  <transition name="dropdown-fade">
                    <div v-if="newTableTypeDropdownOpen" class="select-dropdown">
                      <div 
                        class="select-option"
                        :class="{ active: newTable.table_type === 'cars' }"
                        @click="selectNewTableType('cars')"
                      >
                        Машины
                      </div>
                      <div 
                        class="select-option"
                        :class="{ active: newTable.table_type === 'people' }"
                        @click="selectNewTableType('people')"
                      >
                        Люди
                      </div>
                    </div>
                  </transition>
                </div>
              </div>
            </div>
          </div>

          <!-- Правая часть - настройки -->
          <div class="modal-cells-section">
            <div class="cells-header-compact">
              <h4 class="cells-title-compact">Настройки отображения</h4>
            </div>
            
            <div class="cells-scroll-container">
              <div class="settings-grid">
                <div class="setting-item">
                  <label class="setting-label">
                    <input 
                      type="checkbox" 
                      v-model="newTable.show_fact_table"
                      class="setting-checkbox"
                    />
                    <span class="setting-text">Отображать таблицу "по факту"</span>
                  </label>
                  <span class="setting-hint">
                    Будет показана дополнительная таблица с данными "по факту"
                  </span>
                </div>

                <div v-if="newTable.show_fact_table" class="setting-item">
                  <label class="form-label-compact">Подсказка для таблицы "по факту"</label>
                  <TextConstructor
                    v-model="newTable.fact_table_hint"
                    :placeholder="getDefaultHint(newTable.table_type)"
                    rows="3"
                  />
                </div>

                <div class="setting-item">
                  <label class="form-label-compact">Инструкция к таблице</label>
                  <TextConstructor
                    v-model="newTable.instruction"
                    placeholder="Введите инструкцию для таблицы..."
                    rows="6"
                  />
                </div>

                <div class="setting-item">
                  <h5 class="fields-preview-title">Поля таблицы:</h5>
                  <div class="fields-preview">
                    <div 
                      v-for="field in getTableFields(newTable.table_type)" 
                      :key="field.name"
                      class="preview-field"
                    >
                      <span class="preview-field-name">{{ field.displayName }}</span>
                      <span class="preview-field-type">{{ field.type }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div class="modal-footer">
          <button @click="showAddModal = false" class="modal-cancel">Отмена</button>
          <button @click="createTable" class="modal-confirm">Создать</button>
        </div>
      </div>
    </div>

    <!-- Уведомления -->
    <div v-if="notification.show" class="notification" :class="notification.type">
      <span class="notification-message">{{ notification.message }}</span>
    </div>
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import TextConstructor from './TextConstructor.vue';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    TextConstructor
  },
  data() {
    return {
      searchQuery: '',
      newTable: {
        name: '',
        display_name: '',
        table_type: 'cars',
        show_fact_table: false,
        fact_table_hint: '',
        instruction: '',
        is_active: true
      },
      tables: [],
      showAddModal: false,
      selectedTable: null,
      sortField: null,
      sortDirection: 'asc',
      nameError: '',
      originalHint: '',
      originalInstruction: '',
      tableTypeDropdownOpen: false,
      newTableTypeDropdownOpen: false,
      notification: {
        show: false,
        message: '',
        type: 'info'
      }
    };
  },
  computed: {
    filteredTables() {
      if (!this.searchQuery) return this.tables;
      const query = this.searchQuery.toLowerCase();
      return this.tables.filter(table => 
        table.display_name.toLowerCase().includes(query) || 
        table.name.toLowerCase().includes(query) ||
        table.id.toString().includes(query)
      );
    },
    sortedTables() {
      const tables = [...this.filteredTables];
      
      if (!this.sortField) {
        return tables.sort((a, b) => a.display_name.localeCompare(b.display_name));
      }
      
      return tables.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.id;
            valueB = b.id;
            break;
          case 'name':
            valueA = a.display_name;
            valueB = b.display_name;
            break;
          case 'type':
            valueA = a.table_type;
            valueB = b.table_type;
            break;
          default:
            return 0;
        }
        
        if (valueA < valueB) {
          return this.sortDirection === 'asc' ? -1 : 1;
        }
        if (valueA > valueB) {
          return this.sortDirection === 'asc' ? 1 : -1;
        }
        return 0;
      });
    },
    hintHasChanges() {
      return this.selectedTable && this.selectedTable.fact_table_hint !== this.originalHint;
    },
    instructionHasChanges() {
      return this.selectedTable && this.selectedTable.instruction !== this.originalInstruction;
    }
  },
  methods: {
    validateSystemName() {
      const nameRegex = /^[a-z0-9_]*$/;
      if (!nameRegex.test(this.newTable.name)) {
        this.nameError = "Только латинские буквы, цифры и подчеркивания";
      } else {
        this.nameError = '';
      }
    },
    async refreshData() {
      await this.fetchTables();
    },
    async fetchTables() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/system-tables", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          const data = await response.json();
          this.tables = data;
        }
      } catch (error) {
        console.error("Error fetching system tables:", error);
        this.showNotification("Ошибка при загрузке таблиц", "error");
      }
    },
    async createTable() {
      if (!this.newTable.name.trim() || !this.newTable.display_name.trim()) {
        this.showNotification("Заполните все обязательные поля", "warning");
        return;
      }
      
      // Валидация системного имени
      const nameRegex = /^[a-z0-9_]+$/;
      if (!nameRegex.test(this.newTable.name)) {
        this.showNotification("Системное имя может содержать только латинские буквы, цифры и подчеркивания", "warning");
        return;
      }
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/system-tables", {
          method: "POST",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(this.newTable),
        });
        
        if (response.ok) {
          this.newTable = {
            name: '',
            display_name: '',
            table_type: 'cars',
            show_fact_table: false,
            fact_table_hint: '',
            instruction: '',
            is_active: true
          };
          this.showAddModal = false;
          await this.refreshData();
          this.showNotification("Таблица успешно создана", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при создании таблицы", "error");
        }
      } catch (error) {
        console.error("Error creating system table:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async updateTable(table, field = null) {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/system-tables/${table.id}`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(table),
        });
        
        if (response.ok) {
          let message = "Таблица успешно обновлена";
          if (field === 'display_name') {
            message = "Наименование успешно изменено";
          } else if (field === 'table_type') {
            message = "Тип таблицы успешно изменен";
          } else if (field === 'show_fact_table') {
            message = table.show_fact_table 
              ? "Таблица по факту отображается" 
              : "Таблица по факту не отображается";
          } else if (field === 'fact_table_hint') {
            message = "Подсказка успешно изменена";
          } else if (field === 'instruction') {
            message = "Инструкция успешно изменена";
          }
          
          this.showNotification(message, "success");
          await this.refreshData();
          this.originalHint = table.fact_table_hint || '';
          this.originalInstruction = table.instruction || '';
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при обновлении таблицы", "error");
        }
      } catch (error) {
        console.error("Error updating system table:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async updateTableDisplayName() {
      if (this.selectedTable) {
        await this.updateTable(this.selectedTable, 'display_name');
      }
    },
    async updateTableSettings() {
      if (this.selectedTable) {
        await this.updateTable(this.selectedTable, 'show_fact_table');
      }
    },
    async confirmDeleteTable(table) {
      if (!confirm(`Вы уверены, что хотите удалить таблицу "${table.display_name}"?`)) return;
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/system-tables/${table.id}`, {
          method: "DELETE",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        
        if (response.ok) {
          this.selectedTable = null;
          await this.refreshData();
          this.showNotification("Таблица успешно удалена", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении таблицы", "error");
        }
      } catch (error) {
        console.error("Error deleting system table:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    selectTable(table) {
      this.selectedTable = JSON.parse(JSON.stringify(table));
      this.originalHint = table.fact_table_hint || '';
      this.originalInstruction = table.instruction || '';
    },
    openTable() {
      if (this.selectedTable) {
        this.$router.push(`/table/${this.selectedTable.name}`);
      }
    },
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    getTableTypeLabel(type) {
      return type === 'cars' ? 'Машины' : 'Люди';
    },
    getTableFields(tableType) {
      if (tableType === 'cars') {
        return [
          { name: 'car_number', displayName: 'Номер машины', type: 'Текст' },
          { name: 'car_brand', displayName: 'Марка', type: 'Текст' },
          { name: 'organization', displayName: 'Организация', type: 'Текст' },
          { name: 'unload_place', displayName: 'Место разгрузки', type: 'Текст' },
          { name: 'valid_until', displayName: 'Действует до', type: 'Дата' },
          { name: 'time_range', displayName: 'Время', type: 'Текст' },
          { name: 'status', displayName: 'Статус', type: 'Текст' }
        ];
      } else {
        return [
          { name: 'organization', displayName: 'Организация', type: 'Текст' },
          { name: 'last_name', displayName: 'Фамилия', type: 'Текст' },
          { name: 'first_name', displayName: 'Имя', type: 'Текст' },
          { name: 'middle_name', displayName: 'Отчество', type: 'Текст' },
          { name: 'valid_until', displayName: 'Действует до', type: 'Дата' },
          { name: 'pass_time', displayName: 'Время прохода', type: 'Текст' }
        ];
      }
    },
    getDefaultHint(tableType) {
      if (tableType === 'cars') {
        return 'При прибытии автомобиля ПО ФАКТУ: спроси у водителя организацию, посмотри, есть ли организация в таблице слева, если организация есть - пропустить';
      } else {
        return 'При проходе человека ПО ФАКТУ: проверьте документы, сверьте с данными в системе';
      }
    },
    saveHint() {
      if (this.selectedTable) {
        this.updateTable(this.selectedTable, 'fact_table_hint');
      }
    },
    cancelHintEdit() {
      if (this.selectedTable) {
        this.selectedTable.fact_table_hint = this.originalHint;
      }
    },
    saveInstruction() {
      if (this.selectedTable) {
        this.updateTable(this.selectedTable, 'instruction');
      }
    },
    cancelInstructionEdit() {
      if (this.selectedTable) {
        this.selectedTable.instruction = this.originalInstruction;
      }
    },
    toggleTableTypeDropdown() {
      this.tableTypeDropdownOpen = !this.tableTypeDropdownOpen;
    },
    selectTableType(type) {
      if (this.selectedTable) {
        this.selectedTable.table_type = type;
        this.tableTypeDropdownOpen = false;
        this.updateTable(this.selectedTable, 'table_type');
      }
    },
    toggleNewTableTypeDropdown() {
      this.newTableTypeDropdownOpen = !this.newTableTypeDropdownOpen;
    },
    selectNewTableType(type) {
      this.newTable.table_type = type;
      this.newTableTypeDropdownOpen = false;
    },
    showNotification(message, type = 'info') {
      this.notification = {
        show: true,
        message,
        type
      };
      
      setTimeout(() => {
        this.hideNotification();
      }, 3000);
    },
    
    hideNotification() {
      this.notification.show = false;
    }
  },
  mounted() {
    this.refreshData();
    // Закрываем dropdown при клике вне их
    document.addEventListener('click', (e) => {
      if (!this.$el.contains(e.target)) {
        this.tableTypeDropdownOpen = false;
        this.newTableTypeDropdownOpen = false;
      }
    });
  },
};
</script>

<style scoped>
.table-constructor-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 500px;
  position: relative;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.add-header-button {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.add-header-button:hover {
  background: #3a45b2;
}

.export-btn, .import-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
  background: white;
  color: #000;
}

.export-btn:hover, .import-btn:hover {
  background: #f8f9fa;
  border-color: #ccc;
}

.action-icon {
  width: 16px;
  height: 16px;
}

.content-container {
  display: flex;
  height: 450px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
}

.table-section.with-details {
  width: 40%;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.delete-icon-btn {
  outline: none;
  border: none;
  width: 30px;
  height: 30px;
  padding: 5px;
  border-radius: 10px;
  display: flex;
  align-items:center;
  justify-content: center;
  transition: .2s;
}

.delete-icon {
  width: 20px;
  height: 20px;
}

.delete-icon-btn:hover {
  background-color: #e6e6e6;
  cursor:pointer;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: #a2a2a2;
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: #000;
}

.header-col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #000 !important;
  font-weight: 600 !important;
}

.id-col {
  width: 15%;
  min-width: 60px;
}

.name-col {
  width: 55%;
  min-width: 200px;
}

.type-col {
  width: 30%;
  min-width: 100px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 407px;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f8f9ff;
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.id-value {
  font-weight: 600;
  color: #000;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.type-badge {
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 0.8em;
  font-weight: 600;
}

.type-badge.cars {
  background: linear-gradient(135deg, #f0f4ff 0%, #f0f4ff 100%);
  color: #3a4a6e;
  border: 1px solid #d0d9f0;
}

.type-badge.people {
  background: linear-gradient(135deg, #f0ecff 0%, #f0ecff 100%);
  color: #6d5aa7;
  border: 1px solid #c6b8f0;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: end;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.details-section {
  width: 60%;
  padding: 15px;
  overflow-y: auto;
  background: #fafafa;
}

.details-content {
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 15px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.table-info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.table-info-title {
  display: flex;
  gap: 10px;
  align-items:center;
}

.system-name {
  font-size: 0.85em;
  color: #666;
  background: #f5f5f5;
  border-radius: 6px;
}

.table-type-badge {
  padding: 3px 12px;
  border-radius: 16px;
  font-size: 0.8em;
  font-weight: 500;
}

.table-type-badge.cars {
  background: linear-gradient(135deg, #f0f4ff 0%, #f0f4ff 100%);
  color: #3a4a6e;
  border: 1px solid #d0d9f0;
}

.table-type-badge.people {
  background: linear-gradient(135deg, #f0ecff 0%, #f0ecff 100%);
  color: #6d5aa7;
  border: 1px solid #c6b8f0;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.action-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
}

.view-btn {
  background: #4F5BDF;
  color: white;
}

.view-btn:hover {
  background: #3a45b2;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-row {
  display: flex;
  gap: 16px;
}

.form-group.compact {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight:400;
}

.form-input-sm {
  padding:5px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 0.8em;
  height: 35px;
  transition: border-color 0.2s ease;
  background: #fff;
  width: 100%;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-input-sm:disabled {
  background: #f5f5f5;
  color: #999;
}

/* Стили для кастомного select */
.custom-select {
  position: relative;
  width: 100%;
}

.select-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  background: white;
  cursor: pointer;
  font-size: 0.8em;
  height: 35px;
  transition: all 0.2s ease;
}

.select-header:hover {
  border-color: #ccc;
  background: #f8f9fa;
}

.select-value {
  color: #000;
}

.select-arrow {
  width: 10px;
  height: 10px;
  transition: transform 0.2s ease;
  margin-left: 4px;
  transform: rotate(90deg);
}

.select-arrow.rotated {
  transform: rotate(-90deg);
}

.select-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  margin-top: 4px;
  overflow: hidden;
}

.select-option {
  padding: 6px 12px;
  font-size: 0.8em;
  cursor: pointer;
  transition: background-color 0.2s ease;
  color: #000;
  border-bottom: 1px solid #f0f0f0;
  height: 32px;
  display: flex;
  align-items: center;
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background: #f0f0f0;
}

.select-option.active {
  background: #4F5BDF;
  color: white;
}

.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: all 0.2s ease;
}

.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}

.settings-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-label {
  font-size: 0.9em;
  color: #000;
  font-weight: 500;
}

.checkbox-group {
  padding: 12px;
  background: #f8f9ff;
  border-radius: 10px;
  border: 1px solid #e6e6e6;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-text {
  font-size: 0.8em;
  font-weight: 500;
  color: #000;
}

.hint-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 10px;
}

.instruction-section {
  background: #f8f9ff;
  margin-bottom: 10px;
}

.section-header-with-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  height: 23px;
}

.editor-actions {
  display: flex;
  gap: 5px;
}

.compact-btn {
  padding: 6px 12px;
  border: none;
  border-radius: 10px;
  cursor: pointer;
  font-size: 0.6em;
  font-weight: 500;
  transition: all 0.2s ease;
}

.save-btn {
  background: #4F5BDF;
  color: white;
}

.save-btn:hover {
  background: #3a45b2;
}

.cancel-btn {
  background: #6b7280;
  color: white;
}

.cancel-btn:hover {
  background: #4b5563;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
  width: 100%;
}

.no-results-icon {
  font-size: 3em;
  margin-bottom: 16px;
  opacity: 0.5;
}

.no-results p {
  margin: 0;
  font-size: 1.1em;
}

/* Модальные окна */
.modal-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid #e6e6e6;

}

.horizontal-modal {
  width: 1050px;
  height: 400px;
  max-width: 1050px;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  flex-shrink: 0;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.modal-close {
  background: none;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s;
  border-radius: 50%;
}

.modal-close:hover {
  background: #f5f5f5;
  color: #000;
}

.modal-body-horizontal {
  display: flex;
  flex: 1;
  overflow: hidden;
  padding: 0;
}

.modal-main-info {
  width: 30%;
  padding: 16px;
  border-right: 1px solid #e6e6e6;
  background: #fafafa;
  display: flex;
  flex-direction: column;
}

.main-fields {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-group-compact {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label-compact {
  font-size: 0.8em;
  color: #000;
  font-weight: 500;
}

.input-compact {
  padding: 8px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.85em;
  background: #fff;
  transition: border-color 0.2s;
  height: 32px;
}

.input-compact:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-hint {
  font-size: 0.7em;
  color: #999;
  margin-top: 4px;
}

.form-error {
  font-size: 0.7em;
  color: #ef4444;
  margin-top: 4px;
}

.modal-cells-section {
  width: 70%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.cells-header-compact {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  flex-shrink: 0;
}

.cells-title-compact {
  margin: 0;
  font-size: 1em;
  font-weight: 600;
  color: #000;
}

.cells-scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.settings-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.setting-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.setting-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.setting-text {
  font-size: 0.85em;
  font-weight: 500;
  color: #000;
}

.setting-hint {
  font-size: 0.75em;
  color: #666;
  line-height: 1.4;
}

.fields-preview-title {
  margin: 0;
  font-size: 0.9em;
  font-weight: 600;
  color: #000;
}

.fields-preview {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.preview-field {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  background: #f8f9fa;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
}

.preview-field-name {
  font-size: 0.8em;
  font-weight: 500;
  color: #000;
}

.preview-field-type {
  font-size: 0.7em;
  color: #666;
  background: #e9ecef;
  padding: 2px 6px;
  border-radius: 4px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 16px;
  border-top: 1px solid #e6e6e6;
  background: #fff;
  flex-shrink: 0;
}

.modal-cancel {
  padding: 8px 16px;
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
}

.modal-cancel:hover {
  background: #e9ecef;
}

.modal-confirm {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 600;
  transition: background-color 0.2s ease;
}

.modal-confirm:hover {
  background: #3a45b2;
}

/* Стили для уведомлений */
.notification {
  position: fixed;
  top: 0;
  left: 50%;
  transform: translateX(-50%) translateY(-100%);
  padding: 12px 24px;
  border-radius: 0 0 8px 8px;
  color: white;
  font-weight: 500;
  z-index: 10000;
  text-align: center;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  animation: slideDown 0.3s ease-out forwards;
  min-width: 300px;
}

.notification.success {
  background: #10b981;
}

.notification.error {
  background: #ef4444;
}

.notification.warning {
  background: #f59e0b;
}

.notification.info {
  background: #3b82f6;
}

.notification-message {
  font-size: 0.9em;
}

@keyframes slideDown {
  from {
    transform: translateX(-50%) translateY(-100%);
  }
  to {
    transform: translateX(-50%) translateY(0);
  }
}

@media (max-width: 768px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }
  
  .table-section,
  .details-section,
  .no-selection-message {
    width: 100% !important;
  }
  
  .table-section.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 255px;
  }
  
  .horizontal-modal {
    height: auto;
    max-height: 80vh;
    width: 95%;
  }
  
  .modal-body-horizontal {
    flex-direction: column;
  }
  
  .modal-main-info,
  .modal-cells-section {
    width: 100%;
  }
  
  .modal-main-info {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    padding: 12px;
  }
  
  .table-info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .section-header-with-actions {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .editor-actions {
    align-self: flex-end;
  }
  
  .notification {
    left: 20px;
    right: 20px;
    transform: translateY(-100%);
    min-width: auto;
  }
  
  @keyframes slideDown {
    from {
      transform: translateY(-100%);
    }
    to {
      transform: translateY(0);
    }
  }
}
</style>