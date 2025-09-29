<template>
  <div class="companies-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Управление компаниями</h3>
      <div class="header-controls">
        <SearchComponent
          :title="'Поиск компаний...'"
          v-model="searchQuery"
        />
        <button @click="showAddModal = true" class="add-header-button">
          Добавить
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица компаний -->
      <div class="table-section" :class="{'with-details': selectedCompany}">
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
                <span class="truncate-text" :title="comp.name">
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

      <!-- Правая часть - детали компании -->
      <div v-if="selectedCompany" class="details-section">
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">Редактирование</h3>
              <p class="details-subtitle">компании <strong>{{ selectedCompany.name }}</strong></p>
            </div>
            <div class="details-header-actions">
              <button @click="confirmDeleteCompany(selectedCompany)" class="delete-icon-btn">
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
                    v-model="selectedCompany.name" 
                    @change="updateCompany(selectedCompany)"
                    class="form-input-sm"
                    placeholder="Введите название компании"
                    autocomplete="off"
                  >
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Место разгрузки:</label>
                  <select 
                    v-model="selectedCompany.default_unloading_point_id" 
                    @change="updateCompanyUnloadingPoint(selectedCompany)"
                    class="form-select-sm"
                    autocomplete="off"
                  >
                    <option v-for="point in unloadingPoints" :key="point.id" :value="point.id">
                      {{ point.name }}
                    </option>
                  </select>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div v-else class="no-selection-message">
        <p>Выберите компанию для просмотра</p>
      </div>
    </div>

    <div v-if="filteredCompanies.length === 0" class="no-results">
      <div class="no-results-icon">🏭</div>
      <p>Компании не найдены</p>
    </div>

    <!-- Модальное окно добавления -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content">
        <div class="modal-header">
          <h3>Добавить компанию</h3>
          <button @click="showAddModal = false" class="modal-close">×</button>
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
          <button @click="showAddModal = false" class="modal-cancel">Отмена</button>
          <button @click="addCompany" class="modal-confirm">Добавить</button>
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
      newCompanyName: '',
      companiesWithUsers: [],
      showAddModal: false,
      selectedCompany: null,
      sortField: null,
      sortDirection: 'asc',
      responsibleUsers: [],
      unloadingPoints: []
    };
  },
  computed: {
    filteredCompanies() {
      if (!this.searchQuery) return this.companiesWithUsers;
      const query = this.searchQuery.toLowerCase();
      return this.companiesWithUsers.filter(comp => 
        comp.name.toLowerCase().includes(query) || 
        comp.id.toString().includes(query) ||
        (comp.type && comp.type.toLowerCase().includes(query)) ||
        (comp.responsible_person_name && comp.responsible_person_name.toLowerCase().includes(query)) ||
        (comp.default_unloading_point_name && comp.default_unloading_point_name.toLowerCase().includes(query))
      );
    },
    sortedCompanies() {
      const companies = [...this.filteredCompanies];
      
      if (!this.sortField) {
        // Изначально сортируем по наименованию
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
    }
  },
  methods: {
    async refreshData() {
      await Promise.all([
        this.fetchCompaniesWithUsers(),
        this.fetchResponsibleUsers(),
        this.fetchUnloadingPoints()
      ]);
    },
    async fetchCompaniesWithUsers() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/companies/with-users", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          const data = await response.json();
          this.companiesWithUsers = data.map(comp => ({
            ...comp,
            originalName: comp.name,
            type: comp.type || 'арендатор',
            responsible_person_id: comp.responsible_person_id || null,
            default_unloading_point_id: comp.default_unloading_point_id || null
          }));
        }
      } catch (error) {
        console.error("Error fetching companies:", error);
        this.showNotification("Ошибка при загрузке компаний", "error");
      }
    },
    async fetchResponsibleUsers() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/users", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          const users = await response.json();
          this.responsibleUsers = users.filter(user => 
            user.user_type === 'admin' || user.user_type === 'manager'
          );
        }
      } catch (error) {
        console.error("Error fetching responsible users:", error);
      }
    },
    async fetchUnloadingPoints() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/unloading-points", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          this.unloadingPoints = await response.json();
        }
      } catch (error) {
        console.error("Error fetching unloading points:", error);
      }
    },
    async addCompany() {
      if (!this.newCompanyName.trim()) {
        this.showNotification("Введите название компании", "warning");
        return;
      }
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/companies", {
          method: "POST",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            name: this.newCompanyName,
          }),
        });
        
        if (response.ok) {
          this.newCompanyName = '';
          this.showAddModal = false;
          await this.refreshData();
          this.showNotification("Компания успешно добавлена", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при добавлении компании", "error");
        }
      } catch (error) {
        console.error("Error adding company:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    async updateCompany(comp) {
      if (comp.name === comp.originalName) return;
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/companies/${comp.id}`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            name: comp.name,
          }),
        });
        
        if (response.ok) {
          comp.originalName = comp.name;
          this.showNotification("Компания успешно обновлена", "success");
        } else {
          const error = await response.json();
          comp.name = comp.originalName;
          this.showNotification(error.message || "Ошибка при обновлении компании", "error");
        }
      } catch (error) {
        console.error("Error updating company:", error);
        comp.name = comp.originalName;
        this.showNotification("Ошибка сети", "error");
      }
    },
    async updateCompanyType(comp) {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/companies/${comp.id}/type`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            type: comp.type,
          }),
        });
        
        if (!response.ok) {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при обновлении типа компании", "error");
          await this.refreshData();
        } else {
          this.showNotification("Тип компании успешно обновлен", "success");
        }
      } catch (error) {
        console.error("Error updating company type:", error);
        this.showNotification("Ошибка сети", "error");
        await this.refreshData();
      }
    },
    async updateCompanyResponsible(comp) {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/companies/${comp.id}/responsible`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            responsible_person_id: comp.responsible_person_id,
          }),
        });
        
        if (!response.ok) {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при обновлении ответственного", "error");
          await this.refreshData();
        } else {
          this.showNotification("Ответственный успешно обновлен", "success");
        }
      } catch (error) {
        console.error("Error updating responsible person:", error);
        this.showNotification("Ошибка сети", "error");
        await this.refreshData();
      }
    },
    async updateCompanyUnloadingPoint(comp) {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/companies/${comp.id}/unloading-point`, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            default_unloading_point_id: comp.default_unloading_point_id,
          }),
        });
        
        if (!response.ok) {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при обновлении места разгрузки", "error");
          await this.refreshData();
        } else {
          this.showNotification("Место разгрузки успешно обновлено", "success");
        }
      } catch (error) {
        console.error("Error updating unloading point:", error);
        this.showNotification("Ошибка сети", "error");
        await this.refreshData();
      }
    },
    async confirmDeleteCompany(comp) {
      if (comp.user_count > 0) {
        this.showNotification("Нельзя удалить компанию с пользователями", "warning");
        return;
      }
      
      if (!confirm(`Вы уверены, что хотите удалить компанию "${comp.name}"?`)) return;
      
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(`http://localhost:8080/companies/${comp.id}`, {
          method: "DELETE",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        
        if (response.ok) {
          this.selectedCompany = null;
          await this.refreshData();
          this.showNotification("Компания успешно удалена", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении компании", "error");
        }
      } catch (error) {
        console.error("Error deleting company:", error);
        this.showNotification("Ошибка сети", "error");
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
  height: 255px;
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
  max-height: 255px;
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

/* Правая часть - детали */
.details-section {
  width: 60%;
  padding: 20px;
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
  padding-bottom: 15px;
  border-bottom: 1px solid #e6e6e6;
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

.details-subtitle {
  margin: 0;
  font-size: 10px;
  color: #a2a2a2;
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
  gap: 6px;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight: 500;
}

.form-input-sm {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.95em;
  width: 210px;
  height: 35px;
  transition: border-color 0.2s ease;
  background: #fff;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-select-sm {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  background-color: white;
  font-size: 0.95em;
  width: 210px;
  height: 35px;
  transition: border-color 0.2s ease;
}

.form-select-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
  background: #fafafa;
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
  background: rgba(0, 0, 0, 0.03);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
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
  .no-selection-message {
    width: 100% !important;
  }
  
  .table-section.with-details {
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
  
  .details-section {
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
  
  .form-input-sm,
  .form-select-sm {
    width: 100%;
  }
}
</style>