<template>
  <div class="unload-places-section">
    <div class="detail-group">
      <div class="unload-places__header">
        <label class="detail-label">Места разгрузки (по умолчанию):</label>
        <div
          v-if="hasSelectedPlaces"
          class="places-actions"
        >
          <button 
            class="save-places-btn" 
            :disabled="isSaving"
            @click="saveUnloadPlaces"
          >
            {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
          <button 
            class="cancel-places-btn" 
            :disabled="isSaving"
            @click="cancelUnloadPlacesChanges"
          >
            Отмена
          </button>
        </div>
      </div>
      
      <div class="unload-places-container">
        <div class="unload-places-grid">
          <div 
            v-for="place in allUnloadPlaces" 
            :key="place.id"
            class="place-item"
            :class="{
              'selected': isPlaceSelected(place.id),
              'disabled': !place.is_active
            }"
            @click="toggleUnloadPlace(place)"
          >
            {{ place.name }}
          </div>
        </div>

        <div
          v-if="allUnloadPlaces.length === 0"
          class="no-places-message"
        >
          <p>Нет доступных мест разгрузки</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
export default {
  name: 'SelectUnloadPlaces',
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
  emits: ['places-updated'],
  data() {
    return {
      allUnloadPlaces: [],
      selectedUnloadPlaces: [],
      originalSelectedPlaces: [],
      isSaving: false
    };
  },
  computed: {
    hasSelectedPlaces() {
      return JSON.stringify(this.selectedUnloadPlaces.map(p => p.id).sort()) !== 
             JSON.stringify(this.originalSelectedPlaces.map(p => p.id).sort());
    }
  },
  watch: {
    entity: {
      immediate: true,
      handler(newEntity) {
        if (newEntity && newEntity.id) {
          this.fetchEntityUnloadPlaces(newEntity.id);
        }
      }
    }
  },
  async mounted() {
    await this.fetchAllUnloadPlaces();
    
    if (this.entity && this.entity.id) {
      await this.fetchEntityUnloadPlaces(this.entity.id);
    }
  },
  methods: {
    async fetchAllUnloadPlaces() {
      try {
        const response = await apiRequest("/unload-places", {
        });
        if (response.ok) {
          this.allUnloadPlaces = await response.json();
        }
      } catch (error) {
        console.error("Error fetching unload places:", error);
      }
    },

    async fetchEntityUnloadPlaces(entityId) {
      try {
        const endpoint = this.entityType === 'organization' 
          ? `/organizations/${entityId}/unload-places`
          : `/companies/${entityId}/unload-places`;
        
        const response = await apiRequest(endpoint, {
        });
        if (response.ok) {
          const places = await response.json();
          this.selectedUnloadPlaces = places;
          this.originalSelectedPlaces = [...places];
        } else {
          this.selectedUnloadPlaces = [];
          this.originalSelectedPlaces = [];
        }
      } catch (error) {
        console.error(`Error fetching ${this.entityType} unload places:`, error);
        this.selectedUnloadPlaces = [];
        this.originalSelectedPlaces = [];
      }
    },

    async saveUnloadPlaces() {
      if (!this.entity) return;
      
      this.isSaving = true;
      try {
        const endpoint = this.entityType === 'organization'
          ? `/organizations/${this.entity.id}/unload-places`
          : `/companies/${this.entity.id}/unload-places`;
        
        const response = await apiRequest(endpoint, {
          method: "PUT",
          body: JSON.stringify({
            unload_place_ids: this.selectedUnloadPlaces.map(p => p.id),
          }),
        });
        
        if (response.ok) {
          this.originalSelectedPlaces = [...this.selectedUnloadPlaces];
          this.showNotification("Места разгрузки успешно обновлены", "success");
          this.$emit('places-updated');
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при обновлении мест разгрузки", "error");
          await this.fetchEntityUnloadPlaces(this.entity.id);
        }
      } catch (error) {
        console.error("Error updating unload places:", error);
        this.showNotification("Ошибка сети", "error");
        await this.fetchEntityUnloadPlaces(this.entity.id);
      } finally {
        this.isSaving = false;
      }
    },

    cancelUnloadPlacesChanges() {
      this.selectedUnloadPlaces = [...this.originalSelectedPlaces];
    },

    toggleUnloadPlace(place) {
      if (!place.is_active) return;
      
      const index = this.selectedUnloadPlaces.findIndex(p => p.id === place.id);
      if (index > -1) {
        this.selectedUnloadPlaces.splice(index, 1);
      } else {
        this.selectedUnloadPlaces.push(place);
      }
    },

    isPlaceSelected(placeId) {
      return this.selectedUnloadPlaces.some(p => p.id === placeId);
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
.unload-places-section {
  margin-top: 5px;
}

.unload-places__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 35px;
}

.unload-places-container {
  width: fit-content;
  background: #FFF;
  border-radius: 15px;
  border: 1px solid #e6e6e6;
  padding: 12px;
}

.unload-places-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 5px;
  row-gap: 5px;
}

.place-item {
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
}

.place-item:hover {
  background: #e8e8e8;
}

.place-item.selected {
  background: #4F5BDF;
  color: #FFF;
}

.place-item.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.place-item.disabled:hover {
  background: #f2f2f2;
}

.no-places-message {
  text-align: center;
  padding: 20px;
  color: #6b7280;
  font-style: italic;
}

.places-actions {
  display: flex;
  gap: 8px; 
}

.save-places-btn {
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

.save-places-btn:hover:not(:disabled) {
  background: #3a45b2;
}

.save-places-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.cancel-places-btn {
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

.cancel-places-btn:hover:not(:disabled) {
  background: #4b5563;
}

.cancel-places-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight: 400;
}

@media (max-width: 768px) {
  .unload-places-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .places-actions {
    flex-direction: column;
  }
}
</style>