<template>
  <div class="schedule-tab">
    <div class="schedule-tab__header-top">
      <h4 class="schedule-tab__header-title">
        Режим работы
      </h4>
      <div
        v-if="!readonly"
        class="schedule-tab__header-actions"
      >
        <button
          class="copy-btn"
          data-testid="copy-schedule-btn"
          :disabled="isLoading"
          @click="openCopyModal"
        >
          Скопировать расписание
        </button>
        <button
          class="add-btn"
          :disabled="isLoading"
          @click="openAddModal"
        >
          + Добавить окно
        </button>
      </div>
    </div>
    <p class="schedule-tab__hint">
      Дни без активных окон считаются нерабочими. Круглосуточный режим заменяет
      обычные окна на 24/7.
    </p>

    <div class="schedule-grid">
      <div
        v-for="day in 7"
        :key="day"
        class="schedule-day-card"
      >
        <div class="schedule-day-card__head">
          <span class="day-name">{{ getFullDayName(day - 1) }}</span>
          <label
            class="round-switch-wrap"
            :title="'Круглосуточно (24/7)'"
          >
            <span class="round-switch-label">24/7</span>
            <span class="round-switch">
              <input
                type="checkbox"
                :checked="hasRoundTheClock(day - 1)"
                :disabled="isLoading || readonly"
                @change="toggleRoundTheClock(day - 1, $event)"
              >
              <span class="switch-slider" />
            </span>
          </label>
        </div>
        <div class="schedule-day-card__body">
          <div
            v-if="hasActiveRoundTheClock(day - 1)"
            class="slot-badge slot-badge--round"
          >
            <span class="round-clock-text">круглосуточно</span>
          </div>
          <div
            v-for="slot in getActiveSlotsForDay(day - 1)"
            v-else
            :key="slot.id"
            class="slot-badge"
          >
            <span class="slot-time">
              {{ formatTime(slot.open_time) }} – {{ formatTime(slot.close_time) }}
            </span>
            <div class="slot-badges">
              <span
                v-if="slot.is_next_day"
                class="next-day-badge"
              >+1</span>
            </div>
            <div
              v-if="!readonly"
              class="slot-actions"
            >
              <button
                class="icon-btn"
                title="Редактировать"
                @click="editSlot(slot)"
              >
                <AppIcon
                  name="edit"
                  class="icon"
                />
              </button>
              <button
                class="icon-btn"
                title="Удалить"
                @click="deleteSlot(slot)"
              >
                <AppIcon
                  name="trashcan"
                  class="icon"
                />
              </button>
            </div>
          </div>
          <div
            v-if="!hasActiveRoundTheClock(day - 1) && getActiveSlotsForDay(day - 1).length === 0"
            class="schedule-day-card__empty"
          >
            нет окон
          </div>
        </div>
      </div>
    </div>

    <!-- Модальное окно -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="modalOpen"
          class="modal-overlay"
          @click.self="closeModal"
        >
          <div class="modal-content">
            <div class="modal-header">
              <h3 class="modal-title">
                {{ editingSlotId ? 'Редактировать временнОе окно' : 'Добавить временнОе окно' }}
              </h3>
              <button
                class="modal-close"
                @click="closeModal"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
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

            <div class="modal-body">
              <!-- Выбор дня с кастомным селектом -->
              <div class="field">
                <label class="field-label">День недели *</label>
                <div
                  ref="selectRef"
                  class="custom-select"
                  @click="toggleDayDropdown"
                >
                  <div class="select-trigger">
                    <span>{{ getFullDayName(modalDay) }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ open: dayDropdownOpen }"
                      width="9"
                      height="9"
                    />
                  </div>
                  <transition name="dropdown">
                    <div
                      v-if="dayDropdownOpen"
                      class="select-dropdown"
                    >
                      <div
                        v-for="(dayName, idx) in fullDayNames"
                        :key="idx"
                        class="select-option"
                        :class="{ selected: modalDay === idx }"
                        @click="selectDay(idx)"
                      >
                        {{ dayName }}
                      </div>
                    </div>
                  </transition>
                </div>
              </div>

              <!-- Время -->
              <div class="time-fields">
                <div class="field time-field">
                  <label class="field-label">Открытие *</label>
                  <input
                    v-model="modalOpenTime"
                    type="time"
                    class="modal-input"
                    @change="validateTimes"
                  >
                </div>
                <div class="field time-field">
                  <label class="field-label">Закрытие *</label>
                  <input
                    v-model="modalCloseTime"
                    type="time"
                    class="modal-input"
                    @change="validateTimes"
                  >
                </div>
              </div>

              <!-- Сообщение об ошибке пересечения -->
              <div
                v-if="timeConflictError"
                class="error-hint"
              >
                {{ timeConflictError }}
              </div>

              <!-- Подсказка о следующем дне -->
              <div
                v-if="modalNextDay && !timeConflictError"
                class="next-day-hint"
              >
                ⏰ Закрытие на следующий день
              </div>
            </div>

            <div class="modal-footer">
              <button
                class="modal-btn cancel"
                @click="closeModal"
              >
                Отмена
              </button>
              <button
                class="modal-btn confirm"
                :disabled="!modalOpenTime || !modalCloseTime || isLoading || !!timeConflictError"
                @click="saveSlot"
              >
                {{ editingSlotId ? 'Сохранить' : 'Добавить' }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модалка копирования расписания -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="copyModalOpen"
          class="modal-overlay"
          @click.self="closeCopyModal"
        >
          <div class="modal-content">
            <div class="modal-header">
              <h3 class="modal-title">
                Скопировать расписание
              </h3>
              <button
                class="modal-close"
                @click="closeCopyModal"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
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

            <div class="modal-body copy-modal-body">
              <!-- День-источник -->
              <div class="field">
                <label class="field-label">Скопировать с дня *</label>
                <div
                  ref="copySelectRef"
                  class="custom-select"
                  @click="toggleCopyDayDropdown"
                >
                  <div class="select-trigger">
                    <span>{{ getFullDayName(copySourceDay) }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ open: copyDayDropdownOpen }"
                      width="9"
                      height="9"
                    />
                  </div>
                  <transition name="dropdown">
                    <div
                      v-if="copyDayDropdownOpen"
                      class="select-dropdown"
                    >
                      <div
                        v-for="(dayName, idx) in fullDayNames"
                        :key="idx"
                        class="select-option"
                        :class="{ selected: copySourceDay === idx }"
                        @click.stop="selectCopySourceDay(idx)"
                      >
                        {{ dayName }}
                      </div>
                    </div>
                  </transition>
                </div>
              </div>

              <!-- Превью окон источника -->
              <div class="copy-preview">
                <span class="copy-preview__label">Что скопируется:</span>
                <div
                  v-if="copySourceIsRound"
                  class="copy-preview__value copy-preview__value--round"
                >
                  круглосуточно
                </div>
                <template v-else-if="copySourceSlots.length">
                  <div
                    v-for="slot in copySourceSlots"
                    :key="slot.id"
                    class="copy-preview__value"
                  >
                    {{ formatTime(slot.open_time) }} – {{ formatTime(slot.close_time) }}
                    <span
                      v-if="slot.is_next_day"
                      class="copy-preview__nextday"
                    >след. день</span>
                  </div>
                </template>
                <div
                  v-else
                  class="copy-preview__value copy-preview__value--empty"
                >
                  нерабочий день (окна будут очищены)
                </div>
              </div>

              <!-- Дни-цели -->
              <div class="field">
                <label class="field-label">На какие дни *</label>
                <div class="copy-targets">
                  <label
                    v-for="(dayName, idx) in fullDayNames"
                    :key="idx"
                    class="copy-target"
                    :class="{
                      'copy-target--source': idx === copySourceDay,
                      'copy-target--checked': copyTargetDays.includes(idx),
                    }"
                  >
                    <input
                      type="checkbox"
                      :data-testid="`copy-target-${idx}`"
                      :checked="copyTargetDays.includes(idx)"
                      :disabled="idx === copySourceDay"
                      @change="toggleCopyTargetDay(idx)"
                    >
                    <span class="copy-target__name">{{ dayName }}</span>
                    <span
                      v-if="idx === copySourceDay"
                      class="copy-target__badge"
                    >источник</span>
                  </label>
                </div>
              </div>

              <div class="copy-warning">
                Существующие окна выбранных дней будут заменены.
              </div>
            </div>

            <div class="modal-footer">
              <button
                class="modal-btn cancel"
                @click="closeCopyModal"
              >
                Отмена
              </button>
              <button
                class="modal-btn confirm"
                data-testid="copy-confirm-btn"
                :disabled="!copyTargetDays.length || isLoading"
                @click="performCopySchedule"
              >
                Скопировать
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <ConfirmationModal
      :show="!!deleteConfirmSlot"
      title="Удаление временного окна"
      message="Удалить это временное окно?"
      confirm-text="Удалить"
      cancel-text="Отмена"
      @confirm="performDeleteSlot"
      @cancel="deleteConfirmSlot = null"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import ConfirmationModal from './ConfirmationModal.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
  name: 'WorkScheduleTab',
  components: { AppIcon, ConfirmationModal },
  props: {
    // Базовый URL ресурса-владельца окон, например '/system-tables/12' или
    // '/unload-places/5'. Компонент достраивает '/time-slots[/{id}]'.
    resourceUrl: { type: String, required: true },
    timeSlots: { type: Array, required: true },
    readonly: { type: Boolean, default: false },
  },
  emits: ['update'],
  data() {
    return {
      isLoading: false,
      modalOpen: false,
      editingSlotId: null,
      modalDay: 0,
      modalOpenTime: '',
      modalCloseTime: '',
      dayDropdownOpen: false,
      timeConflictError: '',
      deleteConfirmSlot: null,
      copyModalOpen: false,
      copySourceDay: 0,
      copyTargetDays: [],
      copyDayDropdownOpen: false,
      fullDayNames: ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'],
    };
  },
  computed: {
    // Определяем, является ли текущее окно в модалке "на следующий день"
    modalNextDay() {
      if (!this.modalOpenTime || !this.modalCloseTime) return false;
      return this.modalCloseTime < this.modalOpenTime;
    },

    // Активные окна дня-источника для превью копирования (включая круглосуточное)
    copySourceSlots() {
      return this.timeSlots
        .filter(s => s.day_of_week === this.copySourceDay && s.is_active)
        .slice()
        .sort((a, b) => this.formatTime(a.open_time).localeCompare(this.formatTime(b.open_time)));
    },

    copySourceIsRound() {
      return this.hasActiveRoundTheClock(this.copySourceDay);
    },
  },
  watch: {
    // При изменении времени в модалке проверяем конфликты
    modalOpenTime() {
      this.checkTimeConflict();
    },
    modalCloseTime() {
      this.checkTimeConflict();
    },
    modalDay() {
      this.checkTimeConflict();
    },
  },
  mounted() {
    // Добавляем обработчик клика для закрытия дропдауна
    document.addEventListener('click', this.handleClickOutside);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
  },
  methods: {
    handleClickOutside(event) {
      if (this.dayDropdownOpen && this.$refs.selectRef && !this.$refs.selectRef.contains(event.target)) {
        this.dayDropdownOpen = false;
      }
      if (this.copyDayDropdownOpen && this.$refs.copySelectRef && !this.$refs.copySelectRef.contains(event.target)) {
        this.copyDayDropdownOpen = false;
      }
    },

    getFullDayName(index) {
      return this.fullDayNames[index];
    },

    // Проверяем, есть ли активное круглосуточное окно для дня
    hasActiveRoundTheClock(day) {
      return this.timeSlots.some(
        s => s.day_of_week === day &&
             s.is_active &&
             s.open_time.slice(0,5) === '00:00' &&
             s.close_time.slice(0,5) === '23:59' &&
             !s.is_next_day
      );
    },

    // Проверяем, есть ли вообще круглосуточное окно (даже неактивное)
    hasRoundTheClock(day) {
      return this.timeSlots.some(
        s => s.day_of_week === day &&
             s.open_time.slice(0,5) === '00:00' &&
             s.close_time.slice(0,5) === '23:59' &&
             !s.is_next_day
      );
    },

    // Получить все активные окна для дня (кроме круглосуточного)
    getActiveSlotsForDay(day) {
      return this.timeSlots.filter(
        s => s.day_of_week === day &&
             s.is_active &&
             !(s.open_time.slice(0,5) === '00:00' &&
               s.close_time.slice(0,5) === '23:59' &&
               !s.is_next_day)
      );
    },

    // Получить все окна для дня (включая неактивные)
    getAllSlotsForDay(day) {
      return this.timeSlots.filter(s => s.day_of_week === day);
    },

    formatTime(t) {
      return t.slice(0, 5);
    },

    // Проверка пересечения временных окон
    checkTimeConflict() {
      if (!this.modalOpenTime || !this.modalCloseTime) {
        this.timeConflictError = '';
        return;
      }

      const open = this.modalOpenTime;
      const close = this.modalCloseTime;
      const isNextDay = close < open;

      // Получаем все активные окна для выбранного дня, кроме текущего редактируемого
      const daySlots = this.getActiveSlotsForDay(this.modalDay).filter(
        slot => slot.id !== this.editingSlotId
      );

      // Проверяем пересечение с каждым существующим окном
      for (const slot of daySlots) {
        const slotOpen = this.formatTime(slot.open_time);
        const slotClose = this.formatTime(slot.close_time);
        const slotIsNextDay = slot.is_next_day;

        if (this.isTimeOverlapping(open, close, isNextDay, slotOpen, slotClose, slotIsNextDay)) {
          this.timeConflictError = `Пересекается с окном ${slotOpen}–${slotClose}${slotIsNextDay ? ' (след. день)' : ''}`;
          return;
        }
      }

      this.timeConflictError = '';
    },

    // Проверка пересечения двух временных интервалов
    isTimeOverlapping(open1, close1, nextDay1, open2, close2, nextDay2) {
      // Преобразуем время в минуты для удобства
      const toMinutes = (time) => {
        const [h, m] = time.split(':').map(Number);
        return h * 60 + m;
      };

      let start1 = toMinutes(open1);
      let end1 = toMinutes(close1);
      let start2 = toMinutes(open2);
      let end2 = toMinutes(close2);

      // Если интервал переходит на следующий день
      if (nextDay1) {
        end1 += 24 * 60;
      }
      if (nextDay2) {
        end2 += 24 * 60;
      }

      // Проверяем пересечение
      return (start1 < end2 && end1 > start2);
    },

    // Переключение круглосуточного режима
    async toggleRoundTheClock(day, event) {
      const checked = event.target.checked;
      this.isLoading = true;

      try {
        if (checked) {
          // Включаем круглосуточно
          const existingRoundSlot = this.timeSlots.find(
            s => s.day_of_week === day &&
                 s.open_time.slice(0,5) === '00:00' &&
                 s.close_time.slice(0,5) === '23:59' &&
                 !s.is_next_day
          );

          if (existingRoundSlot) {
            // Если окно уже существует, просто активируем его
            await this.updateSlotActivity(existingRoundSlot.id, true);

            // Деактивируем все остальные окна для этого дня
            const otherSlots = this.getAllSlotsForDay(day).filter(
              s => s.id !== existingRoundSlot.id
            );
            for (const slot of otherSlots) {
              await this.updateSlotActivity(slot.id, false);
            }
          } else {
            // Создаем новое круглосуточное окно
            await this.createSlot(day, '00:00', '23:59', false);

            // Деактивируем все существующие окна для этого дня
            const existingSlots = this.getAllSlotsForDay(day);
            for (const slot of existingSlots) {
              await this.updateSlotActivity(slot.id, false);
            }
          }
        } else {
          // Выключаем круглосуточно - удаляем круглосуточное окно
          const roundSlot = this.timeSlots.find(
            s => s.day_of_week === day &&
                 s.open_time.slice(0,5) === '00:00' &&
                 s.close_time.slice(0,5) === '23:59' &&
                 !s.is_next_day
          );

          if (roundSlot) {
            await this.deleteTimeSlot(roundSlot.id);
          }

          // Активируем все остальные окна для этого дня
          const otherSlots = this.getAllSlotsForDay(day).filter(
            s => !(s.open_time.slice(0,5) === '00:00' &&
                   s.close_time.slice(0,5) === '23:59' &&
                   !s.is_next_day)
          );

          for (const slot of otherSlots) {
            await this.updateSlotActivity(slot.id, true);
          }
        }
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось изменить ', bold: 'режим работы', type: 'error' });
      } finally {
        // Перечитываем состояние всегда: при частичном сбое день остаётся в
        // промежуточном виде, а чекбокс 24/7 залипает в кликнутом состоянии -
        // refetch родителя синхронизирует UI с реальным состоянием сервера.
        this.$emit('update');
        this.isLoading = false;
      }
    },

    async updateSlotActivity(slotId, isActive) {
      const response = await apiRequest(`${this.resourceUrl}/time-slots/${slotId}`,
        {
          method: 'PUT',
          body: JSON.stringify({ is_active: isActive }),
        }
      );

      if (!response.ok) {
        throw new Error('Failed to update slot activity');
      }
    },

    openAddModal() {
      this.editingSlotId = null;
      this.modalDay = 0;
      this.modalOpenTime = '09:00';
      this.modalCloseTime = '18:00';
      this.timeConflictError = '';
      this.modalOpen = true;
    },

    editSlot(slot) {
      this.editingSlotId = slot.id;
      this.modalDay = slot.day_of_week;
      this.modalOpenTime = this.formatTime(slot.open_time);
      this.modalCloseTime = this.formatTime(slot.close_time);
      this.timeConflictError = '';
      this.modalOpen = true;
    },

    closeModal() {
      this.modalOpen = false;
      this.dayDropdownOpen = false;
      this.editingSlotId = null;
      this.timeConflictError = '';
    },

    toggleDayDropdown() {
      this.dayDropdownOpen = !this.dayDropdownOpen;
    },

    selectDay(idx) {
      this.modalDay = idx;
      this.dayDropdownOpen = false;
      this.checkTimeConflict();
    },

    validateTimes() {
      this.checkTimeConflict();
    },

    async saveSlot() {
      if (!this.modalOpenTime || !this.modalCloseTime || this.timeConflictError) return;

      const isNextDay = this.modalCloseTime < this.modalOpenTime;
      this.isLoading = true;

      try {
        // Проверяем, не пытаемся ли добавить окно, когда уже есть круглосуточное
        if (this.hasActiveRoundTheClock(this.modalDay)) {
          throw new Error('Нельзя добавить окно, когда день работает круглосуточно');
        }

        const url = this.editingSlotId
          ? `${this.resourceUrl}/time-slots/${this.editingSlotId}`
          : `${this.resourceUrl}/time-slots`;

        const method = this.editingSlotId ? 'PUT' : 'POST';

        const response = await apiRequest(url, {
          method,
          body: JSON.stringify({
            day_of_week: this.modalDay,
            open_time: this.modalOpenTime,
            close_time: this.modalCloseTime,
            is_next_day: isNextDay,
            ...(method === 'POST' ? { is_active: true } : {}),
          }),
        });

        if (!response.ok) {
          const error = await response.text();
          throw new Error(error);
        }

        this.closeModal();
        this.$emit('update');
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить окно: ', bold: e.message || 'ошибка', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },

    deleteSlot(slot) {
      this.deleteConfirmSlot = slot;
    },

    async performDeleteSlot() {
      const slot = this.deleteConfirmSlot;
      this.deleteConfirmSlot = null;
      if (!slot) return;

      this.isLoading = true;
      try {
        await this.deleteTimeSlot(slot.id);
        this.$emit('update');
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить ', bold: 'временное окно', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },

    async deleteTimeSlot(slotId) {
      const response = await apiRequest(`${this.resourceUrl}/time-slots/${slotId}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Ошибка при удалении');
      }
    },

    async createSlot(day, open, close, isNextDay) {
      const response = await apiRequest(`${this.resourceUrl}/time-slots`, {
        method: 'POST',
        body: JSON.stringify({
          day_of_week: day,
          open_time: open,
          close_time: close,
          is_next_day: isNextDay,
          is_active: true,
        }),
      });

      if (!response.ok) {
        throw new Error('Ошибка при создании');
      }
    },

    openCopyModal() {
      this.copySourceDay = 0;
      this.copyTargetDays = [];
      this.copyDayDropdownOpen = false;
      this.copyModalOpen = true;
    },

    closeCopyModal() {
      this.copyModalOpen = false;
      this.copyDayDropdownOpen = false;
    },

    toggleCopyDayDropdown() {
      this.copyDayDropdownOpen = !this.copyDayDropdownOpen;
    },

    selectCopySourceDay(idx) {
      this.copySourceDay = idx;
      this.copyDayDropdownOpen = false;
      // Источник не может быть целью — снимаем его из выбранных дней
      this.copyTargetDays = this.copyTargetDays.filter(d => d !== idx);
    },

    toggleCopyTargetDay(idx) {
      if (idx === this.copySourceDay) return;
      const pos = this.copyTargetDays.indexOf(idx);
      if (pos === -1) this.copyTargetDays.push(idx);
      else this.copyTargetDays.splice(pos, 1);
    },

    async performCopySchedule() {
      if (!this.copyTargetDays.length || this.isLoading) return;

      const targets = [...this.copyTargetDays];
      // Снимок активных окон источника ДО мутаций (open/close в формате ЧЧ:ММ)
      const sourceSlots = this.timeSlots
        .filter(s => s.day_of_week === this.copySourceDay && s.is_active)
        .map(s => ({
          open: this.formatTime(s.open_time),
          close: this.formatTime(s.close_time),
          isNextDay: s.is_next_day,
        }));

      this.isLoading = true;
      // Отслеживаем, была ли реальная мутация на сервере: даже при частичном
      // сбое UI обязан перечитать состояние (иначе покажет старое расписание,
      // рассинхрон с сервером, а повторный клик ударит по устаревшим id).
      let mutated = false;
      try {
        for (const target of targets) {
          // Снимок старых окон дня-цели ДО создания копий.
          const existing = this.getAllSlotsForDay(target);
          // Полная замена по схеме create-then-delete: сначала создаём копии
          // активных окон источника, затем удаляем старые окна цели. Бэк не
          // валидирует пересечения окон (timeslot_store), поэтому временное
          // сосуществование старых и новых безвредно; при сбое между шагами
          // худший случай - дубли окон (вход остаётся доступным), а не пустой
          // день (вход закрыт), чего атомарности с фронта не гарантировать.
          for (const s of sourceSlots) {
            await this.createSlot(target, s.open, s.close, s.isNextDay);
            mutated = true;
          }
          for (const slot of existing) {
            await this.deleteTimeSlot(slot.id);
            mutated = true;
          }
        }

        this.closeCopyModal();
        useDeletionsStore().notify({
          prefix: 'Расписание скопировано на ',
          bold: targets.map(d => this.getFullDayName(d)).join(', '),
          type: 'success',
        });
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось скопировать ', bold: 'расписание', type: 'error' });
      } finally {
        if (mutated) this.$emit('update');
        this.isLoading = false;
      }
    },
  },
};
</script>

<style scoped>
.schedule-tab {
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 15px;
  font-size: 13px;
  line-height: 1.5;
}

.add-btn {
  padding: 6px 14px;
  background: var(--accent);
  color: var(--accent-contrast);
  border: none;
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.2s;
}
.add-btn:hover:not(:disabled) {
  background: var(--accent-hover);
}
.add-btn:disabled {
  background: var(--border);
  cursor: not-allowed;
}

.schedule-tab__header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.copy-btn {
  padding: 6px 14px;
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.2s, color 0.2s;
}
.copy-btn:hover:not(:disabled) {
  background: var(--accent-tint);
}
.copy-btn:disabled {
  border-color: var(--border);
  color: var(--text-muted);
  cursor: not-allowed;
}

/* Превью окон источника в модалке копирования */
.copy-preview {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 15px;
  margin-bottom: 18px;
}
.copy-preview__label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
}
.copy-preview__value {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}
.copy-preview__value--round {
  color: var(--accent-text);
}
.copy-preview__value--empty {
  color: var(--text-muted);
  font-style: italic;
  font-weight: 400;
}
.copy-preview__nextday {
  margin-left: 6px;
  font-size: 10px;
  font-weight: 500;
  color: var(--text-muted);
}

/* Дни-цели в модалке копирования */
.copy-targets {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 6px;
}
.copy-target {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 12px;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
}
.copy-target:hover {
  border-color: var(--accent);
}
.copy-target--checked {
  border-color: var(--accent);
  background: var(--accent-tint);
}
.copy-target--source {
  cursor: not-allowed;
  background: var(--surface-2);
  border-color: var(--border);
}
.copy-target input {
  accent-color: var(--accent-text);
  cursor: inherit;
}
.copy-target__name {
  font-size: 13px;
  color: var(--text);
}
.copy-target__badge {
  margin-left: auto;
  font-size: 10px;
  color: var(--text-muted);
}

.copy-warning {
  font-size: 12px;
  color: var(--warning-text);
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  padding: 8px 12px;
  border-radius: 15px;
  margin-top: 4px;
}

.schedule-tab__header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.schedule-tab__header-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.schedule-tab__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.schedule-grid {
  display: grid;
  /* Колонка не уже 190px (иначе день недели + тумблер 24/7 не помещаются);
     при нехватке места колонок становится меньше и они растягиваются (1fr).
     calc(25% - 9px) сверху ограничивает до 4 столбцов на широком экране. */
  grid-template-columns: repeat(auto-fill, minmax(max(190px, calc(25% - 9px)), 1fr));
  gap: 12px;
}

.schedule-day-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.schedule-day-card__head {
  background: var(--accent-tint);
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  /* Если день недели + тумблер не влезают в строку - тумблер переносится вниз,
     а не вылезает за карточку. */
  flex-wrap: wrap;
}

.schedule-day-card__body {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.schedule-day-card__empty {
  text-align: center;
  color: var(--text-muted);
  font-size: 11px;
  font-style: italic;
  padding: 6px 0;
}

.day-name {
  font-weight: 600;
  color: var(--text);
  font-size: 13px;
}

.round-switch-wrap {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.round-switch-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  letter-spacing: 0.4px;
}

.round-switch {
  position: relative;
  display: inline-block;
  width: 34px;
  height: 18px;
  cursor: pointer;
}
.round-switch input {
  opacity: 0;
  width: 0;
  height: 0;
  position: absolute;
}
.switch-slider {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--border);
  border-radius: 22px;
  transition: 0.2s;
}
.switch-slider:before {
  position: absolute;
  content: '';
  height: 14px;
  width: 14px;
  left: 2px;
  bottom: 2px;
  background-color: var(--surface);
  border-radius: 50%;
  transition: 0.2s;
}
input:checked + .switch-slider {
  background-color: var(--accent);
}
input:checked + .switch-slider:before {
  transform: translateX(16px);
}

.slot-badge {
  position: relative;
  padding: 6px 8px;
  background: var(--surface-2);
  border-radius: 12px;
  border: 1px solid var(--border);
  font-size: 11px;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  flex-wrap: nowrap;
}

.slot-badge--round {
  background: var(--accent-tint);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.round-clock-text {
  font-weight: 500;
  color: var(--accent-text);
  font-size: 11px;
}

.slot-time {
  font-size: 12px;
  color: var(--text);
  font-weight: 500;
}

.slot-badges {
  display: flex;
  gap: 4px;
}

.next-day-badge {
  background: var(--accent);
  color: var(--accent-contrast);
  padding: 1px 6px;
  border-radius: 10px;
  font-size: 9px;
  font-weight: 500;
}

.slot-actions {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.icon-btn {
  background: none;
  border: none;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}
.icon-btn:hover {
  background-color: var(--border);
}
.icon {
  /* Значок мельче 16px: общая обводка 1.7 садится в волосок, здесь плотнее. */
  stroke-width: 2.2;
  color: var(--text);
  width: 12px;
  height: 12px;
  opacity: 0.6;
}

/* Модальное окно */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  animation: overlayAppear 0.3s ease-out;
}
@keyframes overlayAppear {
  from { background: rgba(0,0,0,0); backdrop-filter: blur(0); }
  to { background: rgba(0,0,0,0.5); backdrop-filter: blur(0.1px); }
}

.modal-content {
  background: var(--surface);
  border-radius: 20px;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px var(--shadow-drop);
  animation: modalAppear 0.3s ease-out;
  margin-top: -50px; /* Поднимаем выше на 100px (50px вверх от центра) */
}

@keyframes modalAppear {
  from { opacity: 0; transform: scale(0.8) translateY(-20px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 22px;
  border-bottom: 1px solid var(--border);
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
  padding: 6px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}
.modal-close:hover {
  background-color: var(--surface-2);
}

.modal-body {
  padding: 22px;
  height: 250px;
  overflow-y: auto;
}

/* Модалка копирования расписания несёт больше контента (селект + превью +
   сетка из 7 дней + предупреждение) - тело растёт по контенту, скролл
   включается только когда не влезает в экран (маленькая высота вьюпорта). */
.copy-modal-body {
  height: auto;
  max-height: calc(var(--app-vh, 1vh) * 65);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 22px 20px;
  border-top: 1px solid var(--border);
}

/* Поля формы */
.field {
  margin-bottom: 18px;
}
.field-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 5px;
}

.time-fields {
  display: flex;
  gap: 15px;
}
.time-field {
  flex: 1;
}

.modal-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
  transition: border-color 0.2s;
  background: var(--surface);
}
.modal-input:focus {
  border-color: var(--accent);
  outline: none;
}

/* Кастомный селект */
.custom-select {
  position: relative;
  cursor: pointer;
}
.select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--surface);
  transition: border-color 0.2s;
}
.select-trigger:hover {
  border-color: var(--accent);
}
.select-arrow {
  transition: transform 0.2s ease;
}
.select-arrow.open {
  transform: rotate(90deg);
}
.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 4px 15px var(--shadow-drop);
  z-index: 10;
  max-height: 150px;
  overflow-y: auto;
}
.select-option {
  padding: 10px 14px;
  cursor: pointer;
  border-bottom: 1px solid var(--border);
  transition: background 0.2s;
  font-size: 13px;
}
.select-option:last-child {
  border-bottom: none;
}
.select-option:hover {
  background: var(--accent-tint);
  color: var(--accent-text);
}
.select-option.selected {
  background: var(--accent-tint);
  color: var(--accent-text);
  font-weight: 500;
}

/* Сообщение об ошибке */
.error-hint {
  font-size: 12px;
  color: var(--danger-text);
  background: var(--danger-bg);
  padding: 8px 12px;
  border-radius: 20px;
  margin: 8px 0;
  display: inline-block;
  width: 100%;
  text-align: center;
}

/* Подсказка о следующем дне */
.next-day-hint {
  font-size: 12px;
  color: var(--warning-text);
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  padding: 8px 12px;
  border-radius: 20px;
  margin: 8px 0;
  display: inline-block;
  width: 100%;
  text-align: center;
}

.modal-btn {
  padding: 10px 22px;
  border: none;
  border-radius: 30px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
  min-width: 90px;
}
.modal-btn.cancel {
  background: var(--surface-2);
  color: var(--text-muted);
}
.modal-btn.cancel:hover {
  background: var(--row-hover);
}
.modal-btn.confirm {
  background: var(--accent);
  color: var(--accent-contrast);
}
.modal-btn.confirm:hover:not(:disabled) {
  background: var(--accent-hover);
}
.modal-btn.confirm:disabled {
  background: var(--border);
  cursor: not-allowed;
}

/* Анимации */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.3s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  transform: scale(0.8) translateY(-20px);
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}
.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

/* Скроллбары */
.days-list::-webkit-scrollbar,
.select-dropdown::-webkit-scrollbar {
  width: 4px;
}
.days-list::-webkit-scrollbar-track,
.select-dropdown::-webkit-scrollbar-track {
  background: var(--surface-2);
  border-radius: 4px;
}
.days-list::-webkit-scrollbar-thumb,
.select-dropdown::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 4px;
}
.days-list::-webkit-scrollbar-thumb:hover,
.select-dropdown::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

@media (max-width: 768px) {
  .day-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .time-fields {
    flex-direction: column;
    gap: 10px;
  }
}
</style>
