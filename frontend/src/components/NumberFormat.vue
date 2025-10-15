<template>
  <div class="number-format-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Управление форматами номеров</h3>
      <div class="header-controls">
        <SearchComponent
          :title="'Поиск форматов...'"
          v-model="searchQuery"
        />
        <button @click="showAddModal = true" class="add-header-button">
          Добавить
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица форматов -->
      <div class="table-section" :class="{'with-details': selectedFormat}">
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
          </div>

          <div class="table-body">
            <div 
              v-for="format in sortedFormats" 
              :key="format.format.id" 
              class="table-row"
              :class="{'selected': selectedFormat && selectedFormat.format.id === format.format.id}"
              @click="selectFormat(format)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ format.format.id }}</span>
              </div>
              <div class="table-col name-col">
                <span class="truncate-text" :title="format.format.name">
                  {{ format.format.name }}
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего форматов: {{ filteredFormats.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали формата -->
      <div v-if="selectedFormat" class="details-section">
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
                <h3 class="details-title">{{ selectedFormat.format.name }}</h3>
                <span class="format-preview">{{ getFormatPreview(selectedFormat) }}</span>
            </div>
            <div class="details-header-actions">
              <button @click="confirmDeleteFormat(selectedFormat.format)" class="delete-icon-btn">
                <img src="@/assets/icons/delete.png" class="delete-icon" />
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="compact-form">
              <div class="form-row">
                <div class="form-group compact">
                  <label class="detail-label">Наименование:</label>
                  <input 
                    v-model="selectedFormat.format.name" 
                    @change="updateFormat(selectedFormat)"
                    class="form-input-sm"
                    placeholder="Название формата"
                    autocomplete="off"
                  >
                </div>
                <div class="form-group compact">
                  <label class="detail-label">Код страны:</label>
                  <input 
                    v-model="selectedFormat.format.country_code" 
                    @change="updateFormat(selectedFormat)"
                    class="form-input-sm"
                    placeholder="RU, AZ, KZ"
                    autocomplete="off"
                  >
                </div>
              </div>

              <div class="default-checkbox-section">
                <label class="default-checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="selectedFormat.format.is_default"
                    @change="handleDefaultFormatChange"
                    class="default-checkbox"
                  />
                  <span class="default-checkbox-text">Формат по умолчанию</span>
                </label>
                <span class="default-checkbox-hint">
                  Этот формат будет выбран по умолчанию при создании нового Т/С
                </span>
              </div>

              <div class="cells-section">
                <label class="section-label">Клетки формата номера:</label>
                <div class="cells-horizontal">
                  <div 
                    v-for="(cell, index) in selectedFormat.cells" 
                    :key="index"
                    class="cell-horizontal-card"
                    @click="editCell(index)"
                  >
                    <div class="cell-horizontal-header">
                      <span class="cell-badge">Клетка №{{ index + 1 }}</span>
                      <span class="cell-type-badge" :class="cell.cell_type">
                        {{ getCellTypeLabel(cell.cell_type) }}
                      </span>
                    </div>
                    <div class="cell-horizontal-details">
                      <span class="cell-length">{{ cell.min_length }}-{{ cell.max_length }} симв.</span>
                      <span v-if="cell.allowed_letters" class="cell-letters" :title="cell.allowed_letters">
                        {{ truncateLetters(cell.allowed_letters) }}
                      </span>
                      <span v-if="cell.cell_type === 'numbers'" class="cell-padding">
                        Дополнение: {{ cell.padding_side === 'left' ? 'слева' : 'справа' }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div v-else class="no-selection-message">
        <p>Выберите формат номеров для просмотра</p>
      </div>
    </div>

    <div v-if="filteredFormats.length === 0" class="no-results">
      <div class="no-results-icon">🚗</div>
      <p>Форматы номеров не найдены</p>
    </div>

    <!-- Модальное окно добавления формата -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content horizontal-modal">
        <div class="modal-header">
          <h3>Добавить формат номеров</h3>
          <button @click="showAddModal = false" class="modal-close">×</button>
        </div>
        
        <div class="modal-body-horizontal">
          <!-- Левая часть - основная информация -->
          <div class="modal-main-info">
            <div class="main-fields">
              <div class="form-group-compact">
                <label class="form-label-compact">Название формата</label>
                <input
                  v-model="newFormat.name"
                  placeholder="Российские номера"
                  class="input-compact"
                >
              </div>
              
              <div class="form-group-compact">
                <label class="form-label-compact">Код страны</label>
                <input
                  v-model="newFormat.country_code"
                  placeholder="RU"
                  class="input-compact"
                >
              </div>

              <div class="default-checkbox-modal">
                <label class="default-checkbox-label-modal">
                  <input 
                    type="checkbox" 
                    v-model="newFormat.is_default"
                    class="default-checkbox"
                  />
                  <span class="default-checkbox-text-modal">Формат по умолчанию</span>
                </label>
                <span class="default-checkbox-hint-modal">
                  Этот формат будет выбран по умолчанию при создании нового Т/С
                </span>
              </div>
            </div>
          </div>

          <!-- Правая часть - клетки -->
          <div class="modal-cells-section">
            <div class="cells-header-compact">
              <h4 class="cells-title-compact">Клетки формата</h4>
              <button @click="addCell" class="add-cell-btn-header">
                + Добавить клетку
              </button>
            </div>
            
            <div class="cells-scroll-container">
              <div class="cells-grid-compact">
                <div 
                  v-for="(cell, index) in newFormat.cells" 
                  :key="index"
                  class="cell-card-compact"
                >
                  <div class="cell-header-mini">
                    <span class="cell-number-mini">Клетка №{{ index + 1 }}</span>
                    <button 
                      v-if="newFormat.cells.length > 1"
                      @click="removeCell(index)"
                      class="remove-cell-btn-mini"
                      title="Удалить клетку"
                    >
                      ×
                    </button>
                  </div>
                  
                  <div class="cell-config-mini">
                    <div class="config-group-mini">
                      <label>Тип</label>
                      <select v-model="cell.cell_type" class="select-mini">
                        <option value="letters">Буквы</option>
                        <option value="numbers">Цифры</option>
                        <option value="mixed">Смешанный</option>
                      </select>
                    </div>
                    
                    <div class="config-group-mini">
                      <label>Длина</label>
                      <div class="length-controls-mini">
                        <input 
                          v-model.number="cell.min_length" 
                          type="number" 
                          min="1" 
                          max="10"
                          class="input-micro"
                          placeholder="мин"
                        >
                        <span class="length-dash">-</span>
                        <input 
                          v-model.number="cell.max_length" 
                          type="number" 
                          min="1" 
                          max="10"
                          class="input-micro"
                          placeholder="макс"
                        >
                      </div>
                    </div>

                    <div v-if="cell.cell_type !== 'numbers'" class="config-group-mini">
                      <label>Алфавит</label>
                      <select v-model="cell.alphabet_type" class="select-mini">
                        <option value="cyrillic">Кириллица</option>
                        <option value="latin">Латиница</option>
                        <option value="both">Оба</option>
                      </select>
                    </div>
                    
                    <div v-if="cell.cell_type === 'numbers'" class="config-group-mini">
                      <label>Дополнение</label>
                      <select v-model="cell.padding_side" class="select-mini">
                        <option value="left">Слева</option>
                        <option value="right">Справа</option>
                      </select>
                    </div>

                    <div v-if="cell.cell_type !== 'numbers'" class="config-group-full-mini">
                      <label>Разрешенные буквы</label>
                      <input 
                        v-model="cell.allowed_letters"
                        placeholder="АВЕКМНОРСТУХ"
                        class="input-compact"
                      >
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div class="modal-footer">
          <button @click="showAddModal = false" class="modal-cancel">Отмена</button>
          <button @click="addFormat" class="modal-confirm">Добавить</button>
        </div>
      </div>
    </div>

    <!-- Модальное окно редактирования клетки -->
    <div v-if="showCellEditModal" class="modal-overlay" @click.self="showCellEditModal = false">
      <div class="modal-content horizontal-modal cell-edit-modal">
        <div class="modal-header">
          <h3>Редактировать клетку {{ editingCellIndex + 1 }}</h3>
          <button @click="showCellEditModal = false" class="modal-close">×</button>
        </div>
        
        <div class="modal-body-horizontal">
          <!-- Левая часть - основные настройки -->
          <div class="modal-main-info">
            <div class="main-fields">
              <div class="form-group-compact">
                <label class="form-label-compact">Тип клетки</label>
                <select v-model="editingCell.cell_type" class="select-mini">
                  <option value="letters">Буквы</option>
                  <option value="numbers">Цифры</option>
                  <option value="mixed">Смешанный</option>
                </select>
              </div>
              
              <div class="form-row-horizontal">
                <div class="form-group-compact">
                  <label class="form-label-compact">Мин. длина</label>
                  <input 
                    v-model.number="editingCell.min_length" 
                    type="number" 
                    min="1" 
                    max="10"
                    class="input-compact"
                  >
                </div>
                <div class="form-group-compact">
                  <label class="form-label-compact">Макс. длина</label>
                  <input 
                    v-model.number="editingCell.max_length" 
                    type="number" 
                    min="1" 
                    max="10"
                    class="input-compact"
                  >
                </div>
              </div>

              <div v-if="editingCell.cell_type !== 'numbers'" class="form-group-compact">
                <label class="form-label-compact">Алфавит</label>
                <select v-model="editingCell.alphabet_type" class="select-mini">
                  <option value="cyrillic">Кириллица</option>
                  <option value="latin">Латиница</option>
                  <option value="both">Оба</option>
                </select>
              </div>
              
              <div v-if="editingCell.cell_type === 'numbers'" class="form-group-compact">
                <label class="form-label-compact">Дополнение нулями</label>
                <select v-model="editingCell.padding_side" class="select-mini">
                  <option value="left">Слева</option>
                  <option value="right">Справа</option>
                </select>
              </div>
            </div>
          </div>

          <!-- Правая часть - дополнительные настройки -->
          <div class="modal-cells-section">
            <div class="cells-header-compact">
              <h4 class="cells-title-compact">Дополнительные настройки</h4>
            </div>
            
            <div class="cells-scroll-container">
              <div class="cell-edit-details">
                <div v-if="editingCell.cell_type !== 'numbers'" class="form-group-compact">
                  <label class="form-label-compact">Разрешенные буквы</label>
                  <input 
                    v-model="editingCell.allowed_letters"
                    placeholder="АВЕКМНОРСТУХ"
                    class="input-compact"
                  >
                  <span class="form-hint">
                    Оставьте пустым для использования всех букв выбранного алфавита
                  </span>
                </div>
                
                <div class="cell-preview-section">
                  <h5 class="preview-title">Предпросмотр клетки</h5>
                  <div class="preview-content">
                    <div class="preview-example">
                      {{ getCellPreview(editingCell) }}
                    </div>
                    <div class="preview-info">
                      <span class="preview-length">
                        Длина: {{ editingCell.min_length }}-{{ editingCell.max_length }} символов
                      </span>
                      <span v-if="editingCell.allowed_letters" class="preview-letters">
                        Разрешённые символы: {{ editingCell.allowed_letters }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div class="modal-footer">
          <button @click="showCellEditModal = false" class="modal-cancel">Отмена</button>
          <button @click="saveCellEdit" class="modal-confirm">Сохранить</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';

export default {
  components: {
    SearchComponent,
    RefreshButton
  },
  data() {
    return {
      searchQuery: '',
      newFormat: {
        name: '',
        country_code: '',
        is_default: false,
        cells: [
          {
            cell_order: 0,
            cell_type: 'letters',
            min_length: 1,
            max_length: 1,
            alphabet_type: 'cyrillic',
            allowed_letters: '',
            padding_side: 'left'
          }
        ]
      },
      formats: [],
      showAddModal: false,
      showCellEditModal: false,
      selectedFormat: null,
      editingCellIndex: null,
      editingCell: null,
      sortField: null,
      sortDirection: 'asc'
    };
  },
  computed: {
    filteredFormats() {
      if (!this.searchQuery) return this.formats;
      const query = this.searchQuery.toLowerCase();
      return this.formats.filter(format => 
        format.format.name.toLowerCase().includes(query) || 
        format.format.id.toString().includes(query) ||
        (format.format.country_code && format.format.country_code.toLowerCase().includes(query))
      );
    },
    sortedFormats() {
      const formats = [...this.filteredFormats];
      
      if (!this.sortField) {
        return formats.sort((a, b) => a.format.name.localeCompare(b.format.name));
      }
      
      return formats.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.format.id;
            valueB = b.format.id;
            break;
          case 'name':
            valueA = a.format.name;
            valueB = b.format.name;
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
    }
  },
  methods: {
    async refreshData() {
      await this.fetchFormats();
    },
    async fetchFormats() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/license-plate-formats", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          const data = await response.json();
          this.formats = data;
        }
      } catch (error) {
        console.error("Error fetching license plate formats:", error);
        this.showNotification("Ошибка при загрузке форматов номеров", "error");
      }
    },
    async addFormat() {
      if (!this.newFormat.name.trim()) {
        this.showNotification("Введите название формата", "warning");
        return;
      }
      
      if (this.newFormat.cells.length === 0) {
        this.showNotification("Добавьте хотя бы одну клетку", "warning");
        return;
      }
      
      // Подготавливаем данные для отправки
      const formatData = {
        name: this.newFormat.name,
        country_code: this.newFormat.country_code || null,
        is_default: this.newFormat.is_default,
        icon: null,
        cells: this.newFormat.cells.map((cell, index) => ({
          cell_order: index,
          cell_type: cell.cell_type,
          min_length: cell.min_length,
          max_length: cell.max_length,
          allowed_letters: cell.allowed_letters || null,
          alphabet_type: cell.cell_type !== 'numbers' ? cell.alphabet_type : null,
          language: 'ru',
          padding_char: '0',
          padding_side: cell.cell_type === 'numbers' ? cell.padding_side : null
        }))
      };
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/license-plate-formats", {
          method: "POST",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(formatData),
        });
        
        if (response.ok) {
          this.newFormat = {
            name: '',
            country_code: '',
            is_default: false,
            cells: [{
              cell_order: 0,
              cell_type: 'letters',
              min_length: 1,
              max_length: 1,
              alphabet_type: 'cyrillic',
              allowed_letters: '',
              padding_side: 'left'
            }]
          };
          this.showAddModal = false;
          await this.refreshData();
          this.showNotification("Формат номеров успешно добавлен", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при добавлении формата", "error");
        }
      } catch (error) {
        console.error("Error adding license plate format:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async updateFormat(format) {
      try {
        const formatData = {
          name: format.format.name,
          country_code: format.format.country_code || null,
          is_default: format.format.is_default,
          icon: null,
          cells: format.cells.map((cell, index) => ({
            id: cell.id,
            cell_order: index,
            cell_type: cell.cell_type,
            min_length: cell.min_length,
            max_length: cell.max_length,
            allowed_letters: cell.allowed_letters || null,
            alphabet_type: cell.cell_type !== 'numbers' ? cell.alphabet_type : null,
            language: 'ru',
            padding_char: '0',
            padding_side: cell.cell_type === 'numbers' ? cell.padding_side : null
          }))
        };
        
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/license-plate-formats/${format.format.id}`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(formatData),
        });
        
        if (response.ok) {
          this.showNotification("Формат номеров успешно обновлен", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при обновлении формата", "error");
          await this.refreshData(); // Перезагружаем данные чтобы откатить изменения
        }
      } catch (error) {
        console.error("Error updating license plate format:", error);
        this.showNotification("Ошибка сети", "error");
        await this.refreshData();
      }
    },
    async handleDefaultFormatChange() {
      // Сохраняем текущее состояние чекбокса
      const isDefault = this.selectedFormat.format.is_default;
      
      try {
        // Если чекбокс выбран - устанавливаем формат по умолчанию
        if (isDefault) {
          await this.setDefaultFormat(this.selectedFormat);
        } else {
          // Если чекбокс снят - обновляем формат без установки по умолчанию
          await this.updateFormat(this.selectedFormat);
          this.showNotification("Формат больше не является форматом по умолчанию", "success");
          
          // После успешного обновления, обновляем данные в формате
          // чтобы избежать рассинхронизации
          await this.refreshData();
          
          // Обновляем selectedFormat актуальными данными из базы
          const updatedFormat = this.formats.find(f => f.format.id === this.selectedFormat.format.id);
          if (updatedFormat) {
            this.selectedFormat = JSON.parse(JSON.stringify(updatedFormat));
          }
        }
      } catch (error) {
        // В случае ошибки откатываем состояние чекбокса
        this.selectedFormat.format.is_default = !isDefault;
        console.error("Error handling default format change:", error);
      }
    },
    async setDefaultFormat(format) {
      try {
        // Сначала снимаем статус по умолчанию со всех форматов
        await this.clearDefaultFormats();
        
        // Устанавливаем новый формат по умолчанию
        const formatData = {
          name: format.format.name,
          country_code: format.format.country_code || null,
          is_default: true,
          icon: null,
          cells: format.cells.map((cell, index) => ({
            id: cell.id,
            cell_order: index,
            cell_type: cell.cell_type,
            min_length: cell.min_length,
            max_length: cell.max_length,
            allowed_letters: cell.allowed_letters || null,
            alphabet_type: cell.cell_type !== 'numbers' ? cell.alphabet_type : null,
            language: 'ru',
            padding_char: '0',
            padding_side: cell.cell_type === 'numbers' ? cell.padding_side : null
          }))
        };
        
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/license-plate-formats/${format.format.id}`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify(formatData),
        });
        
        if (response.ok) {
          await this.refreshData();
          this.showNotification("Формат по умолчанию успешно установлен", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при установке формата по умолчанию", "error");
        }
      } catch (error) {
        console.error("Error setting default format:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async clearDefaultFormats() {
      try {
        const token = localStorage.getItem("token");
        // Снимаем статус default со всех форматов
        await fetch("http://localhost:8080/license-plate-formats/clear-default", {
          method: "POST",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
        });
      } catch (error) {
        console.error("Error clearing default formats:", error);
      }
    },
    async confirmDeleteFormat(format) {
      if (!confirm(`Вы уверены, что хотите удалить формат "${format.name}"?`)) return;
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/license-plate-formats/${format.id}`, {
          method: "DELETE",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        
        if (response.ok) {
          this.selectedFormat = null;
          await this.refreshData();
          this.showNotification("Формат номеров успешно удален", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении формата", "error");
        }
      } catch (error) {
        console.error("Error deleting license plate format:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    selectFormat(format) {
      this.selectedFormat = JSON.parse(JSON.stringify(format));
    },
    editCell(index) {
      this.editingCellIndex = index;
      this.editingCell = JSON.parse(JSON.stringify(this.selectedFormat.cells[index]));
      this.showCellEditModal = true;
    },
    saveCellEdit() {
      if (this.selectedFormat && this.editingCellIndex !== null) {
        this.selectedFormat.cells[this.editingCellIndex] = this.editingCell;
        this.updateFormat(this.selectedFormat);
        this.showCellEditModal = false;
        this.editingCellIndex = null;
        this.editingCell = null;
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
    addCell() {
      this.newFormat.cells.push({
        cell_order: this.newFormat.cells.length,
        cell_type: 'letters',
        min_length: 1,
        max_length: 1,
        alphabet_type: 'cyrillic',
        allowed_letters: '',
        padding_side: 'left'
      });
    },
    removeCell(index) {
      this.newFormat.cells.splice(index, 1);
      // Обновляем порядок клеток
      this.newFormat.cells.forEach((cell, idx) => {
        cell.cell_order = idx;
      });
    },
    getCellTypeLabel(type) {
      const labels = {
        'letters': 'Буквы',
        'numbers': 'Цифры',
        'mixed': 'Смешанный'
      };
      return labels[type] || type;
    },
    getFormatPreview(format) {
      return format.cells.map(cell => {
        if (cell.cell_type === 'numbers') {
          return '0'.repeat(cell.max_length);
        } else {
          return 'A'.repeat(cell.max_length);
        }
      }).join(' ');
    },
    getCellPreview(cell) {
      if (cell.cell_type === 'numbers') {
        return '0'.repeat(cell.max_length);
      } else if (cell.allowed_letters) {
        return cell.allowed_letters.charAt(0).repeat(cell.max_length);
      } else {
        return 'A'.repeat(cell.max_length);
      }
    },
    truncateLetters(letters, maxLength = 12) {
      if (!letters) return '';
      return letters.length > maxLength ? letters.substring(0, maxLength) + '...' : letters;
    },
    showNotification(message, type = 'info') {
      const notification = document.createElement('div');
      notification.className = `notification ${type}`;
      notification.textContent = message;
      notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 12px 20px;
        border-radius: 8px;
        color: white;
        font-weight: 500;
        z-index: 1000;
      `;
      
      if (type === 'success') notification.style.backgroundColor = '#10b981';
      if (type === 'error') notification.style.backgroundColor = '#ef4444';
      if (type === 'warning') notification.style.backgroundColor = '#f59e0b';
      if (type === 'info') notification.style.backgroundColor = '#3b82f6';
      
      document.body.appendChild(notification);
      
      setTimeout(() => {
        notification.remove();
      }, 3000);
    }
  },
  mounted() {
    this.refreshData();
  },
};
</script>

<style scoped>
.number-format-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 400px;
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
  height: 350px;
  width: 100%;
}

/* Левая часть - таблица */
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
  color: #333;
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
  color: #333 !important;
  font-weight: 600 !important;
}

.id-col {
  width: 20%;
  min-width: 60px;
}

.name-col {
  width: 80%;
  min-width: 250px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 307px;
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

/* Правая часть - детали */
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
  align-items: center;
  gap: 10px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
  padding-bottom: 5px;
}

.format-preview {
  font-family: monospace;
  font-size: 0.9em;
  color: #666;
  background: #fff;
  padding: 4px 8px;
  border-radius: 10px;
  border: 1px solid #e6e6e6;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.delete-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: none;
  cursor: pointer;
  padding: 0;
  transition: opacity 0.2s;
  border-radius: 6px;
}

.delete-icon-btn:hover {
  background-color: #fee;
  opacity: 0.8;
}

.delete-icon {
  width: 20px;
  height: 20px;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* Компактная форма */
.compact-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
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
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.95em;
  height: 35px;
  transition: border-color 0.2s ease;
  background: #fff;
  width: 100%;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

/* Чекбокс по умолчанию */
.default-checkbox-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 12px;
  background: #f8f9ff;
  border-radius: 8px;
  border: 1px solid #e6e6e6;
}

.default-checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.default-checkbox-text {
  font-size: 0.9em;
  font-weight: 500;
  color: #333;
}

.default-checkbox-hint {
  font-size: 0.8em;
  color: #666;
  line-height: 1.4;
}

/* Горизонтальная секция клеток */
.cells-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-label {
  font-size: 0.9em;
  color: #666;
  font-weight: 500;
}

.cells-horizontal {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.cell-horizontal-card {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  padding: 8px;
  transition: all 0.2s ease;
  cursor: pointer;
  min-width: 230px;
  max-width: 230px;
  flex: 1;
}

.cell-horizontal-card:hover {
  border-color: #4F5BDF;
  box-shadow: 0 2px 4px rgba(79, 91, 223, 0.1);
  background: #f8f9ff;
}

.cell-horizontal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.cell-badge {
  font-size: 0.75em;
  font-weight: 600;
  color: #666;
}

.cell-type-badge {
  font-size: 0.7em;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 500;
}

.cell-type-badge.letters {
  background: #e0f2fe;
  color: #0369a1;
}

.cell-type-badge.numbers {
  background: #f0fdf4;
  color: #166534;
}

.cell-type-badge.mixed {
  background: #fef3c7;
  color: #92400e;
}

.cell-horizontal-details {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 0.8em;
}

.cell-length {
  color: #666;
  font-weight: 500;
}

.cell-letters {
  font-family: monospace;
  background: #f8fafc;
  padding: 2px 4px;
  border-radius: 10px;
  color: #475569;
  font-size: 0.8em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-padding {
  color: #666;
  font-size: 0.8em;
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

/* ГОРИЗОНТАЛЬНОЕ МОДАЛЬНОЕ ОКНО */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.horizontal-modal {
  width: 100%;
  max-height: 350px;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-radius: 30px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
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
  font-size: 18px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s;
}

.modal-close:hover {
  color: #333;
}

.modal-body-horizontal {
  display: flex;
  flex: 1;
  overflow: hidden;
  padding: 0;
}

/* Левая часть - основная информация */
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
  color: #666;
  font-weight: 500;
}

.input-compact {
  padding: 8px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  font-size: 0.85em;
  background: #fff;
  transition: border-color 0.2s;
  height: 28px;
}

.input-compact:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-row-horizontal {
  display: flex;
  gap: 10px;
}

.form-row-horizontal .form-group-compact {
  flex: 1;
}

/* Чекбокс по умолчанию в модальном окне */
.default-checkbox-modal {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  background: #f8f9ff;
  border-radius: 8px;
  border: 1px solid #e6e6e6;
}

.default-checkbox-label-modal {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.default-checkbox-text-modal {
  font-size: 0.85em;
  font-weight: 500;
  color: #333;
}

.default-checkbox-hint-modal {
  font-size: 0.75em;
  color: #666;
  line-height: 1.4;
}

/* Правая часть - клетки */
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
  color: #333;
}

.add-cell-btn-header {
  padding: 6px 12px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8em;
  font-weight: 500;
  transition: background-color 0.2s ease;
  white-space: nowrap;
}

.add-cell-btn-header:hover {
  background: #3a45b2;
}

.cells-scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  min-height: 175px;
  max-height: 175px;
}

.cells-grid-compact {
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-content: start;
}

.cell-card-compact {
  background: #f8f9fa;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  padding: 10px;
  min-height: 140px;
}

.cell-header-mini {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid #e6e6e6;
}

.cell-number-mini {
  font-size: 0.8em;
  font-weight: 600;
  color: #333;
}

.remove-cell-btn-mini {
  background: #ef4444;
  color: white;
  border: none;
  border-radius: 3px;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 10px;
  line-height: 1;
  transition: background-color 0.2s;
}

.remove-cell-btn-mini:hover {
  background: #dc2626;
}

.cell-config-mini {
  display: flex;
  flex-wrap: wrap;
  gap: 15px;
  row-gap: 5px;
}

.config-group-mini {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.config-group-mini label {
  font-size: 0.7em;
  color: #666;
  font-weight: 500;
}

.select-mini {
  padding: 5px 8px;
  border: 1px solid #e6e6e6;
  border-radius: 4px;
  font-size: 0.75em;
  background: #fff;
  height: 28px;
  width: 300px;
}

.length-controls-mini {
  display: flex;
  align-items: center;
  gap: 2px;
}

.input-micro {
  width: 45px;
  padding: 5px;
  border: 1px solid #e6e6e6;
  border-radius: 4px;
  font-size: 0.75em;
  text-align: center;
  height: 28px;
}

.length-dash {
  color: #666;
  font-weight: 500;
  margin: 0 2px;
  font-size: 0.8em;
}

.config-group-full-mini {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.config-group-full-mini label {
  font-size: 0.7em;
  color: #666;
  font-weight: 500;
}

.select-mini:focus,
.input-micro:focus,
.input-compact:focus {
  border-color: #4F5BDF;
  outline: none;
}

/* Футер модального окна */
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
  border-radius: 6px;
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
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 600;
  transition: background-color 0.2s ease;
}

.modal-confirm:hover {
  background: #3a45b2;
}

/* СТИЛИ ДЛЯ МОДАЛЬНОГО ОКНА РЕДАКТИРОВАНИЯ КЛЕТКИ */
.cell-edit-modal {
  max-height: 320px;
}

.cell-edit-details {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 8px 0;
}

.form-hint {
  font-size: 0.7em;
  color: #999;
  margin-top: 4px;
  line-height: 1.3;
}

.cell-preview-section {
  background: #f8f9ff;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  padding: 12px;
}

.preview-title {
  margin: 0 0 8px 0;
  font-size: 0.85em;
  font-weight: 600;
  color: #333;
}

.preview-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.preview-example {
  font-family: monospace;
  font-size: 1.2em;
  font-weight: 600;
  color: #4F5BDF;
  text-align: center;
  padding: 8px;
  background: #fff;
  border-radius: 6px;
  border: 1px solid #e6e6e6;
}

.preview-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.preview-length,
.preview-letters {
  font-size: 0.75em;
  color: #666;
}

/* Адаптивность */
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
    max-height: 80vh;
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
  
  .cells-grid-compact {
    grid-template-columns: 1fr;
  }

  .cell-edit-modal {
    width: 95%;
    margin: 10px;
  }

  .form-row-horizontal {
    flex-direction: column;
    gap: 12px;
  }
}

@media (max-width: 480px) {
  .management-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }
  
  .header-controls {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }
  
  .add-header-button {
    justify-content: center;
  }
  
  .cells-horizontal {
    flex-direction: column;
  }
  
  .cell-horizontal-card {
    min-width: auto;
  }
  
  .cells-header-compact {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }
  
  .add-cell-btn-header {
    align-self: flex-end;
  }

  .cell-edit-modal .modal-body {
    padding: 16px;
  }

  .cell-edit-details {
    gap: 12px;
  }
}
</style>