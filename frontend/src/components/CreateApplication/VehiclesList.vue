<template>
    <div class="data__list">
        <div class="header-with-badge">
            <h4>Список транспортных средств</h4>
            <span class="vehicles-badge">{{ vehicles.length }}</span>
        </div>
        <div class="vehicles-table">
            <div class="table-header">
                <div class="header-col number-col" @click="$emit('sort', 'number')">
                    <p :class="{ 'active-sort': sortField === 'number' }">№</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'number' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col plate-col" @click="$emit('sort', 'plate')">
                    <p :class="{ 'active-sort': sortField === 'plate' }">Номер</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'plate' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col mark-col" @click="$emit('sort', 'mark')">
                    <p :class="{ 'active-sort': sortField === 'mark' }">Марка</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'mark' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col actions-col">
                    Действия
                </div>
            </div>
            <div class="table-body">
                <div 
                    v-for="(vehicle, index) in vehicles" 
                    :key="vehicle.id"
                    class="table-row"
                >
                    <div class="table-col number-col">{{ index + 1 }}</div>
                    <div class="table-col plate-col">{{ vehicle.plateNumber || 'Не указано' }}</div>
                    <div class="table-col mark-col">{{ vehicle.mark || 'Не указано' }}</div>
                    <div class="table-col actions-col">
                        <button 
                            class="details-btn"
                            @click="showVehicleDetails(vehicle)"
                            title="Детали"
                        >
                            <img 
                                src="@/assets/icons/info.png" 
                                alt="Детали" 
                                class="details-icon"
                            />
                        </button>
                        <button 
                            class="edit-btn"
                            @click="$emit('edit-vehicle', vehicle)"
                            title="Редактировать"
                        >
                            <img 
                                src="@/assets/icons/edit.png" 
                                alt="Редактировать" 
                                class="edit-icon"
                            />
                        </button>
                        <button 
                            class="delete-btn"
                            @click="$emit('delete-vehicle', vehicle.id)"
                            title="Удалить"
                        >
                            <img 
                                src="@/assets/icons/trashcan.png" 
                                alt="Удалить" 
                                class="delete-icon"
                            />
                        </button>
                    </div>
                </div>
                <div v-if="vehicles.length === 0" class="no-vehicles">
                    Нет добавленных транспортных средств
                </div>
            </div>
        </div>

        <!-- Модальное окно деталей транспортного средства -->
        <transition name="modal-fade">
            <div v-if="showDetailsModal" class="modal-overlay" @click.self="closeDetailsModal">
                <div class="modal-wrapper">
                    <!-- Основное модальное окно с деталями ТС -->
                    <div 
                        class="modal-content compact-modal main-modal"
                        :class="{ 'shifted': isMainShifted }"
                    >
                        <div class="modal-header">
                            <h3 class="modal-title">Детальная информация о Т/С</h3>
                            <button @click="closeDetailsModal" class="modal-close">
                                <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                                    <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                                </svg>
                            </button>
                        </div>
                        
                        <div class="modal-body">
                            <div class="vehicle-details" v-if="selectedVehicle">
                                <!-- Секция Основная информация -->
                                <div class="details-section">
                                    <div class="section-header">
                                        <h4 class="section-title">Основная информация</h4>
                                    </div>
                                    <div class="section-body">
                                        <div class="details-grid two-columns">
                                            <div class="detail-item">
                                                <span class="detail-label">Номер Т/С:</span>
                                                <span class="detail-value">{{ selectedVehicle.plateNumber || 'Не указано' }}</span>
                                            </div>
                                            <div class="detail-item">
                                                <span class="detail-label">Марка:</span>
                                                <span class="detail-value">{{ selectedVehicle.mark || 'Не указано' }}</span>
                                            </div>
                                            <div v-if="getFormatName(selectedVehicle.formatId)" class="detail-item full-width">
                                                <span class="detail-label">Формат номера:</span>
                                                <span class="detail-value">{{ getFormatName(selectedVehicle.formatId) }}</span>
                                            </div>
                                            <div v-if="selectedVehicle.organization" class="detail-item">
                                                <span class="detail-label">Организация:</span>
                                                <span class="detail-value">{{ selectedVehicle.organization }}</span>
                                            </div>
                                            <div v-if="selectedVehicle.company" class="detail-item">
                                                <span class="detail-label">Компания:</span>
                                                <span class="detail-value">{{ selectedVehicle.company }}</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Секция Места разгрузки -->
                                <div class="details-section">
                                    <div class="section-header">
                                        <h4 class="section-title">Места разгрузки</h4>
                                    </div>
                                    <div class="section-body">
                                        <div class="places-list">
                                            <div 
                                                v-for="placeId in selectedVehicle.unloadPlaces" 
                                                :key="placeId"
                                                class="place-item"
                                                :class="{ 'active': showPlaceModal && selectedUnloadPlace && selectedUnloadPlace.id === placeId }"
                                                @click="showUnloadPlaceDetails(placeId)"
                                            >
                                                {{ getPlaceName(placeId) }}
                                            </div>
                                            <div v-if="!selectedVehicle.unloadPlaces || selectedVehicle.unloadPlaces.length === 0" class="no-places">
                                                Места разгрузки не указаны
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Бейдж существующего Т/С -->
                                <div v-if="selectedVehicle.isExisting" class="existing-badge">
                                    <span class="badge-text">Существующее Т/С</span>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Дополнительное модальное окно с деталями места разгрузки -->
                    <transition 
                        name="place-slide"
                        @after-leave="onPlaceLeave"
                    >
                        <div v-if="showPlaceModal" class="modal-content compact-modal place-modal">
                            <div class="modal-header">
                                <div class="header-with-status">
                                    <h3 class="modal-title">Информация о месте разгрузки</h3>
                                    <span class="status-badge" :class="getPlaceStatusClass(selectedUnloadPlace)">
                                        {{ getPlaceStatusText(selectedUnloadPlace) }}
                                    </span>
                                     <!-- Информация о времени до открытия/закрытия -->
                            <div v-if="selectedUnloadPlace && selectedUnloadPlace.status === 'active'" class="time-info">
                                {{ getTimeInfoText() }}
                            </div>
                                </div>
                                <button @click="closeUnloadPlaceDetails" class="modal-close">
                                    <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                                        <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                                    </svg>
                                </button>
                            </div>
                            
                           

                            <div class="modal-body">
                                <div class="place-details" v-if="selectedUnloadPlace">
                                    <!-- Секция Основная информация (без статуса) -->
                                    <div class="details-section">
                                        <div class="section-header">
                                            <h4 class="section-title">Основная информация</h4>
                                        </div>
                                        <div class="section-body">
                                            <div class="info-grid">
                                                <div class="info-row">
                                                    <span class="info-label">Наименование:</span>
                                                    <span class="info-value">{{ selectedUnloadPlace.name }}</span>
                                                </div>
                                            </div>
                                            <div v-if="selectedUnloadPlace.status !== 'active' && selectedUnloadPlace.status_comment" class="comment-text">
                                                {{ selectedUnloadPlace.status_comment }}
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Секция Режим работы -->
                                    <div class="details-section">
                                        <div class="section-header">
                                            <h4 class="section-title">Режим работы</h4>
                                        </div>
                                        <div class="section-body">
                                            <div v-if="hasTimeSlots(selectedUnloadPlace)" class="schedule-grid">
                                                <div 
                                                    v-for="day in daysWithSlots(selectedUnloadPlace)" 
                                                    :key="day" 
                                                    class="schedule-day-card"
                                                    :class="{ 'current-day': isCurrentDay(day) }"
                                                >
                                                    <div class="day-name">{{ getFullDayName(day) }}</div>
                                                    <div class="day-slots">
                                                        <div 
                                                            v-for="slot in getSlotsForDay(selectedUnloadPlace.time_slots, day)" 
                                                            :key="slot.id" 
                                                            class="slot-badge"
                                                            :class="{ 
                                                                'active-slot': isCurrentDay(day) && isActiveSlot(slot)
                                                            }"
                                                        >
                                                            <span v-if="isRoundTheClockSlot(slot)" class="round-clock-text">круглосуточно</span>
                                                            <template v-else>
                                                                <span class="slot-time">
                                                                    {{ formatTime(slot.open_time) }} – {{ formatTime(slot.close_time) }}
                                                                </span>
                                                                <div class="slot-badges">
                                                                    <span v-if="slot.is_next_day" class="next-day-badge">+1</span>
                                                                    <span v-if="!slot.is_active" class="inactive-badge">неакт</span>
                                                                </div>
                                                            </template>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                            <div v-else class="no-schedule">
                                                Режим работы не указан
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Секция Местоположение -->
                                    <div class="details-section">
                                        <div class="section-header location-header">
                                            <h4 class="section-title">Местоположение</h4>
                                            <a 
                                                v-if="selectedUnloadPlace.map_link" 
                                                :href="selectedUnloadPlace.map_link" 
                                                target="_blank" 
                                                class="map-link-btn"
                                            >
                                                Как добраться?
                                            </a>
                                        </div>
                                        <div class="section-body photo-body">
                                            <div v-if="selectedUnloadPlace.photos && selectedUnloadPlace.photos.length > 0" class="photo-container">
                                                <div 
                                                    class="photo-wrapper" 
                                                    ref="photoWrapper"
                                                    @mousedown="startDrag"
                                                    @mousemove="onDrag"
                                                    @mouseup="stopDrag"
                                                    @mouseleave="stopDrag"
                                                    @wheel="onZoom"
                                                >
                                                    <img 
                                                        :src="getMainPhotoUrl(selectedUnloadPlace.photos)" 
                                                        alt="Место разгрузки"
                                                        class="place-photo"
                                                        :style="photoStyle"
                                                        draggable="false"
                                                        @load="updateImageDimensions"
                                                    >
                                                </div>
                                                <div class="photo-controls">
                                                    <button @click="zoomIn" class="photo-control-btn">+</button>
                                                    <button @click="zoomOut" class="photo-control-btn">−</button>
                                                    <button @click="resetPhoto" class="photo-control-btn">↺</button>
                                                </div>
                                            </div>
                                            <div v-else class="no-photo-placeholder">
                                                Нет фотографии
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </transition>
                </div>
            </div>
        </transition>
    </div>
</template>

<script>
export default {
    name: 'VehiclesList',
    props: {
        vehicles: {
            type: Array,
            required: true
        },
        sortField: String,
        sortDirection: String,
        allUnloadingPlaces: {
            type: Array,
            default: () => []
        },
        licensePlateFormats: {
            type: Array,
            default: () => []
        }
    },
    emits: ['sort', 'edit-vehicle', 'delete-vehicle'],
    data() {
        return {
            showDetailsModal: false,
            selectedVehicle: null,
            selectedUnloadPlace: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
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
            sortedDays: [0, 1, 2, 3, 4, 5, 6],
            currentTime: new Date()
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
            return new Date().getDay() === 0 ? 6 : new Date().getDay() - 1; // Преобразуем воскресенье (0) в 6
        }
    },
    methods: {
        showVehicleDetails(vehicle) {
            this.selectedVehicle = vehicle;
            this.showDetailsModal = true;
            this.showPlaceModal = false;
            this.selectedUnloadPlace = null;
            this.isMainShifted = false;
            this.resetPhoto();
        },

        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedVehicle = null;
            this.selectedUnloadPlace = null;
            this.showPlaceModal = false;
            this.isMainShifted = false;
            this.resetPhoto();
        },

        showUnloadPlaceDetails(placeId) {
            const place = this.allUnloadingPlaces.find(p => p.id === placeId);
            if (!place) return;

            this.selectedUnloadPlace = place;

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
        },

        // Вспомогательные методы
        getPlaceName(placeId) {
            if (!this.allUnloadingPlaces || this.allUnloadingPlaces.length === 0) {
                return `ID: ${placeId}`;
            }
            const place = this.allUnloadingPlaces.find(p => p.id === placeId);
            return place ? place.name : `ID: ${placeId}`;
        },

        getFormatName(formatId) {
            if (!formatId || !this.licensePlateFormats || this.licensePlateFormats.length === 0) {
                return null;
            }
            for (const format of this.licensePlateFormats) {
                if (format.format && format.format.id === formatId) {
                    return format.format.name;
                }
            }
            return null;
        },

        getFullDayName(dayIndex) {
            return this.fullDayNames[dayIndex] || 'Неизвестно';
        },

        getSlotsForDay(slots, day) {
            return slots.filter(slot => slot.day_of_week === day);
        },

        daysWithSlots(place) {
            if (!place.time_slots) return [];
            const daysWithSlots = new Set(place.time_slots.map(s => s.day_of_week));
            return Array.from(daysWithSlots).sort((a, b) => a - b);
        },

        hasTimeSlots(place) {
            return place.time_slots && place.time_slots.length > 0;
        },

        formatTime(timeStr) {
            if (!timeStr) return '';
            return timeStr.substring(0, 5);
        },

        isRoundTheClockSlot(slot) {
            return slot.open_time.slice(0,5) === '00:00' && slot.close_time.slice(0,5) === '23:59' && !slot.is_next_day;
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

        // Методы для работы с временными интервалами
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
                // Если закрытие на следующий день, то интервал активен с open до 24:00 и с 00:00 до close
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
            if (!this.selectedUnloadPlace || this.selectedUnloadPlace.status !== 'active') return '';
            
            const now = new Date();
            const currentTime = now.getHours() * 60 + now.getMinutes();
            const currentDay = this.currentDay;
            
            // Получаем все активные слоты для текущего дня
            const todaySlots = this.getSlotsForDay(this.selectedUnloadPlace.time_slots, currentDay)
                .filter(slot => slot.is_active);
            
            // Проверяем, есть ли круглосуточный слот
            const roundTheClockSlot = todaySlots.find(slot => 
                slot.open_time.slice(0,5) === '00:00' && slot.close_time.slice(0,5) === '23:59' && !slot.is_next_day
            );
            
            if (roundTheClockSlot) {
                return 'Открыто круглосуточно';
            }
            
            // Ищем активный слот
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
                // Сейчас открыто - ищем время до закрытия
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
                // Сейчас закрыто - ищем ближайший слот
                let nextSlot = null;
                let minWait = Infinity;
                
                // Сначала проверяем слоты сегодня
                for (const slot of todaySlots) {
                    const open = this.parseTimeToMinutes(slot.open_time);
                    
                    if (slot.is_next_day) {
                        // Если слот переходит на следующий день, то открытие сегодня в open
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
                
                // Если сегодня нет, проверяем следующие дни
                if (nextSlot === null) {
                    for (let daysAhead = 1; daysAhead <= 7; daysAhead++) {
                        const nextDay = (currentDay + daysAhead) % 7;
                        const nextDaySlots = this.getSlotsForDay(this.selectedUnloadPlace.time_slots, nextDay)
                            .filter(slot => slot.is_active);
                        
                        if (nextDaySlots.length > 0) {
                            // Берем самый ранний слот в этот день
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

        // Методы для работы с фото
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
        }
    },
    beforeUnmount() {
        if (this.shiftTimer) {
            clearTimeout(this.shiftTimer);
        }
    }
}
</script>

<style scoped>
.data__list {
    padding: 12px;
    flex: 1;
}

.header-with-badge {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 12px;
}

.vehicles-badge {
    background: #1976d2;
    color: white;
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    min-width: 18px;
    text-align: center;
    line-height: 1.2;
}

/* Vehicles table styles */
.vehicles-table {
    width: 100%;
    border: 1px solid #e0e0e0;
    border-radius:20px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.table-header {
    display: flex;
    background: #f5f5f5;
    border-bottom: 1px solid #e0e0e0;
    padding: 10px 12px;
    font-weight: 500;
    color: #666;
    font-size: 13px;
}

.header-col {
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
    user-select: none;
}

.header-col:hover,
.header-col.active-sort {
    color: #333;
}

.header-col:hover .sort-icon,
.header-col.active-sort .sort-icon {
    opacity: 0.8;
}

.sort-icon {
    width: 10px;
    height: 10px;
    transition: all 0.2s ease;
    opacity: 0.4;
    transform: rotate(0deg);
}

.sort-icon.desc {
    transform: rotate(180deg);
    opacity: 0.8;
}

.table-body {
    max-height: 180px;
    overflow-y: auto;
    background: #fff;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.table-body::-webkit-scrollbar {
    display: none;
}

.table-row {
    display: flex;
    padding: 8px 12px;
    border-bottom: 1px solid #f5f5f5;
    align-items: center;
    font-size: 13px;
    transition: background-color 0.2s ease;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: #f8f9fa;
}

.header-col, .table-col {
    padding: 0 4px;
}

.number-col {
    width: 12%;
    text-align: center;
}

.plate-col {
    width: 30%;
}

.mark-col {
    width: 30%;
}

.actions-col {
    width: 28%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
}

.details-btn, .edit-btn, .delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: all 0.2s ease;
}

.details-btn:hover {
    background: #e3f2fd;
}

.edit-btn:hover {
    background: #e8f5e8;
}

.delete-btn:hover {
    background: #ffebee;
}

.details-icon, .edit-icon, .delete-icon {
    width: 14px;
    height: 14px;
    opacity: 0.6;
    transition: opacity 0.2s ease;
}

.details-btn:hover .details-icon {
    opacity: 0.9;
}

.edit-btn:hover .edit-icon {
    opacity: 0.9;
}

.delete-btn:hover .delete-icon {
    opacity: 0.9;
}

.no-vehicles {
    text-align: center;
    padding: 16px;
    color: #666;
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: #333;
    font-weight: 600;
    margin: 0;
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
    z-index: 1000;
    backdrop-filter: blur(1px);
    animation: overlayAppear 0.4s ease-out;
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

/* Контейнер для окон — относительное позиционирование */
.modal-wrapper {
    position: relative;
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
}

.modal-content {
    background: #fff;
    border-radius: 50px;
    padding: 0;
    padding-bottom: 15px;
    width: 520px;
    height: 450px;
    max-height: 450px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
    position: absolute;
}

.modal-content .modal-body {
    overflow-y: auto;
    height: calc(450px - 70px);
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-content .modal-body::-webkit-scrollbar {
    display: none;
}

/* Основное окно */
.modal-content.main-modal {
    left: calc(50% - 260px);
    transition: transform 0.5s cubic-bezier(0.25, 0.1, 0.15, 1);
    transform: translateX(0);
}

.modal-content.main-modal.shifted {
    transform: translateX(-280px);
}

/* Второе окно */
.modal-content.place-modal {
    left: 50%;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 30px 16px;
    border-bottom: 1px solid #f0f0f0;
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
    transition: all 0.2s ease;
}

.modal-close:hover {
    background-color: #f5f5f5;
}

/* Информация о времени */
.time-info {
    font-size: 13px;
    color: #a2a2a2;
}

.modal-body {   
    padding: 20px 30px;
    overflow-y: auto;
    flex: 1;
}

/* Стили для деталей Т/С */
.vehicle-details {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.details-section {
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    background: #fafafa;
    overflow: hidden;
}

.section-header {
    padding: 12px 20px;
    border-bottom: 1px solid #e6e6e6;
    display: flex;
    justify-content: space-between;
    align-items: center;
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
    color: #333;
}

.section-body {
    padding: 16px 20px;
}

.photo-body {
    padding: 0;
}

.details-grid.two-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px 16px;
}

.detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.detail-item.full-width {
    grid-column: 1 / -1;
}

.detail-label {
    font-size: 11px;
    color: #a2a2a2;
    font-weight: 400;
    letter-spacing: 0.3px;
}

.detail-value {
    font-size: 14px;
    color: #333;
    font-weight: 500;
    word-break: break-word;
}

/* Стили для мест разгрузки в основном окне */
.places-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
}

.place-item {
    border: 1px solid #e6e6e6;
    border-radius: 50px;
    padding: 6px 12px;
    font-size: 12px;
    color: #333;
    transition: all 0.2s ease;
    display: inline-block;
    cursor: pointer;
}

.place-item:hover {
    background: #f0f0f0;
    border-color: #4F5BDF;
}

.place-item.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.no-places {
    text-align: center;
    color: #a2a2a2;
    font-size: 13px;
    font-style: italic;
    padding: 10px;
}

.existing-badge {
    display: flex;
    justify-content: center;
    margin-top: 8px;
}

.badge-text {
    background: #e3f2fd;
    color: #1976d2;
    padding: 6px 12px;
    border-radius: 16px;
    font-size: 12px;
    font-weight: 500;
}

/* Стили для деталей места разгрузки */
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

.info-label {
    font-size: 13px;
    color: #a2a2a2;
    font-weight: 400;
}

.info-value {
    font-size: 13px;
    color: #333;
    font-weight: 500;
}

.comment-text {
    font-size: 12px;
    color: #666;
    font-style: italic;
    margin-top: 8px;
    padding: 8px 12px;
    background: #fff3e0;
    border-radius: 8px;
    border-left: 3px solid #f39c12;
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

.map-link-btn {
    padding: 4px 12px;
    background: #f0f3ff;
    color: #4F5BDF;
    text-decoration: none;
    border-radius: 30px;
    font-size: 12px;
    font-weight: 500;
    transition: all 0.2s ease;
    border: 1px solid #4F5BDF;
    white-space: nowrap;
}

.map-link-btn:hover {
    background: #4F5BDF;
    color: white;
}

.photo-container {
    position: relative;
    width: 100%;
    height: 200px;
    overflow: hidden;
    border: 1px solid #e6e6e6;
    background: #f5f5f5;
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
    border: 1px solid #e6e6e6;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    font-weight: bold;
    color: #333;
    transition: all 0.2s ease;
}

.photo-control-btn:hover {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.no-photo-placeholder {
    width: 100%;
    height: 200px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #f5f5f5;
    color: #999;
    font-size: 14px;
    border-radius: 20px;
    border: 1px solid #e6e6e6;
}

/* Стили для режима работы - сетка */
.schedule-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 12px;
}

.schedule-day-card {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 16px;
    overflow: hidden;
    box-shadow: 0 2px 6px rgba(0,0,0,0.05);
    transition: all 0.2s ease;
}

.schedule-day-card.current-day {
    outline: 1px solid #4F5BDF;
}

.schedule-day-card .day-name {
    background: #f0f3ff;
    padding: 8px 10px;
    font-weight: 600;
    color: #4F5BDF;
    font-size: 13px;
    text-align: center;
    border-bottom: 1px solid #e6e6e6;
}

.schedule-day-card .day-slots {
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.slot-badge {
    padding: 6px 8px;
    background: #f8f9fa;
    border-radius: 20px;
    border: 1px solid #e6e6e6;
    font-size: 11px;
    display: flex;
    flex-direction: column;
    align-items: center;
    transition: all 0.2s ease;
}

.slot-badge.active-slot {
    background: #e8f5e9;
    border-color: #81c784;
}

.round-clock-text {
    font-weight: 500;
    color: #000;
    font-size: 11px;
}

.slot-time {
    color: #333;
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
    background: #4F5BDF;
    color: white;
}

.inactive-badge {
    background: #ffebee;
    color: #c62828;
}

.no-schedule {
    text-align: center;
    color: #999;
    font-size: 13px;
    font-style: italic;
    padding: 16px;
    background: #f8f9fa;
    border-radius: 8px;
}

/* Анимации появления/исчезновения основного окна */
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
    transition: all 0.4s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
    transition: all 0.4s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
    opacity: 0;
    transform: scale(0.9) translateY(-20px);
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
    
    .modal-content {
        position: static;
        margin-bottom: 10px;
    }
    
    .modal-content.main-modal {
        left: auto;
        transform: none !important;
    }
    
    .modal-content.main-modal.shifted {
        transform: none !important;
    }
    
    .modal-content.place-modal {
        left: auto;
    }
    
    .details-grid.two-columns {
        grid-template-columns: 1fr;
    }
    
    .table-row {
        flex-wrap: wrap;
    }
    
    .table-col {
        width: 50% !important;
        margin-bottom: 4px;
    }
    
    .actions-col {
        width: 100%;
        justify-content: flex-end;
    }
    
    .modal-content {
        height: auto;
        max-height: 80vh;
    }
    
    .modal-body {
        padding: 16px 20px;
    }
    
    .modal-header {
        padding: 16px 20px;
    }
    
    .header-with-status {
        flex-direction: column;
        align-items: flex-start;
        gap: 8px;
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
    
    .places-list {
        gap: 6px;
    }
    
    .place-item {
        padding: 4px 10px;
        font-size: 11px;
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