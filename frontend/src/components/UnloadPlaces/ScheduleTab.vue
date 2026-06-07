<template>
  <div class="schedule-tab">
    <div class="schedule-tab__header-top">
      <h4 class="schedule-tab__header-title">
        Режим работы
      </h4>
      <button
        v-if="!readonly"
        class="add-btn"
        :disabled="isLoading"
        @click="openAddModal"
      >
        + Добавить окно
      </button>
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
                <img
                  src="@/assets/icons/edit.png"
                  class="icon"
                >
              </button>
              <button
                class="icon-btn"
                title="Удалить"
                @click="deleteSlot(slot)"
              >
                <img
                  src="@/assets/icons/trashcan.png"
                  class="icon"
                >
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
                    <img
                      src="@/assets/icons/arrow.png"
                      class="select-arrow"
                      :class="{ open: dayDropdownOpen }"
                      width="9"
                      height="9"
                    >
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
                ⚠️ {{ timeConflictError }}
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
export default {
  name: 'ScheduleTab',
  props: {
    placeId: { type: Number, required: true },
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
      fullDayNames: ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'],
    };
  },
  computed: {
    // Определяем, является ли текущее окно в модалке "на следующий день"
    modalNextDay() {
      if (!this.modalOpenTime || !this.modalCloseTime) return false;
      return this.modalCloseTime < this.modalOpenTime;
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

        this.$emit('update');
      } catch (e) {
        console.error(e);
        alert('Ошибка при изменении режима');
      } finally {
        this.isLoading = false;
      }
    },

    async updateSlotActivity(slotId, isActive) {
      const response = await apiRequest(`/unload-places/${this.placeId}/time-slots/${slotId}`,
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
          ? `/unload-places/${this.placeId}/time-slots/${this.editingSlotId}`
          : `/unload-places/${this.placeId}/time-slots`;

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
        alert('Ошибка при сохранении: ' + e.message);
      } finally {
        this.isLoading = false;
      }
    },

    async deleteSlot(slot) {
      if (!confirm('Удалить это временное окно?')) return;
      
      this.isLoading = true;
      try {
        await this.deleteTimeSlot(slot.id);
        this.$emit('update');
      } catch (e) {
        console.error(e);
        alert('Ошибка при удалении');
      } finally {
        this.isLoading = false;
      }
    },

    async deleteTimeSlot(slotId) {
      const response = await apiRequest(`/unload-places/${this.placeId}/time-slots/${slotId}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Ошибка при удалении');
      }
    },

    async createSlot(day, open, close, isNextDay) {
      const response = await apiRequest(`/unload-places/${this.placeId}/time-slots`, {
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
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.2s;
}
.add-btn:hover:not(:disabled) {
  background: #3a45b2;
}
.add-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
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
  color: #000;
}

.schedule-tab__hint {
  margin: 0;
  font-size: 12px;
  color: #6b7280;
  line-height: 1.5;
}

.schedule-grid {
  display: grid;
  /* max 4 столбика: min(25%, ...) ограничивает до четверти ширины. */
  grid-template-columns: repeat(auto-fill, minmax(max(150px, calc(25% - 9px)), 1fr));
  gap: 12px;
}

.schedule-day-card {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 14px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.schedule-day-card__head {
  background: #f0f3ff;
  padding: 8px 10px;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.schedule-day-card__body {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.schedule-day-card__empty {
  text-align: center;
  color: #aaa;
  font-size: 11px;
  font-style: italic;
  padding: 6px 0;
}

.day-name {
  font-weight: 600;
  color: #333;
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
  color: #6b7280;
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
  background-color: #ccc;
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
  background-color: white;
  border-radius: 50%;
  transition: 0.2s;
}
input:checked + .switch-slider {
  background-color: #4F5BDF;
}
input:checked + .switch-slider:before {
  transform: translateX(16px);
}

.slot-badge {
  position: relative;
  padding: 6px 8px;
  background: #f8f9fa;
  border-radius: 12px;
  border: 1px solid #e6e6e6;
  font-size: 11px;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  flex-wrap: nowrap;
}

.slot-badge--round {
  background: #eef0ff;
  border-color: #c7caf5;
}

.round-clock-text {
  font-weight: 500;
  color: #4F5BDF;
  font-size: 11px;
}

.slot-time {
  font-size: 12px;
  color: #333;
  font-weight: 500;
}

.slot-badges {
  display: flex;
  gap: 4px;
}

.next-day-badge {
  background: #4F5BDF;
  color: #fff;
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
  background-color: #e6e6e6;
}
.icon {
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
  background: rgba(0,0,0,0.5);
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
  background: white;
  border-radius: 20px;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px rgba(0,0,0,0.3);
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
  border-bottom: 1px solid #f0f0f0;
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
  padding: 6px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}
.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-body {
  padding: 22px;
  height: 250px;
  overflow-y: auto;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 22px 20px;
  border-top: 1px solid #f0f0f0;
}

/* Поля формы */
.field {
  margin-bottom: 18px;
}
.field-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: #555;
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
  border: 1px solid #e0e0e0;
  border-radius: 15px;
  font-size: 14px;
  transition: border-color 0.2s;
  background: white;
}
.modal-input:focus {
  border-color: #4F5BDF;
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
  border: 1px solid #e0e0e0;
  border-radius: 15px;
  background: white;
  transition: border-color 0.2s;
}
.select-trigger:hover {
  border-color: #4F5BDF;
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
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 12px;
  box-shadow: 0 4px 15px rgba(0,0,0,0.1);
  z-index: 10;
  max-height: 150px;
  overflow-y: auto;
}
.select-option {
  padding: 10px 14px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  transition: background 0.2s;
  font-size: 13px;
}
.select-option:last-child {
  border-bottom: none;
}
.select-option:hover {
  background: #f5f7ff;
  color: #4F5BDF;
}
.select-option.selected {
  background: #f0f3ff;
  color: #4F5BDF;
  font-weight: 500;
}

/* Сообщение об ошибке */
.error-hint {
  font-size: 12px;
  color: #d32f2f;
  background: #ffebee;
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
  color: #f39c12;
  background: #fff8e7;
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
  background: #f5f5f5;
  color: #666;
}
.modal-btn.cancel:hover {
  background: #e9e9e9;
}
.modal-btn.confirm {
  background: #4F5BDF;
  color: white;
}
.modal-btn.confirm:hover:not(:disabled) {
  background: #3a45b2;
}
.modal-btn.confirm:disabled {
  background: #ccc;
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
  background: #f5f5f5;
  border-radius: 4px;
}
.days-list::-webkit-scrollbar-thumb,
.select-dropdown::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}
.days-list::-webkit-scrollbar-thumb:hover,
.select-dropdown::-webkit-scrollbar-thumb:hover {
  background: #aaa;
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