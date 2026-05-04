<template>
  <div class="citizenship-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Управление гражданствами
      </h3>
      <div class="header-controls">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск гражданств...'"
        />
        <button
          class="add-header-button"
          @click="showAddModal = true"
        >
          Добавить
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refreshData"
        />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица гражданств -->
      <div
        class="table-section"
        :class="{'with-details': selectedCitizenship}"
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
              class="header-col status-col"
              @click="sortBy('is_active')"
            >
              <p :class="{ 'active-sort': sortField === 'is_active' }">
                Статус
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'is_active',
                  'desc': sortField === 'is_active' && sortDirection === 'desc'
                }" 
              >
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="citizenship in sortedCitizenships" 
              :key="citizenship.id" 
              class="table-row"
              :class="{'selected': selectedCitizenship && selectedCitizenship.id === citizenship.id}"
              @click="selectCitizenship(citizenship)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ citizenship.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="citizenship.name"
                >
                  {{ citizenship.name }}
                  <span
                    v-if="citizenship.is_default"
                    class="default-badge"
                  >По умолчанию</span>
                </span>
              </div>
              <div class="table-col status-col">
                <span
                  class="status-badge"
                  :class="citizenship.is_active ? 'active' : 'inactive'"
                >
                  {{ citizenship.is_active ? 'Активно' : 'Неактивно' }}
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего гражданств: {{ filteredCitizenships.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали гражданства -->
      <div
        v-if="selectedCitizenship"
        class="details-section"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedCitizenship.name }}
              </h3>
              <div class="timestamps-header">
                <span class="timestamp">Создано: {{ formatDate(selectedCitizenship.created_at) }}</span>
                <span
                  v-if="selectedCitizenship.updated_at"
                  class="timestamp"
                >Обновлено: {{ formatDate(selectedCitizenship.updated_at) }}</span>
              </div>
            </div>
            <div class="details-header-actions">
              <button
                class="delete-icon-btn"
                @click="confirmDeleteCitizenship(selectedCitizenship)"
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
              <div class="form-group compact">
                <label class="detail-label">Наименование:</label>
                <input 
                  v-model="selectedCitizenship.name" 
                  class="form-input-sm"
                  placeholder="Название гражданства"
                  autocomplete="off"
                  @change="updateCitizenship(selectedCitizenship)"
                >
              </div>

              <div class="checkbox-section">
                <label class="checkbox-label">
                  <input 
                    v-model="selectedCitizenship.is_active" 
                    type="checkbox"
                    class="checkbox"
                    @change="updateCitizenship(selectedCitizenship)"
                  >
                  <span class="checkbox-text">Активное гражданство</span>
                </label>
                <span class="checkbox-hint">
                  Если отключено, это гражданство нельзя будет выбрать при создании заявок
                </span>
              </div>

              <div class="checkbox-section">
                <label class="checkbox-label">
                  <input 
                    v-model="selectedCitizenship.is_default" 
                    type="checkbox"
                    class="checkbox"
                    @change="handleDefaultCitizenshipChange"
                  >
                  <span class="checkbox-text">Гражданство по умолчанию</span>
                </label>
                <span class="checkbox-hint">
                  Это гражданство будет выбрано по умолчанию при создании новых заявок
                </span>
              </div>

              <div class="checkbox-section">
                <label class="checkbox-label">
                  <input 
                    v-model="selectedCitizenship.patent_required" 
                    type="checkbox"
                    class="checkbox"
                    @change="updateCitizenship(selectedCitizenship)"
                  >
                  <span class="checkbox-text">Требуется патент</span>
                </label>
                <span class="checkbox-hint">
                  Для этого гражданства обязателен патент при оформлении заявок
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите гражданство для просмотра</p>
      </div>
    </div>

    <div
      v-if="filteredCitizenships.length === 0"
      class="no-results"
    >
      <div class="no-results-icon">
        🌍
      </div>
      <p>Гражданства не найдены</p>
    </div>

    <!-- Модальное окно добавления гражданства -->
    <div
      v-if="showAddModal"
      class="modal-overlay"
      @click.self="showAddModal = false"
    >
      <div class="modal-content small-modal">
        <div class="modal-header">
          <h3>Добавить гражданство</h3>
          <button
            class="modal-close"
            @click="showAddModal = false"
          >
            ×
          </button>
        </div>
        
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Название гражданства</label>
            <input
              v-model="newCitizenship.name"
              placeholder="Российская Федерация"
              class="form-input"
              @keyup.enter="addCitizenship"
            >
          </div>
          
          <div class="checkbox-group">
            <label class="checkbox-label">
              <input 
                v-model="newCitizenship.is_default" 
                type="checkbox"
                class="checkbox"
              >
              <span class="checkbox-text">Гражданство по умолчанию</span>
            </label>
          </div>

          <div class="checkbox-group">
            <label class="checkbox-label">
              <input 
                v-model="newCitizenship.patent_required" 
                type="checkbox"
                class="checkbox"
              >
              <span class="checkbox-text">Требуется патент</span>
            </label>
          </div>
        </div>
        
        <div class="modal-footer">
          <button
            class="modal-cancel"
            @click="showAddModal = false"
          >
            Отмена
          </button>
          <button
            class="modal-confirm"
            @click="addCitizenship"
          >
            Добавить
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
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
      newCitizenship: {
        name: '',
        is_default: false,
        patent_required: false
      },
      citizenships: [],
      showAddModal: false,
      selectedCitizenship: null,
      sortField: null,
      sortDirection: 'asc',
      isLoading: false
    };
  },
  computed: {
    filteredCitizenships() {
      if (!this.searchQuery) return this.citizenships;
      const query = this.searchQuery.toLowerCase();
      return this.citizenships.filter(citizenship => 
        citizenship.name.toLowerCase().includes(query) || 
        citizenship.id.toString().includes(query)
      );
    },
    sortedCitizenships() {
      const citizenships = [...this.filteredCitizenships];
      
      if (!this.sortField) {
        return citizenships.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return citizenships.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.id;
            valueB = b.id;
            break;
          case 'name':
            valueA = a.name;
            valueB = b.name;
            break;
          case 'is_active':
            valueA = a.is_active;
            valueB = b.is_active;
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
  mounted() {
    this.refreshData();
  },
  methods: {
    async refreshData() {
      this.isLoading = true;
      try {
        await this.fetchCitizenships();
      } finally {
        this.isLoading = false;
      }
    },
    async fetchCitizenships() {
      try {
        const response = await apiRequest("/citizenships", {
        });
        if (response.ok) {
          const data = await response.json();
          this.citizenships = data;
        }
      } catch (error) {
        console.error("Error fetching citizenships:", error);
        this.showNotification("Ошибка при загрузке гражданств", "error");
      }
    },
    async addCitizenship() {
      if (!this.newCitizenship.name.trim()) {
        this.showNotification("Введите название гражданства", "warning");
        return;
      }
      
      try {
        const response = await apiRequest("/citizenships", {
          method: "POST",
          body: JSON.stringify(this.newCitizenship),
        });
        
        if (response.ok) {
          this.newCitizenship = {
            name: '',
            is_default: false,
            patent_required: false
          };
          this.showAddModal = false;
          await this.refreshData();
          this.showNotification("Гражданство успешно добавлено", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при добавлении гражданства", "error");
        }
      } catch (error) {
        console.error("Error adding citizenship:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async updateCitizenship(citizenship) {
      try {
        const citizenshipData = {
          name: citizenship.name,
          is_active: citizenship.is_active,
          is_default: citizenship.is_default,
          patent_required: citizenship.patent_required
        };
        const response = await apiRequest(`/citizenships/${citizenship.id}`, {
          method: "PUT",
          body: JSON.stringify(citizenshipData),
        });
        
        if (response.ok) {
          // Обновляем данные в таблице
          await this.refreshData();
          // Обновляем выбранное гражданство актуальными данными
          const updatedCitizenship = this.citizenships.find(c => c.id === citizenship.id);
          if (updatedCitizenship) {
            this.selectedCitizenship = JSON.parse(JSON.stringify(updatedCitizenship));
          }
          this.showNotification("Гражданство успешно обновлено", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при обновлении гражданства", "error");
          await this.refreshData(); // Перезагружаем данные чтобы откатить изменения
        }
      } catch (error) {
        console.error("Error updating citizenship:", error);
        this.showNotification("Ошибка сети", "error");
        await this.refreshData();
      }
    },
    async handleDefaultCitizenshipChange() {
      // Сохраняем текущее состояние чекбокса
      const isDefault = this.selectedCitizenship.is_default;
      
      try {
        // Если чекбокс выбран - устанавливаем гражданство по умолчанию
        if (isDefault) {
          await this.setDefaultCitizenship(this.selectedCitizenship);
        } else {
          // Если чекбокс снят - обновляем гражданство без установки по умолчанию
          await this.updateCitizenship(this.selectedCitizenship);
        }
      } catch (error) {
        // В случае ошибки откатываем состояние чекбокса
        this.selectedCitizenship.is_default = !isDefault;
        console.error("Error handling default citizenship change:", error);
      }
    },
    async setDefaultCitizenship(citizenship) {
      try {
        const citizenshipData = {
          name: citizenship.name,
          is_active: citizenship.is_active,
          is_default: true,
          patent_required: citizenship.patent_required
        };
        const response = await apiRequest(`/citizenships/${citizenship.id}`, {
          method: "PUT",
          body: JSON.stringify(citizenshipData),
        });
        
        if (response.ok) {
          await this.refreshData();
          // Обновляем выбранное гражданство
          const updatedCitizenship = this.citizenships.find(c => c.id === citizenship.id);
          if (updatedCitizenship) {
            this.selectedCitizenship = JSON.parse(JSON.stringify(updatedCitizenship));
          }
          this.showNotification("Гражданство по умолчанию успешно установлено", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при установке гражданства по умолчанию", "error");
        }
      } catch (error) {
        console.error("Error setting default citizenship:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async confirmDeleteCitizenship(citizenship) {
      if (!confirm(`Вы уверены, что хотите удалить гражданство "${citizenship.name}"?`)) return;
      
      try {
        const response = await apiRequest(`/citizenships/${citizenship.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          this.selectedCitizenship = null;
          await this.refreshData();
          this.showNotification("Гражданство успешно удалено", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении гражданства", "error");
        }
      } catch (error) {
        console.error("Error deleting citizenship:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    selectCitizenship(citizenship) {
      this.selectedCitizenship = JSON.parse(JSON.stringify(citizenship));
    },
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    formatDate(dateString) {
      if (!dateString) return '-';
      const date = new Date(dateString);
      return date.toLocaleString('ru-RU');
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
};
</script>

<style scoped>
.citizenship-container {
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
  width: 15%;
  min-width: 60px;
}

.name-col {
  width: 60%;
  min-width: 200px;
}

.status-col {
  width: 25%;
  min-width: 100px;
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

.default-badge {
  font-size: 0.7em;
  background: #e0f2fe;
  color: #0369a1;
  padding: 2px 6px;
  border-radius: 4px;
  margin-left: 6px;
  font-weight: 500;
}

.status-badge {
  font-size: 0.8em;
  padding: 4px 8px;
  border-radius: 12px;
  font-weight: 500;
}

.status-badge.active {
  background: linear-gradient(135deg, #f0fdf4 0%, #f0fdf4 100%);
  color: #166534;
  border: 1px solid #bbf7d0;
}

.status-badge.inactive {
  background: linear-gradient(135deg, #fef2f2 0%, #fef2f2 100%);
  color: #dc2626;
  border: 1px solid #fecaca;
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
  margin-bottom: 20px;
  gap: 30px;
}

.details-title-wrapper {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  flex: 1;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.3em;
  font-weight: 600;
}

.timestamps-header {
  display: flex;
  flex-direction: column;
  gap: 0px;
}

.timestamp {
  font-size: 0.75em;
  color: #666;
}

.details-header-actions {
  display: flex;
  align-items: flex-start;
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
  gap: 10px;
  padding-bottom: 15px;
}

.form-group.compact {
  display: flex;
  flex-direction: column;
  gap: 6px;
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
  width: 250px;
  margin-bottom: 10px;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

/* Секции чекбоксов */
.checkbox-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 12px;
  background: #f8f9ff;
  border-radius: 15px;
  border: 1px solid #e6e6e6;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-text {
  font-size: 0.9em;
  font-weight: 500;
  color: #333;
}

.checkbox-hint {
  font-size: 0.8em;
  color: #666;
  line-height: 1.4;
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

/* МОДАЛЬНОЕ ОКНО */
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

.small-modal {
  max-width: 400px;
}

.modal-content {
  width: 100%;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
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

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 0.85em;
  color: #666;
  font-weight: 500;
}

.form-input {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.9em;
  transition: border-color 0.2s;
  background: #fff;
  width: 100%;
  height: 35px;
}

.form-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #e6e6e6;
  background: #fff;
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
  
  .modal-content {
    max-width: 90%;
  }
  
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

  .details-header {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .timestamps-header {
    flex-direction: row;
    gap: 12px;
    flex-wrap: wrap;
  }
}

@media (max-width: 480px) {
  .table-header {
    padding: 0 12px;
  }
  
  .table-row {
    padding: 0 12px;
  }
  
  .header-col,
  .table-col {
    font-size: 12px;
  }
  
  .details-section {
    padding: 12px;
  }
  
  .modal-body {
    padding: 16px;
  }
  
  .modal-footer {
    padding: 12px 16px;
  }

  .timestamps-header {
    flex-direction: column;
    gap: 2px;
  }
}
</style>