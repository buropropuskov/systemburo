<template>
  <div class="companies-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Управление компаниями
      </h3>
      <div class="header-controls">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск компаний...'"
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
      <!-- Левая часть - таблица компаний -->
      <div class="table-section">
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
              @click="sortBy('user_count')"
            >
              <p :class="{ 'active-sort': sortField === 'user_count' }">
                Пользователи
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'user_count',
                  'desc': sortField === 'user_count' && sortDirection === 'desc'
                }" 
              >
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="comp in sortedCompanies" 
              :key="comp.id" 
              class="table-row"
              :class="{'selected': selectedCompany && selectedCompany.id === comp.id}"
              @click="selectCompany(comp)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ comp.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="comp.name"
                >
                  {{ comp.name }}
                </span>
              </div>
              <div class="table-col users-col">
                <span class="cell-content user-count">
                  <span class="count-value">{{ comp.user_count }}</span>
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего компаний: {{ filteredCompanies.length }}</span>
          </div>
        </div>
      </div>

      <!-- Средняя часть - детали компании -->
      <div
        v-if="selectedCompany"
        class="details-section"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedCompany.name }}
              </h3>
            </div>
            <div class="details-header-actions">
              <button
                class="delete-icon-btn"
                @click="confirmDeleteCompany(selectedCompany)"
              >
                <img
                  src="@/assets/icons/delete.png"
                  class="delete-icon"
                >
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
                    v-model="selectedCompany.name" 
                    class="form-input-sm"
                    placeholder="Введите название компании"
                    autocomplete="off"
                    @change="saveCompany(selectedCompany)"
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
              :entity="selectedCompany"
              :entity-type="'company'"
              @places-updated="handlePlacesUpdated"
            />

            <!-- Компонент таблиц по умолчанию -->
            <SelectTables
              :entity="selectedCompany"
              :entity-type="'company'"
              @tables-updated="handleTablesUpdated"
            />
          </div>
        </div>
      </div>
      
      <!-- Правая часть - ответственные лица -->
      <div
        class="responsible-section"
        :class="{'with-details': selectedCompany}"
      >
        <div
          v-if="selectedCompany"
          class="responsible-content"
        >
          <ResponsibleUsersSection
            :entity="selectedCompany"
            :entity-type="'company'"
            @users-updated="handleUsersUpdated"
          />
        </div>
        <div
          v-else
          class="no-selection-message"
        >
          <p>Выберите компанию для просмотра</p>
        </div>
      </div>
    </div>

    <div
      v-if="filteredCompanies.length === 0"
      class="no-results"
    >
      <div class="no-results-icon">
        🏭
      </div>
      <p>Компании не найдены</p>
    </div>

    <!-- Модальное окно добавления -->
    <Teleport to="body">
      <div
        v-if="showAddModal"
        class="modal-overlay"
        @click.self="showAddModal = false"
      >
        <div class="modal-content">
          <div class="modal-header">
            <h3>Добавить компанию</h3>
            <button
              class="modal-close"
              @click="showAddModal = false"
            >
              ×
            </button>
          </div>
          <div class="modal-body">
            <input
              v-model="newCompanyName"
              placeholder="Введите название компании"
              class="modal-input"
              @keyup.enter="addCompany"
            >
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
              @click="addCompany"
            >
              Добавить
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Модальное окно подтверждения удаления -->
    <ConfirmationModal
      :show="showDeleteModal"
      title="Подтверждение удаления"
      :message="deleteMessage"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
      @confirm="removeCompany"
      @cancel="cancelDelete"
    />
  </div>
</template>

<script>
import { mapState, mapActions } from 'pinia';
import { useCompaniesStore } from '@/stores/companies';
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
      newCompanyName: '',
      showAddModal: false,
      showDeleteModal: false,
      selectedCompany: null,
      companyToDelete: null,
      sortField: null,
      sortDirection: 'asc'
    };
  },
  computed: {
    ...mapState(useCompaniesStore, {
      companiesWithUsers: 'itemsWithUsers',
      isLoading: 'isLoading',
    }),
    filteredCompanies() {
      if (!this.searchQuery) return this.companiesWithUsers;
      const query = this.searchQuery.toLowerCase();
      return this.companiesWithUsers.filter(comp => 
        comp.name.toLowerCase().includes(query) || 
        comp.id.toString().includes(query)
      );
    },
    sortedCompanies() {
      const companies = [...this.filteredCompanies];
      
      if (!this.sortField) {
        return companies.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return companies.sort((a, b) => {
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
      return `Вы точно хотите удалить компанию "${this.companyToDelete?.name}"?`;
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
    ...mapActions(useCompaniesStore, ['refresh', 'createCompany', 'updateCompany', 'deleteCompany', 'fetchCompaniesWithUsers']),

    async refreshData() {
      await this.refresh();
    },

    async addCompany() {
      if (!this.newCompanyName.trim()) {
        this.showNotification("Введите название компании", "warning");
        return;
      }

      if (this.isLoading) return;

      const result = await this.createCompany({
        name: this.newCompanyName.trim(),
      });

      if (result.ok) {
        this.newCompanyName = '';
        this.showAddModal = false;
        // Автоматически выбираем новую компанию
        const createdComp = this.companiesWithUsers.find(comp => comp.id === result.data.id);
        if (createdComp) {
          this.selectedCompany = { ...createdComp };
        }
        this.showNotification("Компания успешно создана", "success");
      } else {
        this.showNotification(result.message || "Ошибка при создании компании", "error");
      }
    },

    async saveCompany(comp) {
      if (comp.name === comp.originalName) return;

      const result = await this.updateCompany(comp.id, { name: comp.name });

      if (result.ok) {
        this.showNotification("Компания успешно обновлена", "success");
      } else {
        comp.name = comp.originalName;
        this.showNotification(result.message || "Ошибка при обновлении компании", "error");
      }
    },

    confirmDeleteCompany(comp) {
      if (comp.user_count > 0) {
        this.showNotification("Нельзя удалить компанию с пользователями", "warning");
        return;
      }

      this.companyToDelete = comp;
      this.showDeleteModal = true;
    },

    cancelDelete() {
      this.showDeleteModal = false;
      this.companyToDelete = null;
    },

    async removeCompany() {
      if (!this.companyToDelete) return;

      const result = await this.deleteCompany(this.companyToDelete.id);

      if (result.ok) {
        this.selectedCompany = null;
        this.showDeleteModal = false;
        this.companyToDelete = null;
        this.showNotification("Компания успешно удалена", "success");
      } else {
        this.showNotification(result.message || "Ошибка при удалении компании", "error");
      }
    },

    selectCompany(comp) {
      this.selectedCompany = { ...comp };
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
      this.fetchCompaniesWithUsers();
    },
    handlePlacesUpdated() {
      this.fetchCompaniesWithUsers();
    },
    handleTablesUpdated() {
      this.fetchCompaniesWithUsers();
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
  }
};
</script>

<style scoped>
.companies-management {
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
  height: 400px;
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
  background-color: #f0f2ff;;
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
  margin-top: auto;
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
  z-index: 1000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 0;
  width: 400px;
  max-width: 90%;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
}

.modal-close {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-close:hover {
  color: #333;
}

.modal-body {
  padding: 20px;
}

.modal-input {
  width: 100%;
  padding: 12px 16px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 1em;
  transition: border-color 0.2s ease;
}

.modal-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 20px;
  border-top: 1px solid #e6e6e6;
}

.modal-cancel {
  padding: 10px 20px;
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 500;
  transition: all 0.2s ease;
}

.modal-cancel:hover {
  background: #e9ecef;
}

.modal-confirm {
  padding: 10px 20px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  transition: background-color 0.2s ease;
}

.modal-confirm:hover {
  background: #3a45b2;
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
}
</style>