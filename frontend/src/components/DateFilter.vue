<template>
    <div class="date-filter">
        <div class="date-field" @click="toggleCalendar">
            <div class="field-wrapper">
                <div class="field-input">
                    {{ displayText }}
                </div>
                <img src="@/assets/icons/calendar.png" class="field-icon" />
            </div>
            
            <transition name="calendar-slide">
                <div v-if="showCalendar" class="calendar-modal" @click.stop>
                    <div class="calendar-container">
                        <!-- Header -->
                        <div class="calendar-header">
                            <div class="header-actions">
                                <button @click="prevMonth" class="nav-btn prev-btn">
                                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                        <path d="M15 18L9 12L15 6" stroke="#4F5BDF" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                                    </svg>
                                </button>
                                <div class="date-display">
                                    <span class="current-month-year">{{ capitalizeFirstLetter(currentMonthYear) }}</span>
                                </div>
                                <button @click="nextMonth" class="nav-btn next-btn">
                                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                        <path d="M9 18L15 12L9 6" stroke="#4F5BDF" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                        
                        <div class="calendar-body">
                            <!-- Quick selection слева -->
                            <div class="quick-selection">
                                <div class="quick-buttons-list">
                                    <button @click="setQuickDate('today')" class="quick-btn" :class="{ 'active': isQuickActive('today') }">
                                        Сегодня
                                    </button>
                                    <button @click="setQuickDate('yesterday')" class="quick-btn" :class="{ 'active': isQuickActive('yesterday') }">
                                        Вчера
                                    </button>
                                    <button @click="setQuickDate('tomorrow')" class="quick-btn" :class="{ 'active': isQuickActive('tomorrow') }">
                                        Завтра
                                    </button>
                                    <button @click="setQuickDate('dayBeforeYesterday')" class="quick-btn" :class="{ 'active': isQuickActive('dayBeforeYesterday') }">
                                        Позавчера
                                    </button>
                                    <button @click="setQuickDate('dayAfterTomorrow')" class="quick-btn" :class="{ 'active': isQuickActive('dayAfterTomorrow') }">
                                        Послезавтра
                                    </button>
                                    <button @click="setQuickDate('thisWeek')" class="quick-btn" :class="{ 'active': isQuickActive('thisWeek') }">
                                        Эта неделя
                                    </button>
                                    <button @click="setQuickDate('lastWeek')" class="quick-btn" :class="{ 'active': isQuickActive('lastWeek') }">
                                        Прошлая неделя
                                    </button>
                                    <button @click="setQuickDate('nextWeek')" class="quick-btn" :class="{ 'active': isQuickActive('nextWeek') }">
                                        Следующая неделя
                                    </button>
                                    <button @click="setQuickDate('thisMonth')" class="quick-btn" :class="{ 'active': isQuickActive('thisMonth') }">
                                        Этот месяц
                                    </button>
                                    <button @click="setQuickDate('lastMonth')" class="quick-btn" :class="{ 'active': isQuickActive('lastMonth') }">
                                        Прошлый месяц
                                    </button>
                                    <button @click="setQuickDate('nextMonth')" class="quick-btn" :class="{ 'active': isQuickActive('nextMonth') }">
                                        Следующий месяц
                                    </button>
                                    <button @click="setQuickDate('thisYear')" class="quick-btn" :class="{ 'active': isQuickActive('thisYear') }">
                                        Этот год
                                    </button>
                                    <button @click="setQuickDate('lastYear')" class="quick-btn" :class="{ 'active': isQuickActive('lastYear') }">
                                        Прошлый год
                                    </button>
                                </div>
                            </div>
                            
                            <!-- Calendar справа -->
                            <div class="calendar-main">
                                <div class="calendar-mode-switch">
                                    <button 
                                        class="mode-btn" 
                                        :class="{ 'active': !selectingRange }"
                                        @click="setMode('single')"
                                    >
                                        Один день
                                    </button>
                                    <button 
                                        class="mode-btn" 
                                        :class="{ 'active': selectingRange }"
                                        @click="setMode('range')"
                                    >
                                        Период
                                    </button>
                                </div>
                                
                                <div class="weekdays">
                                    <div v-for="day in weekdays" :key="day" class="weekday">
                                        {{ day }}
                                    </div>
                                </div>
                                
                                <div class="days-grid">
                                    <div 
                                        v-for="day in daysInMonth" 
                                        :key="day.date ? day.date.getTime() : `empty-${day.index}`"
                                        class="day"
                                        :class="getDayClass(day)"
                                        @click="selectDay(day)"
                                    >
                                        <span class="day-number">{{ day.number }}</span>
                                    </div>
                                </div>
                                
                                <!-- Selected range display - всегда отображается -->
                                <div class="selected-range">
                                    <div class="range-display">
                                        <template v-if="!selectingRange">
                                            <span class="range-label">Отобразить дату:</span>
                                            <span class="range-date">{{ formatDateForDisplay(internalSelectedDate) || '...' }}</span>
                                        </template>
                                        <template v-else>
                                            <span class="range-label">Отобразить с</span>
                                            <span class="range-date">{{ formatDateForDisplay(internalRangeStart) || '...' }}</span>
                                            <span class="range-label">по</span>
                                            <span class="range-date">{{ formatDateForDisplay(internalRangeEnd) || '...' }}</span>
                                        </template>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Actions -->
                        <div class="calendar-actions">
                            <button @click="clearSelection" class="action-btn action-btn--clear">
                                Очистить
                            </button>
                            <button @click="applySelection" class="action-btn action-btn--apply">
                                Применить
                            </button>
                        </div>
                    </div>
                </div>
            </transition>
        </div>
    </div>
</template>

<script>
export default {
    name: 'DateFilter',
    props: {
        mode: {
            type: String,
            default: 'range', // 'single' или 'range'
        },
        selectedDate: {
            type: Date,
            default: null,
        },
        dateRangeStart: {
            type: Date,
            default: null,
        },
        dateRangeEnd: {
            type: Date,
            default: null,
        },
    },
    data() {
        return {
            showCalendar: false,
            currentDate: new Date(),
            internalSelectedDate: null,
            internalRangeStart: null,
            internalRangeEnd: null,
            selectingRange: this.mode === 'range', // true для периода, false для одного дня
            activeQuickDate: null,
            weekdays: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
        };
    },
    computed: {
        displayText() {
            if (!this.selectingRange && this.internalSelectedDate) {
                // Режим одного дня
                return this.formatDateForDisplay(this.internalSelectedDate);
            } else if (this.selectingRange) {
                // Режим периода
                if (this.internalRangeStart && this.internalRangeEnd) {
                    const start = this.formatDateForDisplay(this.internalRangeStart);
                    const end = this.formatDateForDisplay(this.internalRangeEnd);
                    return start === end ? start : `${start} — ${end}`;
                } else if (this.internalRangeStart) {
                    return `${this.formatDateForDisplay(this.internalRangeStart)} — ...`;
                }
            }
            return 'Выберите дату';
        },
        
        currentMonthYear() {
            return this.currentDate.toLocaleDateString('ru-RU', {
                month: 'long',
                year: 'numeric'
            }).replace(' г.', '');
        },
        
        daysInMonth() {
            const year = this.currentDate.getFullYear();
            const month = this.currentDate.getMonth();
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            const days = [];
            
            // Добавляем пустые ячейки для начала месяца
            const firstDayOfWeek = firstDay.getDay() || 7; // 1 = понедельник
            for (let i = 1; i < firstDayOfWeek; i++) {
                days.push({ 
                    number: '', 
                    date: null, 
                    isCurrentMonth: false,
                    index: i 
                });
            }
            
            // Добавляем дни месяца
            for (let day = 1; day <= lastDay.getDate(); day++) {
                const date = new Date(year, month, day);
                days.push({
                    number: day,
                    date: date,
                    isCurrentMonth: true,
                    index: firstDayOfWeek + day - 1
                });
            }
            
            // Ограничиваем ровно 6 недель для стабильной высоты
            const totalDaysNeeded = 42; // 6 недель * 7 дней
            while (days.length < totalDaysNeeded) {
                days.push({ 
                    number: '', 
                    date: null, 
                    isCurrentMonth: false,
                    index: days.length 
                });
            }
            
            return days.slice(0, totalDaysNeeded); // Всегда возвращаем ровно 42 дня
        },
    },
    watch: {
        selectedDate: {
            immediate: true,
            handler(newVal) {
                this.internalSelectedDate = newVal ? new Date(newVal) : null;
            }
        },
        dateRangeStart: {
            immediate: true,
            handler(newVal) {
                this.internalRangeStart = newVal ? new Date(newVal) : null;
            }
        },
        dateRangeEnd: {
            immediate: true,
            handler(newVal) {
                this.internalRangeEnd = newVal ? new Date(newVal) : null;
            }
        },
        mode: {
            immediate: true,
            handler(newVal) {
                this.selectingRange = newVal === 'range';
            }
        },
    },
    mounted() {
        document.addEventListener('click', this.handleClickOutside);
        // Устанавливаем режим на основе пропса
        this.selectingRange = this.mode === 'range';
    },
    beforeUnmount() {
        document.removeEventListener('click', this.handleClickOutside);
    },
    methods: {
        capitalizeFirstLetter(string) {
            if (!string) return '';
            return string.charAt(0).toUpperCase() + string.slice(1);
        },
        
        toggleCalendar() {
            this.showCalendar = !this.showCalendar;
            if (this.showCalendar) {
                // Если есть выбранная дата, показываем её месяц
                if (this.internalSelectedDate) {
                    this.currentDate = new Date(this.internalSelectedDate);
                } else if (this.internalRangeStart) {
                    this.currentDate = new Date(this.internalRangeStart);
                } else {
                    this.currentDate = new Date();
                }
            } else {
                // Сбрасываем режим выбора периода при закрытии
                if (this.selectingRange && !this.internalRangeEnd) {
                    this.internalRangeStart = null;
                }
            }
        },
        
        handleClickOutside(event) {
            if (this.showCalendar && !this.$el.contains(event.target)) {
                this.showCalendar = false;
                // Сбрасываем режим выбора периода при закрытии
                if (this.selectingRange && !this.internalRangeEnd) {
                    this.internalRangeStart = null;
                }
            }
        },
        
        setMode(mode) {
            this.selectingRange = mode === 'range';
            
            // Если переключаемся с периода на один день и есть выбранный период
            if (mode === 'single') {
                // Если в диапазоне выбран один день, используем его
                if (this.internalRangeStart && this.internalRangeEnd && 
                    this.areDatesEqual(this.internalRangeStart, this.internalRangeEnd)) {
                    this.internalSelectedDate = new Date(this.internalRangeStart);
                }
                this.internalRangeStart = null;
                this.internalRangeEnd = null;
            }
            // Если переключаемся с одного дня на период и есть выбранная дата
            else if (mode === 'range' && this.internalSelectedDate) {
                this.internalRangeStart = new Date(this.internalSelectedDate);
                this.internalRangeEnd = new Date(this.internalSelectedDate);
                this.internalSelectedDate = null;
            }
            
            this.activeQuickDate = null;
        },
        
        prevMonth() {
            this.currentDate = new Date(
                this.currentDate.getFullYear(),
                this.currentDate.getMonth() - 1,
                1
            );
        },
        
        nextMonth() {
            this.currentDate = new Date(
                this.currentDate.getFullYear(),
                this.currentDate.getMonth() + 1,
                1
            );
        },
        
        getDayClass(day) {
            const classes = {
                'day--current-month': day.isCurrentMonth,
                'day--today': this.isToday(day.date),
                'day--selected': this.isSelected(day.date),
                'day--in-range': this.isInRange(day.date),
                'day--range-start': this.isRangeStart(day.date),
                'day--range-end': this.isRangeEnd(day.date),
            };
            return classes;
        },
        
        isToday(date) {
            if (!date) return false;
            const today = new Date();
            today.setHours(0, 0, 0, 0);
            const checkDate = new Date(date);
            checkDate.setHours(0, 0, 0, 0);
            return checkDate.getTime() === today.getTime();
        },
        
        isSelected(date) {
            if (!date) return false;
            if (!this.selectingRange && this.internalSelectedDate) {
                const selected = new Date(this.internalSelectedDate);
                selected.setHours(0, 0, 0, 0);
                const checkDate = new Date(date);
                checkDate.setHours(0, 0, 0, 0);
                return checkDate.getTime() === selected.getTime();
            }
            return false;
        },
        
        isInRange(date) {
            if (!date || !this.internalRangeStart || !this.internalRangeEnd) return false;
            
            const checkDate = new Date(date);
            checkDate.setHours(0, 0, 0, 0);
            const start = new Date(this.internalRangeStart);
            start.setHours(0, 0, 0, 0);
            const end = new Date(this.internalRangeEnd);
            end.setHours(23, 59, 59, 999);
            
            return checkDate >= start && checkDate <= end;
        },
        
        isRangeStart(date) {
            if (!date || !this.internalRangeStart) return false;
            const checkDate = new Date(date);
            checkDate.setHours(0, 0, 0, 0);
            const start = new Date(this.internalRangeStart);
            start.setHours(0, 0, 0, 0);
            return checkDate.getTime() === start.getTime();
        },
        
        isRangeEnd(date) {
            if (!date || !this.internalRangeEnd) return false;
            const checkDate = new Date(date);
            checkDate.setHours(0, 0, 0, 0);
            const end = new Date(this.internalRangeEnd);
            end.setHours(0, 0, 0, 0);
            return checkDate.getTime() === end.getTime();
        },
        
        selectDay(day) {
            if (!day.isCurrentMonth || !day.date) return;
            
            if (!this.selectingRange) {
                // Выбор одного дня
                this.internalSelectedDate = new Date(day.date);
                this.internalRangeStart = null;
                this.internalRangeEnd = null;
                this.activeQuickDate = null;
            } else {
                // Выбор периода
                if (!this.internalRangeStart || (this.internalRangeStart && this.internalRangeEnd)) {
                    // Начинаем новый выбор периода
                    this.internalRangeStart = new Date(day.date);
                    this.internalRangeEnd = null;
                } else {
                    // Завершаем выбор периода
                    const selectedDate = new Date(day.date);
                    
                    if (selectedDate < this.internalRangeStart) {
                        // Если выбрана дата раньше начала, меняем местами
                        this.internalRangeEnd = new Date(this.internalRangeStart);
                        this.internalRangeStart = selectedDate;
                    } else {
                        this.internalRangeEnd = selectedDate;
                    }
                }
                this.activeQuickDate = null;
            }
        },
        
        areDatesEqual(date1, date2) {
            if (!date1 || !date2) return false;
            const d1 = new Date(date1);
            const d2 = new Date(date2);
            d1.setHours(0, 0, 0, 0);
            d2.setHours(0, 0, 0, 0);
            return d1.getTime() === d2.getTime();
        },
        
        setQuickDate(period) {
            this.activeQuickDate = period;
            const today = new Date();
            today.setHours(0, 0, 0, 0);
            let start, end;
            
            const periods = {
                today: () => {
                    const date = new Date(today);
                    return [date, date];
                },
                yesterday: () => {
                    const date = new Date(today);
                    date.setDate(today.getDate() - 1);
                    return [date, date];
                },
                tomorrow: () => {
                    const date = new Date(today);
                    date.setDate(today.getDate() + 1);
                    return [date, date];
                },
                dayBeforeYesterday: () => {
                    const date = new Date(today);
                    date.setDate(today.getDate() - 2);
                    return [date, date];
                },
                dayAfterTomorrow: () => {
                    const date = new Date(today);
                    date.setDate(today.getDate() + 2);
                    return [date, date];
                },
                thisWeek: () => {
                    const start = new Date(today);
                    const day = start.getDay() || 7; // 0 = воскресенье, 1 = понедельник
                    const diff = start.getDate() - day + 1; // Понедельник этой недели
                    start.setDate(diff);
                    
                    const end = new Date(start);
                    end.setDate(start.getDate() + 6); // Воскресенье этой недели
                    return [start, end];
                },
                lastWeek: () => {
                    const start = new Date(today);
                    const day = start.getDay() || 7;
                    const diff = start.getDate() - day - 6; // Понедельник прошлой недели
                    start.setDate(diff);
                    
                    const end = new Date(start);
                    end.setDate(start.getDate() + 6); // Воскресенье прошлой недели
                    return [start, end];
                },
                nextWeek: () => {
                    const start = new Date(today);
                    const day = start.getDay() || 7;
                    const diff = start.getDate() - day + 8; // Понедельник следующей недели
                    start.setDate(diff);
                    
                    const end = new Date(start);
                    end.setDate(start.getDate() + 6); // Воскресенье следующей недели
                    return [start, end];
                },
                thisMonth: () => [
                    new Date(today.getFullYear(), today.getMonth(), 1),
                    new Date(today.getFullYear(), today.getMonth() + 1, 0)
                ],
                lastMonth: () => [
                    new Date(today.getFullYear(), today.getMonth() - 1, 1),
                    new Date(today.getFullYear(), today.getMonth(), 0)
                ],
                nextMonth: () => [
                    new Date(today.getFullYear(), today.getMonth() + 1, 1),
                    new Date(today.getFullYear(), today.getMonth() + 2, 0)
                ],
                thisYear: () => [
                    new Date(today.getFullYear(), 0, 1),
                    new Date(today.getFullYear(), 11, 31)
                ],
                lastYear: () => [
                    new Date(today.getFullYear() - 1, 0, 1),
                    new Date(today.getFullYear() - 1, 11, 31)
                ],
            };
            
            [start, end] = periods[period]();
            
            // Убедимся, что время сброшено
            start.setHours(0, 0, 0, 0);
            end.setHours(23, 59, 59, 999);
            
            // Определяем, является ли период одним днем
            const isSingleDayPeriod = [
                'today', 'yesterday', 'tomorrow', 
                'dayBeforeYesterday', 'dayAfterTomorrow'
            ].includes(period);
            
            if (isSingleDayPeriod) {
                // Для выбора одного дня переключаем режим на "Один день"
                this.selectingRange = false;
                this.internalSelectedDate = new Date(start);
                this.internalRangeStart = null;
                this.internalRangeEnd = null;
            } else {
                // Для выбора периода используем обе даты
                this.selectingRange = true;
                this.internalRangeStart = new Date(start);
                this.internalRangeEnd = new Date(end);
                this.internalSelectedDate = null;
            }
            
            // Показываем месяц с начальной датой
            this.currentDate = new Date(start);
        },
        
        isQuickActive(period) {
            if (this.activeQuickDate === period) return true;
            
            // Также проверяем, соответствует ли текущий выбор какому-либо быстрому периоду
            if (period === 'today' && this.isToday(this.internalSelectedDate)) return true;
            if (period === 'yesterday') {
                const yesterday = new Date();
                yesterday.setDate(yesterday.getDate() - 1);
                yesterday.setHours(0, 0, 0, 0);
                if (this.internalSelectedDate && this.areDatesEqual(this.internalSelectedDate, yesterday)) return true;
            }
            
            return false;
        },
        
        applySelection() {
            if (!this.selectingRange && this.internalSelectedDate) {
                // Режим одного дня
                const date = new Date(this.internalSelectedDate);
                date.setHours(0, 0, 0, 0);
                this.$emit('update:selectedDate', date);
                this.$emit('update:dateRangeStart', null);
                this.$emit('update:dateRangeEnd', null);
            } else if (this.selectingRange && this.internalRangeStart && this.internalRangeEnd) {
                // Режим периода
                const start = new Date(this.internalRangeStart);
                start.setHours(0, 0, 0, 0);
                const end = new Date(this.internalRangeEnd);
                end.setHours(23, 59, 59, 999);
                
                this.$emit('update:selectedDate', null);
                this.$emit('update:dateRangeStart', start);
                this.$emit('update:dateRangeEnd', end);
            } else if (this.selectingRange && this.internalRangeStart) {
                // Если выбран только начальный день, делаем его и конечным
                const date = new Date(this.internalRangeStart);
                date.setHours(0, 0, 0, 0);
                this.$emit('update:selectedDate', null);
                this.$emit('update:dateRangeStart', date);
                this.$emit('update:dateRangeEnd', date);
            } else {
                // Ничего не выбрано
                this.$emit('update:selectedDate', null);
                this.$emit('update:dateRangeStart', null);
                this.$emit('update:dateRangeEnd', null);
            }
            
            this.$emit('apply');
            this.showCalendar = false;
        },
        
        clearSelection() {
            this.internalSelectedDate = null;
            this.internalRangeStart = null;
            this.internalRangeEnd = null;
            this.activeQuickDate = null;
            
            this.$emit('update:selectedDate', null);
            this.$emit('update:dateRangeStart', null);
            this.$emit('update:dateRangeEnd', null);
            this.$emit('clear');
        },
        
        formatDateForDisplay(date) {
            if (!date) return '';
            const d = new Date(date);
            return d.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },
    },
};
</script>

<style scoped>
.date-filter {
    position: relative;
    width: 215px;
}

.date-field {
    width: 215px;
    height: 35px;
    background-color: #FFF;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
    padding: 0 10px;
    cursor: pointer;
    position: relative;
    transition: border-color 0.2s ease;
}

.date-field:hover {
    border-color: #4F5BDF;
}

.field-wrapper {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 100%;
}

.field-input {
    font-size: 14px;
    color: #000;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 5px;
}

.field-input::placeholder {
    color: #a2a2a2;
}

.field-icon {
    width: 15px;
    height: 15px;
    opacity: 0.6;
    flex-shrink: 0;
}

/* Calendar modal */
.calendar-slide-enter-active,
.calendar-slide-leave-active {
    transition: all 0.3s ease;
}

.calendar-slide-enter-from,
.calendar-slide-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.calendar-modal {
    position: absolute;
    top: calc(100% + 5px);
    left: 0;
    z-index: 1001;
    width: 500px; /* Увеличили ширину для горизонтального расположения */
}

.calendar-container {
    background: white;
    border-radius: 12px;
    border: 1px solid #e6e6e6;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
    overflow: hidden;
}

/* Header - высота 40px */
.calendar-header {
    padding: 8px 16px;
    background: #f8f9ff;
    border-bottom: 1px solid #e6e6e6;
    height: 40px;
    display: flex;
    align-items: center;
}

.header-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
}

.date-display {
    font-weight: 600;
    color: #333;
    padding: 0 8px;
}

.current-month-year {
    font-size: 14px;
    font-weight: 600;
    white-space: nowrap;
}

.nav-btn {
    width: 28px;
    height: 28px;
    border-radius: 6px;
    border: 1px solid #e6e6e6;
    background: white;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s ease;
    flex-shrink: 0;
}

.nav-btn:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

/* Horizontal layout */
.calendar-body {
    display: flex;
    padding: 16px;
    gap: 16px;
    min-height: 300px; /* Фиксированная минимальная высота с учетом выбора даты */
}

/* Quick selection слева */
.quick-selection {
    flex: 0 0 160px; /* Фиксированная ширина для быстрых кнопок */
    border-right: 1px solid #f0f0f0;
    padding-right: 10px;
}

.quick-buttons-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    overflow-y: auto;
    max-height: 291px; /* Ограничиваем высоту списка */
    padding-right: 10px; /* Увеличили отступ для скролла */
}

.quick-btn {
    padding: 6px 8px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 500;
    color: #333;
    cursor: pointer;
    transition: all 0.2s ease;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-height: 28px;
    max-height: 27px; /* Добавлено: фиксированная высота */
    display: flex;
    align-items: center;
}

.quick-btn:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.quick-btn.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.quick-btn.active:hover {
    background: #3a45c0;
}

/* Calendar main справа */
.calendar-main {
    flex: 1;
    display: flex;
    flex-direction: column;
}

/* Mode switch */
.calendar-mode-switch {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
    flex-shrink: 0;
}

.mode-btn {
    flex: 1;
    padding: 6px 12px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 15px; /* Изменено: скругление 15px */
    font-size: 12px;
    font-weight: 500;
    color: #333;
    cursor: pointer;
    transition: all 0.2s ease;
    text-align: center;
}

.mode-btn:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.mode-btn.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.mode-btn.active:hover {
    background: #3a45c0;
}

.weekdays {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 2px;
    margin-bottom: 8px;
    flex-shrink: 0;
}

.weekday {
    text-align: center;
    font-size: 11px;
    color: #666;
    font-weight: 500;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.days-grid {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    grid-template-rows: repeat(6, 1fr); /* Всегда 6 строк */
    gap: 2px;
    flex: 1;
    min-height: 168px; /* 6 строк × 28px */
}

.day {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    font-weight: 500;
    color: #333;
    cursor: pointer;
    border-radius: 15px;
    position: relative;
    transition: all 0.2s ease;
    user-select: none;
    min-height: 24px;
}

.day--current-month {
    cursor: pointer;
}

.day:not(.day--current-month) {
    color: #d0d0d0;
    cursor: default;
}

.day--current-month:hover:not(.day--selected):not(.day--range-start):not(.day--range-end):not(.day--in-range) {
    background: #f5f5f5;
}

.day--today {
    color: #4F5BDF;
    font-weight: 600;
}

/* Один выбранный день - синий */
.day--selected,
.day--range-start,
.day--range-end {
    background: #4F5BDF !important; /* Синий цвет для всех выбранных дней */
    color: white !important;
    font-weight: 600;
}

.day--selected:hover,
.day--range-start:hover,
.day--range-end:hover {
    background: #3a45c0 !important; /* Темнее при наведении */
}

/* Дни внутри периода (между началом и концом) */
.day--in-range:not(.day--range-start):not(.day--range-end) {
    background: #f0f2ff; /* Светло-синий только для дней внутри периода */
    color: #333;
    border-radius: 0;
}

.day--in-range:not(.day--range-start):not(.day--range-end):hover {
    background: #e6e9ff;
}

/* Скругления для крайних дней периода */
.day--range-start {
    border-radius: 15px 5px 5px 15px;
}

.day--range-end {
    border-radius: 5px 15px 15px 5px;
}

.day--range-start.day--range-end {
    border-radius: 15px;
}

/* Selected range display - всегда отображается */
.selected-range {
    margin-top: 12px;
    padding: 8px;
    border: 1px solid #f0f0f0;
    border-radius: 15px;
    background: #fafafa;
    flex-shrink: 0;
    min-height: 36px; /* Фиксированная минимальная высота */
    display: flex;
    align-items: center;
}

.range-display {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    flex-wrap: wrap;
    width: 100%;
}

.range-label {
    font-size: 11px;
    color: #666;
    font-weight: 500;
    white-space: nowrap;
}

.range-date {
    font-size: 12px;
    color: #333;
    font-weight: 500;
    background: white;
    padding: 2px 6px;
    border-radius: 15px;
    border: 1px solid #e6e6e6;
    min-width: 70px;
    text-align: center;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
}

/* Actions */
.calendar-actions {
    display: flex;
    padding: 12px 16px;
    gap: 8px;
    border-top: 1px solid #e6e6e6;
    background: #f8f9ff;
}

.action-btn {
    flex: 1;
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
    border: none;
    outline: none;
    min-height: 32px;
}

.action-btn--clear {
    background: white;
    color: #666;
    border: 1px solid #e6e6e6;
}

.action-btn--clear:hover {
    background: #f5f5f5;
    border-color: #d0d0d0;
}

.action-btn--apply {
    background: #4F5BDF;
    color: white;
    border: 1px solid #4F5BDF;
}

.action-btn--apply:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

/* Scrollbar for quick buttons */
.quick-buttons-list::-webkit-scrollbar {
    width: 4px;
}

.quick-buttons-list::-webkit-scrollbar-track {
    background: #f5f5f5;
    border-radius: 2px;
}

.quick-buttons-list::-webkit-scrollbar-thumb {
    background: #c5c5c5;
    border-radius: 2px;
}

.quick-buttons-list::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}

/* Адаптивность */
@media (max-width: 768px) {
    .date-filter {
        width: 100%;
    }
    
    .calendar-modal {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        width: 90vw;
        max-width: 320px;
        z-index: 1002;
    }
    
    .calendar-body {
        flex-direction: column;
        padding: 12px;
        gap: 12px;
    }
    
    .quick-selection {
        flex: none;
        border-right: none;
        border-bottom: 1px solid #f0f0f0;
        padding-right: 0;
        padding-bottom: 12px;
    }
    
    .quick-buttons-list {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 6px;
        max-height: none;
        overflow-y: visible;
        padding-right: 0;
    }
    
    .calendar-main {
        width: 100%;
    }
    
    .action-btn {
        padding: 10px 12px;
        font-size: 11px;
    }
}
</style>