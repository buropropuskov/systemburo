<template>
  <div class="table-constructor-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Таблицы системы
      </h3>
      <div class="header-controls">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск таблиц...'"
        />
        <button
          class="add-header-button"
          @click="showAddModal = true"
        >
          Создать таблицу
        </button>
        <RefreshButton
          :loading="refreshing"
          @refresh="refreshData"
        />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список таблиц -->
      <div
        class="table-section"
        :class="{'with-details': selectedTable}"
      >
        <div class="table-container">
          <div class="table-header">
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('name')"
            >
              <p :class="{ 'active-sort': sortField === 'name' }">
                Наименование
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'name',
                  'desc': sortField === 'name' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col type-col"
              @click="sortBy('type')"
            >
              <p :class="{ 'active-sort': sortField === 'type' }">
                Тип
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'type',
                  'desc': sortField === 'type' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div class="header-col status-col">
              <p>Статус</p>
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="table in sortedTables" 
              :key="table.table.id" 
              class="table-row"
              :class="{'selected': selectedTable && selectedTable.table.id === table.table.id}"
              @click="selectTable(table)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ table.table.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="table.table.display_name"
                >
                  {{ table.table.display_name }}
                </span>
              </div>
              <div class="table-col type-col">
                <span
                  class="type-badge"
                  :class="table.table.table_type"
                >
                  {{ getTableTypeLabel(table.table.table_type) }}
                </span>
              </div>
              <div class="table-col status-col">
                <span
                  class="status-badge"
                  :class="getTableStatusClass(table)"
                >
                  {{ getTableStatusText(table) }}
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
      <div
        v-if="selectedTable"
        class="details-section"
      >
        <div class="details-tabs">
          <button 
            class="tab-btn" 
            :class="{ 'active': activeTab === 'main' }"
            @click="activeTab = 'main'"
          >
            Основное
          </button>
          <button 
            class="tab-btn" 
            :class="{ 'active': activeTab === 'schedule' }"
            @click="activeTab = 'schedule'"
          >
            Расписание
          </button>
          <button
            class="tab-btn"
            :class="{ 'active': activeTab === 'location' }"
            @click="activeTab = 'location'"
          >
            Местоположение
          </button>
          <button
            class="tab-btn"
            :class="{ 'active': activeTab === 'columns' }"
            @click="activeTab = 'columns'"
          >
            Колонки
          </button>
          <button
            class="tab-btn"
            :class="{ 'active': activeTab === 'appearance' }"
            @click="activeTab = 'appearance'"
          >
            Оформление
          </button>
          <button
            v-if="selectedTable.table.show_fact_table"
            class="tab-btn"
            :class="{ 'active': activeTab === 'fact-columns' }"
            @click="activeTab = 'fact-columns'"
          >
            Колонки (факт)
          </button>
          <button
            v-if="selectedTable.table.show_fact_table"
            class="tab-btn"
            :class="{ 'active': activeTab === 'fact-appearance' }"
            @click="activeTab = 'fact-appearance'"
          >
            Оформление (факт)
          </button>
        </div>

        <!-- Вкладка Основное -->
        <div
          v-if="activeTab === 'main'"
          class="tab-content"
        >
          <div class="details-header">
            <div class="details-title-wrapper">
              <div class="table-info-title">
                <h3 class="details-title">
                  {{ selectedTable.table.display_name }}
                </h3>
                <span
                  class="table-type-badge"
                  :class="selectedTable.table.table_type"
                >
                  {{ getTableTypeLabel(selectedTable.table.table_type) }}
                </span>
              </div>
              <div class="table-info-row">
                <span class="system-name">{{ selectedTable.table.name }}</span>
                <span
                  class="current-status-badge"
                  :class="getTableCurrentStatusClass(selectedTable)"
                >
                  {{ getTableCurrentStatusText(selectedTable) }}
                </span>
              </div>
            </div>
            <div class="details-header-actions">
              <button
                class="action-btn view-btn"
                @click="openTable"
              >
                Открыть
              </button>
              <button
                class="delete-icon-btn"
                @click="confirmDeleteTable(selectedTable)"
              >
                <img
                  src="@/assets/icons/delete.png"
                  class="delete-icon"
                >
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="compact-form">
              <div class="form-row">
                <div class="form-group compact">
                  <label class="detail-label">Наименование таблицы:</label>
                  <input 
                    v-model="selectedTable.table.display_name" 
                    class="form-input-sm"
                    placeholder="Название таблицы"
                    autocomplete="off"
                    @change="updateTableField('display_name')"
                  >
                </div>
                <div class="form-group compact">
                  <label class="detail-label">Тип таблицы:</label>
                  <div class="custom-select">
                    <div
                      class="select-header"
                      @click="toggleTableTypeDropdown"
                    >
                      <span class="select-value">{{ getTableTypeLabel(selectedTable.table.table_type) }}</span>
                      <img
                        src="@/assets/icons/arrow.png"
                        class="select-arrow"
                        :class="{ rotated: tableTypeDropdownOpen }"
                      >
                    </div>
                    <transition name="dropdown-fade">
                      <div
                        v-if="tableTypeDropdownOpen"
                        class="select-dropdown"
                      >
                        <div 
                          class="select-option"
                          :class="{ active: selectedTable.table.table_type === 'cars' }"
                          @click="selectTableType('cars')"
                        >
                          Машины
                        </div>
                        <div 
                          class="select-option"
                          :class="{ active: selectedTable.table.table_type === 'people' }"
                          @click="selectTableType('people')"
                        >
                          Люди
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
              </div>

              <!-- Статус в виде кнопок -->
              <div class="form-group">
                <label class="detail-label">Статус:</label>
                <div class="status-toggle">
                  <button 
                    class="status-btn" 
                    :class="{ 'active': selectedTable.table.status === 'active' }"
                    @click="setTableStatus('active')"
                  >
                    Активно
                  </button>
                  <button 
                    class="status-btn" 
                    :class="{ 'active': selectedTable.table.status === 'inactive' }"
                    @click="setTableStatus('inactive')"
                  >
                    Не активно
                  </button>
                  <button 
                    class="status-btn" 
                    :class="{ 'active': selectedTable.table.status === 'maintenance' }"
                    @click="setTableStatus('maintenance')"
                  >
                    На обслуживании
                  </button>
                </div>
              </div>

              <!-- Комментарий к статусу (только для неактивных) -->
              <div
                v-if="selectedTable.table.status !== 'active'"
                class="form-group"
              >
                <label class="detail-label">Причина:</label>
                <textarea 
                  v-model="selectedTable.table.status_comment" 
                  class="form-textarea"
                  placeholder="Укажите причину"
                  rows="2"
                  @change="updateTableField('status_comment')"
                />
              </div>

              <div class="settings-section">
                <label class="section-label">Настройки отображения:</label>
                
                <div class="checkbox-group">
                  <label class="checkbox-label">
                    <input 
                      v-model="selectedTable.table.show_fact_table" 
                      type="checkbox"
                      class="checkbox-input"
                      @change="updateTableField('show_fact_table')"
                    >
                    <span class="checkbox-text">Отображать таблицу "по факту"</span>
                  </label>
                </div>

                <div
                  v-if="selectedTable.table.show_fact_table"
                  class="hint-section"
                >
                  <div class="section-header-with-actions">
                    <label class="detail-label">Подсказка для таблицы "по факту":</label>
                    <div
                      v-if="hintHasChanges"
                      class="editor-actions"
                    >
                      <button
                        class="compact-btn cancel-btn"
                        @click="cancelHintEdit"
                      >
                        Отмена
                      </button>
                      <button
                        class="compact-btn save-btn"
                        @click="saveHint"
                      >
                        Сохранить
                      </button>
                    </div>
                  </div>
                  <TextConstructor
                    ref="hintConstructor"
                    v-model="selectedTable.table.fact_table_hint"
                    :placeholder="getDefaultHint(selectedTable.table.table_type)"
                    rows="4"
                  />
                </div>
              </div>

              <div class="instruction-section">
                <div class="section-header-with-actions">
                  <label class="detail-label">Инструкция к таблице:</label>
                  <div
                    v-if="instructionHasChanges"
                    class="editor-actions"
                  >
                    <button
                      class="compact-btn cancel-btn"
                      @click="cancelInstructionEdit"
                    >
                      Отмена
                    </button>
                    <button
                      class="compact-btn save-btn"
                      @click="saveInstruction"
                    >
                      Сохранить
                    </button>
                  </div>
                </div>
                <TextConstructor
                  ref="instructionConstructor"
                  v-model="selectedTable.table.instruction"
                  placeholder="Введите инструкцию для таблицы..."
                  rows="4"
                />
              </div>

              <div class="fields-section">
                <label class="section-label">Поля таблицы:</label>
                <div class="fields-list">
                  <div 
                    v-for="field in selectedTable.fields" 
                    :key="field.id"
                    class="field-item"
                  >
                    <span class="field-name">{{ getFieldDisplayName(field.field_name) }}</span>
                    <span class="field-type">{{ getFieldTypeLabel(field.field_type) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Вкладка Расписание -->
        <div
          v-if="activeTab === 'schedule'"
          class="tab-content"
        >
          <SystemTableScheduleTab 
            :table-id="selectedTable.table.id"
            :time-slots="selectedTable.time_slots"
            @update="refreshSelectedTable"
          />
        </div>

        <!-- Вкладка Местоположение -->
        <div
          v-if="activeTab === 'location'"
          class="tab-content"
        >
          <div class="location-section">
            <h4 class="section-title">
              Описание местоположения
            </h4>
            <textarea 
              v-model="selectedTable.table.location_description" 
              class="form-textarea"
              placeholder="Введите описание местоположения..."
              rows="3"
              @change="updateTableField('location_description')"
            />
          </div>

          <div class="location-section">
            <h4 class="section-title">
              Ссылка на карту
            </h4>
            <div class="map-link-group">
              <input 
                v-model="selectedTable.table.map_link" 
                class="form-input"
                placeholder="https://maps.google.com/..."
                autocomplete="off"
                @change="updateTableField('map_link')"
              >
              <a 
                v-if="selectedTable.table.map_link" 
                :href="selectedTable.table.map_link" 
                target="_blank" 
                class="map-link-btn"
              >
                Открыть карту
              </a>
            </div>
          </div>

          <TableConstructorPhotoSection
            :table-id="selectedTable.table.id"
            :photos="selectedTable.photos || []"
            @photos-changed="refreshSelectedTable"
          />
        </div>

        <!-- Вкладка Колонки (#345) -->
        <div
          v-if="activeTab === 'columns'"
          class="tab-content"
        >
          <SystemTableColumnsTab
            :table-id="selectedTable.table.id"
            :table-type="selectedTable.table.table_type"
            :fields="selectedTable.fields || []"
            @update="refreshSelectedTable"
          />
        </div>

        <!-- Вкладка Оформление (#345 фазы 1D+1E) -->
        <div
          v-if="activeTab === 'appearance'"
          class="tab-content"
        >
          <SystemTableAppearanceTab
            :table-id="selectedTable.table.id"
            :table="selectedTable.table"
            @update="refreshSelectedTable"
          />
        </div>

        <!-- Колонки FactTable (#345) -->
        <div
          v-if="activeTab === 'fact-columns' && selectedTable.table.show_fact_table"
          class="tab-content"
        >
          <SystemTableColumnsTab
            :table-id="selectedTable.table.id"
            :table-type="selectedTable.table.table_type"
            :fields="selectedTable.fact_fields || []"
            variant="fact"
            @update="refreshSelectedTable"
          />
        </div>

        <!-- Оформление FactTable (#345) -->
        <div
          v-if="activeTab === 'fact-appearance' && selectedTable.table.show_fact_table"
          class="tab-content"
        >
          <SystemTableAppearanceTab
            :table-id="selectedTable.table.id"
            :table="selectedTable.table"
            variant="fact"
            @update="refreshSelectedTable"
          />
        </div>
      </div>

      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите таблицу для просмотра и редактирования</p>
      </div>
    </div>

    <div
      v-if="filteredTables.length === 0"
      class="no-results"
    >
      <div class="no-results-icon">
        📊
      </div>
      <p>Таблицы не найдены</p>
    </div>

    <!-- Модальное окно создания таблицы -->
    <TableConstructorCreateModal
      v-if="showAddModal"
      @created="onTableCreated"
      @close="showAddModal = false"
    />

    <!-- Уведомления -->
    <div
      v-if="notification.show"
      class="notification"
      :class="notification.type"
    >
      <span class="notification-message">{{ notification.message }}</span>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import TextConstructor from './TextConstructor.vue';
import SystemTableScheduleTab from './SystemTableScheduleTab.vue';
import SystemTableColumnsTab from './SystemTableColumnsTab.vue';
import SystemTableAppearanceTab from './SystemTableAppearanceTab.vue';
import TableConstructorCreateModal from './TableConstructorCreateModal.vue';
import TableConstructorPhotoSection from './TableConstructorPhotoSection.vue';

export default {
  name: 'TableConstructor',
  components: {
    SearchComponent,
    RefreshButton,
    TextConstructor,
    SystemTableScheduleTab,
    SystemTableColumnsTab,
    SystemTableAppearanceTab,
    TableConstructorCreateModal,
    TableConstructorPhotoSection
  },
  data() {
    return {
      searchQuery: '',
      refreshing: false,
      tables: [],
      showAddModal: false,
      selectedTable: null,
      sortField: null,
      sortDirection: 'asc',
      activeTab: 'main',
      originalHint: '',
      originalInstruction: '',
      tableTypeDropdownOpen: false,
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
        table.table.display_name.toLowerCase().includes(query) || 
        table.table.name.toLowerCase().includes(query) ||
        table.table.id.toString().includes(query)
      );
    },
    sortedTables() {
      const tables = [...this.filteredTables];
      
      if (!this.sortField) {
        return tables.sort((a, b) => a.table.display_name.localeCompare(b.table.display_name));
      }
      
      return tables.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.table.id;
            valueB = b.table.id;
            break;
          case 'name':
            valueA = a.table.display_name;
            valueB = b.table.display_name;
            break;
          case 'type':
            valueA = a.table.table_type;
            valueB = b.table.table_type;
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
      return this.selectedTable && this.selectedTable.table.fact_table_hint !== this.originalHint;
    },
    instructionHasChanges() {
      return this.selectedTable && this.selectedTable.table.instruction !== this.originalInstruction;
    }
  },
  mounted() {
    this.refreshData();
    
    // Слушаем события уведомлений от дочерних компонентов
    window.addEventListener('show-notification', this.handleNotification);
    
    // Закрываем dropdown при клике вне них
    document.addEventListener('click', (e) => {
      if (!this.$el.contains(e.target)) {
        this.tableTypeDropdownOpen = false;
      }
    });
  },
  beforeUnmount() {
    window.removeEventListener('show-notification', this.handleNotification);
  },
  watch: {
    // Если активна вкладка фактовой таблицы, а пользователь снял галочку
    // show_fact_table - возвращаем на главную, чтобы не висел пустой контент.
    'selectedTable.table.show_fact_table'(val) {
      if (!val && (this.activeTab === 'fact-columns' || this.activeTab === 'fact-appearance')) {
        this.activeTab = 'main';
      }
    },
    selectedTable(newVal) {
      // При переключении таблицы сбрасываем тикеры fact-вкладок, если на новой
      // таблице фактовая часть выключена.
      if (newVal && !newVal.table?.show_fact_table) {
        if (this.activeTab === 'fact-columns' || this.activeTab === 'fact-appearance') {
          this.activeTab = 'main';
        }
      }
    },
  },
  methods: {
    handleNotification(event) {
      this.showNotification(event.detail.message, event.detail.type);
    },

    async refreshData() {
      this.refreshing = true;
      try {
        await this.fetchTables();
      } finally {
        this.refreshing = false;
      }
    },
    
    async fetchTables() {
      try {
        const response = await apiRequest("/system-tables", {
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
    
    async refreshSelectedTable() {
      if (!this.selectedTable) return;

      try {
        const response = await apiRequest(`/system-tables/${this.selectedTable.table.id}`, {
        });
        if (response.ok) {
          const data = await response.json();
          
          // Исправляем URL фотографий
          if (data.photos) {
            data.photos = data.photos.map(photo => ({
              ...photo,
              photo_url: photo.photo_url
            }));
          }
          
          this.selectedTable = data;
          this.originalHint = data.table.fact_table_hint || '';
          this.originalInstruction = data.table.instruction || '';
          
          // Обновляем в общем списке
          const index = this.tables.findIndex(t => t.table.id === data.table.id);
          if (index !== -1) {
            this.tables[index] = data;
          }
        }
      } catch (error) {
        console.error("Error refreshing table:", error);
      }
    },
    
    async onTableCreated(result) {
      this.showAddModal = false;
      await this.refreshData();
      const newTable = this.tables.find(t => t.table.id === result.id);
      if (newTable) {
        this.selectTable(newTable);
      }
      this.showNotification("Таблица успешно создана", "success");
    },

    async updateTable(field) {
      if (!this.selectedTable) return;
      
      const updateData = {};
      
      switch (field) {
        case 'display_name':
          updateData.display_name = this.selectedTable.table.display_name;
          break;
        case 'table_type':
          updateData.table_type = this.selectedTable.table.table_type;
          break;
        case 'show_fact_table':
          updateData.show_fact_table = this.selectedTable.table.show_fact_table;
          break;
        case 'fact_table_hint':
          updateData.fact_table_hint = this.selectedTable.table.fact_table_hint;
          break;
        case 'instruction':
          updateData.instruction = this.selectedTable.table.instruction;
          break;
        case 'map_link':
          updateData.map_link = this.selectedTable.table.map_link;
          break;
        case 'status':
          updateData.status = this.selectedTable.table.status;
          break;
        case 'status_comment':
          updateData.status_comment = this.selectedTable.table.status_comment;
          break;
        case 'location_description':
          updateData.location_description = this.selectedTable.table.location_description;
          break;
      }
      
      try {
        const response = await apiRequest(`/system-tables/${this.selectedTable.table.id}`, {
          method: "PUT",
          body: JSON.stringify(updateData),
        });
        
        if (response.ok) {
          if (field === 'fact_table_hint') {
            this.originalHint = this.selectedTable.table.fact_table_hint || '';
            this.showNotification("Подсказка успешно сохранена", "success");
          } else if (field === 'instruction') {
            this.originalInstruction = this.selectedTable.table.instruction || '';
            this.showNotification("Инструкция успешно сохранена", "success");
          } else {
            this.showNotification("Изменения сохранены", "success");
          }
          await this.refreshSelectedTable();
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при обновлении", "error");
        }
      } catch (error) {
        console.error("Error updating table:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    updateTableField(field) {
      this.updateTable(field);
    },
    
    setTableStatus(status) {
      if (!this.selectedTable) return;
      this.selectedTable.table.status = status;
      if (status === 'active') {
        this.selectedTable.table.status_comment = null;
      }
      this.updateTable('status');
    },
    
    async confirmDeleteTable(table) {
  if (!confirm(`Вы уверены, что хотите удалить таблицу "${table.table.display_name}"?`)) return;
  
  try {
    console.log("Deleting table ID:", table.table.id);
    
    const response = await apiRequest(`/system-tables/${table.table.id}`, {
      method: "DELETE",
    });
    
    console.log("Response status:", response.status);
    console.log("Response headers:", response.headers);
    
    if (response.ok) {
      this.selectedTable = null;
      this.activeTab = 'main';
      await this.refreshData();
      this.showNotification("Таблица успешно удалена", "success");
    } else {
      // Получаем текст ошибки
      const errorText = await response.text();
      console.error("Error response text:", errorText);
      
      // Пробуем распарсить как JSON, если это возможно
      try {
        const errorJson = JSON.parse(errorText);
        this.showNotification(errorJson.message || errorText, "error");
      } catch {
        // Если не JSON, показываем как текст
        this.showNotification(errorText, "error");
      }
    }
  } catch (error) {
    console.error("Error deleting system table:", error);
    this.showNotification("Ошибка сети", "error");
  }
},
    
    selectTable(table) {
      this.selectedTable = JSON.parse(JSON.stringify(table));
      this.originalHint = table.table.fact_table_hint || '';
      this.originalInstruction = table.table.instruction || '';
      this.activeTab = 'main';
    },
    
    openTable() {
      if (this.selectedTable) {
        this.$router.push(`/table/${this.selectedTable.table.name}`);
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
    
    getTableStatusClass(table) {
      if (table.table.status !== 'active') {
        return 'status-inactive';
      }
      return table.current_status === 'open' ? 'status-open' : 'status-closed';
    },
    
    getTableStatusText(table) {
      if (table.table.status !== 'active') {
        return 'Неактивно';
      }
      return table.current_status === 'open' ? 'Открыто' : 'Закрыто';
    },
    
    getTableCurrentStatusClass(table) {
      if (table.table.status !== 'active') {
        return 'status-inactive-badge';
      }
      return table.current_status === 'open' ? 'status-open-badge' : 'status-closed-badge';
    },
    
    getTableCurrentStatusText(table) {
      if (table.table.status !== 'active') {
        if (table.table.status === 'maintenance') return 'На обслуживании';
        return 'Неактивно';
      }
      return table.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
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
    
    getFieldDisplayName(fieldName) {
      const fields = {
        'car_number': 'Номер машины',
        'car_brand': 'Марка',
        'organization': 'Организация',
        'unload_place': 'Место разгрузки',
        'valid_until': 'Действует до',
        'time_range': 'Время',
        'status': 'Статус',
        'last_name': 'Фамилия',
        'first_name': 'Имя',
        'middle_name': 'Отчество',
        'pass_time': 'Время прохода'
      };
      return fields[fieldName] || fieldName;
    },
    
    getFieldTypeLabel(fieldType) {
      const types = {
        'text': 'Текст',
        'date': 'Дата',
        'time': 'Время',
        'number': 'Число'
      };
      return types[fieldType] || fieldType;
    },
    
    getDefaultHint(tableType) {
      if (tableType === 'cars') {
        return 'При прибытии автомобиля ПО ФАКТУ: спроси у водителя организацию, посмотри, есть ли организация в таблице слева, если организация есть - пропустить';
      } else {
        return 'При проходе человека ПО ФАКТУ: проверьте документы, сверьте с данными в системе';
      }
    },
    
    saveHint() {
      this.updateTable('fact_table_hint');
    },
    
    cancelHintEdit() {
      if (this.selectedTable) {
        this.selectedTable.table.fact_table_hint = this.originalHint;
      }
    },
    
    saveInstruction() {
      this.updateTable('instruction');
    },
    
    cancelInstructionEdit() {
      if (this.selectedTable) {
        this.selectedTable.table.instruction = this.originalInstruction;
      }
    },
    
    toggleTableTypeDropdown() {
      this.tableTypeDropdownOpen = !this.tableTypeDropdownOpen;
    },
    
    selectTableType(type) {
      if (this.selectedTable) {
        this.selectedTable.table.table_type = type;
        this.tableTypeDropdownOpen = false;
        this.updateTable('table_type');
      }
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
}
</script>

<style scoped>
.table-constructor-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 550px;
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

.content-container {
  display: flex;
  height: 500px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
  background: #fff;
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
  width: 40%;
  min-width: 200px;
}

.type-col {
  width: 25%;
  min-width: 100px;
}

.status-col {
  width: 20%;
  min-width: 80px;
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
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  min-width: 70px;
  text-align: center;
}

.type-badge.cars {
  background-color: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.type-badge.people {
  background-color: #e3f2fd;
  color: #1976d2;
  border: 1px solid #bbdefb;
}

.status-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 500;
  min-width: 70px;
  text-align: center;
}

.status-open {
  background-color: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.status-closed {
  background-color: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffcc80;
}

.status-inactive {
  background-color: #ffebee;
  color: #c62828;
  border: 1px solid #ef9a9a;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: right;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
}

.details-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  border-bottom: 1px solid #e6e6e6;
  background: #f8f9fa;
  padding: 8px 16px 0;
  overflow-x: auto;
  scrollbar-width: thin;
}

.tab-btn {
  padding: 10px 18px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: #6b7280;
  transition: color 0.2s ease, background 0.2s ease, border-color 0.2s ease;
  border-radius: 8px 8px 0 0;
  white-space: nowrap;
  flex-shrink: 0;
}

.tab-btn:hover {
  color: #4F5BDF;
  background: #eef0ff;
}

.tab-btn.active {
  color: #4F5BDF;
  border-bottom-color: #4F5BDF;
  background: #fff;
}

@media (max-width: 1100px) {
  .tab-btn {
    padding: 8px 12px;
    font-size: 12px;
  }
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fff;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.table-info-title {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.table-type-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.table-type-badge.cars {
  background-color: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.table-type-badge.people {
  background-color: #e3f2fd;
  color: #1976d2;
  border: 1px solid #bbdefb;
}

.table-info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.system-name {
  font-size: 12px;
  color: #666;
  background: #f5f5f5;
  padding: 2px 8px;
  border-radius: 4px;
}

.current-status-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.status-open-badge {
  background-color: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.status-closed-badge {
  background-color: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffcc80;
}

.status-inactive-badge {
  background-color: #ffebee;
  color: #c62828;
  border: 1px solid #ef9a9a;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 30px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.view-btn {
  background: #4F5BDF;
  color: white;
}

.view-btn:hover {
  background: #3a45b2;
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

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-row {
  display: flex;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group.compact {
  flex: 1;
}

.detail-label {
  font-size: 12px;
  color: #666;
  font-weight: 500;
}

.form-input-sm {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 13px;
  width: 100%;
  transition: border-color 0.2s;
  background: #fff;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-textarea {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 13px;
  width: 100%;
  transition: border-color 0.2s;
  background: #fff;
  resize: vertical;
  font-family: inherit;
}

.form-textarea:focus {
  border-color: #4F5BDF;
  outline: none;
}

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
  font-size: 13px;
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
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 10;
  margin-top: 4px;
  overflow: hidden;
}

.select-option {
  padding: 8px 12px;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.2s;
  border-bottom: 1px solid #f0f0f0;
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background: #f5f7ff;
  color: #4F5BDF;
}

.select-option.active {
  background: #f0f3ff;
  color: #4F5BDF;
  font-weight: 500;
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

.status-toggle {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.status-btn {
  padding: 6px 16px;
  border: 1px solid #e6e6e6;
  background: #fff;
  border-radius: 30px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  color: #666;
}

.status-btn:hover {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.status-btn.active {
  background: #4F5BDF;
  border-color: #4F5BDF;
  color: white;
}

.settings-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-label {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin: 0;
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

.checkbox-input {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: #4F5BDF;
}

.checkbox-text {
  font-size: 13px;
  color: #333;
}

.hint-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 10px;
}

.instruction-section {
  background: #f8f9ff;
  border-radius: 10px;
  padding: 12px;
  border: 1px solid #e6e6e6;
  margin-bottom: 10px;
}

.section-header-with-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.editor-actions {
  display: flex;
  gap: 5px;
}

.compact-btn {
  padding: 4px 10px;
  border: none;
  border-radius: 15px;
  cursor: pointer;
  font-size: 11px;
  font-weight: 500;
  transition: all 0.2s;
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

.fields-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fields-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 150px;
  overflow-y: auto;
  padding-right: 4px;
}

.field-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  background: #f8f9fa;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
}

.field-name {
  font-size: 12px;
  font-weight: 500;
  color: #333;
}

.field-type {
  font-size: 11px;
  color: #666;
  background: #e9ecef;
  padding: 2px 8px;
  border-radius: 12px;
}

.location-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: #333;
}

.map-link-group {
  display: flex;
  gap: 12px;
  align-items: center;
}

.map-link-group .form-input {
  flex: 1;
}

.map-link-btn {
  padding: 8px 16px;
  background: #f0f3ff;
  color: #4F5BDF;
  text-decoration: none;
  border-radius: 30px;
  font-size: 13px;
  white-space: nowrap;
  transition: background-color 0.2s ease;
  border: 1px solid #4F5BDF;
}

.map-link-btn:hover {
  background: #4F5BDF;
  color: white;
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

/* Модальное окно */
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
  z-index: 10000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
  }
  to {
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(0.1px);
  }
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 0;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  animation: modalAppear 0.3s ease-out;
}

@keyframes modalAppear {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #f0f0f0;
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
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
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #f0f0f0;
}

.modal-btn {
  padding: 8px 20px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background-color 0.2s ease;
  min-width: 90px;
}

.modal-btn--cancel {
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e0e0e0;
}

.modal-btn--cancel:hover {
  background: #e9ecef;
}

.modal-btn--confirm {
  background: #4F5BDF;
  color: white;
}

.modal-btn--confirm:hover:not(:disabled) {
  background: #3a45b2;
}

.modal-btn--disabled {
  background: #ccc;
  cursor: not-allowed;
}

/* Анимации */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
  transition: all 0.3s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
  transition: all 0.3s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
  background: rgba(0, 0, 0, 0);
  backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.8) translateY(-20px);
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

/* Скроллбары */
.table-body::-webkit-scrollbar,
.fields-list::-webkit-scrollbar {
  width: 6px;
}

.table-body::-webkit-scrollbar-track,
.fields-list::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb,
.fields-list::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb:hover,
.fields-list::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

@media (max-width: 768px) {
  .form-row {
    flex-direction: column;
  }

  .map-link-group {
    flex-direction: column;
  }

  .map-link-btn {
    width: 100%;
    text-align: center;
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