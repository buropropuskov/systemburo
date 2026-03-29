<template>
  <div class="organizations-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Управление организациями / отделами</h3>
      <div class="header-controls">
        <SearchComponent
          :title="'Поиск организаций...'"
          v-model="searchQuery"
        />
        <button @click="showAddModal = true" class="add-header-button">
          Добавить
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица организаций -->
      <div class="table-section">
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
            <div class="header-col users-col" @click="sortBy('user_count')">
              <p :class="{ 'active-sort': sortField === 'user_count' }">Пользователи</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'user_count',
                  'desc': sortField === 'user_count' && sortDirection === 'desc'
                }" 
              />
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="org in sortedOrganizations" 
              :key="org.id" 
              class="table-row"
              :class="{'selected': selectedOrganization && selectedOrganization.id === org.id}"
              @click="selectOrganization(org)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ org.id }}</span>
              </div>
              <div class="table-col name-col">
                <span class="truncate-text" :title="org.name">
                  {{ org.name }}
                </span>
              </div>
              <div class="table-col users-col">
                <span class="cell-content user-count">
                  <span class="count-value">{{ org.user_count }}</span>
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего организаций: {{ filteredOrganizations.length }}</span>
          </div>
        </div>
      </div>

      <!-- Средняя часть - детали организации -->
      <div v-if="selectedOrganization" class="details-section">
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">{{ selectedOrganization.name }}</h3>
            </div>
            <div class="details-header-actions">
              <button @click="confirmDeleteOrganization(selectedOrganization)" class="delete-icon-btn">
                <img src="@/assets/icons/delete.png" class="delete-icon" />
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="details-grid-two-columns">
              <!-- Левый столбец -->
              <div class="details-column">
                <div class="detail-group">
                  <label class="detail-label">Наименование:</label>
                  <input 
                    v-model="selectedOrganization.name" 
                    @change="updateOrganization(selectedOrganization)"
                    class="form-input-sm"
                    placeholder="Введите название организации"
                    autocomplete="off"
                  >
                </div>
              </div>
              
              <!-- Правый столбец -->
              <div class="details-column">
                <!-- Пустой столбец для выравнивания -->
              </div>
            </div>

            <!-- Компонент мест разгрузки -->
            <SelectUnloadPlaces
              :entity="selectedOrganization"
              :entity-type="'organization'"
              @places-updated="handlePlacesUpdated"
            />

            <!-- Компонент таблиц по умолчанию -->
            <SelectTables
              :entity="selectedOrganization"
              :entity-type="'organization'"
              @tables-updated="handleTablesUpdated"
            />
          </div>
        </div>
      </div>
      
      <!-- Правая часть - ответственные лица -->
      <div class="responsible-section" :class="{'with-details': selectedOrganization}">
        <div v-if="selectedOrganization" class="responsible-content">
          <ResponsibleUsersSection
            :entity="selectedOrganization"
            :entity-type="'organization'"
            @users-updated="handleUsersUpdated"
          />
        </div>
        <div v-else class="no-selection-message">
          <p>Выберите организацию для просмотра</p>
        </div>
      </div>
    </div>

    <div v-if="filteredOrganizations.length === 0" class="no-results">
      <div class="no-results-icon">🏢</div>
      <p>Организации не найдены</p>
    </div>

    <!-- Модальное окно добавления -->
    <transition name="modal-fade">
      <div v-if="showAddModal" class="modal-overlay" @click.self="closeModal">
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">Добавить организацию</h3>
            <button @click="closeModal" class="modal-close">
              <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          
          <div class="modal-body">
            <div class="input-group">
              <label class="input-label">Название организации</label>
              <input
                v-model="newOrganizationName"
                placeholder="Введите название организации"
                class="modal-input"
                @keyup.enter="addOrganization"
                ref="nameInput"
              >
              <div class="input-hint">Обязательное поле</div>
            </div>
          </div>
          
          <div class="modal-footer">
            <button @click="closeModal" class="modal-btn modal-btn--cancel">Отмена</button>
            <button 
              @click="addOrganization" 
              class="modal-btn modal-btn--confirm"
              :disabled="!newOrganizationName.trim()"
              :class="{'modal-btn--disabled': !newOrganizationName.trim()}"
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
    @confirm="deleteOrganization"
    @cancel="cancelDelete"
/>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import ResponsibleUsersSection from './ResponsibleUsersSection.vue';
import SelectUnloadPlaces from './SelectUnloadPlaces.vue';
import SelectTables from './SelectTables.vue';
import ConfirmationModal from './ConfirmationModal.vue';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    ResponsibleUsersSection,
    SelectUnloadPlaces,
    SelectTables,
    ConfirmationModal
  },
  data() {
    return {
      searchQuery: '',
      newOrganizationName: '',
      organizationsWithUsers: [],
      showAddModal: false,
      showDeleteModal: false,
      selectedOrganization: null,
      organizationToDelete: null,
      sortField: null,
      sortDirection: 'asc',
      isLoading: false
    };
  },
  computed: {
    filteredOrganizations() {
      if (!this.searchQuery) return this.organizationsWithUsers;
      const query = this.searchQuery.toLowerCase();
      return this.organizationsWithUsers.filter(org => 
        org.name.toLowerCase().includes(query) || 
        org.id.toString().includes(query)
      );
    },
    sortedOrganizations() {
      const organizations = [...this.filteredOrganizations];
      
      if (!this.sortField) {
        return organizations.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return organizations.sort((a, b) => {
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
          case 'user_count':
            valueA = a.user_count;
            valueB = b.user_count;
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
      return `Вы точно хотите удалить организацию "${this.organizationToDelete?.name}"?`;
    }
  },
  methods: {
    async refreshData() {
      await this.fetchOrganizationsWithUsers();
    },
    async fetchOrganizationsWithUsers() {
      try {
        const response = await apiRequest("/organizations/with-users-extended", {
        });
        if (response.ok) {
          const data = await response.json();
          this.organizationsWithUsers = data.map(org => ({
            ...org,
            originalName: org.name
          }));
        }
      } catch (error) {
        console.error("Error fetching organizations:", error);
        this.showNotification("Ошибка при загрузке организаций", "error");
      }
    },
    
    async addOrganization() {
      if (!this.newOrganizationName.trim()) {
        this.showNotification("Введите название организации", "warning");
        return;
      }
      
      if (this.isLoading) return;
      
      this.isLoading = true;
      
      try {
        const response = await apiRequest("/organizations", {
          method: "POST",
          body: JSON.stringify({
            name: this.newOrganizationName.trim(),
          }),
        });
        
        if (response.ok) {
          const newOrg = await response.json();
          this.newOrganizationName = '';
          this.showAddModal = false;
          await this.refreshData();
          
          // Автоматически выбираем новую организацию
          const createdOrg = this.organizationsWithUsers.find(org => org.id === newOrg.id);
          if (createdOrg) {
            this.selectedOrganization = { ...createdOrg };
          }
          
          this.showNotification("Организация успешно создана", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при создании организации", "error");
        }
      } catch (error) {
        console.error("Error adding organization:", error);
        this.showNotification("Ошибка сети", "error");
      } finally {
        this.isLoading = false;
      }
    },

    async updateOrganization(org) {
      if (org.name === org.originalName) return;
      
      try {
        const response = await apiRequest(`/organizations/${org.id}`, {
          method: "PUT",
          body: JSON.stringify({
            name: org.name,
          }),
        });
        
        if (response.ok) {
          org.originalName = org.name;
          this.showNotification("Организация успешно обновлена", "success");
        } else {
          const error = await response.json();
          org.name = org.originalName;
          this.showNotification(error.message || "Ошибка при обновлении организации", "error");
        }
      } catch (error) {
        console.error("Error updating organization:", error);
        org.name = org.originalName;
        this.showNotification("Ошибка сети", "error");
      }
    },

    confirmDeleteOrganization(org) {
      if (org.user_count > 0) {
        this.showNotification("Нельзя удалить организацию с пользователями", "warning");
        return;
      }
      
      this.organizationToDelete = org;
      this.showDeleteModal = true;
    },
    
    cancelDelete() {
      this.showDeleteModal = false;
      this.organizationToDelete = null;
    },

    async deleteOrganization() {
      if (!this.organizationToDelete) return;
      
      try {
        const response = await apiRequest(`/organizations/${this.organizationToDelete.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          this.selectedOrganization = null;
          this.showDeleteModal = false;
          this.organizationToDelete = null;
          await this.refreshData();
          this.showNotification("Организация успешно удалена", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении организации", "error");
        }
      } catch (error) {
        console.error("Error deleting organization:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },

    selectOrganization(org) {
      this.selectedOrganization = { ...org };
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },

    handleUsersUpdated() {
      this.fetchOrganizationsWithUsers();
    },

    handlePlacesUpdated() {
      this.fetchOrganizationsWithUsers();
    },

    handleTablesUpdated() {
      this.fetchOrganizationsWithUsers();
    },

    closeModal() {
      this.showAddModal = false;
      this.newOrganizationName = '';
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
  watch: {
    showAddModal(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.$refs.nameInput?.focus();
        });
      }
    }
  }
};
</script>

<style scoped>
.organizations-management {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
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
  height: 450px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
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
  min-width: 40px;
}

.name-col {
  width: 55%;
  min-width: 200px;
}

.users-col {
  width: 30%;
  min-width: 120px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 400px;
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
  background-color: #f0f2ff;
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

.user-count {
  display: flex;
  align-items: center;
  gap: 6px;
}

.count-value {
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

.details-section {
  width: fit-content;
  padding: 15px;
  overflow-y: auto;
  background: #fafafa;
  border-right: 1px solid #e6e6e6;
}

.details-content {
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 10px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
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
  padding-bottom: 15px;
}

.details-grid-two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.details-column {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight: 400;
}

.form-input-sm {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 0.8em;
  width: 100%;
  height: 32px;
  transition: border-color 0.2s ease;
  background: #fff;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.responsible-section {
  flex: 1;
  background: #fff;
  display: flex;
  flex-direction: column;
}

.responsible-content {
  padding: 10px;
  height: 100%;
}

.no-selection-message {
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
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

@media (max-width: 968px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }
  
  .table-section,
  .details-section,
  .responsible-section {
    width: 100% !important;
  }
  
  .table-section {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 255px;
  }
  
  .details-grid-two-columns {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
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
  
  .table-header,
  .table-row {
    padding: 0 16px;
  }
  
  .id-col {
    width: 20%;
  }
  
  .name-col {
    width: 50%;
  }
  
  .users-col {
    width: 30%;
  }
  
  .details-section,
  .responsible-content {
    padding: 16px;
  }
  
  .details-header {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }
  
  .details-header-actions {
    align-self: flex-end;
  }
  
  .modal-content {
    width: 95vw;
    margin: 20px;
  }
  
  .modal-header,
  .modal-body,
  .modal-footer {
    padding-left: 20px;
    padding-right: 20px;
  }
  
  @keyframes modalAppear {
    from {
      opacity: 0;
      transform: scale(0.9) translateY(-10px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }
}
</style>