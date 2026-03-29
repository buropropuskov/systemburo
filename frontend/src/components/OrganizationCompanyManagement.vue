<template>
  <div class="organization-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Управление организациями и компаниями</h3>
      <div class="search-container">
        <SearchComponent
          :title="'Поиск...'"
          v-model="searchQuery"
        />
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="tabs-header">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        @click="activeTab = tab.id"
        :class="{ active: activeTab === tab.id }"
        class="tab-button"
      >
        {{ tab.label }}
      </button>
    </div>

    <div class="tab-content">
      <!-- Организации -->
      <div v-if="activeTab === 'organizations'" class="tab-pane">
        <div class="add-item-section">
          <div class="add-item-form">
            <input
              v-model="newOrganizationName"
              placeholder="Введите название организации"
              class="form-input"
              @keyup.enter="addOrganization"
            >
            <button @click="addOrganization" class="add-button">
              <span class="button-icon">+</span>
              Добавить
            </button>
          </div>
        </div>

        <div class="content-split">
          <!-- Левая часть - таблица -->
          <div class="table-section">
            <div class="table-container">
              <div class="table-header-row">
                <div class="table-col id-col">ID</div>
                <div class="table-col name-col">Наименование</div>
                <div class="table-col users-col">Кол-во пользователей</div>
              </div>

              <div class="table-body">
                <div 
                  v-for="org in filteredOrganizations" 
                  :key="org.id" 
                  class="table-row"
                >
                  <div class="table-col id-col">
                    <span class="cell-content">{{ org.id }}</span>
                  </div>
                  <div class="table-col name-col">
                    <input
                      v-model="org.name"
                      @change="updateOrganization(org)"
                      class="editable-input"
                      :class="{ 'editing': org.name !== org.originalName }"
                    >
                  </div>
                  <div class="table-col users-col">
                    <span class="cell-content user-count">
                      {{ org.user_count }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="table-footer">
              <span class="items-count">Всего организаций: {{ filteredOrganizations.length }}</span>
            </div>
          </div>

          <!-- Правая часть - детали (пока пустая) -->
          <div class="details-section">
            <div class="empty-details">
              <div class="empty-icon">🏢</div>
              <p class="empty-text">Выберите организацию для просмотра деталей</p>
            </div>
          </div>
        </div>

        <div v-if="filteredOrganizations.length === 0" class="no-results">
          <div class="no-results-icon">🏢</div>
          <p>Организации не найдены</p>
        </div>
      </div>

      <!-- Компании -->
      <div v-if="activeTab === 'companies'" class="tab-pane">
        <div class="add-item-section">
          <div class="add-item-form">
            <input
              v-model="newCompanyName"
              placeholder="Введите название компании"
              class="form-input"
              @keyup.enter="addCompany"
            >
            <button @click="addCompany" class="add-button">
              <span class="button-icon">+</span>
              Добавить
            </button>
          </div>
        </div>

        <div class="content-split">
          <!-- Левая часть - таблица -->
          <div class="table-section">
            <div class="table-container">
              <div class="table-header-row">
                <div class="table-col id-col">ID</div>
                <div class="table-col name-col">Наименование</div>
                <div class="table-col users-col">Кол-во пользователей</div>
              </div>

              <div class="table-body">
                <div 
                  v-for="comp in filteredCompanies" 
                  :key="comp.id" 
                  class="table-row"
                >
                  <div class="table-col id-col">
                    <span class="cell-content">{{ comp.id }}</span>
                  </div>
                  <div class="table-col name-col">
                    <input
                      v-model="comp.name"
                      @change="updateCompany(comp)"
                      class="editable-input"
                      :class="{ 'editing': comp.name !== comp.originalName }"
                    >
                  </div>
                  <div class="table-col users-col">
                    <span class="cell-content user-count">
                      {{ comp.user_count }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="table-footer">
              <span class="items-count">Всего компаний: {{ filteredCompanies.length }}</span>
            </div>
          </div>

          <!-- Правая часть - детали (пока пустая) -->
          <div class="details-section">
            <div class="empty-details">
              <div class="empty-icon">🏭</div>
              <p class="empty-text">Выберите компанию для просмотра деталей</p>
            </div>
          </div>
        </div>

        <div v-if="filteredCompanies.length === 0" class="no-results">
          <div class="no-results-icon">🏭</div>
          <p>Компании не найдены</p>
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
      activeTab: 'organizations',
      tabs: [
        { id: 'organizations', label: 'Организации' },
        { id: 'companies', label: 'Компании' }
      ],
      searchQuery: '',
      newOrganizationName: '',
      newCompanyName: '',
      organizationsWithUsers: [],
      companiesWithUsers: [],
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
    filteredCompanies() {
      if (!this.searchQuery) return this.companiesWithUsers;
      const query = this.searchQuery.toLowerCase();
      return this.companiesWithUsers.filter(comp => 
        comp.name.toLowerCase().includes(query) || 
        comp.id.toString().includes(query)
      );
    }
  },
  methods: {
    async refreshData() {
      await Promise.all([
        this.fetchOrganizationsWithUsers(),
        this.fetchCompaniesWithUsers()
      ]);
    },
    async fetchOrganizationsWithUsers() {
      try {
        const response = await apiRequest("/organizations/with-users", {
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
    async fetchCompaniesWithUsers() {
      try {
        const response = await apiRequest("/companies/with-users", {
        });
        if (response.ok) {
          const data = await response.json();
          this.companiesWithUsers = data.map(comp => ({
            ...comp,
            originalName: comp.name
          }));
        }
      } catch (error) {
        console.error("Error fetching companies:", error);
        this.showNotification("Ошибка при загрузке компаний", "error");
      }
    },
    async addOrganization() {
      if (!this.newOrganizationName.trim()) {
        this.showNotification("Введите название организации", "warning");
        return;
      }
      
      try {
        const response = await apiRequest("/organizations", {
          method: "POST",
          body: JSON.stringify({
            name: this.newOrganizationName,
          }),
        });
        
        if (response.ok) {
          this.newOrganizationName = '';
          await this.fetchOrganizationsWithUsers();
          this.showNotification("Организация успешно добавлена", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при добавлении организации", "error");
        }
      } catch (error) {
        console.error("Error adding organization:", error);
        this.showNotification("Ошибка сети", "error");
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
    async deleteOrganization(org) {
      if (org.user_count > 0) {
        this.showNotification("Нельзя удалить организацию с пользователями", "warning");
        return;
      }
      
      if (!confirm(`Вы уверены, что хотите удалить организацию "${org.name}"?`)) return;
      
      try {
        const response = await apiRequest(`/organizations/${org.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          await this.fetchOrganizationsWithUsers();
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
    async addCompany() {
      if (!this.newCompanyName.trim()) {
        this.showNotification("Введите название компании", "warning");
        return;
      }
      
      try {
        const response = await apiRequest("/companies", {
          method: "POST",
          body: JSON.stringify({
            name: this.newCompanyName,
          }),
        });
        
        if (response.ok) {
          this.newCompanyName = '';
          await this.fetchCompaniesWithUsers();
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
        const response = await apiRequest(`/companies/${comp.id}`, {
          method: "PUT",
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
    async deleteCompany(comp) {
      if (comp.user_count > 0) {
        this.showNotification("Нельзя удалить компанию с пользователями", "warning");
        return;
      }
      
      if (!confirm(`Вы уверены, что хотите удалить компанию "${comp.name}"?`)) return;
      
      try {
        const response = await apiRequest(`/companies/${comp.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          await this.fetchCompaniesWithUsers();
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
.organization-management {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
  gap: 15px;
  height: 50px;
  border-bottom: 1px solid #e6e6e6;
  padding: 0 20px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: #000;
}

.search-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tabs-header {
  display: flex;
  border-bottom: 1px solid #e6e6e6;
  margin: 0 20px;
}

.tab-button {
  padding: 12px 24px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-weight: 600;
  color: #666;
  transition: all 0.2s ease;
  font-size: 0.95em;
}

.tab-button:hover {
  color: #4F5BDF;
}

.tab-button.active {
  color: #4F5BDF;
  border-bottom-color: #4F5BDF;
}

.tab-content {
  padding: 20px;
}

.tab-pane {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.add-item-section {
  margin-bottom: 20px;
  padding: 16px;
  background: #f8f9ff;
  border-radius: 8px;
}

.add-item-form {
  display: flex;
  gap: 12px;
  align-items: center;
}

.form-input {
  padding: 10px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  flex-grow: 1;
  max-width: 400px;
  font-size: 0.95em;
  transition: border-color 0.2s ease;
  background: #fff;
}

.form-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.add-button {
  padding: 10px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.95em;
  font-weight: 600;
  transition: background-color 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
}

.add-button:hover {
  background: #3a45b2;
}

.button-icon {
  font-weight: bold;
}

.content-split {
  display: flex;
  height: 400px;
  gap: 20px;
}

.table-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.table-container {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  overflow: hidden;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.table-header-row {
  display: flex;
  background: #f8fafc;
  padding: 14px 16px;
  font-weight: 600;
  color: #4b5563;
  border-bottom: 1px solid #e2e8f0;
}

.table-body {
  flex: 1;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 12px 16px;
  border-bottom: 1px solid #f1f5f9;
  align-items: center;
  transition: background-color 0.2s ease;
}

.table-row:hover {
  background-color: #f8fafc;
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.id-col {
  width: 15%;
  min-width: 60px;
}

.name-col {
  width: 55%;
  min-width: 200px;
}

.users-col {
  width: 30%;
  min-width: 120px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.user-count {
  font-weight: 600;
  color: #4b5563;
}

.editable-input {
  padding: 8px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  width: 100%;
  font-size: 0.95em;
  transition: all 0.2s ease;
  background: #fff;
}

.editable-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.editable-input.editing {
  border-color: #f59e0b;
  background: #fffbeb;
}

.table-footer {
  padding: 12px 16px;
  background: #f8fafc;
  border-top: 1px solid #e2e8f0;
  text-align: center;
}

.items-count {
  font-size: 0.9em;
  color: #6b7280;
  font-weight: 500;
}

.details-section {
  width: 40%;
  background: #fafafa;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-details {
  text-align: center;
  color: #9ca3af;
}

.empty-icon {
  font-size: 3em;
  margin-bottom: 12px;
  opacity: 0.5;
}

.empty-text {
  margin: 0;
  font-size: 0.95em;
  max-width: 200px;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #6b7280;
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

@media (max-width: 968px) {
  .content-split {
    flex-direction: column;
    height: auto;
  }
  
  .details-section {
    width: 100%;
    height: 200px;
  }
}

@media (max-width: 768px) {
  .management-header {
    flex-direction: column;
    align-items: flex-start;
    height: auto;
    gap: 12px;
  }
  
  .search-container {
    width: 100%;
  }
  
  .tabs-header {
    margin: 0 16px;
    overflow-x: auto;
  }
  
  .tab-button {
    padding: 10px 16px;
    font-size: 0.9em;
  }
  
  .tab-content {
    padding: 16px;
  }
  
  .add-item-form {
    flex-direction: column;
    align-items: stretch;
  }
  
  .form-input {
    max-width: none;
    margin-bottom: 12px;
  }
  
  .table-header-row,
  .table-row {
    padding: 12px;
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
}
</style>