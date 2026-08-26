<template>
  <div class="date-picker-container">
    <div
      class="date-field"
      @click="toggleDropdown"
    >
      <span class="select-text">{{ displayText }}</span>
      <AppIcon
        name="calendar"
        class="select-icon"
        :class="{ 'select-icon--rotated': isOpen }"
      />
            
      <transition name="dropdown">
        <div 
          v-if="isOpen"
          class="custom-datepicker"
          @click.stop
        >
          <div class="datepicker-header">
            <h4 class="datepicker-title">
              Выберите период
            </h4>
            <button
              class="datepicker-close"
              @click="closeDatePicker"
            >
              ×
            </button>
          </div>
                    
          <div class="quick-buttons">
            <button
              class="quick-btn"
              @click="setQuickDate('today')"
            >
              Сегодня
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('yesterday')"
            >
              Вчера
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('tomorrow')"
            >
              Завтра
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('thisWeek')"
            >
              Эта неделя
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('lastWeek')"
            >
              Прошлая неделя
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('thisMonth')"
            >
              Этот месяц
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('lastMonth')"
            >
              Прошлый месяц
            </button>
            <button
              class="quick-btn"
              @click="setQuickDate('thisYear')"
            >
              Этот год
            </button>
          </div>
                    
          <div class="date-range-section">
            <div class="date-input-group">
              <label>С:</label>
              <input 
                v-model="dateRangeStartInput" 
                type="date"
                class="date-input"
                @change="updateDateRange"
              >
            </div>
            <div class="date-input-group">
              <label>ПО:</label>
              <input 
                v-model="dateRangeEndInput" 
                type="date"
                class="date-input"
                @change="updateDateRange"
              >
            </div>
          </div>
                    
          <div
            v-if="showCalendar"
            class="calendar-section"
          >
            <div class="calendar-header">
              <button
                class="calendar-nav-btn"
                @click="prevMonth"
              >
                &lt;
              </button>
              <span class="current-month">{{ monthNames[currentMonth] }} {{ currentYear }}</span>
              <button
                class="calendar-nav-btn"
                @click="nextMonth"
              >
                &gt;
              </button>
            </div>
            <div class="calendar-weekdays">
              <div
                v-for="day in ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']"
                :key="day"
                class="weekday"
              >
                {{ day }}
              </div>
            </div>
            <div class="calendar-days">
              <div 
                v-for="day in calendarDays" 
                :key="day.date"
                class="calendar-day"
                :class="getDayClasses(day)"
                @click="selectCalendarDate(day)"
              >
                {{ day.day }}
              </div>
            </div>
          </div>
                    
          <div class="datepicker-actions">
            <button
              class="apply-btn"
              @click="applyDateRange"
            >
              Применить
            </button>
            <button
              class="clear-btn"
              @click="clearDateRange"
            >
              Очистить
            </button>
          </div>
        </div>
      </transition>
    </div>
  </div>
</template>

<script>
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'DatePicker',
    components: { AppIcon },
    props: {
        value: {
            type: Object,
            default: () => ({})
        },
        showCalendar: {
            type: Boolean,
            default: true
        }
    },
    emits: ['apply', 'clear', 'input'],
    data() {
        return {
            isOpen: false,
            dateRangeStart: null,
            dateRangeEnd: null,
            dateRangeStartInput: '',
            dateRangeEndInput: '',
            selectedDate: null,
            currentMonth: new Date().getMonth(),
            currentYear: new Date().getFullYear(),
            monthNames: [
                'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
                'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'
            ],
            clickOutsideHandler: null
        };
    },
    computed: {
        displayText() {
            if (this.dateRangeStart && this.dateRangeEnd) {
                const start = this.formatDate(this.dateRangeStart);
                const end = this.formatDate(this.dateRangeEnd);
                return start === end ? start : `${start} - ${end}`;
            } else if (this.selectedDate) {
                return this.formatDate(this.selectedDate);
            }
            return 'Выберите дату';
        },
        
        calendarDays() {
            const days = [];
            const firstDayOfMonth = new Date(this.currentYear, this.currentMonth, 1);
            const lastDayOfMonth = new Date(this.currentYear, this.currentMonth + 1, 0);
            
            // Дни предыдущего месяца
            const firstDayOfWeek = firstDayOfMonth.getDay();
            const daysFromPrevMonth = firstDayOfWeek === 0 ? 6 : firstDayOfWeek - 1;
            
            const prevMonthLastDay = new Date(this.currentYear, this.currentMonth, 0).getDate();
            
            for (let i = daysFromPrevMonth; i > 0; i--) {
                const day = prevMonthLastDay - i + 1;
                const date = new Date(this.currentYear, this.currentMonth - 1, day);
                days.push({
                    day,
                    date: date,
                    isCurrentMonth: false,
                    isToday: false
                });
            }
            
            // Дни текущего месяца
            for (let i = 1; i <= lastDayOfMonth.getDate(); i++) {
                const date = new Date(this.currentYear, this.currentMonth, i);
                const today = new Date();
                days.push({
                    day: i,
                    date: date,
                    isCurrentMonth: true,
                    isToday: date.toDateString() === today.toDateString()
                });
            }
            
            // Дни следующего месяца
            const totalDays = days.length;
            const daysToAdd = 42 - totalDays; // 6 недель по 7 дней
            
            for (let i = 1; i <= daysToAdd; i++) {
                const date = new Date(this.currentYear, this.currentMonth + 1, i);
                days.push({
                    day: i,
                    date: date,
                    isCurrentMonth: false,
                    isToday: false
                });
            }
            
            return days;
        }
    },
    watch: {
        value: {
            immediate: true,
            handler(newValue) {
                if (newValue) {
                    if (newValue.selectedDate) {
                        this.selectedDate = new Date(newValue.selectedDate);
                        this.dateRangeStart = null;
                        this.dateRangeEnd = null;
                    } else if (newValue.dateRangeStart && newValue.dateRangeEnd) {
                        this.dateRangeStart = new Date(newValue.dateRangeStart);
                        this.dateRangeEnd = new Date(newValue.dateRangeEnd);
                        this.selectedDate = null;
                        this.dateRangeStartInput = this.formatDateForInput(this.dateRangeStart);
                        this.dateRangeEndInput = this.formatDateForInput(this.dateRangeEnd);
                    }
                }
            }
        },
        
        isOpen(newValue) {
            if (newValue) {
                this.$nextTick(() => {
                    this.setupClickOutside();
                });
            } else {
                this.removeClickOutside();
            }
        }
    },
    beforeUnmount() {
        this.removeClickOutside();
    },
    methods: {
        toggleDropdown() {
            this.isOpen = !this.isOpen;
        },
        
        closeDatePicker() {
            this.isOpen = false;
        },
        
        setupClickOutside() {
            this.clickOutsideHandler = (e) => {
                if (!this.$el.contains(e.target)) {
                    this.isOpen = false;
                }
            };
            setTimeout(() => {
                document.addEventListener('click', this.clickOutsideHandler);
            }, 0);
        },
        
        removeClickOutside() {
            if (this.clickOutsideHandler) {
                document.removeEventListener('click', this.clickOutsideHandler);
                this.clickOutsideHandler = null;
            }
        },
        
        formatDate(date) {
            if (!date) return '';
            if (typeof date === 'string') {
                date = new Date(date);
            }
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },
        
        formatDateForInput(date) {
            return date ? date.toISOString().split('T')[0] : '';
        },
        
        setQuickDate(period) {
            const today = new Date();
            let start, end;
            
            const periods = {
                today: () => [new Date(today), new Date(today)],
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
                thisWeek: () => {
                    const start = new Date(today);
                    start.setDate(today.getDate() - today.getDay() + (today.getDay() === 0 ? -6 : 1));
                    return [start, new Date(today)];
                },
                lastWeek: () => {
                    const start = new Date(today);
                    start.setDate(today.getDate() - today.getDay() - 6);
                    const end = new Date(start);
                    end.setDate(start.getDate() + 6);
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
                thisYear: () => [
                    new Date(today.getFullYear(), 0, 1),
                    new Date(today.getFullYear(), 11, 31)
                ]
            };
            
            [start, end] = periods[period]();
            start.setHours(0, 0, 0, 0);
            end.setHours(23, 59, 59, 999);
            
            this.dateRangeStart = start;
            this.dateRangeEnd = end;
            this.selectedDate = null;
            this.dateRangeStartInput = this.formatDateForInput(start);
            this.dateRangeEndInput = this.formatDateForInput(end);
        },
        
        updateDateRange() {
            if (this.dateRangeStartInput) {
                const start = new Date(this.dateRangeStartInput);
                start.setHours(0, 0, 0, 0);
                this.dateRangeStart = start;
                this.selectedDate = null;
            }
            if (this.dateRangeEndInput) {
                const end = new Date(this.dateRangeEndInput);
                end.setHours(23, 59, 59, 999);
                this.dateRangeEnd = end;
                this.selectedDate = null;
            }
        },
        
        applyDateRange() {
            this.updateDateRange();
            
            const result = {};
            if (this.selectedDate) {
                result.selectedDate = this.selectedDate;
                result.dateRangeStart = null;
                result.dateRangeEnd = null;
            } else if (this.dateRangeStart && this.dateRangeEnd) {
                result.selectedDate = null;
                result.dateRangeStart = this.dateRangeStart;
                result.dateRangeEnd = this.dateRangeEnd;
            }
            
            this.$emit('input', result);
            this.$emit('apply', result);
            this.isOpen = false;
        },
        
        clearDateRange() {
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.selectedDate = null;
            this.dateRangeStartInput = '';
            this.dateRangeEndInput = '';
            
            this.$emit('input', {});
            this.$emit('clear');
            this.isOpen = false;
        },
        
        prevMonth() {
            if (this.currentMonth === 0) {
                this.currentMonth = 11;
                this.currentYear--;
            } else {
                this.currentMonth--;
            }
        },
        
        nextMonth() {
            if (this.currentMonth === 11) {
                this.currentMonth = 0;
                this.currentYear++;
            } else {
                this.currentMonth++;
            }
        },
        
        getDayClasses(day) {
            const classes = [];
            
            if (!day.isCurrentMonth) {
                classes.push('other-month');
            }
            
            if (day.isToday) {
                classes.push('today');
            }
            
            if (this.isDateSelected(day.date)) {
                classes.push('selected');
            }
            
            if (this.isDateInRange(day.date)) {
                classes.push('in-range');
            }
            
            return classes.join(' ');
        },
        
        isDateSelected(date) {
            if (!this.selectedDate) return false;
            return date.toDateString() === this.selectedDate.toDateString();
        },
        
        isDateInRange(date) {
            if (!this.dateRangeStart || !this.dateRangeEnd) return false;
            
            const time = date.getTime();
            const startTime = this.dateRangeStart.getTime();
            const endTime = this.dateRangeEnd.getTime();
            
            return time >= startTime && time <= endTime;
        },
        
        selectCalendarDate(day) {
            if (!this.isDateSelected(day.date)) {
                this.selectedDate = new Date(day.date);
                this.dateRangeStart = null;
                this.dateRangeEnd = null;
                this.dateRangeStartInput = '';
                this.dateRangeEndInput = '';
            }
        },
        
        // Метод для сброса фильтра из родительского компонента
        reset() {
            this.clearDateRange();
        }
    }
};
</script>

<style scoped>
.date-picker-container {
    position: relative;
}

.date-field {
    width: 200px;
    height: 35px;
    background-color: var(--surface);
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    cursor: pointer;
    position: relative;
    transition: border-color 0.2s;
}

.date-field:hover {
    border-color: var(--border);
}

.select-text {
    font-size: 13px;
    color: var(--text);
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(100% - 20px);
}

.select-icon {
    width: 15px;
    height: 15px;
    transition: transform 0.3s ease;
    flex-shrink: 0;
    color: var(--text);
    stroke-width: 2.2;
}

.select-icon--rotated {
    transform: rotate(180deg);
}

/* Анимации для выпадающего меню */
.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
    transform-origin: top center;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: scale(0.95) translateY(-10px);
}

.dropdown-enter-to,
.dropdown-leave-from {
    opacity: 1;
    transform: scale(1) translateY(0);
}

.custom-datepicker {
    position: absolute;
    top: calc(100% + 5px);
    left: 0;
    width: 320px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 15px;
    z-index: 1001;
    box-shadow: 0 4px 12px var(--shadow-drop);
}

.datepicker-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 15px;
}

.datepicker-title {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
}

.datepicker-close {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s;
}

.datepicker-close:hover {
    color: var(--text);
}

.quick-buttons {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 6px;
    margin-bottom: 15px;
}

.quick-btn {
    padding: 6px 8px;
    border: 1px solid var(--border);
    background: var(--surface);
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    transition: all 0.2s;
    color: var(--text);
    height: 28px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.quick-btn:hover {
    background: var(--surface-2);
    border-color: var(--border);
}

.date-range-section {
    display: flex;
    gap: 10px;
    margin-bottom: 15px;
}

.date-input-group {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.date-input-group label {
    font-size: 12px;
    color: var(--text-muted);
    font-weight: 500;
}

.date-input {
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 13px;
    outline: none;
    height: 32px;
    background: var(--surface-2);
    transition: all 0.2s;
}

.date-input:focus {
    border-color: var(--accent);
    background: var(--surface);
    box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.1);
}

.calendar-section {
    margin-bottom: 15px;
}

.calendar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
}

.calendar-nav-btn {
    background: none;
    border: none;
    font-size: 16px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 4px 8px;
    border-radius: 4px;
    transition: all 0.2s;
}

.calendar-nav-btn:hover {
    background: var(--surface-2);
    color: var(--text);
}

.current-month {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
}

.calendar-weekdays {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 4px;
    margin-bottom: 8px;
}

.weekday {
    text-align: center;
    font-size: 11px;
    color: var(--text-muted);
    font-weight: 500;
    padding: 4px 0;
}

.calendar-days {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 4px;
}

.calendar-day {
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    cursor: pointer;
    border-radius: 6px;
    transition: all 0.2s;
    color: var(--text);
}

.calendar-day:hover:not(.other-month) {
    background: var(--accent-tint);
    color: var(--accent-text);
}

.calendar-day.other-month {
    color: var(--text-muted);
    cursor: default;
}

.calendar-day.today {
    background: var(--accent-tint);
    color: var(--accent-text);
    font-weight: 600;
}

.calendar-day.selected {
    background: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
}

.calendar-day.in-range {
    background: var(--accent-tint);
    color: var(--accent-text);
}

.datepicker-actions {
    display: flex;
    gap: 10px;
}

.apply-btn, .clear-btn {
    flex: 1;
    padding: 10px;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    transition: all 0.2s;
    height: 36px;
}

.apply-btn {
    background: var(--accent);
    color: var(--accent-contrast);
}

.apply-btn:hover {
    background: var(--accent-hover);
}

.clear-btn {
    background: var(--surface-2);
    color: var(--text-muted);
}

.clear-btn:hover {
    background: var(--border);
    color: var(--text);
}

/* Адаптивность */
@media (max-width: 768px) {
    .custom-datepicker {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        width: 90vw;
        max-width: 320px;
        max-height: 80vh;
        overflow-y: auto;
    }
    
    .quick-buttons {
        grid-template-columns: repeat(2, 1fr);
    }
    
    .date-field {
        width: 100%;
    }
}
</style>