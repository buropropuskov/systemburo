<template>
  <div class="ww-editor">
    <div class="ww-editor__header">
      <h4 class="ww-editor__title">
        Предупреждения по времени
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
    <p class="ww-editor__hint">
      Показываются заявителю, когда срок заявки пересекается с окном. Например
      "с 12:00 до 13:00 только малогабарит". Пустой список - предупреждений нет.
    </p>

    <div
      v-if="windows.length === 0"
      class="ww-editor__empty"
    >
      Окон-предупреждений пока нет
    </div>

    <div
      v-else
      class="ww-list"
    >
      <div
        v-for="win in sortedWindows"
        :key="win.id"
        class="ww-card"
        :class="{ 'ww-card--inactive': !win.is_active }"
      >
        <div class="ww-card__cond">
          <span class="ww-badge ww-badge--day">{{ dayLabel(win.day_of_week) }}</span>
          <span class="ww-badge ww-badge--time">
            {{ timeLabel(win) }}
            <span
              v-if="win.is_next_day"
              class="ww-nextday"
            >+1</span>
          </span>
          <span
            v-if="!win.is_active"
            class="ww-badge ww-badge--off"
          >выключено</span>
        </div>
        <div class="ww-card__message">
          {{ win.message }}
        </div>
        <div
          v-if="!readonly"
          class="ww-card__actions"
        >
          <label
            class="ww-switch-wrap"
            :title="win.is_active ? 'Выключить окно' : 'Включить окно'"
          >
            <span class="ww-switch">
              <input
                type="checkbox"
                :checked="win.is_active"
                :disabled="isLoading"
                @change="toggleActive(win)"
              >
              <span class="switch-slider" />
            </span>
          </label>
          <button
            class="icon-btn"
            title="Редактировать"
            @click="editWindow(win)"
          >
            <AppIcon
              name="edit"
              class="icon"
            />
          </button>
          <button
            class="icon-btn"
            title="Удалить"
            @click="deleteWindow(win)"
          >
            <AppIcon
              name="trashcan"
              class="icon"
            />
          </button>
        </div>
      </div>
    </div>

    <!-- Модальное окно добавления/редактирования -->
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
                {{ editingId ? 'Редактировать предупреждение' : 'Добавить предупреждение' }}
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
              <!-- День недели -->
              <div class="field">
                <label class="field-label">Когда действует</label>
                <div
                  ref="selectRef"
                  class="custom-select"
                  @click="toggleDayDropdown"
                >
                  <div class="select-trigger">
                    <span>{{ dayOptionLabel(modalDay) }}</span>
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
                        class="select-option"
                        :class="{ selected: modalDay === null }"
                        @click="selectDay(null)"
                      >
                        Каждый день
                      </div>
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

              <!-- Весь день / интервал -->
              <label class="checkbox-row">
                <input
                  v-model="modalAllDay"
                  type="checkbox"
                >
                <span>Действует весь день</span>
              </label>

              <div
                v-if="!modalAllDay"
                class="time-fields"
              >
                <div class="field time-field">
                  <label class="field-label">С *</label>
                  <input
                    v-model="modalTimeFrom"
                    type="time"
                    class="modal-input"
                  >
                </div>
                <div class="field time-field">
                  <label class="field-label">По *</label>
                  <input
                    v-model="modalTimeTo"
                    type="time"
                    class="modal-input"
                  >
                </div>
              </div>

              <div
                v-if="modalNextDay"
                class="next-day-hint"
              >
                Окончание на следующий день
              </div>

              <!-- Текст -->
              <div class="field">
                <label class="field-label">Текст предупреждения *</label>
                <textarea
                  v-model="modalMessage"
                  class="modal-input modal-textarea"
                  rows="3"
                  maxlength="1000"
                  placeholder="Например: только малогабаритный транспорт"
                />
              </div>

              <!-- Активность -->
              <label class="checkbox-row">
                <input
                  v-model="modalActive"
                  type="checkbox"
                >
                <span>Активно (показывать заявителю)</span>
              </label>
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
                :disabled="!canSave || isLoading"
                @click="saveWindow"
              >
                {{ editingId ? 'Сохранить' : 'Добавить' }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <ConfirmationModal
      :show="!!deleteConfirm"
      title="Удаление предупреждения"
      message="Удалить это окно-предупреждение?"
      confirm-text="Удалить"
      cancel-text="Отмена"
      @confirm="performDelete"
      @cancel="deleteConfirm = null"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import ConfirmationModal from './ConfirmationModal.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
  name: 'WarningWindowsEditor',
  components: { AppIcon, ConfirmationModal },
  props: {
    // Базовый URL ресурса-владельца окон, например '/unload-places/5' или
    // '/system-tables/12'. Компонент достраивает '/warning-windows[/{id}]'.
    resourceUrl: { type: String, required: true },
    windows: { type: Array, required: true },
    readonly: { type: Boolean, default: false },
  },
  emits: ['update'],
  data() {
    return {
      isLoading: false,
      modalOpen: false,
      editingId: null,
      modalDay: null,
      modalAllDay: true,
      modalTimeFrom: '',
      modalTimeTo: '',
      modalMessage: '',
      modalActive: true,
      dayDropdownOpen: false,
      deleteConfirm: null,
      fullDayNames: ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота', 'Воскресенье'],
      shortDayNames: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
    };
  },
  computed: {
    // Сортировка для стабильного показа: сначала "каждый день", потом по дню недели,
    // внутри дня по времени начала; активные выше выключенных.
    sortedWindows() {
      return [...this.windows].sort((a, b) => {
        if (a.is_active !== b.is_active) return a.is_active ? -1 : 1;
        const da = a.day_of_week ?? -1;
        const db = b.day_of_week ?? -1;
        if (da !== db) return da - db;
        return (a.time_from || '').localeCompare(b.time_from || '');
      });
    },
    modalNextDay() {
      if (this.modalAllDay || !this.modalTimeFrom || !this.modalTimeTo) return false;
      return this.modalTimeTo < this.modalTimeFrom;
    },
    canSave() {
      if (!this.modalMessage.trim()) return false;
      if (!this.modalAllDay && (!this.modalTimeFrom || !this.modalTimeTo)) return false;
      return true;
    },
  },
  mounted() {
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

    // Читает сообщение ошибки из envelope (wrapJsonUnwrap кладёт текст бэка в
    // message). Парсинг обёрнут в try/catch: инфра (502/504, HTML-шлюз) может
    // вернуть не-JSON, тогда отдаём понятный дефолт, а не сырой SyntaxError.
    async errorMessage(response) {
      try {
        const data = await response.json();
        return data.message || 'ошибка сервера';
      } catch {
        return 'ошибка сервера';
      }
    },

    dayLabel(day) {
      return day === null || day === undefined ? 'Каждый день' : this.shortDayNames[day];
    },
    dayOptionLabel(day) {
      return day === null || day === undefined ? 'Каждый день' : this.fullDayNames[day];
    },
    timeLabel(win) {
      if (!win.time_from || !win.time_to) return 'весь день';
      return `${this.formatTime(win.time_from)}–${this.formatTime(win.time_to)}`;
    },
    formatTime(t) {
      return t ? t.slice(0, 5) : '';
    },

    toggleDayDropdown() {
      this.dayDropdownOpen = !this.dayDropdownOpen;
    },
    selectDay(idx) {
      this.modalDay = idx;
      this.dayDropdownOpen = false;
    },

    openAddModal() {
      this.editingId = null;
      this.modalDay = null;
      this.modalAllDay = true;
      this.modalTimeFrom = '';
      this.modalTimeTo = '';
      this.modalMessage = '';
      this.modalActive = true;
      this.modalOpen = true;
    },

    editWindow(win) {
      this.editingId = win.id;
      this.modalDay = win.day_of_week ?? null;
      this.modalAllDay = !win.time_from || !win.time_to;
      this.modalTimeFrom = this.formatTime(win.time_from);
      this.modalTimeTo = this.formatTime(win.time_to);
      this.modalMessage = win.message || '';
      this.modalActive = win.is_active;
      this.modalOpen = true;
    },

    closeModal() {
      this.modalOpen = false;
      this.dayDropdownOpen = false;
      this.editingId = null;
    },

    // Тело запроса окна. PUT/POST = full replace: шлём ВСЕ поля явно, включая
    // is_active и is_next_day, иначе бэк сбросит их в дефолт (см. #1183 S2a).
    buildPayload({ day, allDay, timeFrom, timeTo, message, active }) {
      const isNextDay = allDay ? false : timeTo < timeFrom;
      return {
        day_of_week: day,
        time_from: allDay ? null : timeFrom,
        time_to: allDay ? null : timeTo,
        is_next_day: isNextDay,
        message: message.trim(),
        is_active: active,
      };
    },

    async saveWindow() {
      if (!this.canSave) return;
      this.isLoading = true;
      try {
        const payload = this.buildPayload({
          day: this.modalDay,
          allDay: this.modalAllDay,
          timeFrom: this.modalTimeFrom,
          timeTo: this.modalTimeTo,
          message: this.modalMessage,
          active: this.modalActive,
        });
        const url = this.editingId
          ? `${this.resourceUrl}/warning-windows/${this.editingId}`
          : `${this.resourceUrl}/warning-windows`;
        const response = await apiRequest(url, {
          method: this.editingId ? 'PUT' : 'POST',
          body: JSON.stringify(payload),
        });
        if (!response.ok) {
          throw new Error(await this.errorMessage(response));
        }
        this.closeModal();
        this.$emit('update');
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить предупреждение: ', bold: e.message || 'ошибка', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },

    // Быстрое включение/выключение из списка. PUT full-replace всеми полями окна,
    // иначе частичное тело сбросит остальные поля в дефолт.
    async toggleActive(win) {
      this.isLoading = true;
      try {
        const payload = this.buildPayload({
          day: win.day_of_week ?? null,
          allDay: !win.time_from || !win.time_to,
          timeFrom: this.formatTime(win.time_from),
          timeTo: this.formatTime(win.time_to),
          message: win.message,
          active: !win.is_active,
        });
        const response = await apiRequest(`${this.resourceUrl}/warning-windows/${win.id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        if (!response.ok) {
          throw new Error(await this.errorMessage(response));
        }
        this.$emit('update');
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось изменить предупреждение: ', bold: e.message || 'ошибка', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },

    deleteWindow(win) {
      this.deleteConfirm = win;
    },

    async performDelete() {
      const win = this.deleteConfirm;
      this.deleteConfirm = null;
      if (!win) return;
      this.isLoading = true;
      try {
        const response = await apiRequest(`${this.resourceUrl}/warning-windows/${win.id}`, {
          method: 'DELETE',
        });
        if (!response.ok) {
          throw new Error(await this.errorMessage(response));
        }
        this.$emit('update');
      } catch (e) {
        console.error(e);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить предупреждение: ', bold: e.message || 'ошибка', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
  },
};
</script>

<style scoped>
.ww-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-size: 13px;
  line-height: 1.5;
}

.ww-editor__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.ww-editor__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.ww-editor__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.ww-editor__empty {
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
  font-style: italic;
  padding: 18px 0;
  border: 1px dashed var(--border);
  border-radius: 14px;
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

.ww-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ww-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  flex-wrap: wrap;
}
.ww-card--inactive {
  background: var(--surface-2);
  opacity: 0.75;
}

.ww-card__cond {
  display: flex;
  gap: 6px;
  align-items: center;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.ww-badge {
  font-size: 11px;
  font-weight: 500;
  padding: 3px 8px;
  border-radius: 10px;
  white-space: nowrap;
}
.ww-badge--day {
  background: var(--accent-tint);
  color: var(--accent-text);
}
.ww-badge--time {
  background: var(--surface-2);
  color: var(--text);
  border: 1px solid var(--border);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.ww-badge--off {
  background: var(--border);
  color: var(--text-muted);
}

.ww-nextday {
  background: var(--accent);
  color: var(--accent-contrast);
  padding: 0 5px;
  border-radius: 8px;
  font-size: 9px;
  font-weight: 600;
}

.ww-card__message {
  flex: 1;
  min-width: 140px;
  font-size: 12.5px;
  color: var(--text);
  word-break: break-word;
}

.ww-card__actions {
  display: flex;
  gap: 4px;
  align-items: center;
  flex-shrink: 0;
}

.ww-switch-wrap {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
  margin-right: 2px;
}
.ww-switch {
  position: relative;
  display: inline-block;
  width: 34px;
  height: 18px;
  cursor: pointer;
}
.ww-switch input {
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
.ww-switch input:checked + .switch-slider {
  background-color: var(--accent);
}
.ww-switch input:checked + .switch-slider:before {
  transform: translateX(16px);
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
  width: 13px;
  height: 13px;
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
  width: 440px;
  max-width: 90vw;
  box-shadow: 0 20px 60px var(--shadow-drop);
  animation: modalAppear 0.3s ease-out;
  margin-top: -50px;
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
  max-height: calc(var(--app-vh, 1vh) * 60);
  overflow-y: auto;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 22px 20px;
  border-top: 1px solid var(--border);
}

.field {
  margin-bottom: 16px;
}
.field-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 5px;
}

.checkbox-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  margin-bottom: 16px;
}
.checkbox-row input {
  width: 16px;
  height: 16px;
  accent-color: var(--accent-text);
  cursor: pointer;
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
  box-sizing: border-box;
}
.modal-input:focus {
  border-color: var(--accent);
  outline: none;
}
.modal-textarea {
  resize: vertical;
  font-family: inherit;
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
  max-height: 200px;
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

.next-day-hint {
  font-size: 12px;
  color: var(--warning-text);
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  padding: 8px 12px;
  border-radius: 15px;
  margin-bottom: 16px;
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

@media (max-width: 768px) {
  .time-fields {
    flex-direction: column;
    gap: 10px;
  }
}
</style>
