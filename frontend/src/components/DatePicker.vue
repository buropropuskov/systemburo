<template>
    <div class="date-picker" @click.stop>
        <div class="date-picker__header">
            <button @click="prevMonth" class="nav-button">
                <img src="@/assets/icons/arrow.png" class="nav-icon nav-icon--left" />
            </button>
            <span class="month-title">{{ currentMonth }}</span>
            <button @click="nextMonth" class="nav-button">
                <img src="@/assets/icons/arrow.png" class="nav-icon" />
            </button>
        </div>
        <div class="date-picker__body">
            <div class="date-picker__weekdays">
                <div v-for="day in weekdays" :key="day" class="weekday">{{ day }}</div>
            </div>
            <div class="date-picker__days">
                <div 
                    v-for="day in calendarDays" 
                    :key="day.date.getTime()"
                    :class="{
                        'date-picker__day': true,
                        'date-picker__day--selected': isSelected(day.date),
                        'date-picker__day--in-range': isInRange(day.date),
                        'date-picker__day--other-month': !day.isCurrentMonth,
                        'date-picker__day--start': isRangeStart(day.date),
                        'date-picker__day--end': isRangeEnd(day.date)
                    }"
                    @click="selectDate(day.date)"
                >
                    {{ day.day }}
                </div>
            </div>
        </div>
        <div class="date-picker__footer">
            <div class="footer-buttons">
                <button @click="clear">Очистить</button>
                <button @click="apply" class="apply-btn">Применить</button>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    props: {
        selectedDate: Date,
        dateRangeStart: Date,
        dateRangeEnd: Date
    },
    data() {
        return {
            currentDate: new Date(),
            weekdays: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
            localSelectedDate: null,
            localDateRangeStart: null,
            localDateRangeEnd: null,
            isSelectingRange: false
        }
    },
    computed: {
        currentMonth() {
            const month = this.currentDate.toLocaleString('ru-RU', { month: 'long' });
            const year = this.currentDate.getFullYear();
            return month.charAt(0).toUpperCase() + month.slice(1) + ' ' + year;
        },
        calendarDays() {
            const year = this.currentDate.getFullYear();
            const month = this.currentDate.getMonth();
            
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            
            const firstDayWeekday = firstDay.getDay() === 0 ? 6 : firstDay.getDay() - 1;
            
            const days = [];
            
            const prevMonthLastDay = new Date(year, month, 0).getDate();
            for (let i = prevMonthLastDay - firstDayWeekday + 1; i <= prevMonthLastDay; i++) {
                days.push({
                    day: i,
                    date: new Date(year, month - 1, i),
                    isCurrentMonth: false
                });
            }
            
            for (let i = 1; i <= lastDay.getDate(); i++) {
                days.push({
                    day: i,
                    date: new Date(year, month, i),
                    isCurrentMonth: true
                });
            }
            
            const totalCells = 35;
            const nextMonthDays = totalCells - days.length;
            for (let i = 1; i <= nextMonthDays; i++) {
                days.push({
                    day: i,
                    date: new Date(year, month + 1, i),
                    isCurrentMonth: false
                });
            }
            
            return days;
        }
    },
    methods: {
        formatDate(date) {
            if (!date) return '';
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },
        
        selectDate(date) {
            // Если нет выбранных дат - начинаем выбор
            if (!this.localSelectedDate && !this.localDateRangeStart) {
                this.localSelectedDate = date;
                this.isSelectingRange = false;
                return;
            }
            
            // Если выбрана одиночная дата и кликаем на другую дату - переключаемся в режим диапазона
            if (this.localSelectedDate && !this.isSelectingRange) {
                this.localDateRangeStart = this.localSelectedDate;
                this.localSelectedDate = null;
                this.localDateRangeEnd = date;
                this.isSelectingRange = true;
                
                // Сортируем даты в правильном порядке
                if (this.localDateRangeStart > this.localDateRangeEnd) {
                    [this.localDateRangeStart, this.localDateRangeEnd] = [this.localDateRangeEnd, this.localDateRangeStart];
                }
                return;
            }
            
            // Если уже выбран диапазон и кликаем на третью дату - сбрасываем и начинаем заново
            if (this.isSelectingRange && this.localDateRangeStart && this.localDateRangeEnd) {
                this.localSelectedDate = date;
                this.localDateRangeStart = null;
                this.localDateRangeEnd = null;
                this.isSelectingRange = false;
                return;
            }
            
            // Если выбрана только начальная дата диапазона
            if (this.isSelectingRange && this.localDateRangeStart && !this.localDateRangeEnd) {
                this.localDateRangeEnd = date;
                
                // Сортируем даты в правильном порядке
                if (this.localDateRangeStart > this.localDateRangeEnd) {
                    [this.localDateRangeStart, this.localDateRangeEnd] = [this.localDateRangeEnd, this.localDateRangeStart];
                }
            }
        },
        
        isSelected(date) {
            return this.localSelectedDate && this.localSelectedDate.toDateString() === date.toDateString();
        },
        
        isInRange(date) {
            if (!this.localDateRangeStart || !this.localDateRangeEnd) return false;
            return date >= this.localDateRangeStart && date <= this.localDateRangeEnd;
        },
        
        isRangeStart(date) {
            return this.localDateRangeStart && this.localDateRangeStart.toDateString() === date.toDateString();
        },
        
        isRangeEnd(date) {
            return this.localDateRangeEnd && this.localDateRangeEnd.toDateString() === date.toDateString();
        },
        
        prevMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() - 1, 1);
        },
        
        nextMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() + 1, 1);
        },
        
        clear() {
            this.localSelectedDate = null;
            this.localDateRangeStart = null;
            this.localDateRangeEnd = null;
            this.isSelectingRange = false;
            this.$emit('clear');
        },
        
        apply() {
            if (this.localSelectedDate) {
                this.$emit('update:selectedDate', this.localSelectedDate);
            } else if (this.localDateRangeStart && this.localDateRangeEnd) {
                this.$emit('update:dateRange', {
                    start: this.localDateRangeStart,
                    end: this.localDateRangeEnd
                });
            }
            this.$emit('apply');
        }
    },
    watch: {
        selectedDate: {
            immediate: true,
            handler(newVal) {
                this.localSelectedDate = newVal;
                if (newVal) this.isSelectingRange = false;
            }
        },
        dateRangeStart: {
            immediate: true,
            handler(newVal) {
                this.localDateRangeStart = newVal;
                if (newVal) this.isSelectingRange = true;
            }
        },
        dateRangeEnd: {
            immediate: true,
            handler(newVal) {
                this.localDateRangeEnd = newVal;
            }
        }
    }
}
</script>

<style scoped>
.date-picker {
    position: absolute;
    top: calc(100% + 15px);
    left: 0;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    padding: 15px;
    z-index: 1000;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
    width: 300px;
}

.date-picker__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 15px;
}

.nav-button {
    background: none;
    border: none;
    cursor: pointer;
    padding: 5px;
    border-radius: 5px;
}

.nav-button:hover {
    background-color: #f0f0f0;
}

.nav-icon {
    width: 7px;
    height: 7px;
}

.nav-icon--left {
    transform: rotate(180deg);
}

.month-title {
    font-weight: 500;
    font-size: 16px;
}

.date-picker__weekdays {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 5px;
    margin-bottom: 10px;
}

.weekday {
    text-align: center;
    font-size: 12px;
    color: #a2a2a2;
    font-weight: 500;
}

.date-picker__days {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 5px;
}

.date-picker__day {
    text-align: center;
    padding: 8px;
    cursor: pointer;
    border-radius: 5px;
    font-size: 14px;
    position: relative;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.date-picker__day:hover {
    background-color: #f2f2f2;
}

.date-picker__day--selected {
    background-color: #4F5BDF;
    color: white;
}

.date-picker__day--in-range {
    background-color: #e6e8ff;
}

.date-picker__day--start {
    background-color: #4F5BDF;
    color: white;
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
}

.date-picker__day--end {
    background-color: #4F5BDF;
    color: white;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
}

.date-picker__day--other-month {
    color: #ccc;
}

.date-picker__footer {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid #e6e6e6;
}

.footer-buttons {
    display: flex;
    gap: 10px;
}

.footer-buttons button {
    padding: 8px 12px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 5px;
    cursor: pointer;
    font-size: 14px;
}

.footer-buttons button:hover {
    background-color: #f2f2f2;
}

.apply-btn {
    background-color: #4F5BDF !important;
    color: white !important;
    border: none !important;
}

.apply-btn:hover {
    background-color: #7580fc !important;
}
</style>