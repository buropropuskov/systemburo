<template>
  <div
    class="select-tables-section"
    :class="{ card: !selectionMode }"
  >
    <div class="detail-group">
      <div
        v-if="!selectionMode"
        class="sec-title"
      >
        Целевые таблицы <span class="sec-note">(по умолчанию)</span>
        <span
          v-if="hasSelectedTables"
          class="sec-actions"
        >
          <span class="save-hint"><span class="dot" />несохранённые</span>
          <button
            class="btn-mini primary"
            :disabled="isSaving"
            @click="saveOrganizationTables"
          >
            {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
          <button
            class="btn-mini"
            :disabled="isSaving"
            @click="cancelTablesChanges"
          >
            Отмена
          </button>
        </span>
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
import { useDeletionsStore } from '@/stores/deletions'
export default {
  name: 'SelectTables',
  props: {
    entity: {
      type: Object,
      default: null
    },
    entityType: {
      type: String,
      required: true,
      validator: value => ['organization', 'company'].includes(value)
    },
    // Режим «только выбор» (групповые операции): без fetch/save сущности,
    // выбор через v-model (массив id таблиц), без Сохранить/Отмена.
    selectionMode: {
      type: Boolean,
      default: false
    },
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['tables-updated', 'dirty-change', 'update:modelValue'],
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
        if (this.selectionMode) return;
        if (newEntity && newEntity.id) {
          this.fetchEntityTables(newEntity.id);
        }
      }
    },
    // fix 5: поднимаем dirty-состояние таблиц в dirtyTracker родителя.
    hasSelectedTables: {
      immediate: true,
      handler(dirty) {
        if (this.selectionMode) return;
        this.$emit('dirty-change', dirty);
      }
    }
  },
  async mounted() {
    await this.fetchAllTables();

    if (!this.selectionMode && this.entity && this.entity.id) {
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
          this.allTables = data;
        }
      } catch (error) {
        console.error("Error fetching tables:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'таблицы', type: 'error' });
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
          useDeletionsStore().notify({ prefix: 'Таблицы сохранены для ', bold: this.entity.name, type: 'success' });
          this.$emit('tables-updated');
        } else {
          const error = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сохранить таблицы: ', bold: error.message || 'ошибка сервера', type: 'error' });
          await this.fetchEntityTables(this.entity.id);
        }
      } catch (error) {
        console.error("Error updating organization tables:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить таблицы: ', bold: 'ошибка сети', type: 'error' });
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
      if (this.selectionMode) {
        const ids = this.modelValue.includes(tableId)
          ? this.modelValue.filter(id => id !== tableId)
          : [...this.modelValue, tableId];
        this.$emit('update:modelValue', ids);
        return;
      }
      const tableObj = this.getTableObject(table);

      const index = this.selectedTables.findIndex(t => t.id === tableId);
      if (index > -1) {
        this.selectedTables.splice(index, 1);
      } else {
        this.selectedTables.push(tableObj);
      }
    },

    isTableSelected(tableId) {
      if (this.selectionMode) return this.modelValue.includes(tableId);
      return this.selectedTables.some(t => t.id === tableId);
    }
  },
};
</script>

<style scoped>
.select-tables-section {
  box-sizing: border-box;
}

/* карточка-секция (эталон мокапа .card) */
.card {
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px;
  background: var(--surface-sunken);
}

.sec-title {
  font-size: 0.82em;
  font-weight: 700;
  color: var(--accent-text);
  letter-spacing: 0.02em;
  text-transform: uppercase;
  margin: 0 0 12px;
  display: flex;
  align-items: center;
  /* резерв под появляющиеся Сохранить/Отмена (btn-mini 28px) - чтобы их
     появление не двигало чипсы/список ниже */
  min-height: 28px;
  gap: 8px;
}

.sec-note {
  text-transform: none;
  font-weight: 500;
  color: var(--text-muted);
}

.sec-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  text-transform: none;
}

.save-hint {
  font-size: 11px;
  color: var(--warning-text);
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  border-radius: 8px;
  padding: 3px 9px;
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-weight: 600;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--warning);
  display: inline-block;
}

.btn-mini {
  height: 28px;
  border-radius: 8px;
  padding: 0 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  font-family: inherit;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  white-space: nowrap;
}

.btn-mini.primary {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.btn-mini:hover:not(:disabled) {
  filter: brightness(0.97);
}

.btn-mini:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.select-tables-container {
  width: fit-content;
  background: var(--surface);
  border-radius: 15px;
  border: 1px solid var(--border);
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
  background: var(--surface-2);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
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
  background: var(--row-hover);
}

.table-item.selected {
  background: var(--accent);
  color: var(--accent-contrast);
}

.no-tables-message {
  text-align: center;
  padding: 20px;
  color: var(--text-muted);
  font-style: italic;
}

@media (max-width: 768px) {
  .tables-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .sec-actions {
    flex-wrap: wrap;
  }
}
</style>