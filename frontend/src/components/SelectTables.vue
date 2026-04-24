<template>
  <div class="select-tables-section">
    <div class="detail-group">
      <div class="select-tables__header">
        <label class="detail-label">Целевые таблицы (по умолчанию):</label>
        <div
          v-if="hasSelectedTables"
          class="tables-actions"
        >
          <button 
            class="save-tables-btn" 
            :disabled="isSaving"
            @click="saveOrganizationTables"
          >
            {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
          <button 
            class="cancel-tables-btn" 
            :disabled="isSaving"
            @click="cancelTablesChanges"
          >
            Отмена
          </button>
        </div>
      </div>
      
      <div class="select-tables-container">
        <div class="tables-grid">
          <div 
            v-for="table in filteredTables" 
            :key="getTableId(table)"
            class="table-item"
            :class="{
              'selected': isTableSelected(getTableId(table))
            }"
            @click="toggleTable(table)"
          >
            {{ getTableName(table) }}
          </div>
        </div>

        <div
          v-if="filteredTables.length === 0"
          class="no-tables-message"
        >
          <p>Нет доступных таблиц для людей</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
export default {
  name: 'SelectTables',
  props: {
    entity: {
      type: Object,
      required: true
    },
    entityType: {
      type: String,
      required: true,
      validator: value => ['organization', 'company'].includes(value)
    }
  },
  emits: ['tables-updated'],
  data() {
    return {
      allTables: [],
      selectedTables: [],
      originalSelectedTables: [],
      isSaving: false
    };
  },
  computed: {
    // Фильтруем таблицы, оставляем только те, у которых table_type НЕ 'cars'
    filteredTables() {
      return this.allTables.filter(table => {
        const tableType = this.getTableType(table);
        return tableType !== 'cars';
      });
    },
    
    hasSelectedTables() {
      return JSON.stringify(this.selectedTables.map(t => t.id).sort()) !== 
             JSON.stringify(this.originalSelectedTables.map(t => t.id).sort());
    }
  },
  watch: {
    entity: {
      immediate: true,
      handler(newEntity) {
        if (newEntity && newEntity.id) {
          this.fetchEntityTables(newEntity.id);
        }
      }
    }
  },
  async mounted() {
    await this.fetchAllTables();
    
    if (this.entity && this.entity.id) {
      await this.fetchEntityTables(this.entity.id);
    }
  },
  methods: {
    getTableId(table) {
      if (table.table && table.table.id) {
        return table.table.id;
      }
      return table.id;
    },

    getTableName(table) {
      if (table.table && table.table.display_name) {
        return table.table.display_name;
      }
      return table.display_name || 'Без названия';
    },

    getTableType(table) {
      if (table.table && table.table.table_type) {
        return table.table.table_type;
      }
      return table.table_type;
    },

    getTableObject(table) {
      if (table.table) {
        return {
          id: table.table.id,
          display_name: table.table.display_name,
          name: table.table.name,
          table_type: table.table.table_type
        };
      }
      return {
        id: table.id,
        display_name: table.display_name,
        name: table.name,
        table_type: table.table_type
      };
    },

    async fetchAllTables() {
      try {
        const response = await apiRequest("/system-tables", {
        });
        if (response.ok) {
          const data = await response.json();
          console.log('Fetched all tables:', data);
          console.log('Filtered tables (excluding cars):', data.filter(t => this.getTableType(t) !== 'cars'));
          this.allTables = data;
        }
      } catch (error) {
        console.error("Error fetching tables:", error);
        this.showNotification("Ошибка при загрузке таблиц", "error");
      }
    },

    async fetchEntityTables(entityId) {
      try {
        const endpoint = this.entityType === 'organization' 
          ? `/organizations/${entityId}/tables`
          : `/companies/${entityId}/tables`;
        
        const response = await apiRequest(endpoint, {
        });
        if (response.ok) {
          const tables = await response.json();
          console.log(`Fetched ${this.entityType} tables:`, tables);
          
          this.selectedTables = tables.map(table => {
            if (table.table) {
              return {
                id: table.table.id,
                display_name: table.table.display_name,
                name: table.table.name,
                table_type: table.table.table_type
              };
            }
            return {
              id: table.id,
              display_name: table.display_name,
              name: table.name,
              table_type: table.table_type
            };
          });
          
          this.originalSelectedTables = JSON.parse(JSON.stringify(this.selectedTables));
        } else {
          this.selectedTables = [];
          this.originalSelectedTables = [];
        }
      } catch (error) {
        console.error(`Error fetching ${this.entityType} tables:`, error);
        this.selectedTables = [];
        this.originalSelectedTables = [];
      }
    },

    async saveOrganizationTables() {
      if (!this.entity) return;
      
      this.isSaving = true;
      try {
        const endpoint = this.entityType === 'organization'
          ? `/organizations/${this.entity.id}/tables`
          : `/companies/${this.entity.id}/tables`;
        
        const response = await apiRequest(endpoint, {
          method: "PUT",
          body: JSON.stringify({
            table_ids: this.selectedTables.map(t => t.id),
          }),
        });
        
        if (response.ok) {
          this.originalSelectedTables = JSON.parse(JSON.stringify(this.selectedTables));
          this.showNotification("Таблицы по умолчанию успешно обновлены", "success");
          this.$emit('tables-updated');
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при обновлении таблиц", "error");
          await this.fetchEntityTables(this.entity.id);
        }
      } catch (error) {
        console.error("Error updating organization tables:", error);
        this.showNotification("Ошибка сети", "error");
        await this.fetchEntityTables(this.entity.id);
      } finally {
        this.isSaving = false;
      }
    },

    cancelTablesChanges() {
      this.selectedTables = JSON.parse(JSON.stringify(this.originalSelectedTables));
    },

    toggleTable(table) {
      const tableId = this.getTableId(table);
      const tableObj = this.getTableObject(table);
      
      const index = this.selectedTables.findIndex(t => t.id === tableId);
      if (index > -1) {
        this.selectedTables.splice(index, 1);
      } else {
        this.selectedTables.push(tableObj);
      }
    },

    isTableSelected(tableId) {
      return this.selectedTables.some(t => t.id === tableId);
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
.select-tables-section {
  margin-top: 5px;
}

.select-tables__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 35px;
}

.select-tables-container {
  width: fit-content;
  background: #FFF;
  border-radius: 15px;
  border: 1px solid #e6e6e6;
  padding: 12px;
}

.tables-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 5px;
  row-gap: 5px;
}

.table-item {
  height: 30px;
  border-radius: 50px;
  background: #f2f2f2;
  font-size: 12px;
  font-weight: 500;
  color: #a2a2a2;
  width: 140px;
  min-width: 80px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
  user-select: none;
  text-align: center;
}

.table-item:hover {
  background: #e8e8e8;
}

.table-item.selected {
  background: #4F5BDF;
  color: #FFF;
}

.no-tables-message {
  text-align: center;
  padding: 20px;
  color: #6b7280;
  font-style: italic;
}

.tables-actions {
  display: flex;
  gap: 8px;
}

.save-tables-btn {
  padding: 0px 8px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 15px;
  font-size: 0.6em;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.2s ease;
  height: 20px;
}

.save-tables-btn:hover:not(:disabled) {
  background: #3a45b2;
}

.save-tables-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.cancel-tables-btn {
  padding: 0px 8px;
  font-weight: 600;
  background: #6b7280;
  color: white;
  border: none;
  border-radius: 15px;
  font-size: 0.6em;
  cursor: pointer;
  transition: background-color 0.2s ease;
  height: 20px;
}

.cancel-tables-btn:hover:not(:disabled) {
  background: #4b5563;
}

.cancel-tables-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight: 400;
}

@media (max-width: 768px) {
  .tables-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .tables-actions {
    flex-direction: column;
  }
}
</style>