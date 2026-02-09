<template>
    <div class="date-range-section">
        <div class="date__input">
            <label class="input__label">Дата действия <span class="required">*</span></label>
            <div class="date-container">
                <div class="date" v-if="!isOneDay">
                    <p class="date__text">с</p>
                    <div class="datepicker-wrapper">
                        <input 
                            class="input__date" 
                            placeholder="дд.мм.гг" 
                            :value="startDate"
                            @input="$emit('update:start-date', $event.target.value)"
                            @focus="openDatepicker('start')"
                            @blur="$emit('validate-date-range')"
                            :class="{ 'input--error': errors.startDate }"
                            readonly
                        />
                        <div v-if="showStartDatepicker" class="datepicker">
                            <div class="datepicker__header">
                                <button @click="prevMonth" class="datepicker__nav">
                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow datepicker__arrow--left" />
                                </button>
                                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                                <button @click="nextMonth" class="datepicker__nav">
                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow" />
                                </button>
                            </div>
                            <div class="datepicker__weekdays">
                                <div v-for="day in weekdays" :key="day" class="datepicker__weekday">{{ day }}</div>
                            </div>
                            <div class="datepicker__days">
                                <div 
                                    v-for="day in calendarDays" 
                                    :key="day.date"
                                    class="datepicker__day"
                                    :class="{
                                        'datepicker__day--selected': isSelectedDate(day.date),
                                        'datepicker__day--other-month': !day.isCurrentMonth,
                                        'datepicker__day--today': isToday(day.date)
                                    }"
                                    @click="selectStartDate(day.date)"
                                >
                                    {{ day.day }}
                                </div>
                            </div>
                        </div>
                    </div>
                    <p class="date__text">по</p>
                    <div class="datepicker-wrapper">
                        <input 
                            class="input__date" 
                            placeholder="дд.мм.гг" 
                            :value="endDate"
                            @input="$emit('update:end-date', $event.target.value)"
                            @focus="openDatepicker('end')"
                            @blur="$emit('validate-date-range')"
                            :class="{ 'input--error': errors.endDate }"
                            readonly
                        />
                        <div v-if="showEndDatepicker" class="datepicker">
                            <div class="datepicker__header">
                                <button @click="prevMonth" class="datepicker__nav">
                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow datepicker__arrow--left" />
                                </button>
                                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                                <button @click="nextMonth" class="datepicker__nav">
                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow" />
                                </button>
                            </div>
                            <div class="datepicker__weekdays">
                                <div v-for="day in weekdays" :key="day" class="datepicker__weekday">{{ day }}</div>
                            </div>
                            <div class="datepicker__days">
                                <div 
                                    v-for="day in calendarDays" 
                                    :key="day.date"
                                    class="datepicker__day"
                                    :class="{
                                        'datepicker__day--selected': isSelectedDate(day.date),
                                        'datepicker__day--other-month': !day.isCurrentMonth,
                                        'datepicker__day--today': isToday(day.date)
                                    }"
                                    @click="selectEndDate(day.date)"
                                >
                                    {{ day.day }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                <div v-else class="single-date">
                    <div class="datepicker-wrapper">
                        <input 
                            class="input__date" 
                            placeholder="дд.мм.гг" 
                            :value="singleDate"
                            @input="$emit('update:single-date', $event.target.value)"
                            @focus="openDatepicker('single')"
                            @blur="$emit('validate-field', 'singleDate')"
                            :class="{ 'input--error': errors.singleDate }"
                            readonly
                        />
                        <div v-if="showSingleDatepicker" class="datepicker">
                            <div class="datepicker__header">
                                <button @click="prevMonth" class="datepicker__nav">
                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow datepicker__arrow--left" />
                                </button>
                                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                                <button @click="nextMonth" class="datepicker__nav">
                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow" />
                                </button>
                            </div>
                            <div class="datepicker__weekdays">
                                <div v-for="day in weekdays" :key="day" class="datepicker__weekday">{{ day }}</div>
                            </div>
                            <div class="datepicker__days">
                                <div 
                                    v-for="day in calendarDays" 
                                    :key="day.date"
                                    class="datepicker__day"
                                    :class="{
                                        'datepicker__day--selected': isSelectedDate(day.date),
                                        'datepicker__day--other-month': !day.isCurrentMonth,
                                        'datepicker__day--today': isToday(day.date)
                                    }"
                                    @click="selectSingleDate(day.date)"
                                >
                                    {{ day.day }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                <div v-if="errors.startDate || errors.endDate || errors.singleDate" class="error-message">
                    {{ errors.startDate || errors.endDate || errors.singleDate }}
                </div>
            </div>
            <div class="one-day">
                <input 
                    type="checkbox" 
                    class="one-day__checkbox" 
                    :checked="isOneDay"
                    @change="$emit('update:is-one-day', $event.target.checked); $emit('validate-field', 'isOneDay')"
                />
                <p>однодневная заявка</p>
            </div>
        </div>
        <div class="date__input">
            <label class="input__label">Время пребывания (проезда) <span class="required">*</span></label>
            <div class="date">
                <p class="date__text">с</p>
                <input 
                    class="input__date" 
                    placeholder="00:00" 
                    :value="startTime"
                    @input="$emit('update:start-time', $event.target.value)"
                    @blur="$emit('validate-time-range')"
                    :class="{ 'input--error': errors.startTime }"
                    type="time"
                />
                <p class="date__text">по</p>
                <input 
                    class="input__date" 
                    placeholder="00:00" 
                    :value="endTime"
                    @input="$emit('update:end-time', $event.target.value)"
                    @blur="$emit('validate-time-range')"
                    :class="{ 'input--error': errors.endTime }"
                    type="time"
                />
            </div>
            <p class="time-message"></p>
            <div v-if="errors.startTime || errors.endTime" class="error-message">
                {{ errors.startTime || errors.endTime }}
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'DateRangeSection',
    props: {
        isOneDay: Boolean,
        startDate: String,
        endDate: String,
        singleDate: String,
        startTime: String,
        endTime: String,
        roofAccess: Boolean,
        freeParking: Boolean,
        notifySituationCenter: Boolean,
        errors: Object
    },
    emits: [
        'update:is-one-day',
        'update:start-date',
        'update:end-date',
        'update:single-date',
        'update:start-time',
        'update:end-time',
        'update:roof-access',
        'update:free-parking',
        'update:notify-situation-center',
        'validate-field',
        'validate-date-range',
        'validate-time-range'
    ],
    data() {
        const today = new Date();
        return {
            showStartDatepicker: false,
            showEndDatepicker: false,
            showSingleDatepicker: false,
            currentDate: today,
            weekdays: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
        }
    },
    computed: {
        currentYear() {
            return this.currentDate.getFullYear();
        },
        currentMonth() {
            const months = [
                'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
                'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'
            ];
            return months[this.currentDate.getMonth()];
        },
        calendarDays() {
            const year = this.currentDate.getFullYear();
            const month = this.currentDate.getMonth();
            
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            
            const days = [];
            
            const prevMonthLastDay = new Date(year, month, 0).getDate();
            const firstDayOfWeek = firstDay.getDay() === 0 ? 6 : firstDay.getDay() - 1;
            
            for (let i = firstDayOfWeek - 1; i >= 0; i--) {
                const date = new Date(year, month - 1, prevMonthLastDay - i);
                days.push({
                    day: date.getDate(),
                    date: this.formatDate(date),
                    isCurrentMonth: false
                });
            }
            
            for (let i = 1; i <= lastDay.getDate(); i++) {
                const date = new Date(year, month, i);
                days.push({
                    day: i,
                    date: this.formatDate(date),
                    isCurrentMonth: true
                });
            }
            
            const totalCells = 42;
            const nextMonthDays = totalCells - days.length;
            for (let i = 1; i <= nextMonthDays; i++) {
                const date = new Date(year, month + 1, i);
                days.push({
                    day: i,
                    date: this.formatDate(date),
                    isCurrentMonth: false
                });
            }
            
            return days;
        }
    },
    methods: {
        formatDate(date) {
            const day = date.getDate().toString().padStart(2, '0');
            const month = (date.getMonth() + 1).toString().padStart(2, '0');
            const year = date.getFullYear();
            return `${day}.${month}.${year}`;
        },
        
        isSelectedDate(date) {
            if (this.isOneDay) {
                return date === this.singleDate;
            } else {
                return date === this.startDate || date === this.endDate;
            }
        },
        
        isToday(date) {
            const today = this.formatDate(new Date());
            return date === today;
        },
        
        openDatepicker(type) {
            this.showStartDatepicker = false;
            this.showEndDatepicker = false;
            this.showSingleDatepicker = false;
            
            if (type === 'start') {
                this.showStartDatepicker = true;
            } else if (type === 'end') {
                this.showEndDatepicker = true;
            } else if (type === 'single') {
                this.showSingleDatepicker = true;
            }
        },
        
        selectStartDate(date) {
            this.$emit('update:start-date', date);
            this.showStartDatepicker = false;
            this.$emit('validate-date-range');
        },
        
        selectEndDate(date) {
            this.$emit('update:end-date', date);
            this.showEndDatepicker = false;
            this.$emit('validate-date-range');
        },
        
        selectSingleDate(date) {
            this.$emit('update:single-date', date);
            this.showSingleDatepicker = false;
            this.$emit('validate-field', 'singleDate');
        },
        
        prevMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() - 1, 1);
        },
        
        nextMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() + 1, 1);
        }
    },
    mounted() {
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.datepicker-wrapper')) {
                this.showStartDatepicker = false;
                this.showEndDatepicker = false;
                this.showSingleDatepicker = false;
            }
        });
    }
}
</script>

<style scoped>
.date-range-section {
    display: flex;
    gap: 30px;
    align-items: flex-start;
}

.date__input {
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
}

.date-container {
    min-height: 40px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    width: 250px;
}

.date {
    display: flex;
    align-items: center;
    gap: 5px;
    width: 100%;
}

.input__date {
    width: 105px;
    height: 40px;
    border: 1px solid #e6e6e6;
    outline: none;
    background: #FFF;
    border-radius: 10px;
    padding: 5px 10px;
}

.date__text {
    color: #4F5BDF;
    font-weight: 600;
    white-space: nowrap;
}

.one-day {
    display: flex;
    gap: 5px;
    align-items: center;
    margin-top: 5px;
}

.one-day p {
    font-size: 12px;
}

.one-day__checkbox {
    width: 13px;
    height: 13px;
    cursor: pointer;
}

/* Чекбоксы справа */
.additional-options {
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: 100%;
    height: 80px;
    justify-content:center;
}

.option-checkbox {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 12px;
    line-height: 1.2;
    height: 16px;
}

.option-checkbox input[type="checkbox"] {
    width: 12px;
    height: 12px;
    cursor: pointer;
    margin: 0;
    flex-shrink: 0;
}

.option-text {
    color: #333;
    font-size: 13px;
    white-space: nowrap;
}

/* Datepicker styles */
.datepicker-wrapper {
    position: relative;
    display: inline-block;
}

.datepicker {
    position: absolute;
    top: calc(100% + 10px);
    left: 0;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    padding: 10px;
    z-index: 1000;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
    min-width: 250px;
}

.datepicker__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
}

.datepicker__nav {
    background: none;
    border: none;
    cursor: pointer;
    padding: 5px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.datepicker__arrow {
    width: 10px;
    height: 10px;
}

.datepicker__arrow--left {
    transform: rotate(180deg);
}

.datepicker__month {
    font-weight: 600;
    font-size: 14px;
}

.datepicker__weekdays {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 5px;
    margin-bottom: 5px;
}

.datepicker__weekday {
    text-align: center;
    font-size: 12px;
    color: #a2a2a2;
    font-weight: 500;
}

.datepicker__days {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 5px;
}

.datepicker__day {
    text-align: center;
    padding: 5px;
    font-size: 12px;
    cursor: pointer;
    border-radius: 5px;
    transition: background-color 0.2s;
}

.datepicker__day:hover {
    background-color: #f0f0f0;
}

.datepicker__day--selected {
    background-color: #4F5BDF;
    color: white;
}

.datepicker__day--other-month {
    color: #ccc;
}

.datepicker__day--today {
    font-weight: bold;
    border: 1px solid #4F5BDF;
}

.single-date {
    display: flex;
    align-items: center;
    width: 100%;
}

.time-message {
    font-size: 10px;
    width: 250px;
}

.input--error {
    border-color: #ff4444;
}

.error-message {
    font-size: 11px;
    color: #ff4444;
    position: absolute;
    bottom: -15px;
    left: 0;
}

.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
}
</style>