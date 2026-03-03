<template>
    <div class="data__list">
        <div class="header-with-badge">
            <h4>Список сотрудников</h4>
            <span class="employees-badge">{{ employees.length }}</span>
        </div>
        <div class="employees-table">
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
                <div class="header-col lastName-col" @click="$emit('sort', 'lastName')">
                    <p :class="{ 'active-sort': sortField === 'lastName' }">Фамилия</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'lastName' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col firstName-col" @click="$emit('sort', 'firstName')">
                    <p :class="{ 'active-sort': sortField === 'firstName' }">Имя</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'firstName' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col middleName-col" @click="$emit('sort', 'middleName')">
                    <p :class="{ 'active-sort': sortField === 'middleName' }">Отчество</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'middleName' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col actions-col">
                    Действия
                </div>
            </div>
            <div class="table-body">
                <div 
                    v-for="(employee, index) in employees" 
                    :key="employee.id"
                    class="table-row"
                >
                    <div class="table-col number-col">{{ index + 1 }}</div>
                    <div class="table-col lastName-col">
                        {{ employee.lastName || 'Не указано' }}
                    </div>
                    <div class="table-col firstName-col">
                        {{ employee.firstName || 'Не указано' }}
                    </div>
                    <div class="table-col middleName-col">
                        {{ employee.middleName || 'Не указано' }}
                    </div>
                    <div class="table-col actions-col">
                        <button 
                            class="details-btn"
                            @click="showEmployeeDetails(employee)"
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
                            @click="$emit('edit-employee', employee)"
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
                            @click="$emit('delete-employee', employee.id)"
                        >
                            <img 
                                src="@/assets/icons/trashcan.png" 
                                alt="Удалить" 
                                class="delete-icon"
                            />
                        </button>
                    </div>
                </div>
                <div v-if="employees.length === 0" class="no-employees">
                    Нет добавленных сотрудников
                </div>
            </div>
        </div>

        <!-- Модальное окно деталей сотрудника -->
        <transition name="modal-fade">
            <div v-if="showDetailsModal" class="modal-overlay" @click.self="closeDetailsModal">
                <div class="modal-wrapper">
                    <!-- Основное модальное окно с деталями сотрудника -->
                    <div 
                        class="modal-content compact-modal main-modal"
                        :class="{ 'shifted': isMainShifted }"
                    >
                        <div class="modal-header">
                            <h3 class="modal-title">Детальная информация о сотруднике</h3>
                            <button @click="closeDetailsModal" class="modal-close">
                                <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                                    <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                                </svg>
                            </button>
                        </div>
                        
                        <div class="modal-body">
                            <div class="employee-details" v-if="selectedEmployee">
                                <div class="details-section">
                                    <h4 class="section-title">Основная информация</h4>
                                    <div class="details-grid two-columns">
                                        <div class="detail-item">
                                            <label class="detail-label">Фамилия:</label>
                                            <span class="detail-value">{{ selectedEmployee.lastName || 'Не указано' }}</span>
                                        </div>
                                        <div class="detail-item">
                                            <label class="detail-label">Имя:</label>
                                            <span class="detail-value">{{ selectedEmployee.firstName || 'Не указано' }}</span>
                                        </div>
                                        <div class="detail-item">
                                            <label class="detail-label">Отчество:</label>
                                            <span class="detail-value">{{ selectedEmployee.middleName || 'Не указано' }}</span>
                                        </div>
                                        <div class="detail-item">
                                            <label class="detail-label">Должность:</label>
                                            <span class="detail-value">{{ selectedEmployee.position || 'Не указана' }}</span>
                                        </div>
                                        <div class="detail-item">
                                            <label class="detail-label">Гражданство:</label>
                                            <span class="detail-value">{{ selectedEmployee.citizenshipName || 'Не указано' }}</span>
                                        </div>
                                    </div>
                                </div>

                                <div class="details-section">
                                    <h4 class="section-title">Документы</h4>
                                    <div class="details-grid two-columns">
                                        <div class="detail-item">
                                            <label class="detail-label">Серия и номер паспорта:</label>
                                            <div class="sensitive-data">
                                                <span 
                                                    class="data-text"
                                                    :class="{ 'hidden-data': !showFullPassport }"
                                                >
                                                    {{ selectedEmployee.passportSeriesNumber || 'Не указан' }}
                                                </span>
                                                <button 
                                                    v-if="selectedEmployee.passportSeriesNumber"
                                                    @click="togglePassportVisibility"
                                                    class="show-more-btn"
                                                    :class="{ 'active': showFullPassport }"
                                                >
                                                    {{ showFullPassport ? 'Скрыть' : 'Показать' }}
                                                </button>
                                            </div>
                                        </div>
                                        <div v-if="selectedEmployee.patentNumber" class="detail-item">
                                            <label class="detail-label">Номер патента:</label>
                                            <div class="sensitive-data">
                                                <span 
                                                    class="data-text"
                                                    :class="{ 'hidden-data': !showFullPatent }"
                                                >
                                                    {{ selectedEmployee.patentNumber }}
                                                </span>
                                                <button 
                                                    @click="togglePatentVisibility"
                                                    class="show-more-btn"
                                                    :class="{ 'active': showFullPatent }"
                                                >
                                                    {{ showFullPatent ? 'Скрыть' : 'Показать' }}
                                                </button>
                                            </div>
                                        </div>
                                        <div v-if="selectedEmployee.otherPermission" class="detail-item full-width">
                                            <label class="detail-label">Иное разрешение:</label>
                                            <span class="detail-value">{{ selectedEmployee.otherPermission }}</span>
                                        </div>
                                    </div>
                                </div>

                                <div class="details-section">
                                    <h4 class="section-title">Места прохода</h4>
                                    <div class="places-list">
                                        <div 
                                            v-for="tableId in selectedEmployee.targetTables" 
                                            :key="tableId"
                                            class="place-item"
                                            :class="{ 'active': showPlaceModal && selectedTable && selectedTable.table && selectedTable.table.id === tableId }"
                                            @click="showTableDetails(tableId)"
                                        >
                                            {{ getTableName(tableId) }}
                                        </div>
                                        <div v-if="!selectedEmployee.targetTables || selectedEmployee.targetTables.length === 0" class="no-places">
                                            Места прохода не указаны
                                        </div>
                                    </div>
                                </div>

                                <div v-if="selectedEmployee.isExisting" class="existing-badge">
                                    <span class="badge-text">Существующий сотрудник</span>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Дополнительное модальное окно с деталями места прохода -->
                    <transition 
                        name="place-slide"
                        @after-leave="onPlaceLeave"
                    >
                        <div v-if="showPlaceModal" class="modal-content compact-modal place-modal">
                            <div class="modal-header">
                                <div class="header-with-status">
                                    <h3 class="modal-title">Информация о месте прохода</h3>
                                    <span class="status-badge" :class="getTableStatusClass(selectedTable)">
                                        {{ getTableStatusText(selectedTable) }}
                                    </span>
                                    <!-- Информация о времени до открытия/закрытия - внутри header-with-status -->
                                    <div v-if="selectedTable && selectedTable.table && selectedTable.table.status === 'active'" class="time-info">
                                        {{ getTableTimeInfoText() }}
                                    </div>
                                </div>
                                <button @click="closeTableDetails" class="modal-close">
                                    <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                                        <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                                    </svg>
                                </button>
                            </div>

                            <div class="modal-body">
                                <div class="place-details" v-if="selectedTable && selectedTable.table">
                                    <!-- Секция Основная информация (без статуса) -->
                                    <div class="details-section">
                                        <div class="section-header">
                                            <h4 class="section-title">Основная информация</h4>
                                        </div>
                                        <div class="section-body">
                                            <div class="info-grid">
                                                <div class="info-row">
                                                    <span class="info-label">Наименование:</span>
                                                    <span class="info-value">{{ selectedTable.table.display_name }}</span>
                                                </div>
                                                <div class="info-row">
                                                    <span class="info-label">Тип:</span>
                                                    <span class="info-value">{{ getTableTypeLabel(selectedTable.table.table_type) }}</span>
                                                </div>
                                            </div>
                                            <div v-if="selectedTable.table.status !== 'active' && selectedTable.table.status_comment" class="comment-text">
                                                {{ selectedTable.table.status_comment }}
                                            </div>
                                            <div v-if="selectedTable.table.location_description" class="location-description">
                                                {{ selectedTable.table.location_description }}
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Секция Режим работы -->
                                    <div class="details-section">
                                        <div class="section-header">
                                            <h4 class="section-title">Режим работы</h4>
                                        </div>
                                        <div class="section-body">
                                            <div v-if="hasTimeSlots(selectedTable)" class="schedule-grid">
                                                <div 
                                                    v-for="day in daysWithSlots(selectedTable)" 
                                                    :key="day" 
                                                    class="schedule-day-card"
                                                    :class="{ 'current-day': isCurrentDay(day) }"
                                                >
                                                    <div class="day-name">{{ getFullDayName(day) }}</div>
                                                    <div class="day-slots">
                                                        <div 
                                                            v-for="slot in getSlotsForDay(selectedTable.time_slots, day)" 
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
                                                v-if="selectedTable.table.map_link" 
                                                :href="selectedTable.table.map_link" 
                                                target="_blank" 
                                                class="map-link-btn"
                                            >
                                                Как добраться?
                                            </a>
                                        </div>
                                        <div class="section-body photo-body">
                                            <div v-if="selectedTable.photos && selectedTable.photos.length > 0" class="photo-container">
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
                                                        :src="getMainPhotoUrl(selectedTable.photos)" 
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
                    </transition>
                </div>
            </div>
        </transition>
    </div>
</template>

<script>
export default {
    name: 'EmployeesList',
    props: {
        employees: {
            type: Array,
            required: true
        },
        sortField: String,
        sortDirection: String,
        allTables: {
            type: Array,
            default: () => []
        }
    },
    emits: ['sort', 'edit-employee', 'delete-employee'],
    data() {
        return {
            showDetailsModal: false,
            selectedEmployee: null,
            selectedTable: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
            showFullPassport: false,
            showFullPatent: false,
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
        currentDay() {
            return new Date().getDay() === 0 ? 6 : new Date().getDay() - 1;
        },
        photoStyle() {
            return {
                transform: `translate(${this.photoTranslateX}px, ${this.photoTranslateY}px) scale(${this.photoScale})`,
                cursor: this.isDragging ? 'grabbing' : 'grab'
            };
        }
    },
    methods: {
        togglePassportVisibility() {
            this.showFullPassport = !this.showFullPassport;
        },

        togglePatentVisibility() {
            this.showFullPatent = !this.showFullPatent;
        },

        showEmployeeDetails(employee) {
            this.selectedEmployee = employee;
            this.showDetailsModal = true;
            this.showPlaceModal = false;
            this.selectedTable = null;
            this.isMainShifted = false;
            this.resetPhoto();
        },

        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedEmployee = null;
            this.selectedTable = null;
            this.showPlaceModal = false;
            this.isMainShifted = false;
            this.resetPhoto();
        },

        showTableDetails(tableId) {
            console.log('Клик по месту прохода с ID:', tableId);
            console.log('Все доступные таблицы:', this.allTables);
            
            // Ищем таблицу по ID в разных возможных форматах
            const tableData = this.allTables.find(t => {
                if (t.table && t.table.id === tableId) {
                    return true;
                }
                if (t.id === tableId) {
                    return true;
                }
                return false;
            });
            
            if (!tableData) {
                console.error(`Таблица с ID ${tableId} не найдена`);
                alert(`Информация о месте прохода с ID ${tableId} недоступна`);
                return;
            }

            // Нормализуем данные для единообразной работы
            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || (tableData.table && tableData.table.time_slots) || [],
                photos: tableData.photos || (tableData.table && tableData.table.photos) || [],
                current_status: tableData.current_status || (tableData.table && tableData.table.current_status) || 'closed'
            };

            console.log('Выбранная таблица (нормализованная):', this.selectedTable);

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

        closeTableDetails() {
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            this.isMainShifted = false;
        },

        getTableName(tableId) {
            console.log('===== getTableName called =====');
            console.log('Looking for table ID:', tableId);
            console.log('Available allTables:', this.allTables);
            
            // Пробуем найти таблицу в разных форматах
            let foundTable = null;
            
            for (const item of this.allTables) {
                console.log('Checking item:', item);
                
                // Формат с полем table
                if (item.table && item.table.id === tableId) {
                    foundTable = item.table;
                    console.log('Found in table field:', foundTable);
                    break;
                }
                
                // Плоский формат
                if (item.id === tableId) {
                    foundTable = item;
                    console.log('Found as flat object:', foundTable);
                    break;
                }
            }
            
            if (foundTable) {
                const displayName = foundTable.display_name || foundTable.name || `ID: ${tableId}`;
                console.log('Returning display name:', displayName);
                return displayName;
            }
            
            console.warn(`Table with ID ${tableId} not found in allTables`);
            return `Неизвестное место (ID: ${tableId})`;
        },

        getTableTypeLabel(type) {
            return type === 'cars' ? 'Машины' : 'Люди';
        },

        getTableStatusClass(table) {
            if (!table || !table.table) return 'status-inactive';
            if (table.table.status !== 'active') {
                return 'status-inactive';
            }
            return table.current_status === 'open' ? 'status-open' : 'status-closed';
        },

        getTableStatusText(table) {
            if (!table || !table.table) return 'Неизвестно';
            if (table.table.status !== 'active') {
                if (table.table.status === 'maintenance') return 'На обслуживании';
                return 'Неактивно';
            }
            return table.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
        },

        getFullDayName(dayIndex) {
            return this.fullDayNames[dayIndex] || 'Неизвестно';
        },

        getSlotsForDay(slots, day) {
            return slots.filter(slot => slot.day_of_week === day);
        },

        daysWithSlots(table) {
            if (!table || !table.time_slots) return [];
            const daysWithSlots = new Set(table.time_slots.map(s => s.day_of_week));
            return Array.from(daysWithSlots).sort((a, b) => a - b);
        },

        hasTimeSlots(table) {
            return table && table.time_slots && table.time_slots.length > 0;
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
            if (!this.selectedTable || !this.selectedTable.table || this.selectedTable.table.status !== 'active') return '';
            
            const now = new Date();
            const currentTime = now.getHours() * 60 + now.getMinutes();
            const currentDay = this.currentDay;
            
            const todaySlots = this.getSlotsForDay(this.selectedTable.time_slots, currentDay)
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
                        const nextDaySlots = this.getSlotsForDay(this.selectedTable.time_slots, nextDay)
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

        getMainPhotoUrl(photos) {
            if (!photos || photos.length === 0) return null;
            const mainPhoto = photos.find(p => p.is_main) || photos[0];
            return mainPhoto.photo_url;
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

        getAllPassageTables(employee) {
            if (!employee || !employee.passageTables) {
                return [];
            }
            
            if (typeof employee.passageTables === 'string') {
                return employee.passageTables.split(',').map(table => table.trim()).filter(table => table);
            }
            
            if (Array.isArray(employee.passageTables)) {
                return employee.passageTables;
            }
            
            return [];
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

.employees-badge {
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

/* Employees table styles */
.employees-table {
    width: 100%;
    border: 1px solid #e0e0e0;
    border-radius: 12px;
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
    width: 8%;
    text-align: center;
}

.lastName-col {
    width: 22%;
}

.firstName-col {
    width: 22%;
}

.middleName-col {
    width: 22%;
}

.actions-col {
    width: 26%;
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

.no-employees {
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

/* Scrollbar styling */
.table-body::-webkit-scrollbar {
    width: 4px;
}

.table-body::-webkit-scrollbar-track {
    background: #f1f1f1;
}

.table-body::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 2px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
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

.modal-content.compact-modal {
    width: 520px;
}

.modal-content.main-modal {
    left: calc(50% - 260px);
    transition: transform 0.5s cubic-bezier(0.25, 0.1, 0.15, 1);
    transform: translateX(0);
}

.modal-content.main-modal.shifted {
    transform: translateX(-280px);
}

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

.time-info {
    font-size: 13px;
    color: #a2a2a2;

}

.modal-body {
    padding: 20px 30px;
    overflow-y: auto;
    flex: 1;
}

.employee-details {
    display: flex;
    flex-direction: column;
    gap: 16px;
}

.details-section {
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 20px;
    background: #fafafa;
}

.section-title {
    margin: 0 0 12px 0;
    font-size: 14px;
    font-weight: 600;
    color: #333;
    padding-bottom: 8px;
    border-bottom: 1px solid #e6e6e6;
}

.details-grid.two-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
}

.detail-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.detail-item.full-width {
    grid-column: 1 / -1;
}

.detail-label {
    font-size: 12px;
    color: #666;
    font-weight: 500;
}

.detail-value {
    font-size: 13px;
    color: #333;
    font-weight: 500;
}

.sensitive-data {
    display: flex;
    align-items: center;
    gap: 15px;
}

.data-text {
    font-size: 13px;
    color: #333;
    font-weight: 500;
    letter-spacing: 0.5px;
    transition: all 0.3s ease;
    word-break: break-all;
}

.data-text.hidden-data {
    filter: blur(4px);
    user-select: none;
}

.show-more-btn {
    background: #f8f9fa;
    border: 1px solid #e0e0e0;
    color: #4F5BDF;
    font-size: 11px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 15px;
    transition: all 0.2s;
    font-weight: 500;
    white-space: nowrap;
    min-width: 75px;
    text-align: center;
}

.show-more-btn:hover {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.show-more-btn.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
    min-width: 75px;
}

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

/* Стили для деталей места прохода */
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

/* Убираем is_next_day бейдж */
.next-day-badge {
    display: none;
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

/* Анимации */
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

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
        padding: 16px;
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
    
    .sensitive-data {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }
    
    .show-more-btn {
        align-self: flex-start;
    }
}

@media (max-width: 480px) {
    .modal-header {
        padding: 12px 16px;
    }
    
    .modal-title {
        font-size: 14px;
    }
    
    .section-title {
        font-size: 13px;
    }
    
    .detail-label {
        font-size: 11px;
    }
    
    .detail-value {
        font-size: 12px;
    }
    
    .modal-content.compact-modal {
        border-radius: 20px;
    }
    
    .details-section {
        border-radius: 12px;
        padding: 12px;
    }
}
</style>