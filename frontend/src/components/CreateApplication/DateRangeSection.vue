<template>
  <div class="date-range-section">
    <div
      v-if="fieldVisible('entry_date_from')"
      class="date__input"
    >
      <div class="date__label-row">
        <label class="input__label">Дата действия <span
          v-if="fieldRequired('entry_date_from')"
          class="required"
        >*</span></label>
        <div class="qd-dropdown">
          <button
            ref="qdTrigger"
            type="button"
            class="qd-trigger"
            :class="{ 'qd-trigger--open': showQuickMenu }"
            @click.stop="toggleQuickMenu"
          >
            <span>Быстрый выбор</span>
            <svg
              class="qd-caret"
              width="10"
              height="10"
              viewBox="0 0 24 24"
              fill="none"
            >
              <path
                d="M6 9L12 15L18 9"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <Teleport to="body">
            <transition name="qd-fade">
              <div
                v-if="showQuickMenu"
                class="qd-menu"
                :style="qdMenuStyle"
                @click.stop
              >
                <button
                  type="button"
                  class="qd-item"
                  @click="setQuickDate('today')"
                >
                  <span class="qd-item__label">Сегодня</span>
                  <span class="qd-item__date">{{ quickDateOptions.today }}</span>
                </button>
                <button
                  type="button"
                  class="qd-item"
                  @click="setQuickDate('tomorrow')"
                >
                  <span class="qd-item__label">Завтра</span>
                  <span class="qd-item__date">{{ quickDateOptions.tomorrow }}</span>
                </button>
                <button
                  type="button"
                  class="qd-item"
                  @click="setQuickDate('after-tomorrow')"
                >
                  <span class="qd-item__label">Послезавтра</span>
                  <span class="qd-item__date">{{ quickDateOptions.afterTomorrow }}</span>
                </button>
                <div class="qd-sep" />
                <button
                  type="button"
                  class="qd-item"
                  @click="setQuickDate('current-month')"
                >
                  <span class="qd-item__label">{{ quickDateOptions.currentMonthLabel }}</span>
                  <span class="qd-item__date">{{ quickDateOptions.currentMonthRange }}</span>
                </button>
                <button
                  type="button"
                  class="qd-item"
                  @click="setQuickDate('next-month')"
                >
                  <span class="qd-item__label">{{ quickDateOptions.nextMonthLabel }}</span>
                  <span class="qd-item__date">{{ quickDateOptions.nextMonthRange }}</span>
                </button>
              </div>
            </transition>
          </Teleport>
        </div>
      </div>
      <div class="date-container">
        <div
          v-if="!isOneDay"
          class="date"
        >
          <p class="date__text">
            с
          </p>
          <div class="datepicker-wrapper">
            <input
              ref="startDateInput"
              class="input__date"
              placeholder="дд.мм.гггг"
              :value="startDate"
              :class="{ 'input--error': errors.startDate || errors.periodInvalid }"
              maxlength="10"
              @input="onStartDateInput"
              @focus="openDatepicker('start')"
              @blur="handleDateBlur('start')"
              @keydown="preventNonNumeric"
              @keydown.tab="onTabFromStart"
              @paste="preventNonNumericPaste"
            >
          </div>
          <p class="date__text">
            по
          </p>
          <div class="datepicker-wrapper">
            <input
              ref="endDateInput"
              class="input__date"
              placeholder="дд.мм.гггг"
              :value="endDate"
              :class="{ 'input--error': errors.endDate || errors.periodInvalid }"
              maxlength="10"
              @input="onEndDateInput"
              @focus="openDatepicker('end')"
              @blur="handleDateBlur('end')"
              @keydown="preventNonNumeric"
              @paste="preventNonNumericPaste"
            >
          </div>
        </div>
        <div
          v-else
          class="single-date"
        >
          <div class="datepicker-wrapper">
            <input
              ref="singleDateInput"
              class="input__date"
              placeholder="дд.мм.гггг"
              :value="singleDate"
              :class="{ 'input--error': errors.singleDate || errors.periodInvalid }"
              maxlength="10"
              @input="onSingleDateInput"
              @focus="openDatepicker('single')"
              @blur="handleDateBlur('single')"
              @keydown="preventNonNumeric"
              @paste="preventNonNumericPaste"
            >
          </div>
        </div>
        <div
          v-if="errors.startDate || errors.endDate || errors.singleDate"
          class="error-message date-error"
        >
          {{ errors.startDate || errors.endDate || errors.singleDate }}
        </div>
        <!-- Крайний срок «По факту» подсказкой над полями: системный паттерн
             hints.css, is-hinted держит её открытой без наведения, --danger
             красит в цвет ошибки. Полное правило объясняет панель (#2320). -->
        <div
          v-if="errors.periodHint"
          class="hint-anchor hint-anchor--danger is-hinted period-hint-anchor"
          :data-hint="errors.periodHint"
          role="status"
        />
        <Teleport to="body">
          <!-- Мобилка: затемнение под листом - календарь не сливается с формой за ним. -->
          <transition name="datepicker-overlay-fade">
            <div
              v-if="showStartDatepicker || showEndDatepicker || showSingleDatepicker"
              class="datepicker-overlay"
              @click="closeDatepicker"
            />
          </transition>
          <transition name="calendar">
            <div
              v-if="showStartDatepicker || showEndDatepicker || showSingleDatepicker"
              class="datepicker"
              :class="{ 'is-dragging': sheetDragging }"
              :style="sheetOffset ? { ...datepickerStyle, transform: `translateY(${sheetOffset}px)` } : datepickerStyle"
              @click.stop
              @touchstart="onSheetTouchStart"
              @touchmove="onSheetTouchMove"
              @touchend="onSheetTouchEnd"
            >
              <!-- Ползунок bottom-sheet - виден только на мобилке (тянуть для закрытия). -->
              <div
                class="sheet-handle"
                aria-hidden="true"
              />
              <div class="datepicker__header">
                <button
                  class="datepicker__nav"
                  tabindex="-1"
                  @click="prevMonth"
                >
                  <svg
                    width="12"
                    height="12"
                    viewBox="0 0 24 24"
                    fill="none"
                  >
                    <path
                      d="M15 18L9 12L15 6"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                </button>
                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                <button
                  class="datepicker__nav"
                  tabindex="-1"
                  @click="nextMonth"
                >
                  <svg
                    width="12"
                    height="12"
                    viewBox="0 0 24 24"
                    fill="none"
                  >
                    <path
                      d="M9 18L15 12L9 6"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                </button>
              </div>
              <div class="datepicker__weekdays">
                <div
                  v-for="day in weekdays"
                  :key="day"
                  class="datepicker__weekday"
                >
                  {{ day }}
                </div>
              </div>
              <div
                ref="sheetBody"
                class="datepicker__days"
              >
                <div
                  v-for="day in calendarDays"
                  :key="day.date"
                  class="datepicker__day"
                  :class="getDayClass(day, activeDateValue)"
                  @click="selectActiveDate(day)"
                >
                  {{ day.day }}
                </div>
              </div>
            </div>
          </transition>
        </Teleport>
      </div>
      <div class="one-day">
        <input
          type="checkbox"
          class="one-day__checkbox"
          :checked="isOneDay"
          @change="onCheckboxChange"
        >
        <p>однодневная заявка</p>
      </div>
    </div>
    <div
      v-if="fieldVisible('entry_time_from')"
      class="date__input time-section"
    >
      <label class="input__label">Время пребывания (проезда) <span
        v-if="fieldRequired('entry_time_from')"
        class="required"
      >*</span></label>
      <div class="time-wrapper">
        <div class="time-input-group">
          <p class="date__text">
            с
          </p>
          <input
            ref="startTimeInput"
            class="input__time"
            placeholder="чч:мм"
            :value="startTime"
            :class="{ 'input--error': errors.startTime }"
            inputmode="numeric"
            maxlength="5"
            @input="onStartTimeInput"
            @blur="formatTimeField('start')"
            @keydown="preventNonNumeric"
            @paste="preventNonNumericPaste"
          >
        </div>
        <div class="time-input-group">
          <p class="date__text">
            по
          </p>
          <input
            ref="endTimeInput"
            class="input__time"
            placeholder="чч:мм"
            :value="endTime"
            :class="{ 'input--error': errors.endTime }"
            inputmode="numeric"
            maxlength="5"
            @input="onEndTimeInput"
            @blur="formatTimeField('end')"
            @keydown="preventNonNumeric"
            @paste="preventNonNumericPaste"
          >
        </div>
      </div>
      <p class="time-message" />
      <div
        v-if="errors.startTime || errors.endTime"
        class="error-message time-error"
      >
        {{ errors.startTime || errors.endTime }}
      </div>
    </div>
    <div
      v-if="fieldVisible('roof_access') || fieldVisible('free_parking')"
      class="date__input"
    >
      <label class="input__label">Дополнительно</label>
      <div class="additional-options">
        <ToggleSwitch
          v-if="fieldVisible('roof_access')"
          :model-value="roofAccess"
          @update:model-value="$emit('update:roof-access', $event)"
        >
          Доступ на крышу
        </ToggleSwitch>
        <ToggleSwitch
          v-if="fieldVisible('free_parking')"
          :model-value="freeParking"
          @update:model-value="$emit('update:free-parking', $event)"
        >
          Бесплатная парковка
        </ToggleSwitch>
      </div>
    </div>
  </div>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref } from 'vue';
import { useFieldConfig } from '@/composables/useFieldConfig';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { getViewportZoom } from '@/utils/viewportScale';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';

export default {
    name: 'DateRangeSection',
    components: { ToggleSwitch },
    props: {
        isOneDay: Boolean,
        startDate: { type: String, default: null },
        endDate: { type: String, default: null },
        singleDate: { type: String, default: null },
        startTime: { type: String, default: null },
        endTime: { type: String, default: null },
        roofAccess: Boolean,
        freeParking: Boolean,
        errors: { type: Object, default: () => ({}) },
        // Настройка полей выбранного шаблона (#529): { [fieldKey]: { visible, required } }.
        // Потребляется через composable useFieldConfig (setup); дата/время реестром
        // залочены (всегда visible+required), поэтому при пустом конфиге поведение прежнее.
        fieldConfig: { type: Object, default: () => ({}) }
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
        'validate-field',
        'validate-date-range',
        'validate-time-range'
    ],
    setup(props) {
        // Геттер, а не props.fieldConfig напрямую - сохраняет реактивность пропса в хелперах (#529).
        const fieldConfig = useFieldConfig(() => props.fieldConfig);
        // Свайп-вниз-закрытие мобильного листа календаря - общий useSwipeDismiss (как
        // BaseModal/DateFilter). Композабл в setup, состояние календаря - в data, поэтому
        // закрытие идёт через счётчик-сигнал, который гасит watch в Options.
        const sheetBody = ref(null);
        const closeSignal = ref(0);
        const swipe = useSwipeDismiss(() => { closeSignal.value += 1; }, {
            handleSelector: '.sheet-handle',
            getScrollTop: () => sheetBody.value?.scrollTop ?? 0,
        });
        return {
            ...fieldConfig,
            sheetBody,
            closeSignal,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            resetSheetSwipe: swipe.reset,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
        };
    },
    data() {
        const today = new Date();
        return {
            showStartDatepicker: false,
            showEndDatepicker: false,
            showSingleDatepicker: false,
            currentDate: today,
            weekdays: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
            internalToggle: false,
            showQuickMenu: false,
            qdMenuStyle: {},
            datepickerStyle: {}
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
        // Опции "Быстрого выбора": полные даты для одиночных дней и календарных периодов.
        // "На <тек.месяц>" = сегодня..конец месяца, "На <след.месяц>" = весь следующий месяц.
        quickDateOptions() {
            const today = new Date();
            today.setHours(0, 0, 0, 0);
            const months = [
                'январь', 'февраль', 'март', 'апрель', 'май', 'июнь',
                'июль', 'август', 'сентябрь', 'октябрь', 'ноябрь', 'декабрь'
            ];
            const tomorrow = new Date(today); tomorrow.setDate(today.getDate() + 1);
            const afterTomorrow = new Date(today); afterTomorrow.setDate(today.getDate() + 2);
            const currentMonthEnd = new Date(today.getFullYear(), today.getMonth() + 1, 0);
            const nextMonthStart = new Date(today.getFullYear(), today.getMonth() + 1, 1);
            const nextMonthEnd = new Date(today.getFullYear(), today.getMonth() + 2, 0);
            return {
                today: this.formatDate(today),
                tomorrow: this.formatDate(tomorrow),
                afterTomorrow: this.formatDate(afterTomorrow),
                currentMonthLabel: 'На ' + months[today.getMonth()],
                currentMonthRange: `${this.formatDate(today)} - ${this.formatDate(currentMonthEnd)}`,
                nextMonthLabel: 'На ' + months[nextMonthStart.getMonth()],
                nextMonthRange: `${this.formatDate(nextMonthStart)} - ${this.formatDate(nextMonthEnd)}`
            };
        },
        activeDateValue() {
            if (this.showStartDatepicker) return this.startDate;
            if (this.showEndDatepicker) return this.endDate;
            if (this.showSingleDatepicker) return this.singleDate;
            return '';
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
    watch: {
        // Свайп-вниз по листу календаря (сигнал из setup) - закрываем календарь.
        closeSignal() {
            this.closeDatepicker();
        },
        startDate(newVal) {
            if (this.internalToggle) return;
            if (newVal && this.endDate && newVal === this.endDate && !this.isOneDay) {
                this.toggleOneDay(true);
            }
        },
        endDate(newVal) {
            if (this.internalToggle) return;
            if (newVal && this.startDate && newVal === this.startDate && !this.isOneDay) {
                this.toggleOneDay(true);
            }
        },
        singleDate() {
            if (this.internalToggle) return;
            this.$emit('validate-field', 'singleDate');
        }
        // Намеренно не реагируем на startTime/endTime во время ввода:
        // авто-перенос на следующий день делается ТОЛЬКО onBlur (см. formatTimeField).
        // Иначе при наборе "1" в endTime система пыталась сразу переносить дату.
    },
    mounted() {
        document.addEventListener('click', (e) => {
            // Календарь телепортирован в body (вне .datepicker-wrapper) и имеет @click.stop,
            // поэтому клики внутри него не всплывают сюда - дополнительное исключение не нужно.
            // Но если клик всё же дошёл и попал в .datepicker - не закрываем.
            if (!e.target.closest('.datepicker-wrapper') && !e.target.closest('.datepicker')) {
                this.closeDatepicker();
            }
            // Меню "Быстрый выбор" телепортится в body, поэтому исключаем и .qd-menu.
            if (!e.target.closest('.qd-dropdown') && !e.target.closest('.qd-menu')) {
                this.showQuickMenu = false;
            }
        });
        // Меню позиционируется fixed от триггера - при скролле отрываться нельзя, закрываем.
        window.addEventListener('scroll', this.closeQuickMenuOnScroll, true);
        document.addEventListener('keydown', this.handleDatepickerEscape);
        this.validateDateRange();
        this.validateTimeCrossing();
    },
    beforeUnmount() {
        window.removeEventListener('scroll', this.closeQuickMenuOnScroll, true);
        document.removeEventListener('keydown', this.handleDatepickerEscape);
        releaseBodyScrollLock(this);
    },
    methods: {
        // "Быстрый выбор": меню телепортится в body (иначе тонет под гейтом/инпутами
        // из-за вложенных stacking-контекстов). Позицию считаем от триггера.
        toggleQuickMenu() {
            if (this.showQuickMenu) {
                this.showQuickMenu = false;
                return;
            }
            // Взаимоисключение: "Быстрый выбор" закрывает любой открытый календарь.
            this.closeDatepicker();
            const btn = this.$refs.qdTrigger;
            if (!btn) return;
            // Меню телепортится в body внутри зазумленного <html> - rect (device-px)
            // приводим к layout-px делением на zoom; отступ от края - константа в
            // layout-px, её НЕ делим.
            const z = getViewportZoom();
            const r = btn.getBoundingClientRect();
            const gutter = 8;
            const viewportWidth = window.innerWidth / z;
            // Крепим меню правым краем к правому краю триггера: ширина у меню по
            // содержимому (строка длиннее у длинных месяцев), заранее её не знаем.
            const right = Math.max(gutter, viewportWidth - r.right / z);
            this.qdMenuStyle = {
                position: 'fixed',
                top: `${Math.round(r.bottom / z + 6)}px`,
                left: 'auto',
                right: `${Math.round(right)}px`,
                maxWidth: `${Math.round(viewportWidth - right - gutter)}px`,
                zIndex: 12000
            };
            this.showQuickMenu = true;
        },
        closeQuickMenuOnScroll() {
            if (this.showQuickMenu) this.showQuickMenu = false;
            // Календарь тоже fixed - при скролле позиция отрывается от инпута, закрываем.
            if (this.showStartDatepicker || this.showEndDatepicker || this.showSingleDatepicker) {
                this.closeDatepicker();
            }
        },
        setQuickDate(kind) {
            const today = new Date();
            today.setHours(0, 0, 0, 0);
            this.internalToggle = true;
            try {
                if (kind === 'today' || kind === 'tomorrow' || kind === 'after-tomorrow') {
                    const offset = kind === 'today' ? 0 : kind === 'tomorrow' ? 1 : 2;
                    const target = new Date(today);
                    target.setDate(today.getDate() + offset);
                    this.$emit('update:start-date', '');
                    this.$emit('update:end-date', '');
                    this.$emit('update:single-date', this.formatDate(target));
                    this.$emit('update:is-one-day', true);
                } else if (kind === 'current-month' || kind === 'next-month') {
                    const start = kind === 'current-month'
                        ? today
                        : new Date(today.getFullYear(), today.getMonth() + 1, 1);
                    const end = kind === 'current-month'
                        ? new Date(today.getFullYear(), today.getMonth() + 1, 0)
                        : new Date(today.getFullYear(), today.getMonth() + 2, 0);
                    this.$emit('update:single-date', '');
                    this.$emit('update:is-one-day', false);
                    this.$emit('update:start-date', this.formatDate(start));
                    this.$emit('update:end-date', this.formatDate(end));
                }
            } finally {
                this.showQuickMenu = false;
                this.$nextTick(() => {
                    this.internalToggle = false;
                    this.$emit('validate-field', 'isOneDay');
                    this.$emit('validate-date-range');
                    this.$emit('validate-time-range');
                });
            }
        },

        onTabFromStart(e) {
            if (!e.shiftKey && this.$refs.endDateInput) {
                e.preventDefault();
                this.$refs.endDateInput.focus();
            }
        },

        preventNonNumeric(e) {
            const controlKeys = ['Backspace', 'Delete', 'Tab', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'Enter', 'Escape'];
            if (controlKeys.includes(e.key)) return;

            if (e.target.classList.contains('input__date')) {
                if (!/^[0-9.]$/.test(e.key) && e.key.length === 1) {
                    e.preventDefault();
                }
            } else if (e.target.classList.contains('input__time')) {
                if (!/^[0-9:]$/.test(e.key) && e.key.length === 1) {
                    e.preventDefault();
                }
            }
        },

        preventNonNumericPaste(e) {
            const pastedText = (e.clipboardData || window.clipboardData).getData('text');

            if (e.target.classList.contains('input__date')) {
                const cleaned = pastedText.replace(/[^0-9.]/g, '');
                if (cleaned !== pastedText) {
                    e.preventDefault();
                    const start = e.target.selectionStart;
                    const end = e.target.selectionEnd;
                    const currentValue = e.target.value;
                    const newValue = currentValue.slice(0, start) + cleaned + currentValue.slice(end);
                    e.target.value = newValue;
                    e.target.dispatchEvent(new Event('input'));
                }
            } else if (e.target.classList.contains('input__time')) {
                const cleaned = pastedText.replace(/[^0-9:]/g, '');
                if (cleaned !== pastedText) {
                    e.preventDefault();
                    const start = e.target.selectionStart;
                    const end = e.target.selectionEnd;
                    const currentValue = e.target.value;
                    const newValue = currentValue.slice(0, start) + cleaned + currentValue.slice(end);
                    e.target.value = newValue;
                    e.target.dispatchEvent(new Event('input'));
                }
            }
        },

        formatDate(date) {
            const day = date.getDate().toString().padStart(2, '0');
            const month = (date.getMonth() + 1).toString().padStart(2, '0');
            const year = date.getFullYear();
            return `${day}.${month}.${year}`;
        },

        parseDate(dateStr) {
            if (!dateStr) return null;
            const parts = dateStr.split('.');
            if (parts.length !== 3) return null;
            let day = parseInt(parts[0], 10);
            let month = parseInt(parts[1], 10) - 1;
            let year = parseInt(parts[2], 10);
            if (isNaN(day) || isNaN(month) || isNaN(year)) return null;
            if (year < 100) year += 2000;
            const date = new Date(year, month, day);
            if (date.getDate() !== day || date.getMonth() !== month || date.getFullYear() !== year) return null;
            return date;
        },

        formatDateWithDots(input) {
            if (!input) return '';
            const digits = input.replace(/[^\d]/g, '');
            if (digits.length === 0) return '';

            let result = digits.slice(0, 2);
            if (digits.length >= 3) {
                result += '.' + digits.slice(2, 4);
                if (digits.length >= 5) {
                    result += '.' + digits.slice(4, 8);
                }
            }
            return result;
        },

        autoCompleteDate(input) {
            if (!input) return '';
            const digits = input.replace(/[^\d]/g, '');
            if (digits.length === 0) return '';

            if (digits.length <= 2) {
                const day = parseInt(digits, 10);
                if (isNaN(day) || day < 1 || day > 31) return input;

                const today = new Date();
                let year = today.getFullYear();
                let month = today.getMonth();
                let candidate = new Date(year, month, day);

                if (candidate < this.dateOnly(today) || candidate.getDate() !== day) {
                    let nextMonth = month + 1;
                    let nextYear = year;
                    if (nextMonth > 11) { nextMonth = 0; nextYear++; }
                    candidate = new Date(nextYear, nextMonth, day);
                    if (candidate.getDate() !== day) {
                        candidate = new Date(nextYear, nextMonth + 1, 0);
                    }
                }
                return this.formatDate(candidate);
            }

            if (digits.length === 3 || digits.length === 4) {
                const day = parseInt(digits.slice(0, 2), 10);
                const month = parseInt(digits.slice(2, 4), 10);
                if (isNaN(day) || isNaN(month) || day < 1 || day > 31 || month < 1 || month > 12) return input;

                let year = new Date().getFullYear();
                let candidate = new Date(year, month - 1, day);
                if (candidate < this.dateOnly(new Date()) || candidate.getDate() !== day) {
                    candidate = new Date(year + 1, month - 1, day);
                    if (candidate.getDate() !== day) {
                        candidate = new Date(year + 1, month, 0);
                    }
                }
                return this.formatDate(candidate);
            }

            if (digits.length >= 5 && digits.length <= 8) {
                const formatted = this.formatDateWithDots(digits);
                const parsed = this.parseDate(formatted);
                if (parsed) {
                    if (parsed < this.dateOnly(new Date())) {
                        let nextYear = parsed.getFullYear() + 1;
                        let candidate = new Date(nextYear, parsed.getMonth(), parsed.getDate());
                        if (candidate.getDate() !== parsed.getDate()) {
                            candidate = new Date(nextYear, parsed.getMonth() + 1, 0);
                        }
                        return this.formatDate(candidate);
                    }
                    return formatted;
                }
            }
            return input;
        },

        dateOnly(date) {
            if (!date) return null;
            return new Date(date.getFullYear(), date.getMonth(), date.getDate());
        },

        isPastDate(dateStr) {
            const date = this.parseDate(dateStr);
            if (!date) return false;
            const today = new Date();
            today.setHours(0, 0, 0, 0);
            return date < today;
        },

        isToday(dateStr) {
            const today = this.formatDate(new Date());
            return dateStr === today;
        },

        getDayClass(day, selectedDate) {
            return {
                'datepicker__day--other-month': !day.isCurrentMonth,
                'datepicker__day--disabled': this.isPastDate(day.date),
                'datepicker__day--today': this.isToday(day.date),
                'datepicker__day--selected': selectedDate === day.date
            };
        },

        processDateInput(val) {
            val = val.replace(/[^\d]/g, '');
            if (val.length > 8) val = val.slice(0, 8);

            if (val.length >= 2) {
                const day = parseInt(val.slice(0, 2), 10);
                if (day > 31) val = '31' + val.slice(2);
            }
            if (val.length >= 4) {
                const month = parseInt(val.slice(2, 4), 10);
                if (month > 12) val = val.slice(0, 2) + '12' + val.slice(4);
            }

            return this.formatDateWithDots(val);
        },

        onStartDateInput(e) {
            this.$emit('update:start-date', this.processDateInput(e.target.value));
        },

        onEndDateInput(e) {
            this.$emit('update:end-date', this.processDateInput(e.target.value));
        },

        onSingleDateInput(e) {
            this.$emit('update:single-date', this.processDateInput(e.target.value));
            this.$emit('validate-field', 'singleDate');
            this.validateTimeCrossing();
        },

        handleDateBlur(field) {
            let dateStr = '';
            if (field === 'start') dateStr = this.startDate;
            else if (field === 'end') dateStr = this.endDate;
            else if (field === 'single') dateStr = this.singleDate;

            if (!dateStr) {
                this.$emit('validate-date-range');
                return;
            }

            const completed = this.autoCompleteDate(dateStr);
            if (this.isPastDate(completed)) {
                if (field === 'start') this.$emit('update:start-date', '');
                else if (field === 'end') this.$emit('update:end-date', '');
                else if (field === 'single') this.$emit('update:single-date', '');
                this.$emit('validate-date-range');
                return;
            }

            if (completed !== dateStr) {
                if (field === 'start') this.$emit('update:start-date', completed);
                else if (field === 'end') this.$emit('update:end-date', completed);
                else if (field === 'single') this.$emit('update:single-date', completed);
            }

            if (field === 'start' || field === 'end') {
                this.validateDateRange();
            } else {
                this.$emit('validate-field', 'singleDate');
                this.validateTimeCrossing();
            }
        },

        validateDateRange() {
            if (this.internalToggle) return;
            if (this.startDate && this.endDate && this.startDate.length === 10 && this.endDate.length === 10) {
                const start = this.parseDate(this.startDate);
                const end = this.parseDate(this.endDate);
                if (start && end && end < start) {
                    this.$emit('validate-date-range');
                    return;
                }
            }
            if (this.startDate && this.endDate && this.startDate === this.endDate && !this.isOneDay) {
                this.toggleOneDay(true);
            }
            this.$emit('validate-date-range');
        },

        formatTimeString(raw) {
            if (!raw) return '';
            const cleaned = raw.replace(/[^\d:]/g, '');
            if (cleaned.includes(':')) {
                const parts = cleaned.split(':');
                let hours = parts[0].slice(0, 2);
                let minutes = parts[1] ? parts[1].slice(0, 2) : '';
                if (hours.length === 1) hours = '0' + hours;
                if (minutes.length === 1) minutes = '0' + minutes;
                if (minutes.length === 0) minutes = '00';
                let h = parseInt(hours, 10);
                if (isNaN(h)) h = 0;
                if (h > 23) h = 23;
                let m = parseInt(minutes, 10);
                if (isNaN(m)) m = 0;
                if (m > 59) m = 59;
                return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`;
            } else {
                let hours = cleaned.slice(0, 2);
                if (hours.length === 1) hours = '0' + hours;
                let h = parseInt(hours, 10);
                if (isNaN(h)) h = 0;
                if (h > 23) h = 23;
                return `${h.toString().padStart(2, '0')}:00`;
            }
        },

        formatTimeField(field) {
            const raw = field === 'start' ? this.startTime : this.endTime;
            const formatted = this.formatTimeString(raw);
            if (formatted !== raw) {
                if (field === 'start') this.$emit('update:start-time', formatted);
                else this.$emit('update:end-time', formatted);
            }
            this.validateTimeCrossing();
        },

        onStartTimeInput(e) {
            let val = e.target.value;
            val = val.replace(/[^\d:]/g, '');
            if (/^\d{2}$/.test(val) && !val.includes(':')) val += ':';
            if (val.length > 5) val = val.slice(0, 5);
            this.$emit('update:start-time', val);
        },

        onEndTimeInput(e) {
            let val = e.target.value;
            val = val.replace(/[^\d:]/g, '');
            if (/^\d{2}$/.test(val) && !val.includes(':')) val += ':';
            if (val.length > 5) val = val.slice(0, 5);
            this.$emit('update:end-time', val);
        },

        validateTimeCrossing() {
            if (this.internalToggle) return;
            if (!this.startTime || !this.endTime) {
                this.$emit('validate-time-range');
                return;
            }
            if (this.isOneDay && this.startTime > this.endTime) {
                const dateStr = this.singleDate;
                if (dateStr) {
                    const dateObj = this.parseDate(dateStr);
                    if (dateObj) {
                        const nextDay = new Date(dateObj);
                        nextDay.setDate(dateObj.getDate() + 1);
                        const newEndDate = this.formatDate(nextDay);
                        this.internalToggle = true;
                        this.$emit('update:is-one-day', false);
                        this.$emit('update:start-date', dateStr);
                        this.$emit('update:end-date', newEndDate);
                        this.$emit('update:single-date', '');
                        this.$nextTick(() => { this.internalToggle = false; });
                        return;
                    }
                }
            }
            this.$emit('validate-time-range');
        },

        onCheckboxChange(event) {
            this.toggleOneDay(event.target.checked);
        },

        toggleOneDay(newValue) {
            if (this.internalToggle) return;
            this.internalToggle = true;

            const nowOneDay = newValue !== undefined ? newValue : !this.isOneDay;

            if (nowOneDay) {
                let dateToUse = this.startDate || this.endDate;
                if (!dateToUse && this.singleDate) dateToUse = this.singleDate;
                if (dateToUse) this.$emit('update:single-date', dateToUse);
                if (this.startDate) this.$emit('update:start-date', '');
                if (this.endDate) this.$emit('update:end-date', '');
            } else {
                const dateToUse = this.singleDate;
                if (dateToUse) {
                    this.$emit('update:start-date', dateToUse);
                    this.$emit('update:end-date', '');
                }
                if (this.singleDate) this.$emit('update:single-date', '');
            }

            this.$emit('update:is-one-day', nowOneDay);
            this.$emit('validate-field', 'isOneDay');
            this.$emit('validate-date-range');
            this.$emit('validate-time-range');

            this.$nextTick(() => { this.internalToggle = false; });
        },

        openDatepicker(type) {
            this.showStartDatepicker = false;
            this.showEndDatepicker = false;
            this.showSingleDatepicker = false;
            // Взаимоисключение: открытый календарь закрывает "Быстрый выбор".
            this.showQuickMenu = false;

            // Позиционирование телепортнутого календаря от инпута (аналог qdMenuStyle).
            const refByType = { start: 'startDateInput', end: 'endDateInput', single: 'singleDateInput' };
            const input = this.$refs[refByType[type]];
            if (input) {
                // Календарь телепортится в body ВНУТРИ зазумленного <html> (масштаб под 1440
                // на мониторах >1440): inline top/left трактуются в зазумленных CSS-px.
                // getBoundingClientRect отдаёт device-px - делим на zoom, иначе календарь
                // домножается на zoom второй раз и улетает в правый нижний угол.
                // Константы-отступы (+8) уже в layout-px - НЕ делим.
                // На мобилке календарь - bottom-sheet: координаты задаёт @media, inline
                // top/left их бы перебили (инлайн сильнее любого правила).
                if (window.innerWidth <= 768) {
                    // Под листом-модалкой фон не скроллится, как у прочих окон.
                    setBodyScrollLock(this, true);
                    this.datepickerStyle = { zIndex: 12000 };
                } else {
                    const z = getViewportZoom();
                    const r = input.getBoundingClientRect();
                    this.datepickerStyle = {
                        position: 'fixed',
                        top: `${Math.round(r.bottom / z + 8)}px`,
                        left: `${Math.round(r.left / z)}px`,
                        zIndex: 12000
                    };
                }
            }

            if (type === 'start') {
                this.showStartDatepicker = true;
                const parsed = this.parseDate(this.startDate);
                if (parsed) this.currentDate = parsed;
            } else if (type === 'end') {
                this.showEndDatepicker = true;
                const parsed = this.parseDate(this.endDate);
                if (parsed) this.currentDate = parsed;
            } else if (type === 'single') {
                this.showSingleDatepicker = true;
                const parsed = this.parseDate(this.singleDate);
                if (parsed) this.currentDate = parsed;
            }
        },

        handleDatepickerEscape(event) {
            if (event.key !== 'Escape') return;
            if (this.showStartDatepicker || this.showEndDatepicker || this.showSingleDatepicker) {
                this.closeDatepicker();
            }
        },

        closeDatepicker() {
            this.resetSheetSwipe();
            releaseBodyScrollLock(this);
            this.showStartDatepicker = false;
            this.showEndDatepicker = false;
            this.showSingleDatepicker = false;
        },

        selectActiveDate(day) {
            if (this.showStartDatepicker) this.selectStartDate(day);
            else if (this.showEndDatepicker) this.selectEndDate(day);
            else if (this.showSingleDatepicker) this.selectSingleDate(day);
        },

        /**
         * Перевод фокуса на следующее поле после выбора дня. На телефоне не делаем:
         * лист календаря закрывается и тут же выбрасывает клавиатуру в соседнее поле,
         * перекрывая пол-экрана.
         */
        focusNext(refName) {
            if (typeof window !== 'undefined' && typeof window.matchMedia === 'function'
                && window.matchMedia('(max-width: 768px)').matches) return;
            const el = this.$refs[refName];
            if (el) el.focus();
        },

        selectStartDate(day) {
            if (!day.isCurrentMonth || this.isPastDate(day.date)) return;
            this.$emit('update:start-date', day.date);
            this.showStartDatepicker = false;
            this.validateDateRange();
            this.focusNext('endDateInput');
        },

        selectEndDate(day) {
            if (!day.isCurrentMonth || this.isPastDate(day.date)) return;
            this.$emit('update:end-date', day.date);
            this.showEndDatepicker = false;
            this.validateDateRange();
            this.focusNext('startTimeInput');
        },

        selectSingleDate(day) {
            if (!day.isCurrentMonth || this.isPastDate(day.date)) return;
            this.$emit('update:single-date', day.date);
            this.showSingleDatepicker = false;
            this.$emit('validate-field', 'singleDate');
            this.validateTimeCrossing();
            this.focusNext('startTimeInput');
        },

        prevMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() - 1, 1);
        },

        nextMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() + 1, 1);
        }
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
    /* 9px (а не 5): "Быстрый выбор" вынесен из потока и свисает в этот зазор -
       больший gap отодвигает кнопку от инпута. Одинаков для даты и времени -> уровни совпадают. */
    gap: 9px;
    position: relative;
}

.date-container {
    min-height: 40px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    width: 250px;
}

/* Лейбл + компактный "Быстрый выбор" в одной строке - кнопки не сдвигают инпут вниз */
.date__label-row {
    position: relative;
    display: flex;
    align-items: center;
}

/* Дропдаун вне потока: строка лейбла остаётся высотой лейбла -
   инпуты даты совпадают по уровню с инпутами времени. */
.qd-dropdown {
    position: absolute;
    top: 50%;
    right: 0;
    transform: translateY(-50%);
}

.qd-trigger {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 24px;
    padding: 0 10px;
    border-radius: 50px;
    border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
    background: var(--accent-tint);
    font-family: inherit;
    font-size: 11px;
    font-weight: 600;
    color: var(--accent-text);
    cursor: pointer;
    white-space: nowrap;
    transition: background-color 0.2s ease, border-color 0.2s ease;
}

.qd-trigger:hover {
    background: color-mix(in srgb, var(--accent) 18%, var(--surface));
    border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.qd-caret {
    transition: transform 0.2s ease;
}

.qd-trigger--open .qd-caret {
    transform: rotate(180deg);
}

.qd-menu {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 1001;
    /* Ширина по содержимому: "На сентябрь 01.09.2026 - 30.09.2026" в фиксированные
       230px не влезала и вылезала за границу меню. min-width держит форму на
       коротких месяцах ("На май"), max-width задаётся из JS по месту до края экрана. */
    width: max-content;
    min-width: 230px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px;
    background: var(--surface);
    border: 1px solid var(--border, var(--border));
    border-radius: 14px;
    box-shadow: 0 8px 24px var(--shadow-drop);
}

.qd-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 8px 10px;
    border: none;
    background: none;
    border-radius: 9px;
    font-family: inherit;
    cursor: pointer;
    text-align: left;
    transition: background-color 0.15s ease;
}

.qd-item:hover {
    background: var(--accent-tint);
}

/* Если места до края экрана не хватило - режем многоточием название периода,
   дату оставляем целой: ради неё пункт и читают. */
.qd-item__label {
    font-size: 13px;
    color: var(--text);
    white-space: nowrap;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
}

.qd-item__date {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
    flex: none;
}

.qd-sep {
    height: 1px;
    background: var(--accent-tint);
    margin: 4px 6px;
}

.qd-fade-enter-active,
.qd-fade-leave-active {
    transition: opacity 0.15s ease, transform 0.15s ease;
}

.qd-fade-enter-from,
.qd-fade-leave-to {
    opacity: 0;
    transform: translateY(-6px);
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
    border: 1px solid var(--border);
    outline: none;
    background: var(--surface);
    border-radius: 15px;
    padding: 5px 10px;
    font-family: inherit;
    font-size: 13px;
    transition: border-color 0.2s ease;
}

.input__date:focus {
    border-color: var(--accent);
}

.date__text {
    color: var(--accent-text);
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

/* Тумблеры "Дополнительно": без фикс-высоты и justify-center - скрытое поле не оставляет дыру,
   оставшееся встаёт наверх. */
.additional-options {
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 100%;
}

/* Datepicker styles */
.datepicker-wrapper {
    position: relative;
    display: inline-block;
}

/* Затемнение и ползунок - только мобильный лист (см. @media ниже). */
.datepicker-overlay {
    display: none;
}

.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 0 auto 8px;
    flex-shrink: 0;
}

.datepicker {
    /* Позиционирование через datepickerStyle (position:fixed + координаты от инпута). */
    background: var(--surface);
    border-radius: 16px;
    box-shadow: 0 8px 24px var(--shadow-drop);
    border: 1px solid var(--border);
    padding: 16px;
    min-width: 260px;
    z-index: 1000;
}

.datepicker__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
}

.datepicker__nav {
    /* Стрелки нарисованы currentColor. */
    color: var(--accent-text);
    background: none;
    border: none;
    cursor: pointer;
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background-color 0.2s ease;
}

.datepicker__nav:hover {
    background-color: var(--surface-2);
}

.datepicker__nav svg {
    width: 12px;
    height: 12px;
}

.datepicker__month {
    font-weight: 600;
    font-size: 14px;
    color: var(--text);
}

.datepicker__weekdays {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 4px;
    margin-bottom: 12px;
}

.datepicker__weekday {
    text-align: center;
    font-size: 12px;
    font-weight: 500;
    color: var(--text-muted);
    padding: 6px 0;
}

.datepicker__days {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 4px;
}

.datepicker__day {
    text-align: center;
    padding: 8px 0;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border-radius: 8px;
    transition: background-color 0.2s ease;
    color: var(--text);
}

.datepicker__day:hover:not(.datepicker__day--disabled):not(.datepicker__day--selected) {
    background-color: var(--accent-tint);
}

.datepicker__day--selected {
    background-color: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
}

.datepicker__day--other-month {
    color: var(--text-muted);
}

.datepicker__day--today {
    font-weight: 700;
    position: relative;
}

.datepicker__day--today::after {
    content: '';
    position: absolute;
    bottom: 2px;
    left: 50%;
    transform: translateX(-50%);
    width: 4px;
    height: 4px;
    background-color: var(--accent);
    border-radius: 50%;
}

.datepicker__day--today.datepicker__day--selected::after {
    background-color: var(--surface);
}

.datepicker__day--disabled {
    color: var(--border);
    cursor: not-allowed;
    pointer-events: none;
}

.single-date {
    display: flex;
    align-items: center;
    width: 100%;
}

/* Время */
.time-section {
    width: auto;
}

.time-wrapper {
    display: flex;
    gap: 10px;
    align-items: center;
}

.time-input-group {
    display: flex;
    align-items: center;
    gap: 5px;
}

.input__time {
    width: 65px;
    height: 40px;
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 5px 10px;
    font-family: inherit;
    font-size: 13px;
    outline: none;
    transition: border-color 0.2s ease;
    background: var(--surface);
    text-align: center;
}

.input__time:focus {
    border-color: var(--accent);
}

.time-message {
    font-size: 10px;
    width: 250px;
}

.input--error {
    border-color: var(--danger);
}

.error-message {
    background: color-mix(in srgb, var(--danger) 40%, var(--surface));
    color: var(--danger-text);
    padding: 8px 16px;
    border-radius: 24px;
    font-size: 13px;
    font-weight: 600;
    backdrop-filter: blur(5px);
    animation: errorFadeIn 0.5s ease-out;
    width: fit-content;
    max-width: 100%;
    margin-top: 8px;
    word-break: break-word;
    line-height: 1.2;
}

.time-error {
    margin-top: 5px;
}

@keyframes errorFadeIn {
    0% { opacity: 0; }
    100% { opacity: 1; }
}

/* Анимация календаря */
.calendar-enter-active,
.calendar-leave-active {
    transition: all 0.2s ease;
}

.calendar-enter-from {
    opacity: 0;
    transform: translateY(-8px);
}

.calendar-leave-to {
    opacity: 0;
    transform: translateY(-8px);
}

.datepicker-overlay-fade-enter-active,
.datepicker-overlay-fade-leave-active {
    transition: opacity 0.25s ease;
}

.datepicker-overlay-fade-enter-from,
.datepicker-overlay-fade-leave-to {
    opacity: 0;
}

/* Мобильный лист выезжает снизу, а не сползает сверху, как десктопный попап. */
@media (max-width: 768px) {
    .calendar-enter-from,
    .calendar-leave-to {
        opacity: 1;
        transform: translateY(100%);
    }
}

.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

/* Дата (250px) + время + доп.опции в ряд не влезают на узком - стекаем в колонку,
   инпуты растягиваем на всю доступную ширину. */
@media (max-width: 768px) {
    /* Календарь на телефоне - bottom-sheet с затемнением (как DateFilter): раньше
       висел попапом у поля, сливался с формой и на коротком экране не помещался.
       Высота ограничена вьюпортом, сетка дней прокручивается внутри. */
    .datepicker-overlay {
        display: block;
        position: fixed;
        inset: 0;
        z-index: 11999;
        background: var(--overlay);
    }

    .datepicker {
        position: fixed;
        top: auto;
        left: 0;
        right: 0;
        bottom: 0;
        width: 100vw;
        min-width: 0;
        max-height: 92dvh;
        display: flex;
        flex-direction: column;
        padding: 8px 12px 12px;
        border: none;
        border-radius: 16px 16px 0 0;
        box-shadow: 0 -8px 30px var(--shadow-drop);
    }

    /* Лист тянется за пальцем 1:1 - без transition во время жеста. */
    .datepicker.is-dragging {
        transition: none;
    }

    .sheet-handle {
        display: block;
    }

    .datepicker__header,
    .datepicker__weekdays {
        flex-shrink: 0;
    }

    .datepicker__days {
        flex: 1 1 auto;
        min-height: 0;
        overflow-y: auto;
        overscroll-behavior: contain;
        -webkit-overflow-scrolling: touch;
        padding-bottom: 4px;
    }

    /* Тач-таргет дня не меньше 40px, но без лишней высоты попапа. */
    .datepicker__day {
        padding: 10px 0;
        font-size: 14px;
    }

    .date-range-section {
        flex-direction: column;
        gap: 16px;
    }

    .date__input {
        width: 100%;
    }

    .date-container {
        width: 100%;
    }

    .date,
    .single-date {
        width: 100%;
    }

    .datepicker-wrapper {
        flex: 1;
        min-width: 0;
    }

    .input__date {
        width: 100%;
    }

    .time-section {
        width: 100%;
    }

    .time-wrapper {
        width: 100%;
    }

    .time-input-group {
        flex: 1;
    }

    .input__time {
        width: 100%;
        min-width: 0;
    }

    .additional-options {
        width: 100%;
    }
}
</style>
