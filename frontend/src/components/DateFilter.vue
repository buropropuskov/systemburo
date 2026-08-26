<template>
  <div class="date-filter">
    <div
      ref="trigger"
      class="date-field"
      @click="toggleCalendar"
    >
      <div class="field-wrapper">
        <div class="field-input">
          {{ displayText }}
        </div>
        <svg
          class="field-icon"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <rect
            x="3"
            y="5"
            width="18"
            height="16"
            rx="3"
            stroke="currentColor"
            stroke-width="2"
          />
          <path
            d="M3 9.5H21"
            stroke="currentColor"
            stroke-width="2"
          />
          <path
            d="M8 3V7M16 3V7"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
          />
        </svg>
      </div>
            
      <Teleport to="body">
        <!-- Мобилка: затемнение под листом - календарь не сливается со списком за ним.
             На десктопе оверлей скрыт, попап по-прежнему якорится к полю. -->
        <transition name="calendar-overlay-fade">
          <div
            v-if="showCalendar"
            class="calendar-overlay"
            @click="closeCalendar"
          />
        </transition>
        <transition name="calendar-slide">
          <div
            v-if="showCalendar"
            ref="calendar"
            class="calendar-modal"
            :class="{ 'is-dragging': sheetDragging }"
            :style="sheetOffset ? { ...calendarStyle, transform: `translateY(${sheetOffset}px)` } : calendarStyle"
            @click.stop
            @touchstart="onSheetTouchStart"
            @touchmove="onSheetTouchMove"
            @touchend="onSheetTouchEnd"
          >
            <div class="calendar-container">
              <!-- Ползунок bottom-sheet - виден только на мобилке (тянуть для закрытия). -->
              <div
                class="sheet-handle"
                aria-hidden="true"
              />
              <!-- Header -->
              <div class="calendar-header">
                <div class="header-actions">
                  <button
                    class="nav-btn prev-btn"
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
                  <div class="date-display">
                    <span class="current-month-year">{{ capitalizeFirstLetter(currentMonthYear) }}</span>
                  </div>
                  <button
                    class="nav-btn next-btn"
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
              </div>
                        
              <div
                ref="sheetBody"
                class="calendar-body"
              >
                <!-- Quick selection слева -->
                <div class="quick-selection">
                  <div class="quick-buttons-list">
                    <button
                      v-for="period in quickPeriods"
                      :key="period.key"
                      class="quick-btn"
                      :class="{ 'active': isQuickActive(period.key) }"
                      @click="setQuickDate(period.key)"
                    >
                      {{ period.label }}
                    </button>
                  </div>
                </div>
                            
                <!-- Calendar справа -->
                <div class="calendar-main">
                  <div
                    v-if="mode === 'range'"
                    class="calendar-mode-switch"
                  >
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
                    <div
                      v-for="day in weekdays"
                      :key="day"
                      class="weekday"
                    >
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
                <button
                  class="action-btn action-btn--clear"
                  @click="clearSelection"
                >
                  Очистить
                </button>
                <button
                  class="action-btn action-btn--apply"
                  @click="applySelection"
                >
                  Применить
                </button>
              </div>
            </div>
          </div>
        </transition>
      </Teleport>
    </div>
  </div>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref } from 'vue';
import { getViewportZoom } from '@/utils/viewportScale';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { QUICK_PERIODS, isSingleDayPeriod, periodBounds } from '@/utils/datePeriods';

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
    emits: ['apply', 'clear', 'update:dateRangeEnd', 'update:dateRangeStart', 'update:selectedDate'],
    /*
     * Свайп-вниз-закрытие мобильного листа - общий useSwipeDismiss (как BaseModal).
     * Композабл живёт в setup, а состояние календаря - в data (Options API), поэтому
     * закрытие проходит через счётчик-сигнал: setup его увеличивает, watch в Options
     * гасит showCalendar. Свайп берётся только с ползунка или когда тело листа
     * прокручено вверх - иначе жест внутри списка быстрых периодов был бы закрытием.
     */
    setup() {
        const sheetBody = ref(null);
        const closeSignal = ref(0);
        const swipe = useSwipeDismiss(() => { closeSignal.value += 1; }, {
            handleSelector: '.sheet-handle',
            getScrollTop: () => sheetBody.value?.scrollTop ?? 0,
        });
        return {
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
        return {
            showCalendar: false,
            currentDate: new Date(),
            internalSelectedDate: null,
            internalRangeStart: null,
            internalRangeEnd: null,
            selectingRange: this.mode === 'range', // true для периода, false для одного дня
            activeQuickDate: null,
            weekdays: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
            calendarStyle: {},
        };
    },
    computed: {
        // В single-режиме предлагаем только периоды-в-один-день: диапазон родителю
        // такого поля некуда деть, а показанный в календаре месяц без применённого
        // фильтра читается как сломанный поиск.
        quickPeriods() {
            return this.mode === 'range' ? QUICK_PERIODS : QUICK_PERIODS.filter(p => p.single);
        },
        
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
        closeSignal() {
            this.showCalendar = false;
        },
        showCalendar(open) {
            if (!open) this.resetSheetSwipe();
            // Под листом-модалкой фон не скроллится (как у прочих окон). На десктопе
            // календарь - попап у поля и репозиционируется на скролле: там не блокируем.
            if (typeof window !== 'undefined' && window.innerWidth <= 768) {
                setBodyScrollLock(this, open);
            }
            if (open) {
                this.$nextTick(() => {
                    if (!this.showCalendar) return; // успели закрыть до тика - не вешаем слушатели
                    this.updatePosition();
                    window.addEventListener('scroll', this.updatePosition, true);
                    window.addEventListener('resize', this.updatePosition);
                });
            } else {
                window.removeEventListener('scroll', this.updatePosition, true);
                window.removeEventListener('resize', this.updatePosition);
            }
        },
    },
    mounted() {
        // Capture-фаза: внутри BaseModal на .base-modal висит @click.stop, из-за чего
        // клики внутри модалки не всплывают до document и календарь не закрывался.
        // В фазе capture слушатель срабатывает до stopPropagation.
        document.addEventListener('click', this.handleClickOutside, true);
        document.addEventListener('keydown', this.handleEscape);
        // Устанавливаем режим на основе пропса
        this.selectingRange = this.mode === 'range';
    },
    beforeUnmount() {
        document.removeEventListener('click', this.handleClickOutside, true);
        document.removeEventListener('keydown', this.handleEscape);
        releaseBodyScrollLock(this);
        window.removeEventListener('scroll', this.updatePosition, true);
        window.removeEventListener('resize', this.updatePosition);
    },
    methods: {
        capitalizeFirstLetter(string) {
            if (!string) return '';
            return string.charAt(0).toUpperCase() + string.slice(1);
        },
        
        /**
         * Закрытие календаря - единая точка для всех способов: крестик, оверлей,
         * Escape, свайп вниз. Сбрасывает недовыбранный период, как и повторный клик
         * по полю (иначе остаётся «висящая» начальная дата без конечной).
         */
        closeCalendar() {
            if (!this.showCalendar) return;
            this.showCalendar = false;
            if (this.selectingRange && !this.internalRangeEnd) {
                this.internalRangeStart = null;
            }
        },

        handleEscape(event) {
            if (event.key === 'Escape' && this.showCalendar) {
                this.closeCalendar();
            }
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
        
        updatePosition() {
            if (!this.showCalendar) return;
            const trigger = this.$refs.trigger;
            if (!trigger) return;
            // На мобильных вёрстка центрирует попап через @media - не навязываем inline-позицию.
            // Брейкпоинт по ФИЗИЧЕСКОЙ innerWidth (device-width на телефоне ~390, zoom=1);
            // на мониторах >1440 innerWidth>1440 - в эту ветку не попадаем.
            if (window.innerWidth <= 768) {
                this.calendarStyle = {};
                return;
            }
            // Попап телепортится в body ВНУТРИ зазумленного <html> (масштаб под 1440):
            // inline top/left трактуются в зазумленных CSS-px. getBoundingClientRect -
            // device-px, innerWidth/innerHeight - НЕзумленные; приводим всё к layout-px
            // делением на zoom (при zoom=1 - без изменений), width уже в layout-px.
            const z = getViewportZoom();
            const raw = trigger.getBoundingClientRect();
            const rect = { left: raw.left / z, top: raw.top / z, bottom: raw.bottom / z };
            const vw = window.innerWidth / z;
            const vh = window.innerHeight / z;
            const width = 500;
            const margin = 8;
            // Горизонтальный зазор больше вертикального: при клампе у правого края
            // попап не должен прилипать к краю экрана (= padding карточки).
            const edgeMargin = 24;
            let left = rect.left;
            if (left + width > vw - edgeMargin) {
                left = Math.max(edgeMargin, vw - width - edgeMargin);
            }
            let top = rect.bottom + 5;
            const height = this.$refs.calendar ? this.$refs.calendar.offsetHeight : 0;
            // Не влезает снизу - открываем вверх, если там есть место.
            if (height && top + height > vh - margin) {
                const above = rect.top - height - 5;
                top = above >= margin ? above : Math.max(margin, vh - height - margin);
            }
            this.calendarStyle = {
                position: 'fixed',
                top: `${top}px`,
                left: `${left}px`,
                width: `${width}px`,
            };
        },

        handleClickOutside(event) {
            if (!this.showCalendar) return;
            // Календарь телепортирован в body, поэтому this.$el его не содержит -
            // проверяем и триггер, и сам попап через отдельный ref.
            const inTrigger = this.$el.contains(event.target);
            const calendarEl = this.$refs.calendar;
            const inCalendar = calendarEl && calendarEl.contains(event.target);
            if (!inTrigger && !inCalendar) {
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
            const bounds = periodBounds(period);
            if (!bounds) return;
            const [start, end] = bounds;
            this.activeQuickDate = period;

            // Однодневный период показываем как один день - и там, где родитель
            // принимает только его, и в диапазонном поле (границы дня досчитает
            // applySelection по mode).
            if (isSingleDayPeriod(period) || this.mode !== 'range') {
                this.selectingRange = false;
                this.internalSelectedDate = new Date(start);
                this.internalRangeStart = null;
                this.internalRangeEnd = null;
            } else {
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
        
        /*
         * Результат согласуем с пропом mode, а не с внутренним переключателем
         * календаря. Родитель подписан на события своего режима: range-поле слушает
         * только dateRangeStart/End, single-поле - только selectedDate. Пока выход
         * зависел от selectingRange, выбор "не того вида" уходил в никуда: быстрый
         * период ("Этот месяц") в single-поле и "Сегодня" в range-поле эмитили null
         * туда, где родитель слушает, и он молча сбрасывал фильтр, тогда как в поле
         * календаря оставался выбранный период - выглядело как "поиск по дате не
         * работает" (Версии таблиц, мониторинг, отчёт по проходам).
         */
        applySelection() {
            const start = this.internalRangeStart || this.internalSelectedDate;
            const end = this.internalRangeEnd || start;

            if (this.mode === 'range') {
                if (start) {
                    const from = new Date(start);
                    from.setHours(0, 0, 0, 0);
                    const to = new Date(end);
                    to.setHours(23, 59, 59, 999);
                    this.$emit('update:selectedDate', null);
                    this.$emit('update:dateRangeStart', from);
                    this.$emit('update:dateRangeEnd', to);
                } else {
                    this.emitNoSelection();
                }
            } else {
                // Один день: диапазон сюда попасть не может (в single-режиме нет ни
                // переключателя, ни периодов), но если дата пришла как начало
                // диапазона - берём её, а не выбрасываем выбор.
                const day = this.internalSelectedDate || start;
                if (day) {
                    const date = new Date(day);
                    date.setHours(0, 0, 0, 0);
                    this.$emit('update:selectedDate', date);
                    this.$emit('update:dateRangeStart', null);
                    this.$emit('update:dateRangeEnd', null);
                } else {
                    this.emitNoSelection();
                }
            }

            this.$emit('apply');
            this.showCalendar = false;
        },

        emitNoSelection() {
            this.$emit('update:selectedDate', null);
            this.$emit('update:dateRangeStart', null);
            this.$emit('update:dateRangeEnd', null);
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
    background-color: var(--surface);
    border-radius: 15px;
    border: 1px solid var(--border);
    padding: 0 10px;
    cursor: pointer;
    position: relative;
    transition: border-color 0.2s ease;
}

.date-field:hover {
    border-color: var(--accent);
}

.field-wrapper {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 100%;
}

.field-input {
    font-size: 14px;
    color: var(--text);
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 5px;
}

.field-input::placeholder {
    color: var(--text-muted);
}

.field-icon {
    width: 14px;
    height: 14px;
    opacity: 0.7;
    flex-shrink: 0;
}

/* Calendar modal */
.calendar-slide-enter-active,
.calendar-slide-leave-active {
    /* Только opacity/transform: top/left пересчитываются на скролле и не должны анимироваться. */
    transition: opacity 0.3s ease, transform 0.3s ease;
}

.calendar-slide-enter-from,
.calendar-slide-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.calendar-overlay-fade-enter-active,
.calendar-overlay-fade-leave-active {
    transition: opacity 0.25s ease;
}

.calendar-overlay-fade-enter-from,
.calendar-overlay-fade-leave-to {
    opacity: 0;
}

/* Мобильный лист выезжает снизу, а не сползает сверху, как десктопный попап. */
@media (max-width: 768px) {
    .calendar-slide-enter-from,
    .calendar-slide-leave-to {
        opacity: 1;
        transform: translateY(100%);
    }
}

.calendar-modal {
    /* Телепортирован в body: top/left задаёт inline-стиль из updatePosition.
       width нужен здесь, чтобы offsetHeight измерялся до применения inline-стиля. */
    position: fixed;
    z-index: 9999;
    width: 500px;
}

/* Затемнение и ползунок - только мобильный лист (см. @media ниже). */
.calendar-overlay {
    display: none;
}

.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 6px auto 0;
    flex-shrink: 0;
}

.calendar-container {
    background: var(--surface);
    border-radius: 12px;
    border: 1px solid var(--border);
    box-shadow: 0 8px 24px var(--shadow-drop);
    overflow: hidden;
}

/* Header - высота 40px */
.calendar-header {
    padding: 8px 16px;
    background: var(--accent-tint);
    border-bottom: 1px solid var(--border);
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
    color: var(--text);
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
    border: 1px solid var(--border);
    background: var(--surface);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s ease;
    flex-shrink: 0;
}

.nav-btn:hover {
    border-color: var(--accent);
    background: var(--accent-tint);
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
    border-right: 1px solid var(--border);
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
    border: 1px solid var(--border);
    background: var(--surface);
    border-radius: 6px;
    font-size: 12px;
    font-weight: 500;
    color: var(--text);
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
    border-color: var(--accent);
    background: var(--accent-tint);
}

.quick-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.quick-btn.active:hover {
    background: var(--accent-hover);
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
    border: 1px solid var(--border);
    background: var(--surface);
    border-radius: 15px; /* Изменено: скругление 15px */
    font-size: 12px;
    font-weight: 500;
    color: var(--text);
    cursor: pointer;
    transition: all 0.2s ease;
    text-align: center;
}

.mode-btn:hover {
    border-color: var(--accent);
    background: var(--accent-tint);
}

.mode-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.mode-btn.active:hover {
    background: var(--accent-hover);
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
    color: var(--text-muted);
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
    color: var(--text);
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
    color: var(--text-muted);
    cursor: default;
}

.day--current-month:hover:not(.day--selected):not(.day--range-start):not(.day--range-end):not(.day--in-range) {
    background: var(--surface-2);
}

.day--today {
    color: var(--accent-text);
    font-weight: 600;
}

/* Один выбранный день - синий */
.day--selected,
.day--range-start,
.day--range-end {
    background: var(--accent) !important; /* Акцент темы для всех выбранных дней */
    color: var(--accent-contrast) !important;
    font-weight: 600;
}

.day--selected:hover,
.day--range-start:hover,
.day--range-end:hover {
    background: var(--accent-hover) !important; /* Ярче/темнее при наведении - по теме */
}

/* Дни внутри периода (между началом и концом) */
.day--in-range:not(.day--range-start):not(.day--range-end) {
    background: var(--accent-tint); /* Подсветка дней внутри периода */
    color: var(--text);
    border-radius: 0;
}

.day--in-range:not(.day--range-start):not(.day--range-end):hover {
    background: color-mix(in srgb, var(--accent) 18%, var(--surface));
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
    border: 1px solid var(--border);
    border-radius: 15px;
    background: var(--surface-2);
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
    color: var(--text-muted);
    font-weight: 500;
    white-space: nowrap;
}

.range-date {
    font-size: 12px;
    color: var(--text);
    font-weight: 500;
    background: var(--surface);
    padding: 2px 6px;
    border-radius: 15px;
    border: 1px solid var(--border);
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
    border-top: 1px solid var(--border);
    background: var(--accent-tint);
}

.action-btn {
    flex: 1;
    padding: 8px 12px;
    border-radius: 50px;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
    border: none;
    outline: none;
    min-height: 32px;
}

.action-btn--clear {
    background: var(--surface);
    color: var(--text-muted);
    border: 1px solid var(--border);
}

.action-btn--clear:hover {
    background: var(--surface-2);
    border-color: var(--border);
}

.action-btn--apply {
    background: var(--accent);
    color: var(--accent-contrast);
    border: 1px solid var(--accent);
}

.action-btn--apply:hover {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
}

/* Scrollbar for quick buttons */
.quick-buttons-list::-webkit-scrollbar {
    width: 4px;
}

.quick-buttons-list::-webkit-scrollbar-track {
    background: var(--surface-2);
    border-radius: 2px;
}

.quick-buttons-list::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 2px;
}

.quick-buttons-list::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
}

/* Адаптивность */
@media (max-width: 768px) {
    .date-filter {
        width: 100%;
    }

    /* Календарь на телефоне - bottom-sheet: прижат к низу, во всю ширину, с
       затемнением под ним (раньше был центрированный попап 320px без подложки -
       сливался со списком и не помещался в экран). Высота ограничена вьюпортом,
       лишнее прокручивается телом листа. */
    .calendar-overlay {
        display: block;
        position: fixed;
        inset: 0;
        z-index: 9998;
        background: var(--overlay);
    }

    .calendar-modal {
        position: fixed;
        top: auto;
        left: 0;
        right: 0;
        bottom: 0;
        width: 100vw;
        max-width: none;
        max-height: 92dvh;
        display: flex;
        z-index: 9999;
    }

    /* Лист тянется за пальцем 1:1 - без transition во время жеста. */
    .calendar-modal.is-dragging {
        transition: none;
    }

    .calendar-container {
        width: 100%;
        max-height: 92dvh;
        display: flex;
        flex-direction: column;
        border: none;
        border-radius: 16px 16px 0 0;
        box-shadow: 0 -8px 30px var(--shadow-drop);
    }

    .sheet-handle {
        display: block;
    }

    /* Шапка с месяцем и кнопки действий закреплены, прокручивается только тело.
       Фон - поверхность темы, а не подсветка: тонированная заливка на всю ширину
       листа читалась как «залипшее» выделение. */
    .calendar-header,
    .calendar-actions {
        flex-shrink: 0;
        background: var(--surface);
    }


    .calendar-body {
        flex: 1 1 auto;
        flex-direction: column;
        padding: 10px 12px;
        gap: 10px;
        min-height: 0;
        overflow-y: auto;
        overscroll-behavior: contain;
        -webkit-overflow-scrolling: touch;
    }

    .quick-selection {
        flex: none;
        border-right: none;
        border-bottom: 1px solid var(--border);
        padding-right: 0;
        padding-bottom: 10px;
    }

    /* Быстрые периоды - ОДНА строка-карусель с горизонтальной прокруткой: сеткой они
       занимали половину листа и оттесняли сам календарь. Прокрутка не мешает
       вертикальному скроллу тела листа (жест по оси X ловит только эта полоса).
       Обрезка по краям листа гасится отрицательными margin - чипы уезжают под край,
       видно, что ряд продолжается. */
    .quick-selection {
        margin: 0 -12px;
        padding: 0 0 10px;
    }

    .quick-buttons-list {
        display: flex;
        /* Базовый стиль - колонка; для строки-карусели направление задаём явно. */
        flex-direction: row;
        flex-wrap: nowrap;
        gap: 6px;
        max-height: none;
        overflow-x: auto;
        overflow-y: hidden;
        padding: 0 12px 2px;
        scroll-snap-type: x proximity;
        -webkit-overflow-scrolling: touch;
        overscroll-behavior-x: contain;
        scrollbar-width: none;
    }

    .quick-buttons-list::-webkit-scrollbar {
        display: none;
    }

    .quick-btn {
        flex: 0 0 auto;
        padding: 0 12px;
        font-size: 12px;
        line-height: 1;
        height: 30px;
        min-height: 30px;
        max-height: none;
        border-radius: 50px;
        white-space: nowrap;
        text-align: center;
        justify-content: center;
        scroll-snap-align: start;
    }

    .calendar-main {
        width: 100%;
    }

    /* Компактная сетка дней - ячейки ниже, но тач-таргет остаётся 32px. */
    .day {
        height: 32px;
    }

    .selected-range {
        margin-top: 10px;
        padding: 6px 8px;
        min-height: 32px;
    }

    .action-btn {
        padding: 10px 12px;
        font-size: 11px;
    }
}
</style>