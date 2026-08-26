<template>
  <div
    class="modal-content-inner"
    :class="{ 'is-dragging': sheetDragging }"
    :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
    @touchstart="onSheetTouchStart"
    @touchmove="onSheetTouchMove"
    @touchend="onSheetTouchEnd"
  >
    <div
      class="sheet-handle"
      aria-hidden="true"
    />
    <div class="modal-header">
      <div class="header-with-status">
        <h3 class="modal-title">
          Информация о месте разгрузки
        </h3>
        <span
          class="status-badge"
          :class="getPlaceStatusClass(place)"
        >
          {{ getPlaceStatusText(place) }}
        </span>
        <!-- Информация о времени до открытия/закрытия -->
        <div
          v-if="place && place.status === 'active'"
          class="time-info"
        >
          {{ getTimeInfoText() }}
        </div>
      </div>
      <button
        class="modal-close"
        @click="close"
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

    <div
      ref="modalBody"
      class="modal-body"
    >
      <div
        v-if="place"
        class="place-details"
      >
        <!-- Секция Основная информация (без статуса) -->
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
                <span class="info-value">{{ place.name }}</span>
              </div>
            </div>
            <div
              v-if="place.status !== 'active' && place.status_comment"
              class="comment-text"
            >
              {{ place.status_comment }}
            </div>
          </div>
        </div>

        <!-- Секция Режим работы -->
        <div class="details-section">
          <div class="section-header">
            <h4 class="section-title">
              Режим работы
            </h4>
          </div>
          <div class="section-body">
            <div
              v-if="hasTimeSlots(place)"
              class="schedule-grid"
            >
              <div 
                v-for="day in daysWithSlots(place)" 
                :key="day" 
                class="schedule-day-card"
                :class="{ 'current-day': isCurrentDay(day) }"
              >
                <div class="day-name">
                  {{ getFullDayName(day) }}
                </div>
                <div class="day-slots">
                  <div 
                    v-for="slot in getSlotsForDay(place.time_slots, day)" 
                    :key="slot.id" 
                    class="slot-badge"
                    :class="{ 
                      'active-slot': isCurrentDay(day) && isActiveSlot(slot)
                    }"
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

        <!-- Секция Местоположение -->
        <div class="details-section">
          <div class="section-header location-header">
            <h4 class="section-title">
              Местоположение
            </h4>
            <a 
              v-if="place.map_link" 
              :href="place.map_link" 
              target="_blank" 
              class="map-link-btn"
            >
              Как добраться?
            </a>
          </div>
          <div class="section-body photo-body">
            <div
              v-if="place.photos && place.photos.length > 0"
              class="photo-container"
            >
              <div 
                ref="photoWrapper"
                class="photo-wrapper"
                @pointerdown="startDrag"
                @wheel="onZoom"
                @touchstart.stop
              >
                <img 
                  :src="getMainPhotoUrl(place.photos)" 
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
</template>

<script>
import { ref } from 'vue';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';

export default {
    name: 'UnloadPlaceModal',
    props: {
        place: {
            type: Object,
            default: null
        },
        allUnloadingPlaces: {
            type: Array,
            default: () => []
        }
    },
    emits: ['close'],
    setup(props, { emit }) {
        // Свайп-вниз закрывает окно места (эмитит close родителю). Тянуть можно и за
        // ползунок, и за ЛЮБУЮ часть модалки - для этого getScrollTop отдаёт реальную
        // прокрутку тела: из контента свайп стартует, когда тело прокручено вверх
        // (иначе жест уходит в скролл списка). Исключение - фото местоположения: на нём
        // висит собственный pan (Pointer Events), поэтому .photo-wrapper гасит всплытие
        // touchstart (@touchstart.stop в шаблоне), чтобы жесты не конфликтовали.
        const modalBody = ref(null);
        const swipe = useSwipeDismiss(() => emit('close'), {
            handleSelector: '.sheet-handle',
            getScrollTop: () => modalBody.value?.scrollTop ?? 0,
        });
        return {
            modalBody,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
        };
    },
    data() {
        return {
            photoScale: 1.5,
            photoTranslateX: 0,
            photoTranslateY: 0,
            isDragging: false,
            dragPointerId: null,
            dragStartX: 0,
            dragStartY: 0,
            initialTranslateX: 0,
            initialTranslateY: 0,
            imageWidth: 0,
            imageHeight: 0,
            containerWidth: 0,
            containerHeight: 0,
            fullDayNames: ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'],
            currentTime: new Date(),
            windowPointerMoveHandler: null,
            windowPointerUpHandler: null
        }
    },
    computed: {
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
    watch: {
        place(newVal) {
            if (newVal) {
                this.$nextTick(() => {
                    this.updateContainerDimensions();
                });
                this.resetPhoto();
            }
        }
    },
    mounted() {
        // Свой Escape: без него жест закрывал сразу и карточку-родителя, тогда как
        // крестик, затемнение и свайп закрывают только это окно.
        document.addEventListener('keydown', this.handleKeydown, true);
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.handleKeydown, true);
        if (this.windowPointerMoveHandler) {
            window.removeEventListener('pointermove', this.windowPointerMoveHandler);
            window.removeEventListener('pointerup', this.windowPointerUpHandler);
            window.removeEventListener('pointercancel', this.windowPointerUpHandler);
        }
    },
    methods: {
        handleKeydown(e) {
            if (e.key !== 'Escape') return;
            // Гасим всплытие: иначе тот же Escape поймает карточка-родитель и закроет
            // сразу два уровня.
            e.stopPropagation();
            this.$emit('close');
        },

        close() {
            this.$emit('close');
        },

        getPlaceStatusClass(place) {
            if (place.status !== 'active') {
                return 'status-inactive';
            }
            return place.current_status === 'open' ? 'status-open' : 'status-closed';
        },

        getPlaceStatusText(place) {
            if (place.status !== 'active') {
                if (place.status === 'maintenance') return 'На обслуживании';
                return 'Неактивно';
            }
            return place.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
        },

        getMainPhotoUrl(photos) {
            if (!photos || photos.length === 0) return null;
            const mainPhoto = photos.find(p => p.is_main) || photos[0];
            return mainPhoto.photo_url;
        },

        hasTimeSlots(place) {
            return place.time_slots && place.time_slots.length > 0;
        },

        getSlotsForDay(slots, day) {
            return slots.filter(slot => slot.day_of_week === day);
        },

        daysWithSlots(place) {
            if (!place.time_slots) return [];
            const daysWithSlots = new Set(place.time_slots.map(s => s.day_of_week));
            return Array.from(daysWithSlots).sort((a, b) => a - b);
        },

        getFullDayName(dayIndex) {
            return this.fullDayNames[dayIndex] || 'Неизвестно';
        },

        formatTime(timeStr) {
            if (!timeStr) return '';
            return timeStr.substring(0, 5);
        },

        isRoundTheClockSlot(slot) {
            return slot.open_time.slice(0,5) === '00:00' && slot.close_time.slice(0,5) === '23:59' && !slot.is_next_day;
        },

        isCurrentDay(day) {
            return day === this.currentDay;
        },

        isActiveSlot(slot) {
            if (!slot.is_active || this.isRoundTheClockSlot(slot)) return false;
            
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
            const [hours, minutes] = timeStr.split(':').map(Number);
            return hours * 60 + minutes;
        },

        getTimeInfoText() {
            if (!this.place || this.place.status !== 'active') return '';
            
            const now = new Date();
            const currentTime = now.getHours() * 60 + now.getMinutes();
            const currentDay = this.currentDay;
            
            const todaySlots = this.getSlotsForDay(this.place.time_slots, currentDay)
                .filter(slot => slot.is_active);
            
            const roundTheClockSlot = todaySlots.find(slot => 
                slot.open_time.slice(0,5) === '00:00' && slot.close_time.slice(0,5) === '23:59' && !slot.is_next_day
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
                
                if (nextSlot === null) {
                    for (let daysAhead = 1; daysAhead <= 7; daysAhead++) {
                        const nextDay = (currentDay + daysAhead) % 7;
                        const nextDaySlots = this.getSlotsForDay(this.place.time_slots, nextDay)
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
            if (this.isDragging) return;
            event.preventDefault();
            this.isDragging = true;
            // Pointer Events (не mouse) - drag работает и мышью, и пальцем: у PointerEvent
            // есть clientX/clientY и для тача. touch-action:none на .photo-wrapper не даёт
            // браузеру забрать жест под скролл/зум страницы.
            // Запоминаем pointerId ведущего пальца: window-листенеры реагируют на ЛЮБОЙ
            // поинтер, а поверх фото есть кнопки зума - тап вторым пальцем по ним не должен
            // оборвать драг первого.
            this.dragPointerId = event.pointerId;
            this.dragStartX = event.clientX;
            this.dragStartY = event.clientY;
            this.initialTranslateX = this.photoTranslateX;
            this.initialTranslateY = this.photoTranslateY;

            this.windowPointerMoveHandler = this.onDrag;
            this.windowPointerUpHandler = this.stopDrag;
            window.addEventListener('pointermove', this.windowPointerMoveHandler);
            window.addEventListener('pointerup', this.windowPointerUpHandler);
            window.addEventListener('pointercancel', this.windowPointerUpHandler);
        },

        onDrag(event) {
            if (!this.isDragging) return;
            if (event.pointerId !== this.dragPointerId) return;
            const deltaX = event.clientX - this.dragStartX;
            const deltaY = event.clientY - this.dragStartY;
            this.photoTranslateX = this.initialTranslateX + deltaX;
            this.photoTranslateY = this.initialTranslateY + deltaY;
            this.clampTranslate();
        },

        stopDrag(event) {
            // Игнорируем up/cancel НЕ ведущего пальца (второй палец на кнопках зума).
            if (event && event.pointerId != null && event.pointerId !== this.dragPointerId) return;
            if (this.windowPointerMoveHandler) {
                window.removeEventListener('pointermove', this.windowPointerMoveHandler);
                window.removeEventListener('pointerup', this.windowPointerUpHandler);
                window.removeEventListener('pointercancel', this.windowPointerUpHandler);
                this.windowPointerMoveHandler = null;
                this.windowPointerUpHandler = null;
            }
            this.dragPointerId = null;
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
        }
    }
}
</script>

<style scoped>
.modal-content-inner {
    width: 100%;
    height: 450px;
    display: flex;
    flex-direction: column;
    background: var(--surface);
    overflow: hidden;
    border-radius: 50px;
}

/* Ползунок bottom-sheet - виден только на мобилке. */
.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 10px auto 2px;
    flex-shrink: 0;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 30px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
}

.header-with-status {
    display: flex;
    align-items: center;
    gap: 12px;
    row-gap: 0px;
    flex-wrap: wrap;
}

.modal-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
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

.time-info {
    font-size: 13px;
    color: var(--text-muted);
}

.modal-body {   
    padding: 20px 30px;
    overflow-y: auto;
    flex: 1;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-body::-webkit-scrollbar {
    display: none;
}

.place-details {
    display: flex;
    flex-direction: column;
    gap: 16px;
}

.details-section {
    border: 1px solid var(--border);
    border-radius: 20px;
    background: var(--surface-2);
    overflow: hidden;
}

.section-header {
    padding: 12px 20px;
    border-bottom: 1px solid var(--border);
}

.location-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.section-title {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
}

.section-body {
    padding: 16px 20px;
}

.photo-body {
    padding: 0;
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

.info-label {
    font-size: 13px;
    color: var(--text-muted);
    font-weight: 400;
}

.info-value {
    font-size: 13px;
    color: var(--text);
    font-weight: 500;
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

.status-badge {
    width: 120px;
    height: 25px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 30px;
    font-size: 12px;
    font-weight: 500;
}

.status-open {
    background-color: var(--success-bg);
    color: var(--success-text);
    border: 1px solid var(--success);
}

.status-closed {
    background-color: var(--warning-bg);
    color: var(--warning-text);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-inactive {
    background-color: var(--danger-bg);
    color: var(--danger-text);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
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
    /* Тач-жест на фото - наш drag/pan, а не скролл/зум страницы (нужно для Pointer Events). */
    touch-action: none;
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

@media (max-width: 768px) {
    /* Bottom-sheet: во всю ширину снизу, скруглены только верхние углы (#1097 R4-10). */
    .modal-content-inner {
        height: auto;
        max-height: 90dvh;
        border-radius: 16px 16px 0 0;
        /* Плавный снап-назад после свайпа-вниз; во время перетаскивания (is-dragging)
           отключаем, чтобы лист следовал за пальцем 1:1 (как main-modal в VehicleDetailsModal). */
        transition: transform 0.3s ease;
    }
    .modal-content-inner.is-dragging {
        transition: none;
    }

    .sheet-handle {
        display: block;
    }

    .modal-header {
        padding: 6px 16px;
    }
    
    .modal-body {
        padding: 16px 20px;
    }
    
    .details-section {
        border-radius: 16px;
    }
    
    .section-header {
        padding: 10px 16px;
    }
    
    .section-body {
        padding: 14px 16px;
    }
    
    .section-title {
        font-size: 13px;
    }
    
    .header-with-status {
        flex-direction: column;
        align-items: flex-start;
        gap: 8px;
    }
    
    .info-grid {
        flex-direction: column;
        gap: 12px;
    }
    
    .location-header {
        flex-direction: column;
        align-items: flex-start;
        gap: 8px;
    }
    
    .schedule-grid {
        grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    }
}
</style>