<template>
  <div class="unload-places-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Управление местами разгрузки</h3>
      <div class="header-controls">
        <SearchComponent
          :title="'Поиск мест разгрузки...'"
          v-model="searchQuery"
        />
        <button @click="showAddModal = true" class="add-header-button">
          Добавить
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица мест разгрузки -->
      <div class="table-section" :class="{'with-details': selectedPlace}">
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
            <div class="header-col status-col">
              <p>Статус</p>
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="place in sortedUnloadPlaces" 
              :key="place.id" 
              class="table-row"
              :class="{'selected': selectedPlace && selectedPlace.id === place.id}"
              @click="selectPlace(place)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ place.id }}</span>
              </div>
              <div class="table-col name-col">
                <span class="truncate-text" :title="place.name">
                  {{ place.name }}
                </span>
              </div>
              <div class="table-col status-col">
                <span class="status-badge" :class="getStatusClass(place)">
                  {{ getStatusText(place) }}
                </span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего мест разгрузки: {{ filteredUnloadPlaces.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали места разгрузки -->
      <div v-if="selectedPlace" class="details-section">
        <div class="details-tabs">
          <button 
            class="tab-btn" 
            :class="{ 'active': activeTab === 'main' }"
            @click="activeTab = 'main'"
          >
            Основное
          </button>
          <button 
            class="tab-btn" 
            :class="{ 'active': activeTab === 'schedule' }"
            @click="activeTab = 'schedule'"
          >
            Расписание
          </button>
          <button 
            class="tab-btn" 
            :class="{ 'active': activeTab === 'route' }"
            @click="activeTab = 'route'"
          >
            Местоположение и маршрут
          </button>
        </div>

        <!-- Вкладка Основное -->
        <div v-if="activeTab === 'main'" class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">{{ selectedPlace.name }}</h3>
              <span class="current-status-badge" :class="getCurrentStatusClass(selectedPlace)">
                {{ getCurrentStatusText(selectedPlace) }}
              </span>
            </div>
            <div class="details-header-actions">
              <button @click="confirmDeletePlace(selectedPlace)" class="delete-icon-btn">
                <img src="@/assets/icons/delete.png" class="delete-icon" />
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="detail-group">
              <label class="detail-label">Наименование:</label>
              <input 
                v-model="selectedPlace.name" 
                @change="updatePlace(selectedPlace)"
                class="form-input"
                placeholder="Введите название места"
                autocomplete="off"
              >
            </div>

            <div class="detail-group">
              <label class="detail-label">Описание:</label>
              <textarea 
                v-model="selectedPlace.description" 
                @change="updatePlace(selectedPlace)"
                class="form-textarea"
                placeholder="Введите описание"
                rows="2"
              ></textarea>
            </div>

            <!-- Статус в виде кнопок -->
            <div class="detail-group">
              <label class="detail-label">Статус:</label>
              <div class="status-toggle">
                <button 
                  class="status-btn" 
                  :class="{ 'active': selectedPlace.status === 'active' }"
                  @click="setPlaceStatus('active')"
                >
                  Активно
                </button>
                <button 
                  class="status-btn" 
                  :class="{ 'active': selectedPlace.status === 'inactive' }"
                  @click="setPlaceStatus('inactive')"
                >
                  Не активно
                </button>
              </div>
            </div>

            <!-- Комментарий к статусу (только для неактивных) -->
            <div v-if="selectedPlace.status !== 'active'" class="detail-group">
              <label class="detail-label">Причина:</label>
              <textarea 
                v-model="selectedPlace.status_comment" 
                @change="updatePlace(selectedPlace)"
                class="form-textarea"
                placeholder="Укажите причину закрытия"
                rows="2"
              ></textarea>
            </div>
          </div>
        </div>

        <!-- Вкладка Расписание -->
        <div v-if="activeTab === 'schedule'" class="tab-content">
          <ScheduleTab 
            :place-id="selectedPlace.id"
            :time-slots="selectedPlace.time_slots"
            @update="refreshSelectedPlace"
          />
        </div>

        <!-- Вкладка Маршрут -->
        <div v-if="activeTab === 'route'" class="tab-content">
          <div class="route-section">
            <h4 class="section-title">Ссылка на локацию на карте</h4>
            <div class="map-link-group">
              <input 
                v-model="selectedPlace.map_link" 
                @change="updatePlace(selectedPlace)"
                class="form-input"
                placeholder="https://maps.google.com/..."
                autocomplete="off"
              >
              <a 
                v-if="selectedPlace.map_link" 
                :href="selectedPlace.map_link" 
                target="_blank" 
                class="map-link-btn"
              >
                Открыть карту
              </a>
            </div>
          </div>

          <div class="route-section">
            <div class="photos-header">
              <h4 class="section-title">Изображение(-я)</h4>
              <label class="upload-photo-btn">
                + Загрузить
                <input 
                  type="file" 
                  accept="image/*" 
                  multiple 
                  @change="uploadPhotos"
                  style="display: none"
                >
              </label>
            </div>

            <div class="photos-grid">
              <div v-for="photo in selectedPlace.photos" :key="photo.id" class="photo-item" :class="{ 'main-photo': photo.is_main }">
                <div class="photo-preview" @click="viewPhoto(photo)">
                  <img :src="photo.photo_url" :alt="photo.file_name">
                </div>
                <div class="photo-actions">
                  <button v-if="!photo.is_main" @click="setMainPhoto(photo)" class="photo-main-btn" title="Сделать главной">
                    ★
                  </button>
                  <span v-else class="photo-main-badge" title="Главная фотография">★</span>
                  <button @click="deletePhoto(photo)" class="photo-delete-btn" title="Удалить">
                    <img src="@/assets/icons/trashcan.png" class="action-icon-small" />
                  </button>
                </div>
              </div>
              <div v-if="!selectedPlace.photos || selectedPlace.photos.length === 0" class="no-photos">
                <p>Фотографии не загружены</p>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div v-else class="no-selection-message">
        <p>Выберите место разгрузки для просмотра</p>
      </div>
    </div>

    <div v-if="filteredUnloadPlaces.length === 0" class="no-results">
      <div class="no-results-icon">📍</div>
      <p>Места разгрузки не найдены</p>
    </div>

    <!-- Модальное окно добавления места -->
    <transition name="modal-fade">
      <div v-if="showAddModal" class="modal-overlay" @click.self="closeModal">
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">Добавить место разгрузки</h3>
            <button @click="closeModal" class="modal-close">
              <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          
          <div class="modal-body">
            <div class="input-group">
              <label class="input-label">Наименование *</label>
              <input
                v-model="newPlaceName"
                placeholder="Введите название места"
                class="modal-input"
                @keyup.enter="addPlace"
                ref="nameInput"
              >
            </div>
            
            <div class="input-group">
              <label class="input-label">Описание</label>
              <textarea
                v-model="newPlaceDescription"
                placeholder="Введите описание (необязательно)"
                class="modal-textarea"
                rows="3"
              ></textarea>
            </div>
          </div>
          
          <div class="modal-footer">
            <button @click="closeModal" class="modal-btn modal-btn--cancel">Отмена</button>
            <button 
              @click="addPlace" 
              class="modal-btn modal-btn--confirm"
              :disabled="!newPlaceName.trim()"
              :class="{'modal-btn--disabled': !newPlaceName.trim()}"
            >
              Добавить
            </button>
          </div>
        </div>
      </div>
    </transition>

    <!-- Модальное окно просмотра фото -->
    <transition name="modal-fade">
      <div v-if="showPhotoModal" class="modal-overlay" @click.self="showPhotoModal = false">
        <div class="modal-content photo-view-modal">
          <div class="modal-header">
            <h3 class="modal-title">{{ viewingPhoto?.file_name }}</h3>
            <button @click="showPhotoModal = false" class="modal-close">
              <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          <div class="modal-body photo-view-body">
            <img :src="viewingPhoto?.photo_url" class="full-photo" alt="Full size">
          </div>
        </div>
      </div>
    </transition>

    <!-- Уведомления -->
    <div v-if="notification.show" class="notification" :class="notification.type">
      <span class="notification-message">{{ notification.message }}</span>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from '../RefreshButton.vue';
import SearchComponent from '../SearchComponent.vue';
import ScheduleTab from './ScheduleTab.vue';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    ScheduleTab
  },
  data() {
    return {
      searchQuery: '',
      newPlaceName: '',
      newPlaceDescription: '',
      unloadPlaces: [],
      showAddModal: false,
      showPhotoModal: false,
      selectedPlace: null,
      viewingPhoto: null,
      sortField: null,
      sortDirection: 'asc',
      activeTab: 'main',
      notification: {
        show: false,
        message: '',
        type: 'info'
      }
    };
  },
  computed: {
    filteredUnloadPlaces() {
      if (!this.searchQuery) return this.unloadPlaces;
      const query = this.searchQuery.toLowerCase();
      return this.unloadPlaces.filter(place => 
        place.name.toLowerCase().includes(query) || 
        place.id.toString().includes(query)
      );
    },
    sortedUnloadPlaces() {
      const places = [...this.filteredUnloadPlaces];
      
      if (!this.sortField) {
        return places.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return places.sort((a, b) => {
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
    
    // Слушаем события уведомлений от дочерних компонентов
    window.addEventListener('show-notification', this.handleNotification);
  },
  beforeUnmount() {
    window.removeEventListener('show-notification', this.handleNotification);
  },
  methods: {
    handleNotification(event) {
      this.showNotification(event.detail.message, event.detail.type);
    },
    
    async refreshData() {
      await this.fetchUnloadPlaces();
    },
    
    async fetchUnloadPlaces() {
      try {
        const response = await apiRequest("/unload-places", {
        });
        if (response.ok) {
          const data = await response.json();
          this.unloadPlaces = data.map(place => ({
            ...place,
            originalName: place.name,
            originalDescription: place.description,
            originalMapLink: place.map_link,
            originalStatus: place.status,
            originalStatusComment: place.status_comment
          }));
        }
      } catch (error) {
        console.error("Error fetching unload places:", error);
        this.showNotification("Ошибка при загрузке мест разгрузки", "error");
      }
    },
    
    async refreshSelectedPlace() {
  if (!this.selectedPlace) return;
  
  try {
    const response = await apiRequest(`/unload-places/${this.selectedPlace.id}`, {
    });
    if (response.ok) {
      const data = await response.json();
      
      // Исправляем URL фотографий
      if (data.photos) {
        data.photos = data.photos.map(photo => ({
          ...photo,
          photo_url: photo.photo_url
        }));
      }
      
      this.selectedPlace = {
        ...data,
        originalName: data.name,
        originalDescription: data.description,
        originalMapLink: data.map_link,
        originalStatus: data.status,
        originalStatusComment: data.status_comment
      };
      
      // Обновляем в общем списке
      const index = this.unloadPlaces.findIndex(p => p.id === data.id);
      if (index !== -1) {
        this.unloadPlaces[index] = { ...this.selectedPlace };
      }
    }
  } catch (error) {
    console.error("Error refreshing place:", error);
  }
},
    
    async addPlace() {
      if (!this.newPlaceName.trim()) {
        this.showNotification("Введите название места разгрузки", "warning");
        return;
      }
      
      try {
        const response = await apiRequest("/unload-places", {
          method: "POST",
          body: JSON.stringify({
            name: this.newPlaceName,
            description: this.newPlaceDescription || null,
            status: 'active',
            status_comment: null
          }),
        });
        
        if (response.ok) {
          const result = await response.json();
          this.newPlaceName = '';
          this.newPlaceDescription = '';
          this.showAddModal = false;
          await this.refreshData();
          
          const newPlace = this.unloadPlaces.find(p => p.id === result.id);
          if (newPlace) {
            this.selectPlace(newPlace);
          }
          
          this.showNotification("Место разгрузки успешно добавлено", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при добавлении места разгрузки", "error");
        }
      } catch (error) {
        console.error("Error adding unload place:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    async updatePlace(place) {
      const hasChanges = 
        place.name !== place.originalName ||
        place.description !== place.originalDescription ||
        place.map_link !== place.originalMapLink ||
        place.status !== place.originalStatus ||
        place.status_comment !== place.originalStatusComment;

      if (!hasChanges) return;
      
      try {
        const response = await apiRequest(`/unload-places/${place.id}`, {
          method: "PUT",
          body: JSON.stringify({
            name: place.name,
            description: place.description,
            map_link: place.map_link,
            status: place.status,
            status_comment: place.status_comment
          }),
        });
        
        if (response.ok) {
          place.originalName = place.name;
          place.originalDescription = place.description;
          place.originalMapLink = place.map_link;
          place.originalStatus = place.status;
          place.originalStatusComment = place.status_comment;
          
          const index = this.unloadPlaces.findIndex(p => p.id === place.id);
          if (index !== -1) {
            this.unloadPlaces[index] = { ...place };
          }
          
          this.showNotification("Место разгрузки успешно обновлено", "success");
        } else {
          const errorText = await response.text();
          this.revertPlaceChanges(place);
          this.showNotification(errorText || "Ошибка при обновлении места разгрузки", "error");
        }
      } catch (error) {
        console.error("Error updating unload place:", error);
        this.revertPlaceChanges(place);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    revertPlaceChanges(place) {
      place.name = place.originalName;
      place.description = place.originalDescription;
      place.map_link = place.originalMapLink;
      place.status = place.originalStatus;
      place.status_comment = place.originalStatusComment;
    },
    
    setPlaceStatus(status) {
      if (!this.selectedPlace) return;
      this.selectedPlace.status = status;
      if (status === 'active') {
        this.selectedPlace.status_comment = null;
      }
      this.updatePlace(this.selectedPlace);
    },
    
    async confirmDeletePlace(place) {
      if (!confirm(`Вы уверены, что хотите удалить место разгрузки "${place.name}"?`)) return;
      
      try {
        const response = await apiRequest(`/unload-places/${place.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          this.selectedPlace = null;
          this.activeTab = 'main';
          await this.refreshData();
          this.showNotification("Место разгрузки успешно удалено", "success");
        } else {
          const error = await response.json();
          this.showNotification(error.message || "Ошибка при удалении места разгрузки", "error");
        }
      } catch (error) {
        console.error("Error deleting unload place:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    selectPlace(place) {
      this.selectedPlace = { ...place };
      this.activeTab = 'main';
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
      this.newPlaceName = '';
      this.newPlaceDescription = '';
    },
    
    // В методе uploadPhotos, после успешной загрузки, нужно обработать photo_url
async uploadPhotos(event) {
  if (!this.selectedPlace) return;
  
  const files = event.target.files;
  if (!files || files.length === 0) return;
  
  const formData = new FormData();
  for (let i = 0; i < files.length; i++) {
    formData.append('photos', files[i]);
  }
  
  try {
    const response = await apiRequest(`/unload-places/${this.selectedPlace.id}/photos`, {
      method: "POST",
      body: formData,
      headers: {},
    });
    
    if (response.ok) {
      await this.refreshSelectedPlace();
      
      // Исправляем URL фотографий, добавляя правильный порт
      if (this.selectedPlace && this.selectedPlace.photos) {
        this.selectedPlace.photos = this.selectedPlace.photos.map(photo => ({
          ...photo,
          photo_url: photo.photo_url
        }));
      }
      
      this.showNotification("Фотографии успешно загружены", "success");
    } else {
      const errorText = await response.text();
      this.showNotification(errorText || "Ошибка при загрузке фото", "error");
    }
  } catch (error) {
    console.error("Error uploading photos:", error);
    this.showNotification("Ошибка сети", "error");
  }
},
    
    async deletePhoto(photo) {
      if (!confirm(`Удалить фотографию?`)) return;
      
      try {
        const response = await apiRequest(`/unload-places/${this.selectedPlace.id}/photos/${photo.id}`,
          {
            method: "DELETE",
          }
        );
        
        if (response.ok) {
          await this.refreshSelectedPlace();
          this.showNotification("Фотография удалена", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при удалении фото", "error");
        }
      } catch (error) {
        console.error("Error deleting photo:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    async setMainPhoto(photo) {
      try {
        const response = await apiRequest(`/unload-places/${this.selectedPlace.id}/photos/${photo.id}/main`,
          {
            method: "POST",
          }
        );
        
        if (response.ok) {
          await this.refreshSelectedPlace();
          this.showNotification("Главная фотография установлена", "success");
        } else {
          const errorText = await response.text();
          this.showNotification(errorText || "Ошибка при установке главной фотографии", "error");
        }
      } catch (error) {
        console.error("Error setting main photo:", error);
        this.showNotification("Ошибка сети", "error");
      }
    },
    
    viewPhoto(photo) {
      this.viewingPhoto = photo;
      this.showPhotoModal = true;
    },
    
    // Вспомогательные методы
    getStatusClass(place) {
      if (place.status !== 'active') {
        return 'status-inactive';
      }
      return place.current_status === 'open' ? 'status-open' : 'status-closed';
    },
    
    getStatusText(place) {
      if (place.status !== 'active') {
        return 'Неактивно';
      }
      return place.current_status === 'open' ? 'Открыто' : 'Закрыто';
    },
    
    getCurrentStatusClass(place) {
      if (place.status !== 'active') {
        return 'status-inactive-badge';
      }
      return place.current_status === 'open' ? 'status-open-badge' : 'status-closed-badge';
    },
    
    getCurrentStatusText(place) {
      if (place.status !== 'active') {
        return 'Неактивно';
      }
      return place.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
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
.unload-places-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 550px;
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
  height: 500px;
  width: 100%;
}

/* Левая часть - таблица */
.table-section {
  width: 35%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
  background: #fff;
}

.table-section.with-details {
  width: 35%;
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
  width: 55%;
  min-width: 150px;
}

.status-col {
  width: 25%;
  min-width: 80px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 407px;
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

.status-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 500;
  min-width: 70px;
  text-align: center;
}

.status-open {
  background-color: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.status-closed {
  background-color: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffcc80;
}

.status-inactive {
  background-color: #ffebee;
  color: #c62828;
  border: 1px solid #ef9a9a;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: right;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

/* Правая часть - детали */
.details-section {
  width: 65%;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
}

.details-tabs {
  display: flex;
  border-bottom: 1px solid #e6e6e6;
  background: #f8f9fa;
  padding: 0 20px;
}

.tab-btn {
  padding: 12px 24px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  color: #666;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  color: #4F5BDF;
}

.tab-btn.active {
  color: #4F5BDF;
  border-bottom-color: #4F5BDF;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fff;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.current-status-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.status-open-badge {
  background-color: #e6f7e6;
  color: #2e7d32;
  border: 1px solid #a5d6a7;
}

.status-closed-badge {
  background-color: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffcc80;
}

.status-inactive-badge {
  background-color: #ffebee;
  color: #c62828;
  border: 1px solid #ef9a9a;
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

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight:400;
}

.form-input {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 14px;
  width: 100%;
  transition: border-color 0.2s ease;
  background: #fff;
}

.form-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-textarea {
  padding: 8px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 14px;
  width: 100%;
  transition: border-color 0.2s ease;
  background: #fff;
  resize: vertical;
  font-family: inherit;
}

.form-textarea:focus {
  border-color: #4F5BDF;
  outline: none;
}

/* Статус в виде кнопок */
.status-toggle {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.status-btn {
  padding: 6px 16px;
  border: 1px solid #e6e6e6;
  background: #fff;
  border-radius: 30px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
  color: #666;
}

.status-btn:hover {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.status-btn.active {
  background: #4F5BDF;
  border-color: #4F5BDF;
  color: white;
}

/* Стили для маршрута */
.route-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 12px 0;
  font-size: 1em;
  font-weight: 600;
  color: #333;
}

.map-link-group {
  display: flex;
  gap: 12px;
  align-items: center;
}

.map-link-group .form-input {
  flex: 1;
}

.map-link-btn {
  padding: 8px 16px;
  background: #f0f3ff;
  color: #4F5BDF;
  text-decoration: none;
  border-radius: 30px;
  font-size: 13px;
  white-space: nowrap;
  transition: background-color 0.2s ease;
  border: 1px solid #4F5BDF;
}

.map-link-btn:hover {
  background: #4F5BDF;
  color: white;
}

.photos-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.upload-photo-btn {
  padding: 4px 12px;
  background: #f0f3ff;
  color: #4F5BDF;
  border: 1px solid #4F5BDF;
  border-radius: 20px;
  cursor: pointer;
  font-size: 12px;
  transition: background-color 0.2s ease;
}

.upload-photo-btn:hover {
  background: #4F5BDF;
  color: white;
}

.photos-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 12px;
  max-height: 250px;
  overflow-y: auto;
  padding: 4px;
}

.photo-item {
  position: relative;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  overflow: hidden;
  aspect-ratio: 1;
  background: #f8f9fa;
  
}

.photo-item.main-photo {
  border: 2px solid #4F5BDF;
}

.photo-preview {
  width: 100%;
  height: 100%;
  cursor: pointer;
}

.photo-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.photo-actions {
  position: absolute;
  top: 4px;
  right: 4px;
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.photo-item:hover .photo-actions {
  opacity: 1;
}

.photo-main-btn,
.photo-delete-btn,
.photo-main-badge {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  transition: background-color 0.2s ease;
  background: rgba(255, 255, 255, 0.9);
}

.photo-main-btn:hover {
  background: #4F5BDF;
  color: white;
}

.photo-main-badge {
  background: #4F5BDF;
  color: white;
  cursor: default;
}

.photo-delete-btn:hover {
  background: #c62828;
}

.photo-delete-btn:hover .action-icon-small {
  filter: brightness(0) invert(1);
}

.action-icon-small {
  width: 14px;
  height: 14px;
}

.no-photos {
  grid-column: 1 / -1;
  text-align: center;
  padding: 20px;
  color: #a2a2a2;
  background: #f8f9fa;
  border: 1px dashed #e6e6e6;
  border-radius: 8px;
}

.no-selection-message {
  width: 65%;
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

/* Стили для модальных окон */
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
  z-index: 10000;
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

.modal-content.photo-view-modal {
  width: 800px;
  max-width: 90vw;
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
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-body {
  padding: 20px 24px;
  max-height: 60vh;
  overflow-y: auto;
}

.photo-view-body {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  background: #f0f0f0;
}

.full-photo {
  max-width: 100%;
  max-height: 70vh;
  object-fit: contain;
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

.modal-input,
.modal-textarea {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  font-size: 0.9em;
  transition: border-color 0.2s ease;
  background: #fff;
  font-family: inherit;
}

.modal-input:focus,
.modal-textarea:focus {
  border-color: #4F5BDF;
  outline: none;
}

.modal-textarea {
  resize: vertical;
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
  transition: background-color 0.2s ease;
  min-width: 80px;
}

.modal-btn--cancel {
  background: #f8f9fa;
  color: #666;
  border: 1px solid #e0e0e0;
}

.modal-btn--cancel:hover {
  background: #e9ecef;
}

.modal-btn--confirm {
  background: #4F5BDF;
  color: white;
}

.modal-btn--confirm:hover:not(.modal-btn--disabled) {
  background: #3a45b2;
}

.modal-btn--disabled {
  background: #ccc;
  cursor: not-allowed;
}

/* Анимации */
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

/* Скроллбары */
.table-body::-webkit-scrollbar,
.photos-grid::-webkit-scrollbar,
.modal-body::-webkit-scrollbar {
  width: 6px;
}

.table-body::-webkit-scrollbar-track,
.photos-grid::-webkit-scrollbar-track,
.modal-body::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb,
.photos-grid::-webkit-scrollbar-thumb,
.modal-body::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb:hover,
.photos-grid::-webkit-scrollbar-thumb:hover,
.modal-body::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
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
  
  .details-section {
    height: 400px;
  }
  
  .details-title-wrapper {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .map-link-group {
    flex-direction: column;
  }
  
  .map-link-btn {
    width: 100%;
    text-align: center;
  }
  
  .modal-content {
    width: 95%;
    max-height: 80vh;
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
  
  .status-col {
    width: 30%;
  }
}
</style>