<template>
  <div
    class="unload-places-section"
    :class="{ card: !selectionMode }"
  >
    <div class="detail-group">
      <div
        v-if="!selectionMode"
        class="sec-title"
      >
        Места разгрузки <span class="sec-note">(по умолчанию)</span>
        <span
          v-if="hasSelectedPlaces"
          class="sec-actions"
        >
          <span class="save-hint"><span class="dot" />несохранённые</span>
          <button
            class="btn-mini primary"
            :disabled="isSaving"
            @click="saveUnloadPlaces"
          >
            {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
          <button
            class="btn-mini"
            :disabled="isSaving"
            @click="cancelUnloadPlacesChanges"
          >
            Отмена
          </button>
        </span>
      </div>

      <div class="unload-places-container">
        <div class="unload-places-grid">
          <div 
            v-for="place in allUnloadPlaces" 
            :key="place.id"
            class="place-item"
            :class="{ 'selected': isPlaceSelected(place.id) }"
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
import { useDeletionsStore } from '@/stores/deletions'
export default {
  name: 'SelectUnloadPlaces',
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
    // Режим «только выбор» (для групповых операций): без fetch/save сущности,
    // выбор через v-model (массив id мест), без Сохранить/Отмена.
    selectionMode: {
      type: Boolean,
      default: false
    },
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['places-updated', 'dirty-change', 'update:modelValue'],
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
        if (this.selectionMode) return;
        if (newEntity && newEntity.id) {
          this.fetchEntityUnloadPlaces(newEntity.id);
        }
      }
    },
    // fix 5: поднимаем dirty-состояние мест в dirtyTracker родителя.
    hasSelectedPlaces: {
      immediate: true,
      handler(dirty) {
        if (this.selectionMode) return;
        this.$emit('dirty-change', dirty);
      }
    }
  },
  async mounted() {
    await this.fetchAllUnloadPlaces();

    if (!this.selectionMode && this.entity && this.entity.id) {
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
          useDeletionsStore().notify({ prefix: 'Места разгрузки сохранены для ', bold: this.entity.name, type: 'success' });
          this.$emit('places-updated');
        } else {
          const error = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сохранить места разгрузки: ', bold: error.message || 'ошибка сервера', type: 'error' });
          await this.fetchEntityUnloadPlaces(this.entity.id);
        }
      } catch (error) {
        console.error("Error updating unload places:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить места разгрузки: ', bold: 'ошибка сети', type: 'error' });
        await this.fetchEntityUnloadPlaces(this.entity.id);
      } finally {
        this.isSaving = false;
      }
    },

    cancelUnloadPlacesChanges() {
      this.selectedUnloadPlaces = [...this.originalSelectedPlaces];
    },

    toggleUnloadPlace(place) {
      if (this.selectionMode) {
        const ids = this.modelValue.includes(place.id)
          ? this.modelValue.filter(id => id !== place.id)
          : [...this.modelValue, place.id];
        this.$emit('update:modelValue', ids);
        return;
      }
      const index = this.selectedUnloadPlaces.findIndex(p => p.id === place.id);
      if (index > -1) {
        this.selectedUnloadPlaces.splice(index, 1);
      } else {
        this.selectedUnloadPlaces.push(place);
      }
    },

    isPlaceSelected(placeId) {
      if (this.selectionMode) return this.modelValue.includes(placeId);
      return this.selectedUnloadPlaces.some(p => p.id === placeId);
    }
  },
};
</script>

<style scoped>
.unload-places-section {
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

.unload-places-container {
  width: fit-content;
  background: var(--surface);
  border-radius: 15px;
  border: 1px solid var(--border);
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
}

.place-item:hover {
  background: var(--row-hover);
}

.place-item.selected {
  background: var(--accent);
  color: var(--accent-contrast);
}

.no-places-message {
  text-align: center;
  padding: 20px;
  color: var(--text-muted);
  font-style: italic;
}

@media (max-width: 768px) {
  .unload-places-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .sec-actions {
    flex-wrap: wrap;
  }
}
</style>