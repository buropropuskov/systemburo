<template>
    <div class="modal-content-inner">
        <div class="modal-header">
            <div class="header-with-status">
                <h3 class="modal-title">Информация о месте прохода</h3>
                <span class="status-badge" :class="getTableStatusClass">
                    {{ getTableStatusText }}
                </span>
                <div v-if="table && table.table && table.table.status === 'active'" class="time-info">
                    {{ getTableTimeInfoText() }}
                </div>
            </div>
            <button @click="close" class="modal-close">
                <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                    <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                </svg>
            </button>
        </div>

        <div class="modal-body">
            <div class="place-details" v-if="table && table.table">
                <!-- Секция Основная информация -->
                <div class="details-section">
                    <div class="section-header">
                        <h4 class="section-title">Основная информация</h4>
                    </div>
                    <div class="section-body">
                        <div class="info-grid">
                            <div class="info-row">
                                <span class="info-label">Наименование:</span>
                                <span class="info-value">{{ table.table.display_name }}</span>
                            </div>
                            <div class="info-row">
                                <span class="info-label">Тип:</span>
                                <span class="info-value">{{ getTableTypeLabel }}</span>
                            </div>
                        </div>
                        <div v-if="table.table.status !== 'active' && table.table.status_comment" class="comment-text">
                            {{ table.table.status_comment }}
                        </div>
                        <div v-if="table.table.location_description" class="location-description">
                            {{ table.table.location_description }}
                        </div>
                    </div>
                </div>

                <!-- Секция Режим работы -->
                <div class="details-section">
                    <div class="section-header">
                        <h4 class="section-title">Режим работы</h4>
                    </div>
                    <div class="section-body">
                        <div v-if="hasTimeSlots" class="schedule-grid">
                            <div 
                                v-for="day in daysWithSlots" 
                                :key="day" 
                                class="schedule-day-card"
                                :class="{ 'current-day': isCurrentDay(day) }"
                            >
                                <div class="day-name">{{ getFullDayName(day) }}</div>
                                <div class="day-slots">
                                    <div 
                                        v-for="slot in getSlotsForDay(day)" 
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
                            v-if="table.table.map_link" 
                            :href="table.table.map_link" 
                            target="_blank" 
                            class="map-link-btn"
                        >
                            Как добраться?
                        </a>
                    </div>
                    <div class="section-body photo-body">
                        <div v-if="table.photos && table.photos.length > 0" class="photo-container">
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
                                    :src="getMainPhotoUrl" 
                                    alt="Место прохода"
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
</template>

<script>
export default {
    name: 'TableInfoModal',
    props: {
        table: {
            type: Object,
            default: null
        },
        allTables: {
            type: Array,
            default: () => []
        }
    },
    emits: ['close'],
    data() {
        return {
            fullDayNames: ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'],
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
            containerHeight: 0
        };
    },
    computed: {
        currentDay() {
            return new Date().getDay() === 0 ? 6 : new Date().getDay() - 1;
        },
        getTableStatusClass() {
            if (!this.table || !this.table.table) return 'status-inactive';
            if (this.table.table.status !== 'active') return 'status-inactive';
            return this.table.current_status === 'open' ? 'status-open' : 'status-closed';
        },
        getTableStatusText() {
            if (!this.table || !this.table.table) return 'Неизвестно';
            if (this.table.table.status !== 'active') {
                if (this.table.table.status === 'maintenance') return 'На обслуживании';
                return 'Неактивно';
            }
            return this.table.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
        },
        getTableTypeLabel() {
            if (!this.table || !this.table.table) return '';
            return this.table.table.table_type === 'cars' ? 'Машины' : 'Люди';
        },
        hasTimeSlots() {
            return this.table && this.table.time_slots && this.table.time_slots.length > 0;
        },
        daysWithSlots() {
            if (!this.hasTimeSlots) return [];
            const daysWithSlots = new Set(this.table.time_slots.map(s => s.day_of_week));
            return Array.from(daysWithSlots).sort((a, b) => a - b);
        },
        getMainPhotoUrl() {
            if (!this.table || !this.table.photos || this.table.photos.length === 0) return null;
            const mainPhoto = this.table.photos.find(p => p.is_main) || this.table.photos[0];
            return mainPhoto.photo_url;
        },
        photoStyle() {
            return {
                transform: `translate(${this.photoTranslateX}px, ${this.photoTranslateY}px) scale(${this.photoScale})`,
                cursor: this.isDragging ? 'grabbing' : 'grab'
            };
        }
    },
    methods: {
        close() {
            this.$emit('close');
        },

        getFullDayName(dayIndex) {
            return this.fullDayNames[dayIndex] || 'Неизвестно';
        },

        getSlotsForDay(day) {
            if (!this.table || !this.table.time_slots) return [];
            return this.table.time_slots.filter(slot => slot.day_of_week === day);
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

        getTableTimeInfoText() {
            if (!this.table || !this.table.table || this.table.table.status !== 'active') return '';
            
            const now = new Date();
            const currentTime = now.getHours() * 60 + now.getMinutes();
            const currentDay = this.currentDay;
            
            const todaySlots = this.getSlotsForDay(currentDay).filter(slot => slot.is_active);
            
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
                        const nextDaySlots = this.getSlotsForDay(nextDay).filter(slot => slot.is_active);
                        
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

        // Фото
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
    watch: {
        table(newVal) {
            if (newVal) {
                this.$nextTick(() => {
                    this.updateContainerDimensions();
                });
                this.resetPhoto();
            }
        }
    }
};
</script>

<style scoped>
.modal-content-inner {
    width: 100%;
    height: 450px;
    display: flex;
    flex-direction: column;
    background: #fff;
    overflow: hidden;
    border-radius: 50px;
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

.time-info {
    font-size: 13px;
    color: #a2a2a2;
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
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    background: #fafafa;
    overflow: hidden;
}

.section-header {
    padding: 12px 20px;
    border-bottom: 1px solid #e6e6e6;
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

.location-description {
    font-size: 12px;
    color: #333;
    margin-top: 8px;
    padding: 8px 12px;
    background: #f8f9fa;
    border-radius: 8px;
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

@media (max-width: 768px) {
    .modal-header {
        padding: 16px 20px;
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