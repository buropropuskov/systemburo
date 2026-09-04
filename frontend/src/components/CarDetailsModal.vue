<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      @mousedown="onOverlayMousedown"
      @mouseup="onOverlayMouseup"
    >
      <div class="modal-wrapper">
        <!-- Основное модальное окно с деталями автомобиля -->
        <div
          class="car-details-modal main-modal"
          :class="{ 'shifted': isMainShifted }"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>Детали автомобиля</h3>
            <div class="header-actions">
              <button
                class="history-btn"
                @click="openCarHistory"
              >
                <span>История автомобиля</span>
              </button>
              <button
                class="application-btn"
                @click="openApplication"
              >
                <span>Открыть заявку</span>
              </button>
              <button
                class="close-btn"
                @click="close"
              >
                ×
              </button>
            </div>
          </div>

          <div class="modal-content">
            <div class="car-info-section">
              <h4>Информация об автомобиле</h4>
              <div class="info-grid two-columns">
                <div class="info-item">
                  <span class="info-label">Номер машины:</span>
                  <span class="info-value">{{ car.car_number || 'Не указан' }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">Марка:</span>
                  <span class="info-value">{{ car.car_brand || 'Не указана' }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">Организация:</span>
                  <span class="info-value">{{ car.organization_name || car.organization || 'Не указана' }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">Компания:</span>
                  <span class="info-value">{{ car.company || 'Не указана' }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">Действует до:</span>
                  <span class="info-value">{{ formatDate(car.entry_date_to) }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">Время пребывания:</span>
                  <span class="info-value">{{ formatTimeRange(car.entry_time_from, car.entry_time_to) }}</span>
                </div>
              </div>
            
              <div class="places-section">
                <h5>Места разгрузки</h5>
                <div class="places-list">
                  <div 
                    v-for="placeId in car.unload_place_ids" 
                    :key="placeId"
                    class="place-item"
                    @click="showUnloadPlaceDetails(placeId)"
                  >
                    {{ getPlaceName(placeId) }}
                  </div>
                  <div
                    v-if="!car.unload_place_ids || car.unload_place_ids.length === 0"
                    class="no-places"
                  >
                    Места разгрузки не указаны
                  </div>
                </div>
              </div>

              <div class="status-section">
                <h5>Статус</h5>
                <div
                  class="status-badge"
                  :class="getStatusClass"
                >
                  {{ getStatusText }}
                </div>
              </div>
            </div>

            <div class="history-section">
              <div class="section-header">
                <h4>История въездов и выездов</h4>
                <button
                  class="export-btn"
                  :disabled="history.length === 0 || isExporting"
                  @click="exportHistory"
                >
                  <AppIcon
                    v-if="!isExporting"
                    name="export"
                    class="export-icon"
                  />
                  <span v-if="!isExporting">Экспорт</span>
                  <div
                    v-else
                    class="export-loader"
                  />
                </button>
              </div>
            
              <div
                v-if="loadingHistory"
                class="loading-container"
              >
                <LoaderSpinner label="Загрузка истории…" />
              </div>
            
              <div
                v-else-if="history.length === 0"
                class="no-history"
              >
                История отсутствует
              </div>
            
              <div
                v-else
                class="history-timeline"
              >
                <div 
                  v-for="(item, index) in history" 
                  :key="item.id" 
                  class="history-item"
                >
                  <div
                    class="timeline-dot"
                    :class="getActionClass(item.action_type)"
                  />
                  <div
                    v-if="index < history.length - 1"
                    class="timeline-line"
                  />
                
                  <div class="history-content">
                    <div class="history-header">
                      <span class="user-name">{{ item.user_name || 'Система' }}</span>
                      <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                    </div>
                  
                    <div class="action-text">
                      {{ getActionText(item) }}
                    </div>
                  
                    <div
                      v-if="item.comment"
                      class="action-comment"
                    >
                      {{ item.comment }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Дополнительное модальное окно с деталями места разгрузки -->
        <transition 
          name="place-slide"
          @after-leave="onPlaceLeave"
        >
          <div
            v-if="showPlaceModal"
            class="modal-content place-modal"
          >
            <div class="modal-header">
              <div class="header-with-status">
                <h3 class="modal-title">
                  Информация о месте разгрузки
                </h3>
                <span
                  class="status-badge"
                  :class="getPlaceStatusClass(selectedPlace)"
                >
                  {{ getPlaceStatusText(selectedPlace) }}
                </span>
                <div
                  v-if="selectedPlace && selectedPlace.status === 'active'"
                  class="time-info"
                >
                  {{ getTimeInfoText() }}
                </div>
              </div>
              <button
                class="modal-close"
                @click="closeUnloadPlaceDetails"
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
              <div
                v-if="selectedPlace"
                class="place-details"
              >
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Основная информация
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="info-grid">
                      <div class="info-row">
                        <span class="info-label">Наименование:</span>
                        <span class="info-value">{{ selectedPlace.name }}</span>
                      </div>
                    </div>
                    <div
                      v-if="selectedPlace.status !== 'active' && selectedPlace.status_comment"
                      class="comment-text"
                    >
                      {{ selectedPlace.status_comment }}
                    </div>
                  </div>
                </div>

                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Режим работы
                    </h4>
                  </div>
                  <div class="section-body">
                    <div
                      v-if="hasTimeSlots(selectedPlace)"
                      class="schedule-grid"
                    >
                      <div 
                        v-for="day in daysWithSlots(selectedPlace)" 
                        :key="day" 
                        class="schedule-day-card"
                        :class="{ 'current-day': isCurrentDay(day) }"
                      >
                        <div class="day-name">
                          {{ getFullDayName(day) }}
                        </div>
                        <div class="day-slots">
                          <div 
                            v-for="slot in getSlotsForDay(selectedPlace.time_slots, day)" 
                            :key="slot.id" 
                            class="slot-badge"
                            :class="{ 'active-slot': isCurrentDay(day) && isActiveSlot(slot) }"
                          >
                            <span
                              v-if="isRoundTheClockSlot(slot)"
                              class="round-clock-text"
                            >круглосуточно</span>
                            <template v-else>
                              <span class="slot-time">
                                {{ formatTime(slot.open_time) }} – {{ formatTime(slot.close_time) }}
                              </span>
                              <div class="slot-badges">
                                <span
                                  v-if="slot.is_next_day"
                                  class="next-day-badge"
                                >+1</span>
                                <span
                                  v-if="!slot.is_active"
                                  class="inactive-badge"
                                >неакт</span>
                              </div>
                            </template>
                          </div>
                        </div>
                      </div>
                    </div>
                    <div
                      v-else
                      class="no-schedule"
                    >
                      Режим работы не указан
                    </div>
                  </div>
                </div>

                <div class="details-section">
                  <div class="section-header location-header">
                    <h4 class="section-title">
                      Местоположение
                    </h4>
                    <a 
                      v-if="selectedPlace.map_link" 
                      :href="selectedPlace.map_link" 
                      target="_blank" 
                      class="map-link-btn"
                    >
                      Как добраться?
                    </a>
                  </div>
                  <div class="section-body photo-body">
                    <div
                      v-if="selectedPlace.photos && selectedPlace.photos.length > 0"
                      class="photo-container"
                    >
                      <div 
                        ref="photoWrapper" 
                        class="photo-wrapper"
                        @mousedown="startDrag"
                        @mousemove="onDrag"
                        @mouseup="stopDrag"
                        @mouseleave="stopDrag"
                        @wheel="onZoom"
                      >
                        <img 
                          :src="getMainPhotoUrl(selectedPlace.photos)" 
                          alt="Место разгрузки"
                          class="place-photo"
                          :style="photoStyle"
                          draggable="false"
                          @load="updateImageDimensions"
                        >
                      </div>
                      <div class="photo-controls">
                        <button
                          class="photo-control-btn"
                          @click="zoomIn"
                        >
                          +
                        </button>
                        <button
                          class="photo-control-btn"
                          @click="zoomOut"
                        >
                          −
                        </button>
                        <button
                          class="photo-control-btn"
                          @click="resetPhoto"
                        >
                          ↺
                        </button>
                      </div>
                    </div>
                    <div
                      v-else
                      class="no-photo-placeholder"
                    >
                      Нет фотографии
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </transition>
      </div>

      <CarHistoryModal
        v-if="showCarHistory"
        :car-id="car.id"
        :car-number="car.car_number || 'по факту'"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        @close="showCarHistory = false"
      />
    </div>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client'
import CarHistoryModal from './CarHistoryModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useDeletionsStore } from '@/stores/deletions';
import ExcelJS from 'exceljs';
import AppIcon from '@/components/icons/AppIcon.vue';
import { formatMoscowDateTime } from '@/utils/serverTime';

export default {
    name: 'CarDetailsModal',
    components: { AppIcon, LoaderSpinner, CarHistoryModal },
  props: {
    car: {
      type: Object,
      required: true
    },
    currentUserId: {
      type: Number,
      default: null
    },
    currentUserName: {
      type: String,
      default: ''
    },
    allUnloadingPlaces: {
      type: Array,
      default: () => []
    }
  },
    emits: ['close'],
    setup(_, { emit }) {
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
        return { onOverlayMousedown, onOverlayMouseup };
    },
  data() {
    return {
      history: [],
      loadingHistory: false,
      showCarHistory: false,
      showPlaceModal: false,
      selectedPlace: null,
      isMainShifted: false,
      shiftTimer: null,
      isExporting: false,
      photoScale: 1.5,
      photoTranslateX: 0,
      photoTranslateY: 0,
      isDragging: false,
      dragPerformed: false,
      lastDragX: 0,
      lastDragY: 0,
      imageWidth: 0,
      imageHeight: 0,
      containerWidth: 0,
      containerHeight: 0,
      fullDayNames: ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'],
      currentTime: new Date()
    };
  },
  computed: {
    getStatusClass() {
      if (this.car.entry_checked && !this.car.exit_checked) return 'status-on-territory';
      if (this.car.exit_checked) return 'status-exited';
      return 'status-not-entered';
    },
    getStatusText() {
      if (this.car.entry_checked && !this.car.exit_checked) return 'На территории';
      if (this.car.exit_checked) return 'Выехал';
      return 'Не въезжал';
    },
    photoStyle() {
      return {
        transform: `translate(${this.photoTranslateX}px, ${this.photoTranslateY}px) scale(${this.photoScale})`,
        cursor: this.isDragging ? 'grabbing' : 'grab'
      };
    },
    currentDay() {
      return new Date().getDay() === 0 ? 6 : new Date().getDay() - 1;
    }
  },
  mounted() {
    this.loadCarHistory();
  },
  beforeUnmount() {
    if (this.shiftTimer) {
      clearTimeout(this.shiftTimer);
    }
  },
  methods: {
    formatDate(dateString) {
      if (!dateString) return '';
      try {
        const [year, month, day] = dateString.split('-');
        const date = new Date(year, month - 1, day);
        return date.toLocaleDateString('ru-RU');
      } catch {
        return '';
      }
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.split(':');
        if (parts.length >= 2) {
          return `${parts[0]}:${parts[1]}`;
        }
        return timeStr;
      };

      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
    },

    formatDateTime(dateTimeString) {
      if (!dateTimeString) return '';
      return formatMoscowDateTime(new Date(dateTimeString));
    },

    getActionClass(actionType) {
      return actionType === 'entry' ? 'dot-entry' : 'dot-exit';
    },

    getActionText(item) {
      return item.action_type === 'entry' ? 'Отметил о прибытии' : 'Машина уехала';
    },

    getPlaceName(placeId) {
      const place = this.allUnloadingPlaces.find(p => p.id === placeId);
      return place ? place.name : `ID: ${placeId}`;
    },

    getPlaceStatusClass(place) {
      if (!place) return 'status-inactive';
      if (place.status !== 'active') {
        return 'status-inactive';
      }
      return place.current_status === 'open' ? 'status-open' : 'status-closed';
    },

    getPlaceStatusText(place) {
      if (!place) return 'Неизвестно';
      if (place.status !== 'active') {
        if (place.status === 'maintenance') return 'На обслуживании';
        return 'Неактивно';
      }
      return place.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
    },

    getFullDayName(dayIndex) {
      return this.fullDayNames[dayIndex] || 'Неизвестно';
    },

    getSlotsForDay(slots, day) {
      if (!slots) return [];
      return slots.filter(slot => slot.day_of_week === day);
    },

    daysWithSlots(place) {
      if (!place || !place.time_slots) return [];
      const daysWithSlots = new Set(place.time_slots.map(s => s.day_of_week));
      return Array.from(daysWithSlots).sort((a, b) => a - b);
    },

    hasTimeSlots(place) {
      return place && place.time_slots && place.time_slots.length > 0;
    },

    formatTime(timeStr) {
      if (!timeStr) return '';
      return timeStr.substring(0, 5);
    },

    isRoundTheClockSlot(slot) {
      if (!slot) return false;
      return slot.open_time && slot.close_time && 
             slot.open_time.slice(0,5) === '00:00' && 
             slot.close_time.slice(0,5) === '23:59' && 
             !slot.is_next_day;
    },

    getMainPhotoUrl(photos) {
      if (!photos || photos.length === 0) return null;
      const mainPhoto = photos.find(p => p.is_main) || photos[0];
      return mainPhoto.photo_url;
    },

    isCurrentDay(day) {
      return day === this.currentDay;
    },

    isActiveSlot(slot) {
      if (!slot || !slot.is_active || this.isRoundTheClockSlot(slot)) return false;
      
      const now = new Date();
      const currentTime = now.getHours() * 60 + now.getMinutes();
      
      const open = this.parseTimeToMinutes(slot.open_time);
      const close = this.parseTimeToMinutes(slot.close_time);
      
      if (slot.is_next_day) {
        return currentTime >= open || currentTime <= close;
      } else {
        return currentTime >= open && currentTime <= close;
      }
    },

    parseTimeToMinutes(timeStr) {
      if (!timeStr) return 0;
      const [hours, minutes] = timeStr.split(':').map(Number);
      return hours * 60 + minutes;
    },

    getTimeInfoText() {
      if (!this.selectedPlace || this.selectedPlace.status !== 'active') return '';
      
      const now = new Date();
      const currentTime = now.getHours() * 60 + now.getMinutes();
      const currentDay = this.currentDay;
      
      if (!this.selectedPlace.time_slots) return '';
      
      const todaySlots = this.getSlotsForDay(this.selectedPlace.time_slots, currentDay)
        .filter(slot => slot.is_active);
      
      const roundTheClockSlot = todaySlots.find(slot => 
        slot.open_time && slot.close_time &&
        slot.open_time.slice(0,5) === '00:00' && 
        slot.close_time.slice(0,5) === '23:59' && 
        !slot.is_next_day
      );
      
      if (roundTheClockSlot) {
        return 'Открыто круглосуточно';
      }
      
      const activeSlot = todaySlots.find(slot => {
        const open = this.parseTimeToMinutes(slot.open_time);
        const close = this.parseTimeToMinutes(slot.close_time);
        
        if (slot.is_next_day) {
          return currentTime >= open || currentTime <= close;
        } else {
          return currentTime >= open && currentTime <= close;
        }
      });
      
      if (activeSlot) {
        const closeTime = this.parseTimeToMinutes(activeSlot.close_time);
        let minutesUntilClose;
        
        if (activeSlot.is_next_day) {
          if (currentTime <= closeTime) {
            minutesUntilClose = closeTime - currentTime;
          } else {
            minutesUntilClose = (24 * 60 - currentTime) + closeTime;
          }
        } else {
          minutesUntilClose = closeTime - currentTime;
        }
        
        return this.formatTimeUntil(minutesUntilClose, 'закрытия');
      } else {
        let nextSlot = null;
        let minWait = Infinity;
        
        for (const slot of todaySlots) {
          const open = this.parseTimeToMinutes(slot.open_time);
          
          if (slot.is_next_day) {
            if (currentTime < open) {
              const wait = open - currentTime;
              if (wait < minWait) {
                minWait = wait;
                nextSlot = slot;
              }
            }
          } else {
            if (currentTime < open) {
              const wait = open - currentTime;
              if (wait < minWait) {
                minWait = wait;
                nextSlot = slot;
              }
            }
          }
        }
        
        if (nextSlot === null && this.selectedPlace.time_slots) {
          for (let daysAhead = 1; daysAhead <= 7; daysAhead++) {
            const nextDay = (currentDay + daysAhead) % 7;
            const nextDaySlots = this.getSlotsForDay(this.selectedPlace.time_slots, nextDay)
              .filter(slot => slot.is_active);
            
            if (nextDaySlots.length > 0) {
              const earliestSlot = nextDaySlots.sort((a, b) => 
                this.parseTimeToMinutes(a.open_time) - this.parseTimeToMinutes(b.open_time)
              )[0];
              
              const wait = (daysAhead * 24 * 60) + this.parseTimeToMinutes(earliestSlot.open_time) - currentTime;
              return this.formatTimeUntil(wait, 'открытия', true);
            }
          }
        }
        
        if (nextSlot) {
          return this.formatTimeUntil(minWait, 'открытия');
        }
      }
      
      return '';
    },

    formatTimeUntil(minutes, action, isDays = false) {
      if (isDays) {
        const days = Math.floor(minutes / (24 * 60));
        const hours = Math.floor((minutes % (24 * 60)) / 60);
        const mins = minutes % 60;
        
        if (days > 0) {
          return `До ${action}: ${days} дн. ${hours} ч. ${mins} мин.`;
        } else if (hours > 0) {
          return `До ${action}: ${hours} ч. ${mins} мин.`;
        } else {
          return `До ${action}: ${mins} мин.`;
        }
      } else {
        const hours = Math.floor(minutes / 60);
        const mins = minutes % 60;
        
        if (hours > 0) {
          return `До ${action}: ${hours} ч. ${mins} мин.`;
        } else {
          return `До ${action}: ${mins} мин.`;
        }
      }
    },

    updateImageDimensions(event) {
      const img = event.target;
      this.imageWidth = img.naturalWidth;
      this.imageHeight = img.naturalHeight;
      this.updateContainerDimensions();
    },

    updateContainerDimensions() {
      const wrapper = this.$refs.photoWrapper;
      if (wrapper) {
        this.containerWidth = wrapper.clientWidth;
        this.containerHeight = wrapper.clientHeight;
      }
    },

    calculateMaxTranslate() {
      const containerAspect = this.containerWidth / this.containerHeight;
      const imageAspect = this.imageWidth / this.imageHeight;

      let displayWidth, displayHeight;

      if (imageAspect > containerAspect) {
        displayWidth = this.containerWidth;
        displayHeight = this.containerWidth / imageAspect;
      } else {
        displayHeight = this.containerHeight;
        displayWidth = this.containerHeight * imageAspect;
      }

      displayWidth *= this.photoScale;
      displayHeight *= this.photoScale;

      const maxX = Math.max(0, (displayWidth - this.containerWidth) / 2);
      const maxY = Math.max(0, (displayHeight - this.containerHeight) / 2);

      return { maxX, maxY };
    },

    clampTranslate() {
      const { maxX, maxY } = this.calculateMaxTranslate();
      this.photoTranslateX = Math.max(-maxX, Math.min(maxX, this.photoTranslateX));
      this.photoTranslateY = Math.max(-maxY, Math.min(maxY, this.photoTranslateY));
    },

    startDrag(event) {
      this.isDragging = true;
      this.dragPerformed = false;
      this.lastDragX = event.clientX;
      this.lastDragY = event.clientY;
      event.preventDefault();
    },

    onDrag(event) {
      if (this.isDragging) {
        if (!this.dragPerformed) {
          this.dragPerformed = true;
        }

        const deltaX = event.clientX - this.lastDragX;
        const deltaY = event.clientY - this.lastDragY;
        
        this.photoTranslateX += deltaX;
        this.photoTranslateY += deltaY;
        
        this.clampTranslate();
        
        this.lastDragX = event.clientX;
        this.lastDragY = event.clientY;
        event.preventDefault();
      }
    },

    stopDrag() {
      this.isDragging = false;
    },

    onZoom(event) {
      event.preventDefault();
      const delta = event.deltaY > 0 ? -0.1 : 0.1;
      const newScale = Math.max(1.5, Math.min(3, this.photoScale + delta));
      this.photoScale = newScale;
      this.clampTranslate();
    },

    zoomIn() {
      this.photoScale = Math.min(3, this.photoScale + 0.2);
      this.clampTranslate();
    },

    zoomOut() {
      this.photoScale = Math.max(1.5, this.photoScale - 0.2);
      this.clampTranslate();
    },

    resetPhoto() {
      this.photoScale = 1.5;
      this.photoTranslateX = 0;
      this.photoTranslateY = 0;
    },

    async loadCarHistory() {
      this.loadingHistory = true;
      try {
        const response = await apiRequest(`/cars/${this.car.id}/history`, {});
        
        if (response.ok) {
          const allHistory = await response.json();
          this.history = allHistory.filter(item => 
            item.action_type === 'entry' || item.action_type === 'exit'
          );
        }
      } catch (error) {
        console.error('Ошибка сети при загрузке истории:', error);
      } finally {
        this.loadingHistory = false;
      }
    },

    async exportHistory() {
      if (this.history.length === 0) return;
      
      this.isExporting = true;
      
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('История въездов/выездов');
        
        const headers = [
          'Дата и время',
          'Пользователь',
          'Действие',
          'Комментарий'
        ];
        
        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = {
            type: 'pattern',
            pattern: 'solid',
            fgColor: { argb: 'FF4F5BDF' }
          };
          cell.font = {
            name: 'Verdana',
            size: 11,
            bold: true,
            color: { argb: 'FFFFFFFF' }
          };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
          };
        });
        
        this.history.forEach((item, index) => {
          const row = worksheet.addRow([
            this.formatDateTime(item.created_at),
            item.user_name || 'Система',
            this.getActionText(item),
            item.comment || ''
          ]);
          
          row.height = 20;
          const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          
          row.eachCell((cell) => {
            cell.fill = {
              type: 'pattern',
              pattern: 'solid',
              fgColor: { argb: fillColor }
            };
            cell.font = {
              name: 'Verdana',
              size: 9,
              color: { argb: 'FF333333' }
            };
            cell.alignment = { vertical: 'middle' };
            cell.border = {
              top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
            };
          });
        });
        
        const lastDataRow = this.history.length;
        
        for (let row = 1; row <= lastDataRow + 1; row++) {
          const rightCell = worksheet.getCell(row, 4);
          rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
          const leftCell = worksheet.getCell(row, 1);
          leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        for (let col = 1; col <= 4; col++) {
          const topCell = worksheet.getCell(1, col);
          topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        for (let col = 1; col <= 4; col++) {
          const bottomCell = worksheet.getCell(lastDataRow + 1, col);
          bottomCell.border = { ...bottomCell.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
        }
        
        worksheet.addRow([]);
        
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserName || 'Пользователь']);
        const infoRow2 = worksheet.addRow(['Дата формирования:', formatMoscowDateTime()]);
        
        [infoRow1, infoRow2].forEach(row => {
          row.eachCell((cell) => {
            cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
            cell.border = {
              top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
              right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
            };
          });
        });
        
        worksheet.columns = [
          { width: 25 },
          { width: 40 },
          { width: 30 },
          { width: 40 }
        ];
        
        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        
        a.download = `История_автомобиля_${this.car.car_number}_${formatMoscowDateTime().replace(/[.: ]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
        
      } catch (error) {
        console.error('Error exporting to Excel:', error);
        useDeletionsStore().notify({ bold: 'Ошибка при экспорте в Excel', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },

    showUnloadPlaceDetails(placeId) {
      const place = this.allUnloadingPlaces.find(p => p.id === placeId);
      if (!place) return;

      this.selectedPlace = place;

      if (this.shiftTimer) {
        clearTimeout(this.shiftTimer);
        this.shiftTimer = null;
      }

      this.isMainShifted = true;

      this.shiftTimer = setTimeout(() => {
        this.showPlaceModal = true;
        this.resetPhoto();
        this.$nextTick(() => {
          this.updateContainerDimensions();
        });
        this.shiftTimer = null;
      }, 300);
    },

    closeUnloadPlaceDetails() {
      this.showPlaceModal = false;
    },

    onPlaceLeave() {
      this.isMainShifted = false;
      this.selectedPlace = null;
    },

    openCarHistory() {
      this.showCarHistory = true;
    },

    openApplication() {
      // Переход к заявке не реализован: кнопка сейчас ничего не делает.
    },

    close() {
      if (this.shiftTimer) {
        clearTimeout(this.shiftTimer);
      }
      this.$emit('close');
    }
  }
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 11000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.car-details-modal {
  background: var(--surface);
  border-radius: 30px;
  width: 650px;
  max-width: 95%;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px var(--shadow-drop);
  position: absolute;
}

.car-details-modal.main-modal {
  left: calc(50% - 325px);
  transition: transform 0.5s cubic-bezier(0.25, 0.1, 0.15, 1);
  transform: translateX(0);
}

.car-details-modal.main-modal.shifted {
  transform: translateX(-350px); /* Увеличил сдвиг для ширины 650px */
}

.modal-content.place-modal {
  background: var(--surface);
  border-radius: 50px;
  padding: 0;
  padding-bottom: 15px;
  width: 520px;
  height: 450px;
  max-height: 450px;
  box-shadow: 0 20px 60px var(--shadow-drop);
  display: flex;
  flex-direction: column;
  position: absolute;
  left: 50%;
}

.modal-content.place-modal .modal-body {
  overflow-y: auto;
  height: calc(450px - 70px);
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.modal-content.place-modal .modal-body::-webkit-scrollbar {
  display: none;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.header-with-status {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.history-btn, .application-btn {
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
}

.history-btn:hover, .application-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--text-muted);
  cursor: pointer;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: var(--surface-2);
  color: var(--text);
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
  background-color: var(--surface-2);
}

.modal-content {
  padding: 15px 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.car-info-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 15px;
}

.car-info-section h4 {
  margin: 0 0 12px 0;
  font-size: 15px;
  color: var(--accent-text);
  font-weight: 600;
}

.info-grid.two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 20px;
  margin-bottom: 15px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 400;
}

.info-value {
  font-size: 14px;
  color: var(--text);
  font-weight: 500;
  word-break: break-word;
}

.places-section, .status-section {
  margin-top: 10px;
}

.places-section h5, .status-section h5 {
  margin: 0 0 8px 0;
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 500;
}

.places-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.place-item {
  border: 1px solid var(--border);
  border-radius: 50px;
  padding: 4px 12px;
  font-size: 12px;
  color: var(--text);
  transition: all 0.2s ease;
  cursor: pointer;
}

.place-item:hover {
  background: var(--border);
  border-color: var(--accent);
}

.place-item.active {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.no-places {
  color: var(--text-muted);
  font-size: 12px;
  font-style: italic;
  padding: 4px 0;
}

.status-badge {
  display: inline-block;
  padding: 6px 16px;
  border-radius: 50px;
  font-size: 13px;
  font-weight: 500;
}

.status-on-territory {
  background: color-mix(in srgb, var(--accent) 10%, var(--surface));
  color: var(--accent-text);
  border: 1px solid rgba(79, 91, 223, 0.3);
}

.status-exited {
  background: color-mix(in srgb, var(--danger) 10%, var(--surface));
  color: var(--danger-text);
  border: 1px solid rgba(220, 38, 38, 0.3);
}

.status-not-entered {
  background: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.history-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 15px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.history-section h4 {
  margin: 0;
  font-size: 15px;
  color: var(--accent-text);
  font-weight: 600;
}

.export-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 28px;
}

.export-btn:hover:not(:disabled) {
  background: var(--surface-2);
  border-color: var(--accent);
}

.export-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.export-icon {
  width: 12px;
  height: 12px;
}

.export-loader {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border);
  border-top: 2px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.history-timeline {
  position: relative;
  padding-left: 20px;
}

.history-item {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  position: relative;
}

.history-item:last-child {
  margin-bottom: 0;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 4px;
  z-index: 1;
}

.timeline-line {
  position: absolute;
  left: -16px;
  top: 18px;
  width: 2px;
  height: calc(100% + 2px);
  background: var(--border);
}

.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }

.history-content {
  flex: 1;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
}

.user-name {
  font-weight: 500;
  color: var(--text);
  font-size: 13px;
}

.action-time {
  color: var(--text-muted);
  font-size: 11px;
}

.action-text {
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 4px;
  padding-left: 6px;
  border-left: 2px solid var(--border);
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 30px;
  gap: 10px;
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid var(--surface-2);
  border-top: 3px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.no-history {
  text-align: center;
  color: var(--text-muted);
  padding: 30px 20px;
  font-size: 13px;
  font-style: italic;
}

.time-info {
  font-size: 13px;
  color: var(--text-muted);
}

.place-details {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-grid {
  display: flex;
  flex-direction: row;
  gap: 30px;
}

.info-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.comment-text {
  font-size: 12px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 8px;
  padding: 8px 12px;
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  border-radius: 8px;
  border-left: 3px solid var(--warning);
}

.map-link-btn {
  padding: 4px 12px;
  background: var(--accent-tint);
  color: var(--accent-text);
  text-decoration: none;
  border-radius: 30px;
  font-size: 12px;
  font-weight: 500;
  transition: all 0.2s ease;
  border: 1px solid var(--accent);
  white-space: nowrap;
}

.map-link-btn:hover {
  background: var(--accent);
  color: var(--accent-contrast);
}

.photo-container {
  position: relative;
  width: 100%;
  height: 200px;
  overflow: hidden;
  border: 1px solid var(--border);
  background: var(--surface-2);
}

.photo-wrapper {
  width: 100%;
  height: 100%;
  overflow: hidden;
  position: relative;
  cursor: grab;
}

.photo-wrapper:active {
  cursor: grabbing;
}

.place-photo {
  width: 100%;
  height: 100%;
  object-fit: contain;
  transition: transform 0.1s ease;
  user-select: none;
  -webkit-user-drag: none;
  display: block;
}

.photo-controls {
  position: absolute;
  bottom: 10px;
  right: 10px;
  display: flex;
  gap: 5px;
  z-index: 10;
}

.photo-control-btn {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid var(--border);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: bold;
  color: var(--text);
  transition: all 0.2s ease;
}

.photo-control-btn:hover {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.no-photo-placeholder {
  width: 100%;
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 14px;
  border-radius: 20px;
  border: 1px solid var(--border);
}

.schedule-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 12px;
}

.schedule-day-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 2px 6px var(--shadow-drop);
  transition: all 0.2s ease;
}

.schedule-day-card.current-day {
  outline: 1px solid var(--accent);
}

.schedule-day-card .day-name {
  background: var(--accent-tint);
  padding: 8px 10px;
  font-weight: 600;
  color: var(--accent-text);
  font-size: 13px;
  text-align: center;
  border-bottom: 1px solid var(--border);
}

.schedule-day-card .day-slots {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.slot-badge {
  padding: 6px 8px;
  background: var(--surface-2);
  border-radius: 20px;
  border: 1px solid var(--border);
  font-size: 11px;
  display: flex;
  flex-direction: column;
  align-items: center;
  transition: all 0.2s ease;
}

.slot-badge.active-slot {
  background: var(--success-bg);
  border-color: var(--success);
}

.round-clock-text {
  font-weight: 500;
  color: var(--text);
  font-size: 11px;
}

.slot-time {
  color: var(--text);
  font-weight: 500;
}

.slot-badges {
  display: flex;
}

.next-day-badge,
.inactive-badge {
  padding: 2px 6px;
  border-radius: 20px;
  font-size: 9px;
  font-weight: 500;
}

.next-day-badge {
  background: var(--accent);
  color: var(--accent-contrast);
}

.inactive-badge {
  background: var(--danger-bg);
  color: var(--danger-text);
}

.no-schedule {
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  font-style: italic;
  padding: 16px;
  background: var(--surface-2);
  border-radius: 8px;
}

/* Анимация второго окна (выезд снизу + появление) */
.place-slide-enter-active,
.place-slide-leave-active {
  transition: transform 0.6s cubic-bezier(0.2, 0.9, 0.1, 1), opacity 0.5s ease;
}
.place-slide-enter-from {
  transform: translateY(100vh);
  opacity: 0;
}
.place-slide-enter-to {
  transform: translateY(0);
  opacity: 1;
}
.place-slide-leave-from {
  transform: translateY(0);
  opacity: 1;
}
.place-slide-leave-to {
  transform: translateY(600px);
  opacity: 0;
}

@media (max-width: 768px) {
  .modal-wrapper {
    flex-direction: column;
    gap: 10px;
  }
  
  .car-details-modal {
    position: static;
    margin-bottom: 10px;
  }
  
  .car-details-modal.main-modal {
    left: auto;
    transform: none !important;
  }
  
  .car-details-modal.main-modal.shifted {
    transform: none !important;
  }
  
  .modal-content.place-modal {
    position: static;
    left: auto;
    margin-bottom: 10px;
  }
  
  .modal-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  
  .header-actions {
    width: 100%;
    flex-wrap: wrap;
  }
  
  .info-grid.two-columns {
    grid-template-columns: 1fr;
    gap: 8px;
  }
  
  .header-with-status {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .schedule-grid {
    grid-template-columns: 1fr;
  }
}
</style>