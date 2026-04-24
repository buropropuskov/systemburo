<template>
  <div class="user-types-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Типы пользователей
      </h3>
      <div class="header-controls">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск типов...'"
        />
        <button
          class="add-header-button"
          @click="showAddModal = true"
        >
          Создать тип
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список типов -->
      <div
        class="types-section"
        :class="{'with-details': selectedType}"
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
              class="header-col users-col"
              @click="sortBy('users_count')"
            >
              <p :class="{ 'active-sort': sortField === 'users_count' }">
                Пользователи
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'users_count',
                  'desc': sortField === 'users_count' && sortDirection === 'desc'
                }" 
              >
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="type in sortedTypes" 
              :key="type.id" 
              class="table-row"
              :class="{'selected': selectedType && selectedType.id === type.id}"
              @click="selectType(type)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ type.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="type.name"
                >
                  {{ type.name }}
                </span>
              </div>
              <div class="table-col users-col">
                <span class="users-count">{{ type.users_count }}</span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего типов: {{ filteredTypes.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали типа -->
      <div
        v-if="selectedType"
        class="details-section"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedType.name }}
              </h3>
              <div class="type-info-row">
                <span class="system-name">{{ selectedType.code }}</span>
                <span class="users-count-badge">Пользователей: {{ selectedType.users_count }}</span>
              </div>
            </div>
            <div class="details-header-actions">
              <button
                class="delete-icon-btn"
                @click="confirmDeleteType(selectedType)"
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
              <div class="form-column">
                <div class="form-group compact">
                  <label class="detail-label">Наименование типа:</label>
                  <input 
                    v-model="selectedType.name" 
                    class="form-input-sm"
                    placeholder="Название типа"
                    autocomplete="off"
                    @change="updateTypeName"
                  >
                </div>
                <div class="form-group compact">
                  <label class="detail-label">Системное имя:</label>
                  <input 
                    v-model="selectedType.code" 
                    class="form-input-sm"
                    disabled
                    placeholder="Системное имя"
                  >
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите тип пользователя для просмотра и редактирования</p>
      </div>
    </div>

    <div
      v-if="filteredTypes.length === 0"
      class="no-results"
    >
      <div class="no-results-icon">
        👥
      </div>
      <p>Типы пользователей не найдены</p>
    </div>

    <!-- Модальное окно создания типа -->
    <transition name="modal-fade">
      <div
        v-if="showAddModal"
        class="modal-overlay"
        @click.self="closeModal"
      >
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">
              Создать новый тип пользователя
            </h3>
            <button
              class="modal-close"
              @click="closeModal"
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
            <div class="input-group">
              <label class="input-label">Наименование типа *</label>
              <input
                ref="nameInput"
                v-model="newType.name"
                placeholder="Менеджер"
                class="modal-input"
                @keyup.enter="createType"
              >
              <div class="input-hint">
                Обязательное поле
              </div>
            </div>
            
            <div class="input-group">
              <label class="input-label">Системное имя *</label>
              <input
                v-model="newType.code"
                placeholder="manager"
                class="modal-input"
                @input="validateSystemName"
                @keyup.enter="createType"
              >
              <div class="input-hint">
                Латинские буквы, цифры и подчеркивания
              </div>
              <span
                v-if="nameError"
                class="form-error"
              >{{ nameError }}</span>
            </div>
          </div>
          
          <div class="modal-footer">
            <button
              class="modal-btn modal-btn--cancel"
              @click="closeModal"
            >
              Отмена
            </button>
            <button 
              class="modal-btn modal-btn--confirm" 
              :disabled="!isFormValid"
              :class="{'modal-btn--disabled': !isFormValid}"
              @click="createType"
            >
              Создать
            </button>
          </div>
        </div>
      </div>
    </transition>

    <!-- Модальное окно подтверждения удаления -->
    <ConfirmationModal
      :show="showDeleteModal"
      title="Подтверждение удаления"
      :message="deleteMessage"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
      @confirm="deleteType"
      @cancel="cancelDelete"
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
import ConfirmationModal from './ConfirmationModal.vue';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    ConfirmationModal
  },
  data() {
    return {
      searchQuery: '',
      newType: {
        name: '',
        code: ''
      },
      types: [],
      showAddModal: false,
      showDeleteModal: false,
      selectedType: null,
      typeToDelete: null,
      sortField: null,
      sortDirection: 'asc',
      nameError: '',
      notification: {
        show: false,
        message: '',
        type: 'info'
      },
      isLoading: false
    };
  },
  computed: {
    filteredTypes() {
      if (!this.searchQuery) return this.types;
      const query = this.searchQuery.toLowerCase();
      return this.types.filter(type => 
        type.name.toLowerCase().includes(query) || 
        type.code.toLowerCase().includes(query) ||
        type.id.toString().includes(query)
      );
    },
    sortedTypes() {
      const types = [...this.filteredTypes];
      
      if (!this.sortField) {
        return types.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return types.sort((a, b) => {
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
          case 'users_count':
            valueA = a.users_count;
            valueB = b.users_count;
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
    deleteMessage() {
      return `Вы точно хотите удалить тип пользователя "${this.typeToDelete?.name}"?`;
    },
    isFormValid() {
      return this.newType.name.trim() && 
             this.newType.code.trim() && 
             !this.nameError &&
             !this.isLoading;
    }
  },
  watch: {
    showAddModal(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.$refs.nameInput?.focus();
        });
      }
    }
  },
  mounted() {
    this.refreshData();
  },
  methods: {
    validateSystemName() {
      const nameRegex = /^[a-z0-9_]*$/;
      if (!nameRegex.test(this.newType.code)) {
        this.nameError = "Только латинские буквы, цифры и подчеркивания";
      } else {
        this.nameError = '';
      }
    },
    async refreshData() {
      await this.fetchTypes();
    },
    async fetchTypes() {
      try {
        const response = await apiRequest("/user-types-management", {
        });
        if (response.ok) {
          const data = await response.json();
          this.types = data;
        }
      } catch (error) {
        console.error("Error fetching user types:", error);
        this.showNotification("Ошибка при загрузке типов пользователей", "error");
      }
    },
    async createType() {
      if (!this.isFormValid) {
        this.showNotification("Заполните все обязательные поля корректно", "warning");
        return;
      }
      
      if (this.isLoading) return;
      
      this.isLoading = true;
      
      try {
        const response = await apiRequest("/user-types-management", {
          method: "POST",
          body: JSON.stringify(this.newType),
        });
        
        if (response.ok) {
          this.newType = {
            name: '',
            code: ''
          };
          this.showAddModal = false;
          await this.refreshData();
          this.showNotification("Тип пользователя успешно создан", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при создании типа пользователя", "error");
        }
      } catch (error) {
        console.error("Error creating user type:", error);
        this.showNotification("Ошибка сети", "error");
      } finally {
        this.isLoading = false;
      }
    },
    async updateTypeName() {
      if (this.selectedType) {
        await this.updateType(this.selectedType);
      }
    },
    async updateType(type) {
      try {
        const response = await apiRequest(`/user-types-management/${type.id}`, {
          method: "PUT",
          body: JSON.stringify({
            name: type.name,
            code: type.code
          }),
        });
        
        if (response.ok) {
          this.showNotification("Тип пользователя успешно обновлен", "success");
          await this.refreshData();
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при обновлении типа пользователя", "error");
        }
      } catch (error) {
        console.error("Error updating user type:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    confirmDeleteType(type) {
      if (type.users_count > 0) {
        this.showNotification("Нельзя удалить тип, к которому привязаны пользователи", "warning");
        return;
      }
      
      this.typeToDelete = type;
      this.showDeleteModal = true;
    },
    
    cancelDelete() {
      this.showDeleteModal = false;
      this.typeToDelete = null;
    },

    async deleteType() {
      if (!this.typeToDelete) return;
      
      try {
        const response = await apiRequest(`/user-types-management/${this.typeToDelete.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          this.selectedType = null;
          this.showDeleteModal = false;
          this.typeToDelete = null;
          await this.refreshData();
          this.showNotification("Тип пользователя успешно удален", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении типа пользователя", "error");
        }
      } catch (error) {
        console.error("Error deleting user type:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    selectType(type) {
      this.selectedType = JSON.parse(JSON.stringify(type));
    },
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    closeModal() {
      this.showAddModal = false;
      this.newType = {
        name: '',
        code: ''
      };
      this.nameError = '';
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
  }
};
</script>

<style scoped>
.user-types-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 450px;
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
  height: 400px;
  width: 100%;
}

.types-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
}

.types-section.with-details {
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
  width: 20%;
  min-width: 60px;
}

.name-col {
  width: 50%;
  min-width: 200px;
}

.users-col {
  width: 30%;
  min-width: 100px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 357px;
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

.users-count {
  font-size: 14px;
  font-weight: bold;
  color: #000;
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
  gap: 8px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.type-info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.system-name {
  font-size: 0.85em;
  color: #666;
  background: #f5f5f5;
  border-radius: 6px;
}

.users-count-badge {
  font-size: 0.8em;
  color: #666;
  background: #f0f4ff;
  padding: 4px 8px;
  border-radius: 12px;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
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
  gap: 10px;
}

.form-column {
  display: flex;
  flex-direction: column;
  gap: 8px;
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
  padding:5px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 0.8em;
  height: 35px;
  transition: border-color 0.2s ease;
  background: #fff;
  width: 300px;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-input-sm:disabled {
  background: #f5f5f5;
  color: #999;
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

/* Стили для улучшенного модального окна */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
  }
  to {
    background: rgba(0, 0, 0, 0.3);
    backdrop-filter: blur(1px);
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
  padding: 20px 24px 16px;
  border-bottom: 1px solid #f0f0f0;
}

.modal-title {
  margin: 0;
  font-size: 1.1em;
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
  transform: rotate(90deg);
}

.modal-body {
  padding: 20px 24px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.input-label {
  font-size: 0.85em;
  font-weight: 500;
  color: #555;
  margin-bottom: 2px;
}

.modal-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  font-size: 0.9em;
  transition: all 0.2s ease;
  background: #fff;
}

.modal-input:focus {
  border-color: #4F5BDF;
  outline: none;
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.modal-input::placeholder {
  color: #aaa;
}

.input-hint {
  font-size: 0.75em;
  color: #888;
  margin-top: 2px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px 20px;
  border-top: 1px solid #f0f0f0;
}

.modal-btn {
  padding: 8px 20px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
  min-width: 80px;
}

.modal-btn--cancel {
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e0e0e0;
}

.modal-btn--cancel:hover {
  background: #e9ecef;
  border-color: #ccc;
}

.modal-btn--confirm {
  background: #4F5BDF;
  color: white;
}

.modal-btn--confirm:hover:not(.modal-btn--disabled) {
  background: #3a45b2;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(79, 91, 223, 0.3);
}

.modal-btn--disabled {
  background: #ccc;
  cursor: not-allowed;
  transform: none !important;
  box-shadow: none !important;
}

/* Анимации для модального окна */
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

@media (max-width: 768px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }
  
  .types-section,
  .details-section,
  .no-selection-message {
    width: 100% !important;
  }
  
  .types-section.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 255px;
  }
  
  .modal-content {
    height: auto;
    max-height: 80vh;
    width: 95%;
  }
  
  .type-info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .form-input-sm,
  .form-input {
    width: 100% !important;
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